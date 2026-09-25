// Package services (da API) orquestra regras de negócio de refresh tokens.
package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/rotaperfumes/shared/repositories"
)

const (
	// RefreshTokenBytes é o tamanho do token aleatório em bytes (será hex-encoded).
	RefreshTokenBytes = 32
	// RefreshTokenTTL é a validade do refresh token (7 dias).
	RefreshTokenTTL = 7 * 24 * time.Hour
)

var (
	ErrRefreshTokenNotFound = errors.New("refresh token não encontrado")
	ErrRefreshTokenExpired  = errors.New("refresh token expirado")
	ErrRefreshTokenRevoked  = errors.New("refresh token revogado")
)

// RefreshTokenService gerencia lifecycle dos refresh tokens.
type RefreshTokenService struct {
	repo *repositories.RefreshTokenRepository
}

// NewRefreshTokenService cria um RefreshTokenService.
func NewRefreshTokenService() *RefreshTokenService {
	return &RefreshTokenService{
		repo: repositories.NewRefreshTokenRepository(),
	}
}

// generateRandomToken gera um token hexadecimal aleatório.
func generateRandomToken() (string, error) {
	bytes := make([]byte, RefreshTokenBytes)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateRefreshToken cria um novo refresh token e retorna o token em texto puro (não o hash).
// O IP e UserAgent são armazenados para auditoria. Aceita Execer (*sql.DB ou
// *sql.Tx) para ser usado dentro da transação de rotação.
func (s *RefreshTokenService) GenerateRefreshToken(ctx context.Context, db repositories.Execer, usuarioID int64, ipOrigem, userAgent string) (string, error) {
	token, err := generateRandomToken()
	if err != nil {
		return "", err
	}

	// Hash do token para armazenamento (nunca salvar o token em texto puro).
	tokenHash := hashToken(token)

	rt := &repositories.RefreshToken{
		UsuarioID: usuarioID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(RefreshTokenTTL),
		IPOrigem:  ipOrigem,
		UserAgent: userAgent,
	}

	if err := s.repo.Create(ctx, db, rt); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}

	log.Printf("[refresh] token criado: usuario_id=%d ip=%s", usuarioID, ipOrigem)
	return token, nil
}

// ValidateRefreshToken valida um refresh token e retorna os dados do token.
// Retorna erro se expirado, revogado ou não encontrado.
func (s *RefreshTokenService) ValidateRefreshToken(ctx context.Context, db *sql.DB, token string) (*repositories.RefreshToken, error) {
	tokenHash := hashToken(token)

	rt, err := s.repo.FindByTokenHash(ctx, db, tokenHash)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrRefreshTokenNotFound
		}
		return nil, err
	}

	// Verifica se está expirado.
	if time.Now().After(rt.ExpiresAt) {
		return nil, ErrRefreshTokenExpired
	}

	// Verifica se foi revogado.
	if rt.RevokedAt.Valid {
		return nil, ErrRefreshTokenRevoked
	}

	return rt, nil
}

// RevokeToken revoga um refresh token específico.
func (s *RefreshTokenService) RevokeToken(ctx context.Context, db *sql.DB, token string) error {
	tokenHash := hashToken(token)

	rt, err := s.repo.FindByTokenHash(ctx, db, tokenHash)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrRefreshTokenNotFound
		}
		return err
	}

	if err := s.repo.Revoke(ctx, db, rt.ID); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrRefreshTokenRevoked
		}
		return err
	}

	log.Printf("[refresh] token revogado: id=%d usuario_id=%d", rt.ID, rt.UsuarioID)
	return nil
}

// RefreshRotation representa a rotação de um refresh token em andamento: o
// token antigo já foi revogado de forma atômica dentro de uma transação, que
// só é confirmada quando o novo token é inserido (Issue). Se algo falhar
// antes, Rollback desfaz a revogação e o token antigo continua válido.
type RefreshRotation struct {
	svc   *RefreshTokenService
	tx    *sql.Tx
	oldID int64
	done  bool
}

// BeginRotation abre uma transação e revoga o refresh token oldID com
// UPDATE condicional (revoked_at IS NULL). Se nenhuma linha for afetada —
// outra requisição concorrente já revogou o mesmo token — devolve
// ErrRefreshTokenRevoked e nada é emitido. O chamador deve sempre chamar
// Rollback (via defer); após um Issue bem-sucedido ele é no-op.
func (s *RefreshTokenService) BeginRotation(ctx context.Context, db *sql.DB, oldID int64) (*RefreshRotation, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin refresh rotation: %w", err)
	}
	if err := s.repo.Revoke(ctx, tx, oldID); err != nil {
		_ = tx.Rollback()
		if errors.Is(err, repositories.ErrNotFound) {
			log.Printf("[refresh] rotação recusada: token já revogado id=%d", oldID)
			return nil, ErrRefreshTokenRevoked
		}
		return nil, err
	}
	return &RefreshRotation{svc: s, tx: tx, oldID: oldID}, nil
}

// Issue insere o novo refresh token na mesma transação da revogação e
// confirma (commit). Em caso de erro, a transação é desfeita.
func (r *RefreshRotation) Issue(ctx context.Context, usuarioID int64, ipOrigem, userAgent string) (string, error) {
	if r.done {
		return "", errors.New("refresh rotation já finalizada")
	}
	token, err := r.svc.GenerateRefreshToken(ctx, r.tx, usuarioID, ipOrigem, userAgent)
	if err != nil {
		r.Rollback()
		return "", err
	}
	r.done = true
	if err := r.tx.Commit(); err != nil {
		return "", fmt.Errorf("commit refresh rotation: %w", err)
	}
	log.Printf("[refresh] token rotacionado: antigo_id=%d usuario_id=%d", r.oldID, usuarioID)
	return token, nil
}

// Rollback desfaz a rotação (a revogação do token antigo). É no-op se a
// rotação já foi confirmada ou desfeita.
func (r *RefreshRotation) Rollback() {
	if r.done {
		return
	}
	r.done = true
	_ = r.tx.Rollback()
}

// RevokeAllUserTokens revoga todos os refresh tokens de um usuário.
func (s *RefreshTokenService) RevokeAllUserTokens(ctx context.Context, db *sql.DB, usuarioID int64) error {
	if err := s.repo.RevokeAllByUser(ctx, db, usuarioID); err != nil {
		return err
	}
	log.Printf("[refresh] todos tokens revogados: usuario_id=%d", usuarioID)
	return nil
}

// CleanupExpired remove tokens expirados antigos.
func (s *RefreshTokenService) CleanupExpired(ctx context.Context, db *sql.DB) (int64, error) {
	n, err := s.repo.DeleteExpired(ctx, db)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		log.Printf("[refresh] cleanup: %d tokens expirados removidos", n)
	}
	return n, nil
}

// hashToken aplica SHA256 no token para armazenamento.
// Em produção, considere usar HMAC-SHA256 com segredo adicional.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
