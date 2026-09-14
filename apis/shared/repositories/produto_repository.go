// Package repositories contém funções puras de acesso a dados (stateless).
// Produto queries - cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/models"
)

// ProdutoRepository agrupa queries da tabela produtos.
type ProdutoRepository struct{}

// NewProdutoRepository cria um repositório stateless.
func NewProdutoRepository() *ProdutoRepository {
	return &ProdutoRepository{}
}

// produtoColunas usa COALESCE em nota_olfativa pois a coluna é NULLable no
// banco, mas o model.Produto.NotaOlfativa é string (não ponteiro) — NULL
// vira "". data_lancamento é NULLable e o model usa *time.Time, então não
// precisa de COALESCE (Scan lida com NULL via sql.NullTime).
const produtoColunas = `id, sku, descricao, categoria, marca, COALESCE(nota_olfativa, ''), preco_tabela, custo_unitario, unidade, data_lancamento, ativo, created_at, updated_at`

// ProdutoFiltro agrupa os filtros opcionais aceitos por List.
// Campos vazios/nil são ignorados (não filtram).
type ProdutoFiltro struct {
	Categoria string
	Marca     string
	Ativo     *bool
	Q         string // busca textual em descricao OU sku (LIKE)
}

// where monta a cláusula WHERE (sem a palavra "WHERE") e os args correspondentes.
// Retorna string vazia quando não há filtros.
func (f ProdutoFiltro) where() (string, []any) {
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
	if f.Q != "" {
		conds = append(conds, "(descricao LIKE ? OR sku LIKE ?)")
		like := "%" + f.Q + "%"
		args = append(args, like, like)
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// List retorna produtos paginados conforme o filtro informado, mais o total
// para meta-dados de paginação.
func (r *ProdutoRepository) List(ctx context.Context, db *sql.DB, page, limit int, filtro ProdutoFiltro) ([]models.Produto, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	whereClause, args := filtro.where()

	var total int
	countQ := "SELECT COUNT(*) FROM produtos" + whereClause
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count produtos: %w", err)
	}

	q := "SELECT " + produtoColunas + " FROM produtos" + whereClause + " ORDER BY id ASC LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), limit, offset)

	rows, err := db.QueryContext(ctx, q, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list produtos: %w", err)
	}
	defer rows.Close()

	var out []models.Produto
	for rows.Next() {
		p, err := scanProduto(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list produtos iteração: %w", err)
	}
	return out, total, nil
}

// GetByID busca um produto pelo ID. Retorna ErrNotFound se não existir.
func (r *ProdutoRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Produto, error) {
	q := "SELECT " + produtoColunas + " FROM produtos WHERE id = ? LIMIT 1"
	row := db.QueryRowContext(ctx, q, id)
	return scanProduto(row)
}

// SetAtivo ativa/inativa um produto (exclusão lógica). Retorna ErrNotFound
// se não existir.
func (r *ProdutoRepository) SetAtivo(ctx context.Context, db *sql.DB, id int64, ativo bool) error {
	const q = `UPDATE produtos SET ativo = ? WHERE id = ?`
	res, err := db.ExecContext(ctx, q, ativo, id)
	if err != nil {
		return fmt.Errorf("repositories: set ativo produto: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountTotal retorna o total de produtos cadastrados.
func (r *ProdutoRepository) CountTotal(ctx context.Context, db *sql.DB) (int, error) {
	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM produtos`).Scan(&total); err != nil {
		return 0, fmt.Errorf("repositories: count total produtos: %w", err)
	}
	return total, nil
}

// CountPorAtivo retorna o total de produtos com o status ativo informado.
func (r *ProdutoRepository) CountPorAtivo(ctx context.Context, db *sql.DB, ativo bool) (int, error) {
	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM produtos WHERE ativo = ?`, ativo).Scan(&total); err != nil {
		return 0, fmt.Errorf("repositories: count produtos por ativo: %w", err)
	}
	return total, nil
}

// Create insere um novo produto e preenche p.ID com o id gerado.
func (r *ProdutoRepository) Create(ctx context.Context, db *sql.DB, p *models.Produto) error {
	const q = `
		INSERT INTO produtos (sku, descricao, categoria, marca, nota_olfativa, preco_tabela, custo_unitario, unidade, data_lancamento, ativo)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := db.ExecContext(ctx, q,
		p.SKU,
		p.Descricao,
		p.Categoria,
		p.Marca,
		nullStringFrom(p.NotaOlfativa),
		p.PrecoTabela,
		p.CustoUnitario,
		p.Unidade,
		nullTimeFrom(p.DataLancamento),
		p.Ativo,
	)
	if err != nil {
		return fmt.Errorf("repositories: create produto: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repositories: create produto lastInsertId: %w", err)
	}
	p.ID = id
	return nil
}

// Update atualiza os campos editáveis de um produto (sku e ativo não são
// alterados por aqui). Retorna ErrNotFound se não existir.
func (r *ProdutoRepository) Update(ctx context.Context, db *sql.DB, id int64, p *models.Produto) error {
	const q = `
		UPDATE produtos
		SET descricao = ?, categoria = ?, marca = ?, nota_olfativa = ?, preco_tabela = ?, custo_unitario = ?, unidade = ?, data_lancamento = ?
		WHERE id = ?`
	res, err := db.ExecContext(ctx, q,
		p.Descricao,
		p.Categoria,
		p.Marca,
		nullStringFrom(p.NotaOlfativa),
		p.PrecoTabela,
		p.CustoUnitario,
		p.Unidade,
		nullTimeFrom(p.DataLancamento),
		id,
	)
	if err != nil {
		return fmt.Errorf("repositories: update produto: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: update produto rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// nullStringFrom converte uma string vazia em sql.NullString{Valid: false},
// para gravar NULL em colunas NULLable (ex: nota_olfativa).
func nullStringFrom(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// nullTimeFrom converte um *time.Time em sql.NullTime, para gravar NULL em
// colunas NULLable (ex: data_lancamento).
func nullTimeFrom(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

func scanProduto(s rowScanner) (*models.Produto, error) {
	var p models.Produto
	var dataLancamento sql.NullTime
	if err := s.Scan(
		&p.ID,
		&p.SKU,
		&p.Descricao,
		&p.Categoria,
		&p.Marca,
		&p.NotaOlfativa,
		&p.PrecoTabela,
		&p.CustoUnitario,
		&p.Unidade,
		&dataLancamento,
		&p.Ativo,
		&p.CreatedAt,
		&p.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan produto: %w", err)
	}
	if dataLancamento.Valid {
		p.DataLancamento = &dataLancamento.Time
	}
	return &p, nil
}
