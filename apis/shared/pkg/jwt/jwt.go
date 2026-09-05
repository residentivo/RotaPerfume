// Package jwt provides JSON Web Token generation and validation.
package jwt

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"rotaperfumes/shared/pkg/config"
)

// Claims represents the JWT claims structure.
type Claims struct {
	UserID int64  `json:"sub"`
	Login  string `json:"login"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// JWTService handles token generation and validation.
type JWTService struct {
	secret []byte
	ttl    time.Duration
	issuer string
}

// NewJWTService creates a new JWT service instance.
func NewJWTService(cfg *config.Config) (*JWTService, error) {
	secret := []byte(cfg.JWTSecret)
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters long")
	}

	ttl, err := parseDuration(cfg.JWTTTL)
	if err != nil {
		ttl = 24 * time.Hour
	}

	return &JWTService{
		secret: secret,
		ttl:    ttl,
		issuer: cfg.JWTIssuer,
	}, nil
}

// GenerateToken creates a new JWT token for the given user.
func (s *JWTService) GenerateToken(userID int64, login, role string) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.ttl)

	claims := &Claims{
		UserID: userID,
		Login:  login,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    s.issuer,
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// ValidateToken validates a JWT token and returns its claims.
func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

// parseDuration parses duration strings like "24h", "1h30m", etc.
func parseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}
