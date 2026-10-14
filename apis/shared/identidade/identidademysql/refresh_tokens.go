package identidademysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/vlog"
)

const arqRefresh = "refresh_tokens.go"

// refreshTokenColunas é a lista lida por scanRefreshToken.
const refreshTokenColunas = `id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent, revoked_reason`

// RefreshTokens é o repositório de refresh_tokens (porta
// identidadeapp.RefreshTokens). Ligado ao pool ou à tx da rotação.
type RefreshTokens struct {
	q     db.Querier
	agora func() time.Time
}

// NovoRefreshTokens liga o repositório ao pool ou a uma tx. q nil: panic.
func NovoRefreshTokens(q db.Querier) *RefreshTokens {
	if q == nil {
		panic("identidademysql: NovoRefreshTokens exige Querier")
	}
	return &RefreshTokens{q: q, agora: time.Now}
}

// PorHash busca pelo hash; identidade.ErrRefreshInvalido se não existir.
func (r *RefreshTokens) PorHash(ctx context.Context, hash string) (*identidade.RefreshToken, error) {
	q := `
		SELECT ` + refreshTokenColunas + `
		FROM refresh_tokens
		WHERE token_hash = ?
		LIMIT 1`
	rt, err := scanRefreshToken(r.q.QueryRowContext(ctx, q, hash))
	vlog.Printf(arqRefresh, "RefreshTokens.PorHash", "SELECT refresh_tokens encontrado=%t", err == nil)
	return rt, err
}

// Inserir grava o token (só o hash) e preenche rt.ID.
func (r *RefreshTokens) Inserir(ctx context.Context, rt *identidade.RefreshToken) error {
	const q = `
		INSERT INTO refresh_tokens (usuario_id, token_hash, expires_at, ip_origem, user_agent)
		VALUES (?, ?, ?, ?, ?)`
	res, err := r.q.ExecContext(ctx, q, rt.UsuarioID, rt.HashToken, rt.ExpiraEm, rt.IP, rt.UserAgent)
	if err != nil {
		return fmt.Errorf("identidademysql: insert refresh_token: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("identidademysql: last insert id: %w", err)
	}
	rt.ID = id
	vlog.Printf(arqRefresh, "RefreshTokens.Inserir", "INSERT refresh_tokens id=%d", id)
	return nil
}

// Revogar é atômico: o UPDATE só afeta token ainda não revogado. Sem linha
// afetada (inexistente ou já revogado por outra requisição) devolve
// identidade.ErrRefreshRevogado.
func (r *RefreshTokens) Revogar(ctx context.Context, id int64, m identidade.MotivoRevogacao) error {
	const q = `UPDATE refresh_tokens SET revoked_at = ?, revoked_reason = ? WHERE id = ? AND revoked_at IS NULL`
	n, err := r.afetadas(ctx, "revoke refresh_token", q, r.agora(), string(m), id)
	if err != nil {
		return err
	}
	vlog.Printf(arqRefresh, "RefreshTokens.Revogar", "UPDATE refresh_tokens revogar afetadas=%d", n)
	if n == 0 {
		return identidade.ErrRefreshRevogado
	}
	return nil
}

// RevogarTodosDoUsuario revoga os tokens ainda ativos do usuário; os já
// revogados mantêm o motivo original.
func (r *RefreshTokens) RevogarTodosDoUsuario(ctx context.Context, usuarioID int64, m identidade.MotivoRevogacao) error {
	const q = `UPDATE refresh_tokens SET revoked_at = ?, revoked_reason = ? WHERE usuario_id = ? AND revoked_at IS NULL`
	if _, err := r.q.ExecContext(ctx, q, r.agora(), string(m), usuarioID); err != nil {
		return fmt.Errorf("identidademysql: revoke all refresh_tokens: %w", err)
	}
	return nil
}

// MarcarReuso grava agora em reuso_detectado_em se o token foi rotacionado e
// o último reuso é NULL ou <= limite (SEC-12). UPDATE condicional atômico:
// em duas requisições concorrentes só uma marca. true = marcou (cortar).
func (r *RefreshTokens) MarcarReuso(ctx context.Context, id int64, agora, limite time.Time) (bool, error) {
	const q = `
		UPDATE refresh_tokens SET reuso_detectado_em = ?
		WHERE id = ? AND revoked_reason = 'rotacao'
		  AND (reuso_detectado_em IS NULL OR reuso_detectado_em <= ?)`
	n, err := r.afetadas(ctx, "marcar reuso refresh_token", q, agora, id, limite)
	if err != nil {
		return false, err
	}
	vlog.Printf(arqRefresh, "RefreshTokens.MarcarReuso", "UPDATE refresh_tokens marcar reuso afetadas=%d", n)
	return n > 0, nil
}

// DesfazerReuso limpa a marca se ainda for a desta chamada (corte falhou).
func (r *RefreshTokens) DesfazerReuso(ctx context.Context, id int64, marca time.Time) error {
	const q = `UPDATE refresh_tokens SET reuso_detectado_em = NULL WHERE id = ? AND reuso_detectado_em = ?`
	if _, err := r.q.ExecContext(ctx, q, id, marca); err != nil {
		return fmt.Errorf("identidademysql: desfazer reuso refresh_token: %w", err)
	}
	return nil
}

// ApagarExpirados apaga um lote de tokens com expires_at < corte (CHORE-02).
// ORDER BY id + LIMIT mantém cada DELETE curto.
func (r *RefreshTokens) ApagarExpirados(ctx context.Context, corte time.Time, lote int) (int64, error) {
	const q = `DELETE FROM refresh_tokens WHERE expires_at < ? ORDER BY id LIMIT ?`
	n, err := r.afetadas(ctx, "delete expired refresh_tokens", q, corte, lote)
	if err != nil {
		return 0, err
	}
	vlog.Printf(arqRefresh, "RefreshTokens.ApagarExpirados", "DELETE refresh_tokens afetadas=%d", n)
	return n, nil
}

// afetadas executa e devolve RowsAffected (erro nele também é erro).
func (r *RefreshTokens) afetadas(ctx context.Context, op, q string, args ...any) (int64, error) {
	res, err := r.q.ExecContext(ctx, q, args...)
	if err != nil {
		return 0, fmt.Errorf("identidademysql: %s: %w", op, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("identidademysql: %s rows affected: %w", op, err)
	}
	return n, nil
}

func scanRefreshToken(s db.Linha) (*identidade.RefreshToken, error) {
	var (
		rt        identidade.RefreshToken
		revogado  sql.NullTime
		motivo    sql.NullString
		ip, agent sql.NullString
	)
	err := s.Scan(&rt.ID, &rt.UsuarioID, &rt.HashToken, &rt.ExpiraEm, &revogado, &ip, &agent, &motivo)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, identidade.ErrRefreshInvalido
	}
	if err != nil {
		return nil, fmt.Errorf("identidademysql: scan refresh_token: %w", err)
	}
	rt.IP, rt.UserAgent = ip.String, agent.String
	if revogado.Valid {
		t := revogado.Time
		rt.RevogadoEm = &t
	}
	if motivo.Valid {
		rt.Motivo = identidade.MotivoRevogacao(motivo.String)
	}
	return &rt, nil
}
