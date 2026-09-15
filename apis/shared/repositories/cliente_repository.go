// Package repositories contém funções puras de acesso a dados (stateless).
// Cliente queries - cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/models"
)

// ClienteRepository agrupa queries da tabela clientes.
type ClienteRepository struct{}

// NewClienteRepository cria um repositório stateless.
func NewClienteRepository() *ClienteRepository {
	return &ClienteRepository{}
}

// clienteColunas usa COALESCE em bairro pois a coluna é NULLable no banco,
// mas o model.Cliente.Bairro é string (não ponteiro) — NULL vira "".
const clienteColunas = `id, cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, COALESCE(bairro, ''), data_cadastro, ativo, created_at, updated_at`

// ClienteFiltro agrupa os filtros opcionais aceitos por List.
// Campos vazios/nil são ignorados (não filtram).
type ClienteFiltro struct {
	UF       string
	Segmento string
	Ativo    *bool
	Q        string // busca textual em razao_social OU cnpj (LIKE)
	OrderBy  string // campo de ordenação (whitelist: ver clienteOrderWhitelist); default "id"
	OrderDir string // "asc" ou "desc" (case-insensitive); default "asc"
}

// clienteOrderWhitelist mapeia os campos de ordenação aceitos pela API para
// as colunas SQL reais da tabela clientes.
var clienteOrderWhitelist = map[string]string{
	"id":            "id",
	"razao_social":  "razao_social",
	"cnpj":          "cnpj",
	"segmento":      "segmento",
	"cidade":        "cidade",
	"uf":            "uf",
	"data_cadastro": "data_cadastro",
	"ativo":         "ativo",
	"created_at":    "created_at",
	"updated_at":    "updated_at",
}

// orderBy monta a cláusula ORDER BY a partir de OrderBy/OrderDir, com
// default "id ASC" (comportamento atual).
func (f ClienteFiltro) orderBy() string {
	return buildOrderByClause(clienteOrderWhitelist, f.OrderBy, f.OrderDir, "id", "ASC")
}

// where monta a cláusula WHERE (sem a palavra "WHERE") e os args correspondentes.
// Retorna string vazia quando não há filtros.
func (f ClienteFiltro) where() (string, []any) {
	var conds []string
	var args []any

	if f.UF != "" {
		conds = append(conds, "uf = ?")
		args = append(args, f.UF)
	}
	if f.Segmento != "" {
		conds = append(conds, "segmento = ?")
		args = append(args, f.Segmento)
	}
	if f.Ativo != nil {
		conds = append(conds, "ativo = ?")
		args = append(args, *f.Ativo)
	}
	if f.Q != "" {
		conds = append(conds, "(razao_social LIKE ? OR cnpj LIKE ?)")
		like := "%" + f.Q + "%"
		args = append(args, like, like)
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// List retorna clientes paginados conforme o filtro informado, mais o total
// para meta-dados de paginação.
func (r *ClienteRepository) List(ctx context.Context, db *sql.DB, page, limit int, filtro ClienteFiltro) ([]models.Cliente, int, error) {
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
	countQ := "SELECT COUNT(*) FROM clientes" + whereClause
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count clientes: %w", err)
	}

	q := "SELECT " + clienteColunas + " FROM clientes" + whereClause + filtro.orderBy() + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), limit, offset)

	rows, err := db.QueryContext(ctx, q, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list clientes: %w", err)
	}
	defer rows.Close()

	var out []models.Cliente
	for rows.Next() {
		c, err := scanCliente(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list clientes iteração: %w", err)
	}
	return out, total, nil
}

// ExistsByID verifica se existe um cliente com o id informado (ativo ou não).
func (r *ClienteRepository) ExistsByID(ctx context.Context, db *sql.DB, id int64) (bool, error) {
	const q = `SELECT 1 FROM clientes WHERE id = ? LIMIT 1`
	var one int
	err := db.QueryRowContext(ctx, q, id).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("repositories: exists cliente: %w", err)
	}
	return true, nil
}

// GetByID busca um cliente pelo ID. Retorna ErrNotFound se não existir.
func (r *ClienteRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Cliente, error) {
	q := "SELECT " + clienteColunas + " FROM clientes WHERE id = ? LIMIT 1"
	row := db.QueryRowContext(ctx, q, id)
	return scanCliente(row)
}

