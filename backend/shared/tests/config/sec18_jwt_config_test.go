package config_test

// SEC-18 (Lote 24, TestBrain): JWT na config.
//   - JWT_SECRET com menos de config.JWTSecretMinBytes (32) bytes ou igual
//     ao placeholder do .env.example → config.ErrJWTSecretCurto; a mensagem
//     cita JWT_SECRET e nunca o valor. 32 bytes é aceito.
//   - JWT_ISSUER com trim; vazio/só espaços → "rotaperfumes".
//   - JWT_TTL <= 0 → erro "config: JWT_TTL inválido".
//   - Ordem do SEC-11 mantida: os erros de JWT vêm antes do ErrCredenciaisDB,
//     e o LoadSemCredenciaisDB mantém as mesmas validações.

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

const sec18Placeholder = "troque-por-um-segredo-forte-e-aleatorio"

func TestSEC18_Load_JWTSecret(t *testing.T) {
	casos := []struct {
		nome    string
		segredo *string
		wantErr bool
	}{
		{"ausente", nil, true},
		{"vazio", sp(""), true},
		{"1 byte", sp("#"), true},
		{"31 bytes", sp("segredo-sec18-curto-31-bytes-xx"), true},
		{"31 bytes multibyte (16 runas)", sp(strings.Repeat("ã", 15) + "a"), true},
		{"placeholder do .env.example (39 bytes)", sp(sec18Placeholder), true},
		{"32 bytes", sp(strings.Repeat("x", 32)), false},
		{"32 bytes multibyte (16 runas)", sp(strings.Repeat("ã", 16)), false},
		{"64 bytes", sp(strings.Repeat("x", 64)), false},
		{"placeholder com sufixo não é o placeholder", sp(sec18Placeholder + "-1"), false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			env := sec11Base()
			env["DB_USUARIO"] = sp("u")
			env["DB_SENHA"] = sp("s")
			env["JWT_SECRET"] = c.segredo
			sec11Env(t, env)

			cfg, err := config.Load()
			if !c.wantErr {
				require.NoError(t, err)
				assert.Equal(t, *c.segredo, cfg.JWTSecret)
				return
			}
			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.ErrorIs(t, err, config.ErrJWTSecretCurto)
			assert.Contains(t, err.Error(), "JWT_SECRET")
			if c.segredo != nil && *c.segredo != "" {
				assert.NotContains(t, err.Error(), *c.segredo, "a mensagem nunca traz o valor")
			}
		})
	}
	assert.Contains(t, config.ErrJWTSecretCurto.Error(), "32", "a mensagem cita o mínimo")
}

func TestSEC18_Load_JWTIssuer(t *testing.T) {
	casos := []struct {
		nome   string
		issuer *string
		want   string
	}{
		{"ausente → padrão", nil, "rotaperfumes"},
		{"vazio → padrão", sp(""), "rotaperfumes"},
		{"só espaços → padrão", sp("   "), "rotaperfumes"},
		{"só tab/quebra → padrão", sp("\t\n "), "rotaperfumes"},
		{"valor com espaços nas pontas é aparado", sp("  meu-emissor \t"), "meu-emissor"},
		{"valor simples", sp("emissor-x"), "emissor-x"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			env := sec11Base()
			env["DB_USUARIO"] = sp("u")
			env["DB_SENHA"] = sp("s")
			env["JWT_ISSUER"] = c.issuer
			sec11Env(t, env)

			cfg, err := config.Load()
			require.NoError(t, err)
			assert.Equal(t, c.want, cfg.JWTIssuer)
		})
	}
}

func TestSEC18_Load_JWTTTL(t *testing.T) {
	casos := []struct {
		nome    string
		ttl     *string
		wantErr bool
		want    time.Duration
	}{
		{"ausente → 24h", nil, false, 24 * time.Hour},
		{"zero", sp("0"), true, 0},
		{"zero segundos", sp("0s"), true, 0},
		{"negativo -1h", sp("-1h"), true, 0},
		{"negativo -1ns", sp("-1ns"), true, 0},
		{"1ns é positivo", sp("1ns"), false, time.Nanosecond},
		{"15m", sp("15m"), false, 15 * time.Minute},
		{"não é duração", sp("abc"), true, 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			env := sec11Base()
			env["DB_USUARIO"] = sp("u")
			env["DB_SENHA"] = sp("s")
			env["JWT_TTL"] = c.ttl
			sec11Env(t, env)

			cfg, err := config.Load()
			if c.wantErr {
				require.Error(t, err)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), "config: JWT_TTL inválido")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, cfg.JWTTTL)
		})
	}
}

// Sem credenciais do banco e com um erro de JWT/COOKIE_SECURE, o erro de
// JWT/COOKIE_SECURE vem primeiro (SEC-11), tanto no Load quanto no
// LoadSemCredenciaisDB.
func TestSEC18_SEC21_Load_PrecedenciaSobreCredenciaisDB(t *testing.T) {
	casos := []struct {
		nome    string
		env     map[string]*string
		wantErr string
		wantIs  error
	}{
		{"JWT_SECRET de 31 bytes", map[string]*string{"JWT_SECRET": sp(strings.Repeat("x", 31))}, "JWT_SECRET", config.ErrJWTSecretCurto},
		{"JWT_SECRET placeholder", map[string]*string{"JWT_SECRET": sp(sec18Placeholder)}, "JWT_SECRET", config.ErrJWTSecretCurto},
		{"JWT_TTL zero", map[string]*string{"JWT_TTL": sp("0")}, "JWT_TTL", nil},
		{"JWT_TTL negativo", map[string]*string{"JWT_TTL": sp("-1h")}, "JWT_TTL", nil},
		{"COOKIE_SECURE inválido", map[string]*string{"COOKIE_SECURE": sp("abc")}, "COOKIE_SECURE", nil},
	}
	carregadores := []struct {
		nome string
		load func() (*config.Config, error)
	}{
		{"Load", config.Load},
		{"LoadSemCredenciaisDB", config.LoadSemCredenciaisDB},
	}
	for _, ld := range carregadores {
		for _, c := range casos {
			t.Run(ld.nome+"/"+c.nome, func(t *testing.T) {
				env := sec11Base()
				env["DB_USUARIO"] = nil
				env["DB_SENHA"] = nil
				for k, v := range c.env {
					env[k] = v
				}
				sec11Env(t, env)

				cfg, err := ld.load()
				require.Error(t, err)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), c.wantErr)
				assert.False(t, errors.Is(err, config.ErrCredenciaisDB), "o erro de credenciais só vem por último")
				if c.wantIs != nil {
					assert.ErrorIs(t, err, c.wantIs)
				}
			})
		}
	}
}
