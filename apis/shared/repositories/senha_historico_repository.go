// Package repositories contém funções puras de acesso a dados (stateless).
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// SenhaHistorico representa um registro de auditoria de alteração de senha.
type SenhaHistorico struct {
	ID                 int64
	UsuarioID          int64
	ResetadoPorID      sql.NullInt64
	SenhaHashAnterior  string
	IPOrigem           string
	UserAgent          string
	TipoReset          string // "usuario" | "admin" | "primeiro_acesso" | "esquecimento"
	CreatedAt          time.Time
}

// NewSenhaHistoricoRepository cria um repositório stateless.
func NewSenhaHistoricoRepository() *SenhaHistoricoRepository {
	return &SenhaHistoricoRepository{}
}

// SenhaHistoricoRepository agrupa queries da tabela senha_historico.
type SenhaHistoricoRepository struct{}

// Create insere um novo registro de histórico de senha.
func (r *SenhaHistoricoRepository) Create(ctx context.Context, db *sql.DB, h *SenhaHistorico) error {
	const q = `
		INSERT INTO senha_historico (usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset)
		VALUES (?, ?, ?, ?, ?, ?)`
	var resetadoPorID interface{}
	if h.ResetadoPorID.Valid {
		resetadoPorID = h.ResetadoPorID.Int64
	} else {
		resetadoPorID = nil
	}
	res, err := db.ExecContext(ctx, q, h.UsuarioID, resetadoPorID, h.SenhaHashAnterior, h.IPOrigem, h.UserAgent, h.TipoReset)
	if err != nil {
		return fmt.Errorf("repositories: insert senha_historico: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repositories: last insert id: %w", err)
	}
	h.ID = id
	return nil
}

// senhaHistoricoOrderWhitelist mapeia os campos de ordenação aceitos pela
// API para as colunas SQL reais da tabela senha_historico.
var senhaHistoricoOrderWhitelist = map[string]string{
	"id":         "id",
	"usuario_id": "usuario_id",
	"tipo_reset": "tipo_reset",
	"created_at": "created_at",
}

// FindByUsuario lista histórico de senhas de um usuário (paginado).
// orderBy/orderDir controlam a ordenação (whitelist: ver
// senhaHistoricoOrderWhitelist); default "id DESC" (comportamento atual).
func (r *SenhaHistoricoRepository) FindByUsuario(ctx context.Context, db *sql.DB, usuarioID int64, page, limit int, orderBy, orderDir string) ([]SenhaHistorico, int, error) {
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

	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM senha_historico WHERE usuario_id = ?`, usuarioID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count senha_historico: %w", err)
	}

	orderClause := buildOrderByClause(senhaHistoricoOrderWhitelist, orderBy, orderDir, "id", "DESC")
	q := `
		SELECT id, usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset, created_at
		FROM senha_historico
		WHERE usuario_id = ?` + orderClause + `
		LIMIT ? OFFSET ?`
	rows, err := db.QueryContext(ctx, q, usuarioID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list senha_historico: %w", err)
	}
	defer rows.Close()

	var out []SenhaHistorico
	for rows.Next() {
		h, err := scanSenhaHistorico(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *h)
	}
	return out, total, rows.Err()
}

// FindAll lista todos os históricos de senhas (paginado) — admin only.
// orderBy/orderDir controlam a ordenação (whitelist: ver
// senhaHistoricoOrderWhitelist); default "id DESC" (comportamento atual).
func (r *SenhaHistoricoRepository) FindAll(ctx context.Context, db *sql.DB, page, limit int, orderBy, orderDir string) ([]SenhaHistorico, int, error) {
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

	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM senha_historico`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count senha_historico: %w", err)
	}

	orderClause := buildOrderByClause(senhaHistoricoOrderWhitelist, orderBy, orderDir, "id", "DESC")
	q := `
		SELECT id, usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset, created_at
		FROM senha_historico` + orderClause + `
		LIMIT ? OFFSET ?`
	rows, err := db.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list senha_historico: %w", err)
	}
	defer rows.Close()

	var out []SenhaHistorico
	for rows.Next() {
		h, err := scanSenhaHistorico(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *h)
	}
	return out, total, rows.Err()
}

func scanSenhaHistorico(s rowScanner) (*SenhaHistorico, error) {
	var h SenhaHistorico
	var resetadoPorID sql.NullInt64
	if err := s.Scan(
		&h.ID,
		&h.UsuarioID,
		&resetadoPorID,
		&h.SenhaHashAnterior,
		&h.IPOrigem,
		&h.UserAgent,
		&h.TipoReset,
		&h.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan senha_historico: %w", err)
	}
	if resetadoPorID.Valid {
		h.ResetadoPorID = resetadoPorID
	}
	return &h, nil
}
