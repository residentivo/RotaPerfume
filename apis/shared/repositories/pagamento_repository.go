// Package repositories contém funções puras de acesso a dados (stateless).
// Pagamento queries - cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/vlog"
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
	VendedorID      int64  // > 0 restringe aos pagamentos de pedidos desse vendedor
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
	vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "declarando variável conds")
	var conds []string
	vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "declarando variável args")
	var args []any

	vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "verificando se f.StatusPagamento != \"\"")
	if f.StatusPagamento != "" {
		vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "adicionando cláusula de filtro/SQL [status_pagamento = ?] em conds")
		conds = append(conds, "status_pagamento = ?")
		vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.StatusPagamento)
	}
	vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "verificando se f.FormaPagamento != \"\"")
	if f.FormaPagamento != "" {
		vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "adicionando cláusula de filtro/SQL [forma_pagamento = ?] em conds")
		conds = append(conds, "forma_pagamento = ?")
		vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.FormaPagamento)
	}
	vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "verificando se f.PedidoID > 0")
	if f.PedidoID > 0 {
		vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "adicionando cláusula de filtro/SQL [pedido_id = ?] em conds")
		conds = append(conds, "pedido_id = ?")
		vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.PedidoID)
	}
	vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "verificando se f.VencimentoDe != \"\"")
	if f.VencimentoDe != "" {
		vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "adicionando cláusula de filtro/SQL [data_vencimento >= ?] em conds")
		conds = append(conds, "data_vencimento >= ?")
		vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.VencimentoDe)
	}
	vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "verificando se f.VencimentoAte != \"\"")
	if f.VencimentoAte != "" {
		vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "adicionando cláusula de filtro/SQL [data_vencimento <= ?] em conds")
		conds = append(conds, "data_vencimento <= ?")
		vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.VencimentoAte)
	}
	vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "verificando se f.VendedorID > 0")
	if f.VendedorID > 0 {
		vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "adicionando cláusula de filtro/SQL [pedido_id IN (SELECT pedido_id_origem FROM pedidos WHERE vendedor_id = ?)] em conds")
		conds = append(conds, "pedido_id IN (SELECT pedido_id_origem FROM pedidos WHERE vendedor_id = ?)")
		vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
		args = append(args, f.VendedorID)
	}

	vlog.Printf("pagamento_repository.go", "PagamentoFiltro.where", "verificando se len(conds) == 0")
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// List retorna pagamentos paginados conforme o filtro informado, mais o
// total para meta-dados de paginação.
func (r *PagamentoRepository) List(ctx context.Context, db *sql.DB, page, limit int, filtro PagamentoFiltro) ([]models.Pagamento, int, error) {
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "verificando se page < 1")
	if page < 1 {
		vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "atribuindo page = 1")
		page = 1
	}
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "verificando se limit < 1")
	if limit < 1 {
		vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "atribuindo limit = 20")
		limit = 20
	}
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "verificando se limit > 100")
	if limit > 100 {
		vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "atribuindo limit = 100")
		limit = 100
	}
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "definindo offset = (page - 1) * limit")
	offset := (page - 1) * limit

	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "definindo whereClause, args com resultado de chamada a filtro.where")
	whereClause, args := filtro.where()

	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "declarando variável total")
	var total int
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "montando texto da query SQL SELECT em pagamentos em countQ")
	countQ := "SELECT COUNT(*) FROM pagamentos" + whereClause
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query countQ, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count pagamentos: %w", err)
	}

	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "montando texto da query SQL SELECT em q")
	q := "SELECT " + pagamentoColunas + " FROM pagamentos" + whereClause + filtro.orderBy() + " LIMIT ? OFFSET ?"
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "adicionando 2 parâmetro(s) de placeholder em queryArgs (valores omitidos)")
	queryArgs := append(append([]any{}, args...), limit, offset)

	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, queryArgs...)
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "verificando se err != nil")
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list pagamentos: %w", err)
	}
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "declarando variável out")
	var out []models.Pagamento
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		p, err := scanPagamento(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *p)
	}
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "loop concluído; itens acumulados em out: %d", len(out))
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.List", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list pagamentos iteração: %w", err)
	}
	return out, total, nil
}

