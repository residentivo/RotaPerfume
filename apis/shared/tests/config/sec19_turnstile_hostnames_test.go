package config_test

// SEC-19 (Lote 28): TURNSTILE_HOSTNAMES são os hostnames aceitos na
// resposta do siteverify. Ausente ou vazia, a lista é derivada dos
// hostnames de CORS_ALLOWED_ORIGINS (minúsculas, sem repetição); definida,
// cada item passa por trim e minúsculas e um item inválido derruba o Load
// com config.ErrTurnstileHostnames, sem o valor na mensagem.

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

func sec19Env(t *testing.T, hostnames *string, origens string) {
	t.Helper()
	env := sec11Base()
	env["DB_USUARIO"] = sp(sec11Usuario)
	env["DB_SENHA"] = sp(sec11Senha)
	env["CORS_ALLOWED_ORIGINS"] = sp(origens)
	env["TURNSTILE_HOSTNAMES"] = hostnames
	sec11Env(t, env)
}

func TestSEC19_Load_HostnamesDerivadosDoCORS(t *testing.T) {
	casos := []struct {
		nome      string
		hostnames *string
		origens   string
		want      []string
	}{
		{"ausente, só as origens de dev", nil, "", []string{"localhost", "127.0.0.1"}},
		{"vazia conta como ausente", sp(""), "", []string{"localhost", "127.0.0.1"}},
		{"só espaços conta como ausente", sp("   "), "", []string{"localhost", "127.0.0.1"}},
		{
			"origens com porta, maiúsculas e repetição", nil,
			"https://Ivo-Inspiron-15-3530:8443,https://192.168.168.106:8443,https://ivo-inspiron-15-3530",
			[]string{"localhost", "127.0.0.1", "ivo-inspiron-15-3530", "192.168.168.106"},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			sec19Env(t, c.hostnames, c.origens)

			cfg, err := config.Load()
			require.NoError(t, err)
			assert.Equal(t, c.want, cfg.TurnstileHostnames)
		})
	}
}

func TestSEC19_Load_HostnamesExplicitosNormalizados(t *testing.T) {
	sec19Env(t, sp(" Ivo-Inspiron-15-3530 ,192.168.168.106,IVO-INSPIRON-15-3530"), "https://outro.example.com")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, []string{"ivo-inspiron-15-3530", "192.168.168.106"}, cfg.TurnstileHostnames,
		"a env substitui a derivação do CORS")
}

func TestSEC19_Load_HostnamesInvalidosDerrubamASubida(t *testing.T) {
	for _, raw := range []string{
		"https://segredo-host.example.com",
		"segredo-host.example.com:8443",
		"segredo-host.example.com/login",
		"*.segredo-host.example.com",
		"segredo host.example.com",
		"segredo-host.example.com,,outro",
		"segredo-host.example.com,",
		",segredo-host.example.com",
		"segredo-host\t.example.com",
	} {
		t.Run(raw, func(t *testing.T) {
			sec19Env(t, sp(raw), "")

			cfg, err := config.Load()
			require.Error(t, err)
			assert.True(t, errors.Is(err, config.ErrTurnstileHostnames), "err=%v", err)
			assert.Contains(t, err.Error(), "TURNSTILE_HOSTNAMES inválida")
			assert.NotContains(t, err.Error(), "segredo-host", "a mensagem não leva o valor")
			assert.Nil(t, cfg)
		})
	}
}
