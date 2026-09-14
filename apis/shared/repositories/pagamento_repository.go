// Package repositories contém funções puras de acesso a dados (stateless).
// Pagamento queries - cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/models"
)

// PagamentoRepository agrupa queries da tabela pagamentos.
type PagamentoRepository struct{}

// NewPagamentoRepository cria um repositório stateless.
func NewPagamentoRepository() *PagamentoRepository {
	return &PagamentoRepository{}
}

// pagamentoColunas usa a PK literal pagamento_id (não há coluna id
// desacoplada nesta tabela — ver sql/12_ddl_pagamentos.sql).
const pagamentoColunas = `pagamento_id, pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento, created_at, updated_at`

// PagamentoFiltro agrupa os filtros opcionais aceitos por List.
// Campos zero-value são ignorados (não filtram).
type PagamentoFiltro struct {
	StatusPagamento string
	FormaPagamento  string
	PedidoID        int64
	VencimentoDe    string // formato AAAA-MM-DD (inclusive)
	VencimentoAte   string // formato AAAA-MM-DD (inclusive)
	OrderBy         string // campo de ordenação (whitelist: ver pagamentoOrderWhitelist); default "pagamento_id"
	OrderDir        string // "asc" ou "desc" (case-insensitive); default "asc"
}

// pagamentoOrderWhitelist mapeia os campos de ordenação aceitos pela API
// para as colunas SQL reais da tabela pagamentos.
var pagamentoOrderWhitelist = map[string]string{
	"pagamento_id":     "pagamento_id",
	"pedido_id":        "pedido_id",
	"forma_pagamento":  "forma_pagamento",
	"parcelas":         "parcelas",
	"valor":            "valor",
	"taxa_pct":         "taxa_pct",
	"valor_liquido":    "valor_liquido",
	"data_vencimento":  "data_vencimento",
	"data_pagamento":   "data_pagamento",
	"status_pagamento": "status_pagamento",
	"created_at":       "created_at",
	"updated_at":       "updated_at",
}

// orderBy monta a cláusula ORDER BY a partir de OrderBy/OrderDir, com
// default "pagamento_id ASC" (comportamento atual).
func (f PagamentoFiltro) orderBy() string {
	return buildOrderByClause(pagamentoOrderWhitelist, f.OrderBy, f.OrderDir, "pagamento_id", "ASC")
}

// where monta a cláusula WHERE (sem a palavra "WHERE") e os args correspondentes.
// Retorna string vazia quando não há filtros.
func (f PagamentoFiltro) where() (string, []any) {
	var conds []string
	var args []any

	if f.StatusPagamento != "" {
		conds = append(conds, "status_pagamento = ?")
		args = append(args, f.StatusPagamento)
	}
	if f.FormaPagamento != "" {
		conds = append(conds, "forma_pagamento = ?")
		args = append(args, f.FormaPagamento)
	}
	if f.PedidoID > 0 {
		conds = append(conds, "pedido_id = ?")
		args = append(args, f.PedidoID)
	}
	if f.VencimentoDe != "" {
		conds = append(conds, "data_vencimento >= ?")
		args = append(args, f.VencimentoDe)
	}
	if f.VencimentoAte != "" {
		conds = append(conds, "data_vencimento <= ?")
		args = append(args, f.VencimentoAte)
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// List retorna pagamentos paginados conforme o filtro informado, mais o
// total para meta-dados de paginação.
func (r *PagamentoRepository) List(ctx context.Context, db *sql.DB, page, limit int, filtro PagamentoFiltro) ([]models.Pagamento, int, error) {
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
	countQ := "SELECT COUNT(*) FROM pagamentos" + whereClause
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count pagamentos: %w", err)
	}

	q := "SELECT " + pagamentoColunas + " FROM pagamentos" + whereClause + filtro.orderBy() + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), limit, offset)

	rows, err := db.QueryContext(ctx, q, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list pagamentos: %w", err)
	}
	defer rows.Close()

	var out []models.Pagamento
	for rows.Next() {
		p, err := scanPagamento(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list pagamentos iteração: %w", err)
	}
	return out, total, nil
}

// GetByID busca um pagamento pela PK pagamento_id. Retorna ErrNotFound se
// não existir.
func (r *PagamentoRepository) GetByID(ctx context.Context, db *sql.DB, pagamentoID int64) (*models.Pagamento, error) {
	q := "SELECT " + pagamentoColunas + " FROM pagamentos WHERE pagamento_id = ? LIMIT 1"
	row := db.QueryRowContext(ctx, q, pagamentoID)
	return scanPagamento(row)
}

// Create insere um novo pagamento e preenche p.PagamentoID com o id gerado.
func (r *PagamentoRepository) Create(ctx context.Context, db *sql.DB, p *models.Pagamento) error {
	const q = `
		INSERT INTO pagamentos (pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := db.ExecContext(ctx, q,
		p.PedidoID,
		p.FormaPagamento,
		p.Parcelas,
		p.Valor,
		p.TaxaPct,
		p.ValorLiquido,
		p.DataVencimento,
		nullTimeFrom(p.DataPagamento),
		p.StatusPagamento,
	)
	if err != nil {
		return fmt.Errorf("repositories: create pagamento: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repositories: create pagamento lastInsertId: %w", err)
	}
	p.PagamentoID = id
	return nil
}

// Update atualiza os campos editáveis de um pagamento (pagamento_id e
// pedido_id não são alterados por aqui — integridade do vínculo com o
// pedido de origem). Retorna ErrNotFound se não existir.
func (r *PagamentoRepository) Update(ctx context.Context, db *sql.DB, pagamentoID int64, p *models.Pagamento) error {
	const q = `
		UPDATE pagamentos
		SET forma_pagamento = ?, parcelas = ?, valor = ?, taxa_pct = ?, valor_liquido = ?, data_vencimento = ?, data_pagamento = ?, status_pagamento = ?
		WHERE pagamento_id = ?`
	res, err := db.ExecContext(ctx, q,
		p.FormaPagamento,
		p.Parcelas,
		p.Valor,
		p.TaxaPct,
		p.ValorLiquido,
		p.DataVencimento,
		nullTimeFrom(p.DataPagamento),
		p.StatusPagamento,
		pagamentoID,
	)
	if err != nil {
		return fmt.Errorf("repositories: update pagamento: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: update pagamento rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanPagamento(s rowScanner) (*models.Pagamento, error) {
	var p models.Pagamento
	var dataPagamento sql.NullTime
	if err := s.Scan(
		&p.PagamentoID,
		&p.PedidoID,
		&p.FormaPagamento,
		&p.Parcelas,
		&p.Valor,
		&p.TaxaPct,
		&p.ValorLiquido,
		&p.DataVencimento,
		&dataPagamento,
		&p.StatusPagamento,
		&p.CreatedAt,
		&p.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan pagamento: %w", err)
	}
	if dataPagamento.Valid {
		p.DataPagamento = &dataPagamento.Time
	}
	return &p, nil
}
