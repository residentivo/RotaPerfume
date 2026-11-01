package config_test

// SEC-21 (Lote 24, TestBrain): COOKIE_SECURE.
//   - Ausente, vazio ou só espaços → true (padrão seguro).
//   - strconv.ParseBool com trim: "false"/"0"/"f"/"FALSE" → false;
//     "true"/"TRUE"/"1"/"t" → true.
//   - Qualquer outro valor derruba a subida com
//     `config: COOKIE_SECURE inválido (%q): esperado true ou false`.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

func TestSEC21_Load_CookieSecure(t *testing.T) {
	casos := []struct {
		nome    string
		valor   *string
		want    bool
		wantErr string
	}{
		{"ausente → true", nil, true, ""},
		{"vazio → true", sp(""), true, ""},
		{"só espaços → true", sp("  \t "), true, ""},
		{"false", sp("false"), false, ""},
		{"FALSE", sp("FALSE"), false, ""},
		{"False", sp("False"), false, ""},
		{"0", sp("0"), false, ""},
		{"f", sp("f"), false, ""},
		{"false com espaços", sp("  false "), false, ""},
		{"true", sp("true"), true, ""},
		{"TRUE", sp("TRUE"), true, ""},
		{"1", sp("1"), true, ""},
		{"t", sp("t"), true, ""},
		{"abc", sp("abc"), false, `config: COOKIE_SECURE inválido ("abc"): esperado true ou false`},
		{"sim", sp("sim"), false, `config: COOKIE_SECURE inválido ("sim"): esperado true ou false`},
		{"no", sp("no"), false, `config: COOKIE_SECURE inválido ("no"): esperado true ou false`},
		{"2", sp("2"), false, `config: COOKIE_SECURE inválido ("2"): esperado true ou false`},
		{"yes com espaços (aparado na mensagem)", sp(" yes "), false, `config: COOKIE_SECURE inválido ("yes"): esperado true ou false`},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			env := sec11Base()
			env["DB_USUARIO"] = sp("u")
			env["DB_SENHA"] = sp("s")
			env["COOKIE_SECURE"] = c.valor
			sec11Env(t, env)

			cfg, err := config.Load()
			if c.wantErr != "" {
				require.Error(t, err)
				assert.Nil(t, cfg)
				assert.EqualError(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, cfg.CookieSecure)
		})
	}
}

// O LoadSemCredenciaisDB também lê COOKIE_SECURE (mesma função de carga).
func TestSEC21_LoadSemCredenciaisDB_CookieSecure(t *testing.T) {
	for _, c := range []struct {
		valor *string
		want  bool
	}{{nil, true}, {sp("false"), false}} {
		env := sec11Base()
		env["DB_USUARIO"] = nil
		env["DB_SENHA"] = nil
		env["COOKIE_SECURE"] = c.valor
		sec11Env(t, env)
		cfg, err := config.LoadSemCredenciaisDB()
		require.NoError(t, err)
		assert.Equal(t, c.want, cfg.CookieSecure)
	}
}
