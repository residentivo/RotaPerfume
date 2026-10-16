package catalogomysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqProdutos = "produtos.go"

// produtoColunas: nota_olfativa é NULLable e vira "" (COALESCE);
// data_lancamento é NULLable e vira nil (sql.NullTime).
const produtoColunas = `id, sku, descricao, categoria, marca, COALESCE(nota_olfativa, ''), preco_tabela, custo_unitario, unidade, data_lancamento, ativo, created_at, updated_at`

// Produtos é o repositório de produtos (porta catalogoapp.Produtos). Cada
// método executa a própria consulta: sem cache entre requests (R4).
type Produtos struct{ q db.Querier }

// NovoProdutos liga o repositório ao pool (ou a uma tx). q nil: panic.
func NovoProdutos(q db.Querier) *Produtos {
	if q == nil {
		panic("catalogomysql: NovoProdutos exige Querier")
	}
	return &Produtos{q: q}
}

// Listar pagina os produtos que casam com o filtro, mais o total.
func (r *Produtos) Listar(ctx context.Context, f catalogo.FiltroProduto, p kernel.Pagina) (kernel.Resultado[catalogo.Produto], error) {
	w, args := whereProduto(f)
	var total int
	if err := r.q.QueryRowContext(ctx, "SELECT COUNT(*) FROM produtos"+w, args...).Scan(&total); err != nil {
		return kernel.Resultado[catalogo.Produto]{}, fmt.Errorf("catalogomysql: contar produtos: %w", err)
	}
	q := "SELECT " + produtoColunas + " FROM produtos" + w + ordemProduto(f.Ordenacao) + " LIMIT ? OFFSET ?"
	itens, err := db.Consultar(ctx, r.q, "catalogomysql: listar produtos", q, db.Paginada(args, p), scanProduto)
	if err != nil {
		return kernel.Resultado[catalogo.Produto]{}, err
	}
	vlog.Printf(arqProdutos, "Produtos.Listar", "SELECT produtos linhas=%d total=%d", len(itens), total)
	return kernel.NovoResultado(itens, p, total), nil
}

// whereProduto traduz o filtro em WHERE parametrizado (C-03).
func whereProduto(f catalogo.FiltroProduto) (string, []any) {
	var conds []string
	var args []any
	if f.Categoria != "" {
		conds = append(conds, "categoria = ?")
		args = append(args, f.Categoria)
	}
	if f.Marca != "" {
		conds = append(conds, "marca = ?")
		args = append(args, f.Marca)
	}
	if f.Ativo != nil {
		conds = append(conds, "ativo = ?")
		args = append(args, *f.Ativo)
	}
	if f.Busca != "" {
		conds = append(conds, "(descricao LIKE ? OR sku LIKE ?)")
		like := "%" + f.Busca + "%"
		args = append(args, like, like)
	}
	return db.Where(conds), args
}

// PorID busca um produto; catalogo.ErrProdutoNaoEncontrado se não existir.
func (r *Produtos) PorID(ctx context.Context, id int64) (*catalogo.Produto, error) {
	q := "SELECT " + produtoColunas + " FROM produtos WHERE id = ? LIMIT 1"
	p, err := scanProduto(r.q.QueryRowContext(ctx, q, id))
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqProdutos, "Produtos.PorID", "SELECT produtos linhas=1")
	return p, nil
}

// ExisteSKU informa se há produto com o SKU exato.
func (r *Produtos) ExisteSKU(ctx context.Context, sku catalogo.SKU) (bool, error) {
	const q = `SELECT 1 FROM produtos WHERE sku = ? LIMIT 1`
	var um int
	err := r.q.QueryRowContext(ctx, q, sku.String()).Scan(&um)
	if errors.Is(err, sql.ErrNoRows) {
		vlog.Printf(arqProdutos, "Produtos.ExisteSKU", "SELECT produtos existe=false")
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("catalogomysql: existe sku: %w", err)
	}
	vlog.Printf(arqProdutos, "Produtos.ExisteSKU", "SELECT produtos existe=true")
	return true, nil
}

