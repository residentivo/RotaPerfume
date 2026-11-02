package config_test

// SEC-37: TURNSTILE_SECRET_KEY passa por TrimSpace e a válvula
// TURNSTILE_PERMITIR_SECRET_TESTE é lida com ParseBool estrito (ausente =
// false; inválida derruba o Load). O Load NÃO recusa a secret de teste com
// COOKIE_SECURE=true: quem recusa é o startup da API (bootstrap.ValidarCaptcha),
// para as CLIs e os testes continuarem funcionando.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

const sec37Secret1x = "1x0000000000000000000000000000000AA"

func sec37Env(t *testing.T, secret, permitir *string) {
	t.Helper()
	env := sec11Base()
	env["DB_USUARIO"] = sp("u")
	env["DB_SENHA"] = sp("p")
	env["TURNSTILE_SECRET_KEY"] = secret
	env["TURNSTILE_PERMITIR_SECRET_TESTE"] = permitir
	sec11Env(t, env)
}

func TestSEC37_PermitirSecretTeste_Leitura(t *testing.T) {
	casos := []struct {
		nome     string
		valor    *string
		esperado bool
	}{
		{"ausente", nil, false},
		{"vazio", sp(""), false},
		{"só espaços", sp("  "), false},
		{"true", sp("true"), true},
		{"TRUE com espaços", sp(" TRUE "), true},
		{"1", sp("1"), true},
		{"false", sp("false"), false},
		{"0", sp("0"), false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			sec37Env(t, sp(sec37Secret1x), c.valor)
			cfg, err := config.Load()
			require.NoError(t, err)
			assert.Equal(t, c.esperado, cfg.TurnstilePermitirSecretTeste)
		})
	}
}

func TestSEC37_PermitirSecretTeste_InvalidoDerrubaLoad(t *testing.T) {
	for _, v := range []string{"sim", "2", "yes", "on"} {
		t.Run(v, func(t *testing.T) {
			sec37Env(t, sp(sec37Secret1x), sp(v))
			cfg, err := config.Load()
			require.Error(t, err)
			assert.Nil(t, cfg)
			assert.Equal(t, `config: TURNSTILE_PERMITIR_SECRET_TESTE inválido ("`+v+`"): esperado true ou false`, err.Error())

			_, err = config.LoadSemCredenciaisDB()
			assert.Error(t, err)
		})
	}
}

// TestSEC37_LoadAceitaSecretDeTesteEmProducao: as CLIs (Load e
// LoadSemCredenciaisDB) não quebram com COOKIE_SECURE=true + secret de teste.
func TestSEC37_LoadAceitaSecretDeTesteEmProducao(t *testing.T) {
	sec37Env(t, sp(" "+sec37Secret1x+"\t"), nil)
	cfg, err := config.Load()
	require.NoError(t, err)
	assert.True(t, cfg.CookieSecure)
	assert.Equal(t, sec37Secret1x, cfg.TurnstileSecretKey, "TrimSpace na secret")
	assert.False(t, cfg.TurnstilePermitirSecretTeste)

	cfg, err = config.LoadSemCredenciaisDB()
	require.NoError(t, err)
	assert.Equal(t, sec37Secret1x, cfg.TurnstileSecretKey)
}