// GetByID busca um pagamento pela PK pagamento_id. Retorna ErrNotFound se
// não existir.
func (r *PagamentoRepository) GetByID(ctx context.Context, db *sql.DB, pagamentoID int64) (*models.Pagamento, error) {
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.GetByID", "montando texto da query SQL SELECT em q")
	q := "SELECT " + pagamentoColunas + " FROM pagamentos WHERE pagamento_id = ? LIMIT 1"
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.GetByID", "definindo row com resultado de execução SQL via db.QueryRowContext (query q, args omitidos)")
	row := db.QueryRowContext(ctx, q, pagamentoID)
	return scanPagamento(row)
}

// Create insere um novo pagamento e preenche p.PagamentoID com o id gerado.
func (r *PagamentoRepository) Create(ctx context.Context, db *sql.DB, p *models.Pagamento) error {
	const q = `
		INSERT INTO pagamentos (pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Create", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
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
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Create", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: create pagamento: %w", err)
	}
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Create", "definindo id, err com resultado de chamada a res.LastInsertId")
	id, err := res.LastInsertId()
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Create", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: create pagamento lastInsertId: %w", err)
	}
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Create", "atribuindo p.PagamentoID = id")
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
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Update", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
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
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Update", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update pagamento: %w", err)
	}
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Update", "definindo n, err com resultado de chamada a res.RowsAffected")
	n, err := res.RowsAffected()
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Update", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update pagamento rowsAffected: %w", err)
	}
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Update", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ExistsByPedidoID reporta se existe algum pagamento vinculado ao pedido
// informado. Usado por DeletePedido (handlers) para bloquear a exclusão de
// pedidos com pagamentos vinculados.
func (r *PagamentoRepository) ExistsByPedidoID(ctx context.Context, db *sql.DB, pedidoID int64) (bool, error) {
	const q = `SELECT 1 FROM pagamentos WHERE pedido_id = ? LIMIT 1`
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.ExistsByPedidoID", "declarando variável one")
	var one int
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.ExistsByPedidoID", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query q, args omitidos) com leitura do resultado")
	err := db.QueryRowContext(ctx, q, pedidoID).Scan(&one)
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.ExistsByPedidoID", "verificando se err != nil")
	if err != nil {
		vlog.Printf("pagamento_repository.go", "PagamentoRepository.ExistsByPedidoID", "verificando se err == sql.ErrNoRows")
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("repositories: exists pagamento por pedido_id: %w", err)
	}
	return true, nil
}

// Delete remove um pagamento pela PK pagamento_id (hard delete — não há
// coluna deleted_at nesta tabela). Retorna ErrNotFound se não existir.
func (r *PagamentoRepository) Delete(ctx context.Context, db *sql.DB, pagamentoID int64) error {
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Delete", "definindo res, err com resultado de execução SQL via db.ExecContext (query DELETE em pagamentos, args omitidos)")
	res, err := db.ExecContext(ctx, `DELETE FROM pagamentos WHERE pagamento_id = ?`, pagamentoID)
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Delete", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: delete pagamento: %w", err)
	}
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Delete", "definindo n, err com resultado de chamada a res.RowsAffected")
	n, err := res.RowsAffected()
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Delete", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: delete pagamento rowsAffected: %w", err)
	}
	vlog.Printf("pagamento_repository.go", "PagamentoRepository.Delete", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanPagamento(s rowScanner) (*models.Pagamento, error) {
	vlog.Printf("pagamento_repository.go", "scanPagamento", "declarando variável p")
	var p models.Pagamento
	vlog.Printf("pagamento_repository.go", "scanPagamento", "declarando variável dataPagamento")
	var dataPagamento sql.NullTime
	vlog.Printf("pagamento_repository.go", "scanPagamento", "definindo err com resultado de leitura das colunas via s.Scan e verificando se err != nil")
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
		vlog.Printf("pagamento_repository.go", "scanPagamento", "verificando se err == sql.ErrNoRows")
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan pagamento: %w", err)
	}
	vlog.Printf("pagamento_repository.go", "scanPagamento", "verificando se dataPagamento.Valid")
	if dataPagamento.Valid {
		vlog.Printf("pagamento_repository.go", "scanPagamento", "atribuindo p.DataPagamento = &dataPagamento.Time")
		p.DataPagamento = &dataPagamento.Time
	}
	return &p, nil
}
