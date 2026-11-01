package config_test

// SEC-17 (Lote 27): CORS_ALLOWED_ORIGINS não aceita "*" nem "null" (sem
// diferenciar maiúsculas, com trim). O CORS responde com Allow-Credentials:
// um curinga liberaria os cookies de sessão para qualquer site. Load()
// devolve config.ErrCORSOrigemInvalida (erro, não panic; cfg nil).

import (
	"errors"
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
	for _, c := range casos {
		t.Run(c.origens, func(t *testing.T) {
			sec17Env(t, c.origens)

			cfg, err := config.Load()
			require.NoError(t, err)
			want := append([]string{"http://localhost:3000", "http://127.0.0.1:3000"}, c.extras...)
			assert.Equal(t, want, cfg.CORSAllowedOrigins)
		})
	}
}
