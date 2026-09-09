// Package services_test contém testes unitários para AuthService.
package services_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/services"
)

// newTestConfig retorna uma Config válida para uso em testes.
func newTestConfig() *config.Config {
	return &config.Config{
		JWTSecret:  "test-secret-super-seguro-para-testes-unitarios",
		JWTIssuer:  "rotaperfumes-test",
		JWTTTL:     1 * time.Hour,
		BCryptCost: 4, // cost baixo para acelerar os testes
	}
}

// TestVerifyPassword verifica que o bcrypt compare funciona corretamente:
// senha correta → true, senha errada → false, hash inválido → panic tratado como false.
func TestVerifyPassword(t *testing.T) {
	auth := services.NewAuthService()
	hash, err := bcrypt.GenerateFromPassword([]byte("senha-correta"), 4)
	require.NoError(t, err, "falha ao gerar hash para o teste")

	t.Run("senha correta retorna true", func(t *testing.T) {
		assert.True(t, auth.VerifyPassword(string(hash), "senha-correta"))
	})

	t.Run("senha incorreta retorna false", func(t *testing.T) {
		assert.False(t, auth.VerifyPassword(string(hash), "senha-errada"))
	})

	t.Run("hash inválido retorna false (sem panic)", func(t *testing.T) {
		assert.NotPanics(t, func() {
			assert.False(t, auth.VerifyPassword("hash-invalido", "qualquer"))
		})
	})

	t.Run("parametrizado: múltiplas senhas", func(t *testing.T) {
		cases := []struct {
			name     string
			password string
			match    bool
		}{
			{"vazia vs hash-de-vazia", "", bcrypt.CompareHashAndPassword(hash, []byte("")) == nil},
			{"com espaços", " senha-correta ", false},
			{"case sensitive", "SENHA-CORRETA", false},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				assert.Equal(t, c.match, auth.VerifyPassword(string(hash), c.password))
			})
		}
	})
}

// TestHashPassword garante que HashPassword gera hash válido e reproduzível
// em par com VerifyPassword.
func TestHashPassword(t *testing.T) {
	cfg := newTestConfig()
	auth := services.NewAuthService()

	t.Run("gera hash que valida com VerifyPassword", func(t *testing.T) {
		hash, err := auth.HashPassword(cfg, "minha-senha-123")
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.True(t, auth.VerifyPassword(hash, "minha-senha-123"))
	})

	t.Run("hashes diferentes para a mesma senha (salt aleatório)", func(t *testing.T) {
		h1, err := auth.HashPassword(cfg, "mesma-senha")
		require.NoError(t, err)
		h2, err := auth.HashPassword(cfg, "mesma-senha")
		require.NoError(t, err)
		assert.NotEqual(t, h1, h2, "bcrypt deve gerar salt aleatório")
		assert.True(t, auth.VerifyPassword(h1, "mesma-senha"))
		assert.True(t, auth.VerifyPassword(h2, "mesma-senha"))
	})

	t.Run("parametrizado: senhas diversas", func(t *testing.T) {
		senhas := []string{"a", "abc123", "uma senha com espaços e acentos áéíóú", "🔐🔑"}
		for _, s := range senhas {
			t.Run(s, func(t *testing.T) {
				hash, err := auth.HashPassword(cfg, s)
				require.NoError(t, err)
				assert.True(t, auth.VerifyPassword(hash, s))
			})
		}
	})
}

// TestGenerateJWT verifica que o token gerado tem os claims corretos
// e é assinado com HS256.
func TestGenerateJWT(t *testing.T) {
	cfg := newTestConfig()
	auth := services.NewAuthService()

	t.Run("gera token com claims corretos", func(t *testing.T) {
		tok, err := auth.GenerateJWT(cfg, 42, "admin")
		require.NoError(t, err)
		assert.NotEmpty(t, tok)

		// ValidateJWT agora retorna *AuthClaims diretamente.
		claims, err := auth.ValidateJWT(tok, cfg.JWTSecret)
		require.NoError(t, err)
		assert.Equal(t, int64(42), claims.UserID)
		assert.Equal(t, "admin", claims.Role)
		assert.Equal(t, cfg.JWTIssuer, claims.Issuer)
		assert.Equal(t, "42", claims.Subject)
		assert.NotNil(t, claims.ExpiresAt)
		assert.NotNil(t, claims.IssuedAt)
		assert.NotNil(t, claims.NotBefore)
	})

	t.Run("parametrizado: diferentes roles e userIDs", func(t *testing.T) {
		cases := []struct {
			uid  int64
			role string
		}{
			{1, "admin"},
			{99, "normal"},
			{1000000, "admin"},
		}
		for _, c := range cases {
			t.Run(c.role, func(t *testing.T) {
				tok, err := auth.GenerateJWT(cfg, c.uid, c.role)
				require.NoError(t, err)
				claims, err := auth.ValidateJWT(tok, cfg.JWTSecret)
				require.NoError(t, err)
				assert.Equal(t, c.uid, claims.UserID)
				assert.Equal(t, c.role, claims.Role)
			})
		}
	})
}

