package catalogomysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqEstoques = "estoques.go"

// estoqueColunas inclui a descrição do produto (LEFT JOIN: um snapshot cujo
// SKU não exista mais em produtos não some da listagem).
const estoqueColunas = `e.id, e.data_snapshot, e.sku, COALESCE(p.descricao, '') AS produto_descricao, e.saldo, e.ruptura, e.created_at, e.updated_at`
const estoqueFrom = ` FROM estoque e LEFT JOIN produtos p ON p.sku = e.sku`

// estoqueRanked numera os snapshots de cada SKU do mais recente para o mais
// antigo (window function, MySQL 8.0+); rn = 1 é a última posição.
const estoqueRanked = `
		SELECT ` + estoqueColunas + `,
			ROW_NUMBER() OVER (PARTITION BY e.sku ORDER BY e.data_snapshot DESC, e.id DESC) AS rn
		` + estoqueFrom

// Estoques é o repositório de estoque (porta catalogoapp.Estoques). Cada
// método executa a própria consulta: sem cache entre requests (R4).
type Estoques struct{ q db.Querier }

// NovoEstoques liga o repositório ao pool (ou a uma tx). q nil: panic.
func NovoEstoques(q db.Querier) *Estoques {
	if q == nil {
		panic("catalogomysql: NovoEstoques exige Querier")
	}
	return &Estoques{q: q}
}

// Historico pagina a série temporal completa que casa com o filtro.
func (r *Estoques) Historico(ctx context.Context, f catalogo.FiltroEstoque, p kernel.Pagina) (kernel.Resultado[catalogo.RegistroEstoque], error) {
	w, args := whereEstoque(f)
	countQ := "SELECT COUNT(*)" + estoqueFrom + w
	q := "SELECT " + estoqueColunas + estoqueFrom + w + ordemEstoque(f.Ordenacao) + " LIMIT ? OFFSET ?"
	return r.listar(ctx, "listar estoque", "Estoques.Historico", countQ, q, args, p)
}

// UltimaPosicao pagina só o snapshot mais recente de cada SKU. Com data_ate,
// é a última posição até aquela data ("como estava naquele dia").
func (r *Estoques) UltimaPosicao(ctx context.Context, f catalogo.FiltroEstoque, p kernel.Pagina) (kernel.Resultado[catalogo.RegistroEstoque], error) {
	w, args := whereEstoque(f)
	countQ := "SELECT COUNT(*) FROM (" + estoqueRanked + w + ") ranked WHERE rn = 1"
	q := "SELECT id, data_snapshot, sku, produto_descricao, saldo, ruptura, created_at, updated_at FROM (" +
		estoqueRanked + w + ") ranked WHERE rn = 1" + ordemEstoqueRanked(f.Ordenacao) + " LIMIT ? OFFSET ?"
	return r.listar(ctx, "listar estoque última posição", "Estoques.UltimaPosicao", countQ, q, args, p)
}

func (r *Estoques) listar(ctx context.Context, op, funcao, countQ, q string, args []any, p kernel.Pagina) (kernel.Resultado[catalogo.RegistroEstoque], error) {
	var total int
	if err := r.q.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return kernel.Resultado[catalogo.RegistroEstoque]{}, fmt.Errorf("catalogomysql: contar (%s): %w", op, err)
	}
	itens, err := db.Consultar(ctx, r.q, "catalogomysql: "+op, q, db.Paginada(args, p), scanEstoque)
	if err != nil {
		return kernel.Resultado[catalogo.RegistroEstoque]{}, err
	}
	vlog.Printf(arqEstoques, funcao, "SELECT estoque linhas=%d total=%d", len(itens), total)
	return kernel.NovoResultado(itens, p, total), nil
}

// whereEstoque traduz o filtro em WHERE parametrizado, com colunas do alias
// "e" (datas como AAAA-MM-DD, sem fuso).
func whereEstoque(f catalogo.FiltroEstoque) (string, []any) {
	var conds []string
	var args []any
	if f.SKU != "" {
		conds = append(conds, "e.sku = ?")
		args = append(args, f.SKU)
	}
	if f.De != nil {
		conds = append(conds, "e.data_snapshot >= ?")
		args = append(args, f.De.String())
	}
	if f.Ate != nil {
		conds = append(conds, "e.data_snapshot <= ?")
		args = append(args, f.Ate.String())
	}
	if f.Ruptura != nil {
		conds = append(conds, "e.ruptura = ?")
		args = append(args, *f.Ruptura)
	}
	return db.Where(conds), args
}

// PorID busca um registro; catalogo.ErrEstoqueNaoEncontrado se não existir.
func (r *Estoques) PorID(ctx context.Context, id int64) (*catalogo.RegistroEstoque, error) {
	q := "SELECT " + estoqueColunas + estoqueFrom + " WHERE e.id = ? LIMIT 1"
	e, err := scanEstoque(r.q.QueryRowContext(ctx, q, id))
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqEstoques, "Estoques.PorID", "SELECT estoque linhas=1")
	return e, nil
}

// Inserir grava o registro e preenche e.ID.
func (r *Estoques) Inserir(ctx context.Context, e *catalogo.RegistroEstoque) error {
	const q = `
		INSERT INTO estoque (data_snapshot, sku, saldo, ruptura)
		VALUES (?, ?, ?, ?)`
	res, err := r.q.ExecContext(ctx, q, e.DataSnapshot.Format(kernel.LayoutData), e.SKU.String(), e.Saldo, e.Ruptura)
	if err != nil {
		return fmt.Errorf("catalogomysql: inserir estoque: %w", db.SemValorDuplicado(err)) // NB-3: 1062 de (data_snapshot, sku) sem o valor
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("catalogomysql: inserir estoque lastInsertId: %w", err)
	}
	e.ID = id
	vlog.Printf(arqEstoques, "Estoques.Inserir", "INSERT estoque id=%d", id)
	return nil
}

// AtualizarSaldo grava saldo e ruptura (SKU e data são a chave de negócio).
func (r *Estoques) AtualizarSaldo(ctx context.Context, id int64, saldo int, ruptura bool) error {
	const q = `
		UPDATE estoque
		SET saldo = ?, ruptura = ?
		WHERE id = ?`
	res, err := r.q.ExecContext(ctx, q, saldo, ruptura, id)
	if err != nil {
		return fmt.Errorf("catalogomysql: atualizar saldo estoque: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("catalogomysql: atualizar saldo estoque rowsAffected: %w", err)
	}
	vlog.Printf(arqEstoques, "Estoques.AtualizarSaldo", "UPDATE estoque afetadas=%d", n)
	if n == 0 {
		return catalogo.ErrEstoqueNaoEncontrado
	}
	return nil
}

func scanEstoque(s linha) (*catalogo.RegistroEstoque, error) {
	var e catalogo.RegistroEstoque
	err := s.Scan(&e.ID, &e.DataSnapshot, &e.SKU, &e.ProdutoDescricao, &e.Saldo, &e.Ruptura, &e.CreatedAt, &e.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, catalogo.ErrEstoqueNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("catalogomysql: ler estoque: %w", err)
	}
	return &e, nil
}