// SetAtivo ativa/inativa um cliente. Retorna ErrNotFound se não existir.
func (r *ClienteRepository) SetAtivo(ctx context.Context, db *sql.DB, id int64, ativo bool) error {
	const q = `UPDATE clientes SET ativo = ? WHERE id = ?`
	res, err := db.ExecContext(ctx, q, ativo, id)
	if err != nil {
		return fmt.Errorf("repositories: set ativo cliente: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountTotal retorna o total de clientes cadastrados.
func (r *ClienteRepository) CountTotal(ctx context.Context, db *sql.DB) (int, error) {
	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clientes`).Scan(&total); err != nil {
		return 0, fmt.Errorf("repositories: count total clientes: %w", err)
	}
	return total, nil
}

// CountPorAtivo retorna o total de clientes com o status ativo informado.
func (r *ClienteRepository) CountPorAtivo(ctx context.Context, db *sql.DB, ativo bool) (int, error) {
	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clientes WHERE ativo = ?`, ativo).Scan(&total); err != nil {
		return 0, fmt.Errorf("repositories: count clientes por ativo: %w", err)
	}
	return total, nil
}

// CountNovosNoPeriodo retorna o total de clientes cujo data_cadastro caiu no
// período informado. periodo: "today" (dia atual) ou "month" (mês atual).
func (r *ClienteRepository) CountNovosNoPeriodo(ctx context.Context, db *sql.DB, periodo string) (int, error) {
	var whereClause string
	if periodo == "today" {
		whereClause = "data_cadastro = CURDATE()"
	} else {
		whereClause = "YEAR(data_cadastro) = YEAR(CURDATE()) AND MONTH(data_cadastro) = MONTH(CURDATE())"
	}

	q := "SELECT COUNT(*) FROM clientes WHERE " + whereClause
	var total int
	if err := db.QueryRowContext(ctx, q).Scan(&total); err != nil {
		return 0, fmt.Errorf("repositories: count novos clientes: %w", err)
	}
	return total, nil
}

// SegmentoContagem representa a contagem de clientes de um segmento.
type SegmentoContagem struct {
	Segmento string `json:"segmento"`
	Total    int    `json:"total"`
}

// CountPorSegmento retorna a distribuição de clientes por segmento,
// ordenada do maior para o menor total.
func (r *ClienteRepository) CountPorSegmento(ctx context.Context, db *sql.DB) ([]SegmentoContagem, error) {
	const q = `
		SELECT segmento, COUNT(*) AS total
		FROM clientes
		GROUP BY segmento
		ORDER BY total DESC`

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("repositories: count por segmento: %w", err)
	}
	defer rows.Close()

	var out []SegmentoContagem
	for rows.Next() {
		var sc SegmentoContagem
		if err := rows.Scan(&sc.Segmento, &sc.Total); err != nil {
			return nil, fmt.Errorf("repositories: scan segmento: %w", err)
		}
		out = append(out, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repositories: count por segmento iteração: %w", err)
	}
	return out, nil
}

// UFContagem representa a contagem de clientes de uma UF.
type UFContagem struct {
	UF    string `json:"uf"`
	Total int    `json:"total"`
}

// CountPorUF retorna a distribuição de clientes por UF, ordenada do maior
// para o menor total.
func (r *ClienteRepository) CountPorUF(ctx context.Context, db *sql.DB) ([]UFContagem, error) {
	const q = `
		SELECT uf, COUNT(*) AS total
		FROM clientes
		GROUP BY uf
		ORDER BY total DESC`

	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("repositories: count por uf: %w", err)
	}
	defer rows.Close()

	var out []UFContagem
	for rows.Next() {
		var uc UFContagem
		if err := rows.Scan(&uc.UF, &uc.Total); err != nil {
			return nil, fmt.Errorf("repositories: scan uf: %w", err)
		}
		out = append(out, uc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repositories: count por uf iteração: %w", err)
	}
	return out, nil
}

// NextClienteIDOrigem retorna o próximo valor disponível para
// cliente_id_origem (MAX atual + 1). cliente_id_origem é UNIQUE e obrigatório
// na tabela, mas não é gerado automaticamente pelo banco — clientes criados
// via API (fora do CSV de origem) recebem um valor sequencial aqui.
func (r *ClienteRepository) NextClienteIDOrigem(ctx context.Context, db *sql.DB) (int64, error) {
	var next int64
	const q = `SELECT COALESCE(MAX(cliente_id_origem), 0) + 1 FROM clientes`
	if err := db.QueryRowContext(ctx, q).Scan(&next); err != nil {
		return 0, fmt.Errorf("repositories: next cliente_id_origem: %w", err)
	}
	return next, nil
}

// Create insere um novo cliente e preenche c.ID com o id gerado.
func (r *ClienteRepository) Create(ctx context.Context, db *sql.DB, c *models.Cliente) error {
	const q = `
		INSERT INTO clientes (cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := db.ExecContext(ctx, q,
		c.ClienteIDOrigem,
		c.CNPJ,
		c.RazaoSocial,
		c.Segmento,
		c.Cidade,
		c.UF,
		c.Bairro,
		c.DataCadastro,
		c.Ativo,
	)
	if err != nil {
		return fmt.Errorf("repositories: create cliente: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repositories: create cliente lastInsertId: %w", err)
	}
	c.ID = id
	return nil
}

// Update atualiza os campos editáveis de um cliente (cliente_id_origem e
// ativo não são alterados por aqui). Retorna ErrNotFound se não existir.
func (r *ClienteRepository) Update(ctx context.Context, db *sql.DB, id int64, c *models.Cliente) error {
	const q = `
		UPDATE clientes
		SET cnpj = ?, razao_social = ?, segmento = ?, cidade = ?, uf = ?, bairro = ?, data_cadastro = ?
		WHERE id = ?`
	res, err := db.ExecContext(ctx, q,
		c.CNPJ,
		c.RazaoSocial,
		c.Segmento,
		c.Cidade,
		c.UF,
		c.Bairro,
		c.DataCadastro,
		id,
	)
	if err != nil {
		return fmt.Errorf("repositories: update cliente: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: update cliente rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanCliente(s rowScanner) (*models.Cliente, error) {
	var c models.Cliente
	if err := s.Scan(
		&c.ID,
		&c.ClienteIDOrigem,
		&c.CNPJ,
		&c.RazaoSocial,
		&c.Segmento,
		&c.Cidade,
		&c.UF,
		&c.Bairro,
		&c.DataCadastro,
		&c.Ativo,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan cliente: %w", err)
	}
	return &c, nil
}
