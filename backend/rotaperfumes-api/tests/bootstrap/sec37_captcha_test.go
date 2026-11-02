package bootstrap_test

// SEC-37: com COOKIE_SECURE=true (produção) a API não sobe com secret de
// teste da Cloudflare (bootstrap.ValidarCaptcha, chamado por cmd/server logo
// após config.Load), salvo com a válvula TURNSTILE_PERMITIR_SECRET_TESTE=true,
// que gera log de SEGURANCA. Em dev (COOKIE_SECURE=false) segue o aviso
// atual. config.Load e bootstrap.Montar não validam: CLIs e testes que usam
// a secret de teste com CookieSecure=true continuam funcionando.

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
)

const (
	sec37Secret1x    = "1x0000000000000000000000000000000AA"
	sec37LogTeste    = "[config] Turnstile em modo de teste: hostname/action não conferidos"
	sec37LogProducao = "[config] SEGURANCA: secret de teste do Turnstile em producao (TURNSTILE_PERMITIR_SECRET_TESTE=true): captcha desligado"
)

func sec37Cfg(seguro bool, secret string, permitir bool) *config.Config {
	cfg := testCfg()
	cfg.CookieSecure = seguro
	cfg.TurnstileSecretKey = secret
	cfg.TurnstilePermitirSecretTeste = permitir
	return cfg
}

// sec37MontarELog monta a API (sqlmock) e devolve o log do startup.
func sec37MontarELog(t *testing.T, cfg *config.Config) string {
	t.Helper()
	logs := capturarLog(t)
	pool, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	app := bootstrap.Montar(cfg, pool)
	require.NotNil(t, app.Handler)
	return logs.String()
}

func TestSEC37_ValidarCaptcha(t *testing.T) {
	casos := []struct {
		nome     string
		seguro   bool
		secret   string
		permitir bool
		erro     bool
	}{
		{"produção + 1x: erro", true, sec37Secret1x, false, true},
		{"produção + 2x: erro", true, "2x0000000000000000000000000000000AA", false, true},
		{"produção + 3x: erro", true, "3x0000000000000000000000000000000AA", false, true},
		{"produção + 1x minúsculo: não é secret de teste", true, "1x0000000000000000000000000000000aa", false, false},
		{"produção + site key de teste: não é secret", true, "1x00000000000000000000AA", false, false},
		{"produção + secret real", true, "0x4AAAAAAAsecretRealDeProducao", false, false},
		{"produção + secret vazia", true, "", false, false},
		{"dev + 1x", false, sec37Secret1x, false, false},
		{"produção + 1x + válvula", true, sec37Secret1x, true, false},
		{"dev + 1x + válvula", false, sec37Secret1x, true, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			err := bootstrap.ValidarCaptcha(sec37Cfg(c.seguro, c.secret, c.permitir))
			if !c.erro {
				assert.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, bootstrap.ErrTurnstileSecretTeste)
			assert.NotContains(t, err.Error(), c.secret, "a mensagem nunca traz a secret")
			assert.NotContains(t, err.Error(), "0000000000", "a mensagem nunca traz a secret")
			assert.Contains(t, err.Error(), "TURNSTILE_PERMITIR_SECRET_TESTE=true")
		})
	}
}

// TestSEC37_ValidarCaptcha_TrimViaLoad: " 1x…AA " no ambiente vira 1x…AA
// pelo TrimSpace do config.Load, e o startup recusa.
func TestSEC37_ValidarCaptcha_TrimViaLoad(t *testing.T) {
	sec37EnvValido(t)
	t.Setenv("TURNSTILE_SECRET_KEY", "  "+sec37Secret1x+" \t")
	cfg, err := config.Load()
	require.NoError(t, err, "config.Load não valida o captcha (CLIs)")
	assert.Equal(t, sec37Secret1x, cfg.TurnstileSecretKey)
	assert.True(t, cfg.CookieSecure)
	assert.ErrorIs(t, bootstrap.ValidarCaptcha(cfg), bootstrap.ErrTurnstileSecretTeste)
}

func TestSEC37_LogsDoMontar(t *testing.T) {
	t.Run("dev + 1x: aviso atual, sem log de produção", func(t *testing.T) {
		logs := sec37MontarELog(t, sec37Cfg(false, sec37Secret1x, false))
		assert.Contains(t, logs, sec37LogTeste)
		assert.NotContains(t, logs, "SEGURANCA")
	})
	t.Run("dev + 1x + válvula: aviso atual, sem log de produção", func(t *testing.T) {
		logs := sec37MontarELog(t, sec37Cfg(false, sec37Secret1x, true))
		assert.Contains(t, logs, sec37LogTeste)
		assert.NotContains(t, logs, "SEGURANCA")
	})
	t.Run("produção + 1x + válvula: log de SEGURANCA", func(t *testing.T) {
		logs := sec37MontarELog(t, sec37Cfg(true, sec37Secret1x, true))
		assert.Contains(t, logs, sec37LogProducao)
		assert.NotContains(t, logs, sec37Secret1x)
	})
	t.Run("produção + 1x sem válvula: Montar continua montando (validação só no main)", func(t *testing.T) {
		logs := sec37MontarELog(t, sec37Cfg(true, sec37Secret1x, false))
		assert.Contains(t, logs, sec37LogTeste)
	})
}

// TestSEC37_ApiEnvExampleSemSecretDeTeste: o modelo de produção não sugere
// a secret (nem a site key) de teste e documenta a válvula desligada.
func TestSEC37_ApiEnvExampleSemSecretDeTeste(t *testing.T) {
	raiz, err := cmdutil.FindProjectRoot()
	require.NoError(t, err)
	dados, err := os.ReadFile(filepath.Join(raiz, "deploy", "api.env.example"))
	require.NoError(t, err)
	texto := string(dados)
	assert.NotRegexp(t, regexp.MustCompile(`[123]x0{20,}A{2}`), texto, "nenhuma chave de teste da Cloudflare no modelo de produção")
	assert.Regexp(t, regexp.MustCompile(`(?m)^TURNSTILE_SECRET_KEY=\s*$`), texto)
	assert.Regexp(t, regexp.MustCompile(`(?m)^# TURNSTILE_PERMITIR_SECRET_TESTE=false\s*$`), texto)
	assert.NotRegexp(t, regexp.MustCompile(`(?m)^TURNSTILE_PERMITIR_SECRET_TESTE=`), texto, "a válvula não vem ligada")
}

// sec37EnvValido: ambiente mínimo para config.Load passar.
func sec37EnvValido(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"JWT_SECRET": "segredo-sec37-com-pelo-menos-32-bytes!", "JWT_TTL": "1h",
		"PASSWORD_PEPPER": "pepper-sec37-com-pelo-menos-32-bytes!!", "DB_USUARIO": "u", "DB_SENHA": "s",
		"COOKIE_SECURE": "", "TURNSTILE_PERMITIR_SECRET_TESTE": "", "TURNSTILE_HOSTNAMES": "",
		"DB_TLS_CA": "", "DB_TLS_SERVER_NAME": "", "CORS_ALLOWED_ORIGINS": "", "PRODUTO_IMAGENS_DIR": t.TempDir(),
		"ARGON2_MEMORIA_KIB": "", "ARGON2_ITERACOES": "", "ARGON2_PARALELISMO": "",
		"REFRESH_REUSE_SUPPRESS_WINDOW": "", "REFRESH_CLEANUP_INTERVAL": "", "REFRESH_TOKEN_RETENCAO": "",
	} {
		t.Setenv(k, v)
	}
}
