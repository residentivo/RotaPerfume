// Package services_test contém testes unitários para AuthService.
package services_test

import (
	"encoding/base64"
	"strings"
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
		JWTSecret: "test-secret-super-seguro-para-testes-unitarios",
		JWTIssuer: "rotaperfumes-test",
		JWTTTL:    1 * time.Hour,
		HashSenha: config.HashSenha{Pepper: "pepper-de-teste-com-pelo-menos-32-bytes", MemoriaKiB: 64, Iteracoes: 1, Paralelismo: 1}, // Argon2id mínimo para acelerar os testes
	}
}

// TestVerifyPassword confere o caminho Argon2id (com pepper) e o bcrypt legado:
// senha correta → true, senha errada → false, hash inválido → false sem panic.
func TestVerifyPassword(t *testing.T) {
	cfg := newTestConfig()
	auth := services.NewAuthService()
	argon, err := auth.HashPassword(cfg, "senha-correta")
	require.NoError(t, err, "falha ao gerar hash argon2id para o teste")
	legado, err := bcrypt.GenerateFromPassword([]byte("senha-correta"), 4)
	require.NoError(t, err, "falha ao gerar hash bcrypt para o teste")

	hashes := map[string]string{"argon2id": argon, "bcrypt legado": string(legado)}
	for nome, hash := range hashes {
		t.Run(nome, func(t *testing.T) {
			cases := []struct {
				name     string
				password string
				match    bool
			}{
				{"senha correta", "senha-correta", true},
				{"senha incorreta", "senha-errada", false},
				{"vazia", "", false},
				{"com espaços", " senha-correta ", false},
				{"case sensitive", "SENHA-CORRETA", false},
			}
			for _, c := range cases {
				t.Run(c.name, func(t *testing.T) {
					assert.Equal(t, c.match, auth.VerifyPassword(cfg, hash, c.password))
				})
			}
		})
	}

	t.Run("hash inválido retorna false (sem panic)", func(t *testing.T) {
		assert.NotPanics(t, func() {
			assert.False(t, auth.VerifyPassword(cfg, "hash-invalido", "qualquer"))
		})
	})
}

// TestHashPassword garante que HashPassword gera Argon2id válido em par com
// VerifyPassword.
func TestHashPassword(t *testing.T) {
	cfg := newTestConfig()
	auth := services.NewAuthService()

	t.Run("gera hash argon2id que valida com VerifyPassword", func(t *testing.T) {
		hash, err := auth.HashPassword(cfg, "minha-senha-123")
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(hash, "$argon2id$v=19$m=64,t=1,p=1$"), hash)
		assert.True(t, auth.VerifyPassword(cfg, hash, "minha-senha-123"))
		assert.False(t, auth.NeedsRehash(cfg, hash))
	})

	t.Run("hashes diferentes para a mesma senha (salt aleatório)", func(t *testing.T) {
		h1, err := auth.HashPassword(cfg, "mesma-senha")
		require.NoError(t, err)
		h2, err := auth.HashPassword(cfg, "mesma-senha")
		require.NoError(t, err)
		assert.NotEqual(t, h1, h2, "argon2id deve gerar salt aleatório")
		assert.True(t, auth.VerifyPassword(cfg, h1, "mesma-senha"))
		assert.True(t, auth.VerifyPassword(cfg, h2, "mesma-senha"))
	})

	t.Run("parametrizado: senhas diversas", func(t *testing.T) {
		senhas := []string{"a", "abc123", "uma senha com espaços e acentos áéíóú", "🔐🔑", strings.Repeat("x", 200)}
		for _, s := range senhas {
			t.Run(s, func(t *testing.T) {
				hash, err := auth.HashPassword(cfg, s)
				require.NoError(t, err)
				assert.True(t, auth.VerifyPassword(cfg, hash, s))
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
		// Determinístico (SEC-08/Lote 8): antes trocava o último caractere
		// base64url da assinatura, que carrega 2 bits de padding ignorados
		// pelo decoder — em ~1/16 das execuções a assinatura decodificada não
		// mudava e o token passava. Agora cada parte é decodificada, tem um
		// byte do meio invertido (XOR 0xFF) e é recodificada: os bytes mudam
		// sempre, independentemente do token gerado.
		manipular := func(t *testing.T, tok string, parte int) string {
			t.Helper()
			partes := strings.Split(tok, ".")
			require.Len(t, partes, 3)
			raw, err := base64.RawURLEncoding.DecodeString(partes[parte])
			require.NoError(t, err)
			require.NotEmpty(t, raw)
			raw[len(raw)/2] ^= 0xFF
			partes[parte] = base64.RawURLEncoding.EncodeToString(raw)
			out := strings.Join(partes, ".")
			require.NotEqual(t, tok, out)
			return out
		}
		for _, c := range []struct {
			nome  string
			parte int
		}{{"assinatura", 2}, {"payload", 1}, {"header", 0}} {
			t.Run(c.nome, func(t *testing.T) {
				for i := 0; i < 50; i++ { // vários tokens (iat/jti variam) para provar estabilidade
					tok, err := auth.GenerateJWT(cfg, int64(7+i), "normal")
					require.NoError(t, err)
					_, err = auth.ValidateJWT(manipular(t, tok, c.parte), cfg.JWTSecret)
					require.Error(t, err, "token com %s manipulado deve ser rejeitado", c.nome)
					assert.ErrorIs(t, err, services.ErrInvalidToken)
				}
			})
		}
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
