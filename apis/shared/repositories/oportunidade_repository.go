// Package repositories contém funções puras de acesso a dados (stateless).
// Oportunidade queries - cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/models"
)

// OportunidadeRepository agrupa queries da tabela oportunidades.
type OportunidadeRepository struct{}

// NewOportunidadeRepository cria um repositório stateless.
func NewOportunidadeRepository() *OportunidadeRepository {
	return &OportunidadeRepository{}
}

// oportunidadeColunas lista as colunas da tabela oportunidades na ordem
// usada pelo scan.
const oportunidadeColunas = `oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda, created_at, updated_at`

// OportunidadeFiltro agrupa os filtros opcionais aceitos por List.
// Campos zero-value/vazios são ignorados (não filtram).
type OportunidadeFiltro struct {
	ClienteID       int64
	VendedorID      int64
	Etapa           string
	Origem          string
	DataAberturaDe  string // formato AAAA-MM-DD (inclusive)
	DataAberturaAte string // formato AAAA-MM-DD (inclusive)
	Q               string // busca textual em origem OU etapa (LIKE)
	OrderBy         string // campo de ordenação (whitelist: ver oportunidadeOrderWhitelist); default "oportunidade_id"
	OrderDir        string // "asc" ou "desc" (case-insensitive); default "asc"
}

// oportunidadeOrderWhitelist mapeia os campos de ordenação aceitos pela API
// para as colunas SQL reais da tabela oportunidades.
var oportunidadeOrderWhitelist = map[string]string{
	"id":                "oportunidade_id",
	"data_abertura":     "data_abertura",
	"valor_estimado":    "valor_estimado",
	"probabilidade_pct": "probabilidade_pct",
	"etapa":             "etapa",
	"origem":            "origem",
	"created_at":        "created_at",
	"updated_at":        "updated_at",
}

// orderBy monta a cláusula ORDER BY a partir de OrderBy/OrderDir, com
// default "oportunidade_id ASC".
func (f OportunidadeFiltro) orderBy() string {
	return buildOrderByClause(oportunidadeOrderWhitelist, f.OrderBy, f.OrderDir, "oportunidade_id", "ASC")
}

// where monta a cláusula WHERE (sem a palavra "WHERE") e os args correspondentes.
// Retorna string vazia quando não há filtros.
func (f OportunidadeFiltro) where() (string, []any) {
	var conds []string
	var args []any

	if f.ClienteID > 0 {
		conds = append(conds, "cliente_id = ?")
		args = append(args, f.ClienteID)
	}
	if f.VendedorID > 0 {
		conds = append(conds, "vendedor_id = ?")
		args = append(args, f.VendedorID)
	}
	if f.Etapa != "" {
		conds = append(conds, "etapa = ?")
		args = append(args, f.Etapa)
	}
	if f.Origem != "" {
		conds = append(conds, "origem = ?")
		args = append(args, f.Origem)
	}
	if f.DataAberturaDe != "" {
		conds = append(conds, "data_abertura >= ?")
		args = append(args, f.DataAberturaDe)
	}
	if f.DataAberturaAte != "" {
		conds = append(conds, "data_abertura <= ?")
		args = append(args, f.DataAberturaAte)
	}
	if f.Q != "" {
		conds = append(conds, "(origem LIKE ? OR etapa LIKE ?)")
		like := "%" + f.Q + "%"
		args = append(args, like, like)
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// List retorna oportunidades paginadas conforme o filtro informado, mais o
// total para meta-dados de paginação.
func (r *OportunidadeRepository) List(ctx context.Context, db *sql.DB, page, limit int, filtro OportunidadeFiltro) ([]models.Oportunidade, int, error) {
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
	countQ := "SELECT COUNT(*) FROM oportunidades" + whereClause
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count oportunidades: %w", err)
	}

	q := "SELECT " + oportunidadeColunas + " FROM oportunidades" + whereClause + filtro.orderBy() + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), limit, offset)

	rows, err := db.QueryContext(ctx, q, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list oportunidades: %w", err)
	}
	defer rows.Close()

	var out []models.Oportunidade
	for rows.Next() {
		o, err := scanOportunidade(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *o)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list oportunidades iteração: %w", err)
	}
	return out, total, nil
}

// GetByID busca uma oportunidade pelo ID. Retorna ErrNotFound se não existir.
func (r *OportunidadeRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Oportunidade, error) {
	q := "SELECT " + oportunidadeColunas + " FROM oportunidades WHERE oportunidade_id = ? LIMIT 1"
	row := db.QueryRowContext(ctx, q, id)
	return scanOportunidade(row)
}

// Create insere uma nova oportunidade e preenche o.OportunidadeID com o id
// gerado nativamente pelo AUTO_INCREMENT do MySQL.
func (r *OportunidadeRepository) Create(ctx context.Context, db *sql.DB, o *models.Oportunidade) error {
	const q = `
		INSERT INTO oportunidades (cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	res, err := db.ExecContext(ctx, q,
		o.ClienteID,
		o.VendedorID,
		o.Origem,
		o.DataAbertura,
		o.Etapa,
		o.ProbabilidadePct,
		o.ValorEstimado,
		o.DataFechamento,
		o.CicloDias,
		o.MotivoPerda,
	)
	if err != nil {
		return fmt.Errorf("repositories: create oportunidade: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repositories: create oportunidade lastInsertId: %w", err)
	}
	o.OportunidadeID = id
	return nil
}

// Update atualiza os campos editáveis de uma oportunidade existente.
// Retorna ErrNotFound se não existir.
func (r *OportunidadeRepository) Update(ctx context.Context, db *sql.DB, id int64, o *models.Oportunidade) error {
	const q = `
		UPDATE oportunidades
		SET cliente_id = ?, vendedor_id = ?, origem = ?, data_abertura = ?, etapa = ?, probabilidade_pct = ?, valor_estimado = ?, data_fechamento = ?, ciclo_dias = ?, motivo_perda = ?
		WHERE oportunidade_id = ?`
	res, err := db.ExecContext(ctx, q,
		o.ClienteID,
		o.VendedorID,
		o.Origem,
		o.DataAbertura,
		o.Etapa,
		o.ProbabilidadePct,
		o.ValorEstimado,
		o.DataFechamento,
		o.CicloDias,
		o.MotivoPerda,
		id,
	)
	if err != nil {
		return fmt.Errorf("repositories: update oportunidade: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: update oportunidade rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete remove uma oportunidade pela PK oportunidade_id (hard delete — não
// há coluna deleted_at nesta tabela). Retorna ErrNotFound se não existir.
func (r *OportunidadeRepository) Delete(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM oportunidades WHERE oportunidade_id = ?`, id)
	if err != nil {
		return fmt.Errorf("repositories: delete oportunidade: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: delete oportunidade rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanOportunidade(s rowScanner) (*models.Oportunidade, error) {
	var o models.Oportunidade
	if err := s.Scan(
		&o.OportunidadeID,
		&o.ClienteID,
		&o.VendedorID,
		&o.Origem,
		&o.DataAbertura,
		&o.Etapa,
		&o.ProbabilidadePct,
		&o.ValorEstimado,
		&o.DataFechamento,
		&o.CicloDias,
		&o.MotivoPerda,
		&o.CreatedAt,
		&o.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan oportunidade: %w", err)
	}
	return &o, nil
}
