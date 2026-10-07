// Package services expõe funções de negócio stateless (sem dependência de HTTP).
//
// AuthService agrupa as operações de autenticação: busca de usuário,
// verificação de senha, geração/validação de JWT e reset de senha.
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
)

// Erros semânticos para a camada de auth.
var (
	ErrInvalidCredentials = errors.New("auth: credenciais inválidas")
	ErrUserInactive       = errors.New("auth: usuário inativo")
	ErrInvalidToken       = errors.New("auth: token inválido")
)

// AuthClaims é o payload JWT da aplicação.
type AuthClaims struct {
	UserID int64  `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// AuthService agrega as operações de autenticação.
type AuthService struct {
	repo *repositories.UsuarioRepository
}

// NewAuthService cria um AuthService com seu repositório.
func NewAuthService() *AuthService {
	return &AuthService{
		repo: repositories.NewUsuarioRepository(),
	}
}

// GetUsuarioByEmail busca um usuário por email.
func (s *AuthService) GetUsuarioByEmail(ctx context.Context, db *sql.DB, email string) (*models.Usuario, error) {
	return s.repo.GetByEmail(ctx, db, email)
}

// VerifyPassword confere a senha contra o hash (Argon2id com pepper ou bcrypt
// legado). Ver VerificarSenha.
func (s *AuthService) VerifyPassword(cfg *config.Config, hash, password string) bool {
	return VerificarSenha(cfg.HashSenha, hash, password)
}

// NeedsRehash indica se o hash deve ser regravado com Argon2id e os parâmetros
// atuais (bcrypt legado ou parâmetros antigos). Ver PrecisaRehash.
func (s *AuthService) NeedsRehash(cfg *config.Config, hash string) bool {
	return PrecisaRehash(cfg.HashSenha, hash)
}

// VerifyDummyPassword roda um Argon2id descartável com os parâmetros atuais.
// O login chama quando o e-mail não existe, para que o tempo de resposta não
// revele quais contas existem.
func (s *AuthService) VerifyDummyPassword(cfg *config.Config, password string) {
	_, _ = GerarHashSenha(cfg.HashSenha, password)
}

// GenerateJWT gera um token JWT assinado com HS256.
func (s *AuthService) GenerateJWT(cfg *config.Config, userID int64, role string) (string, error) {
	now := time.Now()
	claims := AuthClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.JWTIssuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.JWTTTL)),
			NotBefore: jwt.NewNumericDate(now),
			Subject:   fmt.Sprintf("%d", userID),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}

// ValidateJWT valida um token e retorna os claims como *AuthClaims.
func (s *AuthService) ValidateJWT(tokenString, secret string) (*AuthClaims, error) {
	claims := &AuthClaims{}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"HS256"}))
	tok, err := parser.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		return []byte(secret), nil
	})
	if err != nil || !tok.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// ResetPassword atualiza o hash de um usuário. deveTrocarSenha indica
// se o usuário deve ser forçado a trocar a senha no próximo login.
func (s *AuthService) ResetPassword(ctx context.Context, db *sql.DB, userID int64, newHash string, deveTrocarSenha bool) error {
	return s.repo.UpdatePasswordHash(ctx, db, userID, newHash, deveTrocarSenha)
}

// RehashPassword troca o hash pelo novo só se ainda for oldHash, sem mexer
// em deve_trocar_senha nem no corte de sessão (migração transparente do
// bcrypt para Argon2id no login, SEC-13).
func (s *AuthService) RehashPassword(ctx context.Context, db *sql.DB, userID int64, oldHash, newHash string) error {
	return s.repo.RehashPassword(ctx, db, userID, oldHash, newHash)
}

// HashPassword gera o hash Argon2id (com pepper) da senha. Ver GerarHashSenha.
func (s *AuthService) HashPassword(cfg *config.Config, password string) (string, error) {
	hash, err := GerarHashSenha(cfg.HashSenha, password)
	if err != nil {
		return "", fmt.Errorf("auth: hash falhou: %w", err)
	}
	return hash, nil
}
