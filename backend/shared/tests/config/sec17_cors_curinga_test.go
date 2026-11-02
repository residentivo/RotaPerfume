package config_test

// SEC-17 (Lote 27): CORS_ALLOWED_ORIGINS não aceita "*" nem "null" (sem
// diferenciar maiúsculas, com trim). O CORS responde com Allow-Credentials:
// um curinga liberaria os cookies de sessão para qualquer site. Load()
// devolve config.ErrCORSOrigemInvalida (erro, não panic; cfg nil).

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

func sec17Env(t *testing.T, origens string) {
	t.Helper()
	env := sec11Base()
	env["DB_USUARIO"] = sp(sec11Usuario)
	env["DB_SENHA"] = sp(sec11Senha)
	env["CORS_ALLOWED_ORIGINS"] = sp(origens)
	sec11Env(t, env)
}

func TestSEC17_Load_RecusaCuringaENull(t *testing.T) {
	for _, origens := range []string{
		"*",
		" * ",
		"https://a.com,*",
		"https://a.com, * ,https://b.com",
		"null",
		"NULL",
		" Null ",
		"https://a.com,null",
	} {
		t.Run(origens, func(t *testing.T) {
			sec17Env(t, origens)

			cfg, err := config.Load()
			require.Error(t, err)
			assert.True(t, errors.Is(err, config.ErrCORSOrigemInvalida), "err=%v", err)
			assert.Nil(t, cfg)
		})
	}
}

func TestSEC17_Load_OrigensExplicitasOK(t *testing.T) {
	casos := []struct {
		origens string
		extras  []string
	}{
		{"https://a.com", []string{"https://a.com"}},
		{" https://a.com , https://b.com ", []string{"https://a.com", "https://b.com"}},
		{"", nil},
		// Só a entrada inteira igual a "*"/"null" é recusada.
		{"https://null.example.com", []string{"https://null.example.com"}},
	}
	// SEC-36: as origens de dev só entram com COOKIE_SECURE=false.
	for _, dev := range []bool{false, true} {
		for _, c := range casos {
			t.Run(fmt.Sprintf("dev=%t/%s", dev, c.origens), func(t *testing.T) {
				sec17Env(t, c.origens)
				if dev {
					t.Setenv("COOKIE_SECURE", "false")
				}

				cfg, err := config.Load()
				require.NoError(t, err)
				var want []string
				if dev {
					want = append(want, "http://localhost:3000", "http://127.0.0.1:3000")
				}
				want = append(want, c.extras...)
				assert.ElementsMatch(t, want, cfg.CORSAllowedOrigins)
				if dev {
					assert.Equal(t, want, cfg.CORSAllowedOrigins, "dev primeiro, depois as explícitas")
				}
			})
		}
	}
}

// SEC-36: localhost explícito em CORS_ALLOWED_ORIGINS continua aceito em
// produção (decisão consciente de quem configura).
func TestSEC36_Load_LocalhostExplicitoAceitoEmProducao(t *testing.T) {
	sec17Env(t, "http://localhost:3000")
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.True(t, cfg.CookieSecure)
	assert.Equal(t, []string{"http://localhost:3000"}, cfg.CORSAllowedOrigins)
}
