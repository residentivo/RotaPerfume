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
	"golang.org/x/crypto/bcrypt"

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

// VerifyPassword compara hash bcrypt com senha em texto puro.
func (s *AuthService) VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
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

// ResetPassword atualiza o hash bcrypt de um usuário. deveTrocarSenha indica
// se o usuário deve ser forçado a trocar a senha no próximo login.
func (s *AuthService) ResetPassword(ctx context.Context, db *sql.DB, userID int64, newHash string, deveTrocarSenha bool) error {
	return s.repo.UpdatePasswordHash(ctx, db, userID, newHash, deveTrocarSenha)
}

// HashPassword gera um hash bcrypt com o cost configurado.
func (s *AuthService) HashPassword(cfg *config.Config, password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cfg.BCryptCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash falhou: %w", err)
	}
	return string(hash), nil
}