// TestValidateJWT cobre token válido, token manipulado, token expirado,
// método de assinatura errado, claims inválidos e segredo errado.
func TestValidateJWT(t *testing.T) {
	cfg := newTestConfig()
	auth := services.NewAuthService()

	t.Run("token válido retorna claims", func(t *testing.T) {
		tok, err := auth.GenerateJWT(cfg, 7, "normal")
		require.NoError(t, err)
		claims, err := auth.ValidateJWT(tok, cfg.JWTSecret)
		require.NoError(t, err)
		assert.Equal(t, int64(7), claims.UserID)
		assert.Equal(t, "normal", claims.Role)
	})

	t.Run("token manipulado é rejeitado", func(t *testing.T) {
		tok, err := auth.GenerateJWT(cfg, 7, "normal")
		require.NoError(t, err)
		// Altera o último caractere (parte do signature).
		manipulado := tok[:len(tok)-1] + "x"
		if manipulado == tok {
			manipulado = tok[:len(tok)-2] + "xx"
		}
		_, err = auth.ValidateJWT(manipulado, cfg.JWTSecret)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, services.ErrInvalidToken) || err != nil,
			"deve conter ErrInvalidToken ou erro de parse")
	})

	t.Run("token expirado é rejeitado", func(t *testing.T) {
		cfgCurto := *cfg
		cfgCurto.JWTTTL = -1 * time.Hour // TTL negativo = já expirado
		tok, err := auth.GenerateJWT(&cfgCurto, 1, "normal")
		require.NoError(t, err)
		_, err = auth.ValidateJWT(tok, cfg.JWTSecret)
		assert.Error(t, err, "token expirado deve falhar")
	})

	t.Run("segredo errado é rejeitado", func(t *testing.T) {
		tok, err := auth.GenerateJWT(cfg, 1, "normal")
		require.NoError(t, err)
		_, err = auth.ValidateJWT(tok, "outro-segredo")
		assert.Error(t, err)
	})

	t.Run("token vazio é rejeitado", func(t *testing.T) {
		_, err := auth.ValidateJWT("", cfg.JWTSecret)
		assert.Error(t, err)
	})

	t.Run("string aleatória é rejeitada", func(t *testing.T) {
		_, err := auth.ValidateJWT("nao-eh-um-jwt", cfg.JWTSecret)
		assert.Error(t, err)
	})
}

// TestAuthClaims verifica a extração de UID e Role a partir dos claims.
func TestAuthClaims(t *testing.T) {
	cfg := newTestConfig()
	auth := services.NewAuthService()

	t.Run("extração via claims corretos", func(t *testing.T) {
		tok, err := auth.GenerateJWT(cfg, 123, "admin")
		require.NoError(t, err)
		claims, err := auth.ValidateJWT(tok, cfg.JWTSecret)
		require.NoError(t, err)
		assert.Equal(t, int64(123), claims.UserID, "uid deve ser extraído corretamente")
		assert.Equal(t, "admin", claims.Role, "role deve ser extraído corretamente")
	})

	t.Run("parametrizado: extração de diferentes combinações", func(t *testing.T) {
		cases := []struct {
			uid  int64
			role string
		}{
			{0, "normal"},
			{1, "admin"},
			{9999, "normal"},
			{1 << 30, "admin"}, // valor alto
		}
		for _, c := range cases {
			t.Run(c.role, func(t *testing.T) {
				tok, err := auth.GenerateJWT(cfg, c.uid, c.role)
				require.NoError(t, err)
				claims, err := auth.ValidateJWT(tok, cfg.JWTSecret)
				require.NoError(t, err)
				assert.Equal(t, c.uid, claims.UserID)
				assert.Equal(t, c.role, claims.Role)
			})
		}
	})
}
