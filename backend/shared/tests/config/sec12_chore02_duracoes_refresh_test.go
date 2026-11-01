package config_test

// SEC-12 / CHORE-02 (Lote 12): durações de refresh token lidas pelo Load.
//   - REFRESH_REUSE_SUPPRESS_WINDOW: padrão 30m, faixa [1m, 24h].
//   - REFRESH_CLEANUP_INTERVAL: padrão 6h, "0" desativa, mínimo 1m.
//   - REFRESH_TOKEN_RETENCAO: padrão 720h, faixa [24h, 8760h].
// Valor inválido ou fora da faixa → erro do Load (estilo JWT_TTL), com o
// nome da variável na mensagem.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

type casoDuracao struct {
	nome    string
	raw     *string // nil = ausente
	want    time.Duration
	wantErr bool
}

func rodarCasosDuracao(t *testing.T, chave string, casos []casoDuracao, campo func(*config.Config) time.Duration) {
	t.Helper()
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			env := sec11Base()
			env["DB_USUARIO"] = sp("u")
			env["DB_SENHA"] = sp("s")
			env[chave] = c.raw
			sec11Env(t, env)

			cfg, err := config.Load()
			if c.wantErr {
				require.Error(t, err)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), chave)
				assert.Contains(t, err.Error(), "inválido")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, campo(cfg))
		})
	}
}

func TestSEC12_Load_RefreshReuseSuppressWindow(t *testing.T) {
	rodarCasosDuracao(t, "REFRESH_REUSE_SUPPRESS_WINDOW", []casoDuracao{
		{nome: "ausente → padrão 30m", raw: nil, want: 30 * time.Minute},
		{nome: "vazio → padrão 30m", raw: sp(""), want: 30 * time.Minute},
		{nome: "só espaços → padrão 30m", raw: sp("   "), want: 30 * time.Minute},
		{nome: "mínimo 1m", raw: sp("1m"), want: time.Minute},
		{nome: "máximo 24h", raw: sp("24h"), want: 24 * time.Hour},
		{nome: "45m com espaços", raw: sp(" 45m "), want: 45 * time.Minute},
		{nome: "10s abaixo do mínimo", raw: sp("10s"), wantErr: true},
		{nome: "59s abaixo do mínimo", raw: sp("59s"), wantErr: true},
		{nome: "25h acima do máximo", raw: sp("25h"), wantErr: true},
		{nome: "0 fora da faixa", raw: sp("0"), wantErr: true},
		{nome: "negativo", raw: sp("-5m"), wantErr: true},
		{nome: "abc não é duração", raw: sp("abc"), wantErr: true},
		{nome: "número sem unidade", raw: sp("30"), wantErr: true},
	}, func(c *config.Config) time.Duration { return c.RefreshReuseSuppressWindow })
}

func TestCHORE02_Load_RefreshCleanupInterval(t *testing.T) {
	rodarCasosDuracao(t, "REFRESH_CLEANUP_INTERVAL", []casoDuracao{
		{nome: "ausente → padrão 6h", raw: nil, want: 6 * time.Hour},
		{nome: "vazio → padrão 6h", raw: sp(""), want: 6 * time.Hour},
		{nome: `"0" desativa`, raw: sp("0"), want: 0},
		{nome: `"0s" desativa`, raw: sp("0s"), want: 0},
		{nome: `" 0 " desativa`, raw: sp(" 0 "), want: 0},
		{nome: "mínimo 1m", raw: sp("1m"), want: time.Minute},
		{nome: "72h (sem máximo)", raw: sp("72h"), want: 72 * time.Hour},
		{nome: "30s abaixo do mínimo", raw: sp("30s"), wantErr: true},
		{nome: "negativo", raw: sp("-1h"), wantErr: true},
		{nome: "abc não é duração", raw: sp("abc"), wantErr: true},
	}, func(c *config.Config) time.Duration { return c.RefreshCleanupInterval })
}

func TestCHORE02_Load_RefreshTokenRetencao(t *testing.T) {
	rodarCasosDuracao(t, "REFRESH_TOKEN_RETENCAO", []casoDuracao{
		{nome: "ausente → padrão 720h", raw: nil, want: 720 * time.Hour},
		{nome: "vazio → padrão 720h", raw: sp(""), want: 720 * time.Hour},
		{nome: "mínimo 24h", raw: sp("24h"), want: 24 * time.Hour},
		{nome: "máximo 8760h", raw: sp("8760h"), want: 8760 * time.Hour},
		{nome: "48h", raw: sp("48h"), want: 48 * time.Hour},
		{nome: "1h abaixo do mínimo", raw: sp("1h"), wantErr: true},
		{nome: "23h59m abaixo do mínimo", raw: sp("23h59m"), wantErr: true},
		{nome: "8761h acima do máximo", raw: sp("8761h"), wantErr: true},
		{nome: "0 fora da faixa", raw: sp("0"), wantErr: true},
		{nome: "xyz não é duração", raw: sp("xyz"), wantErr: true},
	}, func(c *config.Config) time.Duration { return c.RefreshTokenRetencao })
}

// Os três padrões juntos, sem nenhuma env de refresh.
func TestSEC12_CHORE02_Load_PadroesJuntos(t *testing.T) {
	env := sec11Base()
	env["DB_USUARIO"] = sp("u")
	env["DB_SENHA"] = sp("s")
	sec11Env(t, env)

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, 30*time.Minute, cfg.RefreshReuseSuppressWindow)
	assert.Equal(t, 6*time.Hour, cfg.RefreshCleanupInterval)
	assert.Equal(t, 720*time.Hour, cfg.RefreshTokenRetencao)
}