// Inserir grava o produto e preenche p.ID.
func (r *Produtos) Inserir(ctx context.Context, p *catalogo.Produto) error {
	const q = `
		INSERT INTO produtos (sku, descricao, categoria, marca, nota_olfativa, preco_tabela, custo_unitario, unidade, data_lancamento, ativo)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := r.q.ExecContext(ctx, q, p.SKU.String(), p.Descricao, p.Categoria, p.Marca,
		nullString(p.NotaOlfativa), p.PrecoTabela, p.CustoUnitario, p.Unidade, nullTime(p.DataLancamento), p.Ativo)
	if err != nil {
		return fmt.Errorf("catalogomysql: inserir produto: %w", db.SemValorDuplicado(err)) // NB-3: 1062 do SKU sem o valor
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("catalogomysql: inserir produto lastInsertId: %w", err)
	}
	p.ID = id
	vlog.Printf(arqProdutos, "Produtos.Inserir", "INSERT produtos id=%d", id)
	return nil
}

// Atualizar grava os campos editáveis (SKU e ativo não mudam por aqui).
func (r *Produtos) Atualizar(ctx context.Context, id int64, c catalogo.CamposProduto) error {
	const q = `
		UPDATE produtos
		SET descricao = ?, categoria = ?, marca = ?, nota_olfativa = ?, preco_tabela = ?, custo_unitario = ?, unidade = ?, data_lancamento = ?
		WHERE id = ?`
	return r.exec(ctx, "atualizar produto", "UPDATE produtos", q, c.Descricao, c.Categoria, c.Marca,
		nullString(c.NotaOlfativa), c.PrecoTabela, c.CustoUnitario, c.Unidade, nullTime(c.DataLancamento), id)
}

// DefinirAtivo ativa/inativa o produto (exclusão lógica).
func (r *Produtos) DefinirAtivo(ctx context.Context, id int64, ativo bool) error {
	const q = `UPDATE produtos SET ativo = ? WHERE id = ?`
	return r.exec(ctx, "definir ativo produto", "UPDATE produtos ativo", q, ativo, id)
}

// exec roda um UPDATE por id: nenhuma linha casada vira
// ErrProdutoNaoEncontrado (o DSN usa clientFoundRows=true, BUG-04).
func (r *Produtos) exec(ctx context.Context, op, rotulo, q string, args ...any) error {
	res, err := r.q.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("catalogomysql: %s: %w", op, db.SemValorDuplicado(err))
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("catalogomysql: %s rowsAffected: %w", op, err)
	}
	vlog.Printf(arqProdutos, "Produtos.exec", "%s afetadas=%d", rotulo, n)
	if n == 0 {
		return catalogo.ErrProdutoNaoEncontrado
	}
	return nil
}

// ContarTotal devolve o total de produtos. Leitura para métricas (sem
// chamador HTTP: o Dashboard migrado no ARQ-06 não conta produtos, como o
// legado; o ARQ-07 decide se fica).
func (r *Produtos) ContarTotal(ctx context.Context) (int, error) {
	var total int
	if err := r.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM produtos`).Scan(&total); err != nil {
		return 0, fmt.Errorf("catalogomysql: contar total produtos: %w", err)
	}
	return total, nil
}

// ContarPorAtivo devolve o total de produtos com o status informado.
func (r *Produtos) ContarPorAtivo(ctx context.Context, ativo bool) (int, error) {
	var total int
	if err := r.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM produtos WHERE ativo = ?`, ativo).Scan(&total); err != nil {
		return 0, fmt.Errorf("catalogomysql: contar produtos por ativo: %w", err)
	}
	return total, nil
}

// nullTime grava nil como NULL (data_lancamento opcional).
func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func scanProduto(s linha) (*catalogo.Produto, error) {
	var p catalogo.Produto
	var dataLancamento sql.NullTime
	err := s.Scan(&p.ID, &p.SKU, &p.Descricao, &p.Categoria, &p.Marca, &p.NotaOlfativa,
		&p.PrecoTabela, &p.CustoUnitario, &p.Unidade, &dataLancamento, &p.Ativo, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, catalogo.ErrProdutoNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("catalogomysql: ler produto: %w", err)
	}
	if dataLancamento.Valid {
		p.DataLancamento = &dataLancamento.Time
	}
	return &p, nil
}
