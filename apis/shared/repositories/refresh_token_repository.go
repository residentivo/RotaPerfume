package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ARQ-03: o contexto Identidade migrou para identidade/identidademysql. Aqui
// ficam RevokeAllByVendedorID (desligamento de vendedor, mesma tx do I-09,
// ARQ-04) e FindByUsuario (sem chamador de produção; sai no ARQ-07 junto com
// este arquivo).

// RevokeReason é o motivo gravado em refresh_tokens.revoked_reason (SEC-07).
type RevokeReason string

// Motivos (espelham o ENUM da coluna; ver identidade.MotivoRevogacao).
const (
	RevokeReasonRotacao        RevokeReason = "rotacao"
	RevokeReasonLogout         RevokeReason = "logout"
	RevokeReasonRevogacaoMassa RevokeReason = "revogacao_massa"
	RevokeReasonSenha          RevokeReason = "senha"
	RevokeReasonInativacao     RevokeReason = "inativacao"
	RevokeReasonDesconhecido   RevokeReason = ""
)

// RefreshToken representa um token de refresh no banco.
type RefreshToken struct {
	ID            int64
	UsuarioID     int64
	TokenHash     string
	ExpiresAt     time.Time
	RevokedAt     sql.NullTime
	RevokedReason RevokeReason
	IPOrigem      string
	UserAgent     string
}

const refreshTokenColunas = `id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent, revoked_reason`

// RefreshTokenRepository agrupa as queries legadas de refresh_tokens.
type RefreshTokenRepository struct{}

// NewRefreshTokenRepository cria um repositório stateless.
func NewRefreshTokenRepository() *RefreshTokenRepository {
	return &RefreshTokenRepository{}
}

// RevokeAllByVendedorID revoga os refresh tokens ainda válidos dos usuários
// do vendedor, na tx do desligamento (SEC-06/I-09). Idempotente; devolve
// quantos tokens revogou.
func (r *RefreshTokenRepository) RevokeAllByVendedorID(ctx context.Context, db Execer, vendedorID int64, reason RevokeReason) (int64, error) {
	const q = `
		UPDATE refresh_tokens rt
		JOIN usuarios u ON u.id = rt.usuario_id
		SET rt.revoked_at = ?, rt.revoked_reason = ?
		WHERE u.id_vendedor = ? AND rt.revoked_at IS NULL`
	res, err := db.ExecContext(ctx, q, time.Now(), string(reason), vendedorID)
	if err != nil {
		return 0, fmt.Errorf("repositories: revoke refresh_tokens por vendedor: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("repositories: revoke refresh_tokens por vendedor rows affected: %w", err)
	}
	return n, nil
}

// FindByUsuario lista refresh tokens de um usuário (paginado).
func (r *RefreshTokenRepository) FindByUsuario(ctx context.Context, db *sql.DB, usuarioID int64, page, limit int) ([]RefreshToken, int, error) {
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
	countQ := `SELECT COUNT(*) FROM refresh_tokens WHERE usuario_id = ?`
	if err := db.QueryRowContext(ctx, countQ, usuarioID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count refresh_tokens: %w", err)
	}

	q := `
		SELECT ` + refreshTokenColunas + `
		FROM refresh_tokens
		WHERE usuario_id = ?
		ORDER BY id DESC
		LIMIT ? OFFSET ?`
	rows, err := db.QueryContext(ctx, q, usuarioID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list refresh_tokens: %w", err)
	}
	defer rows.Close()

	var out []RefreshToken
	for rows.Next() {
		rt, err := scanRefreshToken(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *rt)
	}
	return out, total, rows.Err()
}

func scanRefreshToken(s rowScanner) (*RefreshToken, error) {
	var rt RefreshToken
	var revokedReason sql.NullString
	if err := s.Scan(&rt.ID, &rt.UsuarioID, &rt.TokenHash, &rt.ExpiresAt, &rt.RevokedAt, &rt.IPOrigem, &rt.UserAgent, &revokedReason); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan refresh_token: %w", err)
	}
	if revokedReason.Valid {
		rt.RevokedReason = RevokeReason(revokedReason.String)
	}
	return &rt, nil
}
