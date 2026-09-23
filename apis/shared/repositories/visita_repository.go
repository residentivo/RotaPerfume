// Package repositories contém funções puras de acesso a dados (stateless).
// Visita queries - cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/models"
)

// VisitaRepository agrupa queries da tabela visitas.
type VisitaRepository struct{}

// NewVisitaRepository cria um repositório stateless.
func NewVisitaRepository() *VisitaRepository {
	return &VisitaRepository{}
}

// visitaColunas lista as colunas da tabela visitas na ordem usada pelo scan.
const visitaColunas = `visita_id, cliente_id, vendedor_id, data_visita, resultado, duracao_min, created_at, updated_at`

// VisitaFiltro agrupa os filtros opcionais aceitos por List.
// Campos zero-value/vazios são ignorados (não filtram).
type VisitaFiltro struct {
	ClienteID     int64
	VendedorID    int64
	Resultado     string
	DataVisitaDe  string // formato AAAA-MM-DD (inclusive)
	DataVisitaAte string // formato AAAA-MM-DD (inclusive)
	Q             string // busca textual em resultado (LIKE)
	OrderBy       string // campo de ordenação (whitelist: ver visitaOrderWhitelist); default "visita_id"
	OrderDir      string // "asc" ou "desc" (case-insensitive); default "asc"
}

// visitaOrderWhitelist mapeia os campos de ordenação aceitos pela API
// para as colunas SQL reais da tabela visitas.
var visitaOrderWhitelist = map[string]string{
	"id":          "visita_id",
	"visita_id":   "visita_id",
	"data_visita": "data_visita",
	"duracao_min": "duracao_min",
	"resultado":   "resultado",
	"created_at":  "created_at",
	"updated_at":  "updated_at",
}

// orderBy monta a cláusula ORDER BY a partir de OrderBy/OrderDir, com
// default "visita_id ASC".
func (f VisitaFiltro) orderBy() string {
	return buildOrderByClause(visitaOrderWhitelist, f.OrderBy, f.OrderDir, "visita_id", "ASC")
}

// where monta a cláusula WHERE (sem a palavra "WHERE") e os args correspondentes.
// Retorna string vazia quando não há filtros.
func (f VisitaFiltro) where() (string, []any) {
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
	if f.Resultado != "" {
		conds = append(conds, "resultado = ?")
		args = append(args, f.Resultado)
	}
	if f.DataVisitaDe != "" {
		conds = append(conds, "data_visita >= ?")
		args = append(args, f.DataVisitaDe)
	}
	if f.DataVisitaAte != "" {
		conds = append(conds, "data_visita <= ?")
		args = append(args, f.DataVisitaAte)
	}
	if f.Q != "" {
		conds = append(conds, "resultado LIKE ?")
		args = append(args, "%"+f.Q+"%")
	}

	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// List retorna visitas paginadas conforme o filtro informado, mais o total
// para meta-dados de paginação.
func (r *VisitaRepository) List(ctx context.Context, db *sql.DB, page, limit int, filtro VisitaFiltro) ([]models.Visita, int, error) {
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
	countQ := "SELECT COUNT(*) FROM visitas" + whereClause
	if err := db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count visitas: %w", err)
	}

	q := "SELECT " + visitaColunas + " FROM visitas" + whereClause + filtro.orderBy() + " LIMIT ? OFFSET ?"
	queryArgs := append(append([]any{}, args...), limit, offset)

	rows, err := db.QueryContext(ctx, q, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list visitas: %w", err)
	}
	defer rows.Close()

	var out []models.Visita
	for rows.Next() {
		v, err := scanVisita(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list visitas iteração: %w", err)
	}
	return out, total, nil
}

// GetByID busca uma visita pelo ID. Retorna ErrNotFound se não existir.
func (r *VisitaRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Visita, error) {
	q := "SELECT " + visitaColunas + " FROM visitas WHERE visita_id = ? LIMIT 1"
	row := db.QueryRowContext(ctx, q, id)
	return scanVisita(row)
}

// Create insere uma nova visita e preenche v.VisitaID com o id gerado
// nativamente pelo AUTO_INCREMENT do MySQL.
func (r *VisitaRepository) Create(ctx context.Context, db *sql.DB, v *models.Visita) error {
	const q = `
		INSERT INTO visitas (cliente_id, vendedor_id, data_visita, resultado, duracao_min)
		VALUES (?, ?, ?, ?, ?)`
	res, err := db.ExecContext(ctx, q,
		v.ClienteID,
		v.VendedorID,
		v.DataVisita,
		v.Resultado,
		v.DuracaoMin,
	)
	if err != nil {
		return fmt.Errorf("repositories: create visita: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repositories: create visita lastInsertId: %w", err)
	}
	v.VisitaID = id
	return nil
}

// Update atualiza os campos editáveis de uma visita existente.
// Retorna ErrNotFound se não existir.
func (r *VisitaRepository) Update(ctx context.Context, db *sql.DB, id int64, v *models.Visita) error {
	const q = `
		UPDATE visitas
		SET cliente_id = ?, vendedor_id = ?, data_visita = ?, resultado = ?, duracao_min = ?
		WHERE visita_id = ?`
	res, err := db.ExecContext(ctx, q,
		v.ClienteID,
		v.VendedorID,
		v.DataVisita,
		v.Resultado,
		v.DuracaoMin,
		id,
	)
	if err != nil {
		return fmt.Errorf("repositories: update visita: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: update visita rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete remove uma visita pela PK visita_id (hard delete — não há coluna
// deleted_at nesta tabela). Retorna ErrNotFound se não existir.
func (r *VisitaRepository) Delete(ctx context.Context, db *sql.DB, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM visitas WHERE visita_id = ?`, id)
	if err != nil {
		return fmt.Errorf("repositories: delete visita: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: delete visita rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanVisita(s rowScanner) (*models.Visita, error) {
	var v models.Visita
	if err := s.Scan(
		&v.VisitaID,
		&v.ClienteID,
		&v.VendedorID,
		&v.DataVisita,
		&v.Resultado,
		&v.DuracaoMin,
		&v.CreatedAt,
		&v.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan visita: %w", err)
	}
	return &v, nil
}
