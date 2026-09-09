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
// O IP e UserAgent são armazenados para auditoria.
func (s *RefreshTokenService) GenerateRefreshToken(ctx context.Context, db *sql.DB, usuarioID int64, ipOrigem, userAgent string) (string, error) {
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
		return err
	}

	log.Printf("[refresh] token revogado: id=%d usuario_id=%d", rt.ID, rt.UsuarioID)
	return nil
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
