// Package repositories contém funções puras de acesso a dados (stateless).
//
// Recebem *sql.DB explicitamente para evitar ciclo de dependência com
// os services da API. Cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RevokeReason é o motivo gravado em refresh_tokens.revoked_reason junto com
// revoked_at (SEC-07). Os valores espelham o ENUM da coluna.
type RevokeReason string

const (
	// RevokeReasonRotacao: token trocado por um novo no /refresh (single-use).
	RevokeReasonRotacao RevokeReason = "rotacao"
	// RevokeReasonLogout: logout explícito do usuário.
	RevokeReasonLogout RevokeReason = "logout"
	// RevokeReasonRevogacaoMassa: revogação de todas as sessões por motivo de
	// segurança (ex.: reuso de token já rotacionado — SEC-07).
	RevokeReasonRevogacaoMassa RevokeReason = "revogacao_massa"
	// RevokeReasonSenha: troca de senha pelo usuário ou reset pelo admin.
	RevokeReasonSenha RevokeReason = "senha"
	// RevokeReasonInativacao: usuário (ou vendedor dele) inativado.
	RevokeReasonInativacao RevokeReason = "inativacao"
	// RevokeReasonDesconhecido representa revoked_reason NULL em token já
	// revogado (revogação legada, anterior ao SEC-07).
	RevokeReasonDesconhecido RevokeReason = ""
)

// RefreshToken representa um token de refresh no banco.
type RefreshToken struct {
	ID        int64
	UsuarioID int64
	TokenHash string
	ExpiresAt time.Time
	RevokedAt sql.NullTime
	// RevokedReason é o motivo da revogação; RevokeReasonDesconhecido ("")
	// quando a coluna é NULL (token ativo ou revogação legada).
	RevokedReason RevokeReason
	IPOrigem      string
	UserAgent     string
}

// refreshTokenColunas é a lista de colunas lida por scanRefreshToken.
const refreshTokenColunas = `id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent, revoked_reason`

// NewRefreshTokenRepository cria um repositório stateless.
func NewRefreshTokenRepository() *RefreshTokenRepository {
	return &RefreshTokenRepository{}
}

// RefreshTokenRepository agrupa queries da tabela refresh_tokens.
type RefreshTokenRepository struct{}

// Create insere um novo refresh token. Aceita Execer (*sql.DB ou *sql.Tx)
// para participar da transação de rotação (revoga o antigo + insere o novo).
func (r *RefreshTokenRepository) Create(ctx context.Context, db Execer, rt *RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (usuario_id, token_hash, expires_at, ip_origem, user_agent)
		VALUES (?, ?, ?, ?, ?)`
	res, err := db.ExecContext(ctx, q, rt.UsuarioID, rt.TokenHash, rt.ExpiresAt, rt.IPOrigem, rt.UserAgent)
	if err != nil {
		return fmt.Errorf("repositories: insert refresh_token: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repositories: last insert id: %w", err)
	}
	rt.ID = id
	return nil
}

// FindByTokenHash busca um refresh token pelo hash. Retorna ErrNotFound se não existir.
func (r *RefreshTokenRepository) FindByTokenHash(ctx context.Context, db *sql.DB, hash string) (*RefreshToken, error) {
	q := `
		SELECT ` + refreshTokenColunas + `
		FROM refresh_tokens
		WHERE token_hash = ?
		LIMIT 1`
	row := db.QueryRowContext(ctx, q, hash)
	return scanRefreshToken(row)
}

// Revoke marca um refresh token como revogado (preenche revoked_at) de forma
// atômica: o UPDATE só afeta o registro se ele ainda não foi revogado. Em duas
// requisições concorrentes com o mesmo token, só uma revoga; a outra recebe
// ErrNotFound (0 linhas afetadas = inexistente ou já revogado).
// Aceita Execer (*sql.DB ou *sql.Tx) para participar de transação.
func (r *RefreshTokenRepository) Revoke(ctx context.Context, db Execer, id int64, reason RevokeReason) error {
	const q = `UPDATE refresh_tokens SET revoked_at = ?, revoked_reason = ? WHERE id = ? AND revoked_at IS NULL`
	res, err := db.ExecContext(ctx, q, time.Now(), string(reason), id)
	if err != nil {
		return fmt.Errorf("repositories: revoke refresh_token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: revoke refresh_token rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeAllByUser revoga todos os refresh tokens ainda ativos de um usuário,
// gravando o motivo (SEC-07). Tokens já revogados mantêm o motivo original.
func (r *RefreshTokenRepository) RevokeAllByUser(ctx context.Context, db *sql.DB, usuarioID int64, reason RevokeReason) error {
	const q = `UPDATE refresh_tokens SET revoked_at = ?, revoked_reason = ? WHERE usuario_id = ? AND revoked_at IS NULL`
	_, err := db.ExecContext(ctx, q, time.Now(), string(reason), usuarioID)
	if err != nil {
		return fmt.Errorf("repositories: revoke all refresh_tokens: %w", err)
	}
	return nil
}

// RevokeAllByVendedorID revoga todos os refresh tokens ainda válidos dos
// usuários vinculados ao vendedor (usuarios.id_vendedor). Aceita Execer para
// participar da transação de desligamento do vendedor (SEC-06). Idempotente:
// sem tokens pendentes, afeta 0 linhas e não retorna erro. Retorna o número
// de tokens revogados.
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

// DeleteExpired remove tokens expirados e não revogados mais antigos que 30 dias.
func (r *RefreshTokenRepository) DeleteExpired(ctx context.Context, db *sql.DB) (int64, error) {
	const q = `
		DELETE FROM refresh_tokens
		WHERE (expires_at < ? AND revoked_at IS NOT NULL)
		   OR (expires_at < ? AND revoked_at IS NULL)`
	cutoff := time.Now().Add(-30 * 24 * time.Hour) // 30 dias
	cutoffExpired := time.Now()
	res, err := db.ExecContext(ctx, q, cutoffExpired, cutoff)
	if err != nil {
		return 0, fmt.Errorf("repositories: delete expired refresh_tokens: %w", err)
	}
	n, _ := res.RowsAffected()
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
	var revokedAt sql.NullTime
	var revokedReason sql.NullString
	if err := s.Scan(
		&rt.ID,
		&rt.UsuarioID,
		&rt.TokenHash,
		&rt.ExpiresAt,
		&revokedAt,
		&rt.IPOrigem,
		&rt.UserAgent,
		&revokedReason,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan refresh_token: %w", err)
	}
	if revokedAt.Valid {
		rt.RevokedAt = revokedAt
	}
	if revokedReason.Valid {
		rt.RevokedReason = RevokeReason(revokedReason.String)
	}
	return &rt, nil
}
