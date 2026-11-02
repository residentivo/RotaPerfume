package config_test

// SEC-42: DB_TLS_REQUIRED (padrão true). Com DB_HOST fora do loopback literal
// (localhost, 127.0.0.0/8, ::1; sem DNS) e DB_TLS_CA vazio, o Load falha com
// ErrDBTLSObrigatorio. DB_TLS_REQUIRED=false explícito libera com log AVISO.
// Só true/false são aceitos; outro valor é erro (ErrDBTLSRequired), mesmo com
// host loopback. TST-12: Descricao() informa min=TLS1.3.

import (
	"bytes"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

// sec42Env: ambiente válido com DB_HOST/DB_TLS_CA/DB_TLS_REQUIRED do caso
// (nil = ausente).
func sec42Env(t *testing.T, host, ca, required *string) {
	t.Helper()
	env := sec11Base()
	env["DB_USUARIO"] = sp("u")
	env["DB_SENHA"] = sp("segredo-sec42")
	env["DB_HOST"] = host
	env["DB_TLS_CA"] = ca
	env["DB_TLS_REQUIRED"] = required
	sec11Env(t, env)
}

// sec42CapturarLog redireciona o log padrão durante o teste.
func sec42CapturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	saida := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(saida) })
	return &buf
}

func TestSEC42_HostLoopback(t *testing.T) {
	casos := []struct {
		host     string
		loopback bool
	}{
		{"localhost", true},
		{"LOCALHOST", true},
		{" localhost ", true},
		{"127.0.0.1", true},
		{"127.1.2.3", true},
		{"127.255.255.254", true},
		{"::1", true},
		{"[::1]", true},
		{"192.168.168.106", false},
		{"10.0.0.1", false},
		{"0.0.0.0", false},
		{"::", false},
		{"rotaperfumes-mariadb", false},
		{"host.docker.internal", false},
		{"localhost.localdomain", false},
		{"localhost.", false},
		{"127.0.0.1.nip.io", false},
		{"mariadb.lan", false},
		{"", false},
	}
	for _, c := range casos {
		t.Run(c.host, func(t *testing.T) {
			assert.Equal(t, c.loopback, config.HostLoopback(c.host))
		})
	}
}

func TestSEC42_HostRemotoSemCA_Erro(t *testing.T) {
	for _, host := range []string{"192.168.168.106", "rotaperfumes-mariadb", "host.docker.internal", "::ffff:10.0.0.1"} {
		for nome, required := range map[string]*string{"padrão (ausente)": nil, "vazio": sp(""), "true": sp("true"), "true com espaços": sp(" true ")} {
			t.Run(host+"/"+nome, func(t *testing.T) {
				sec42Env(t, sp(host), nil, required)
				cfg, err := config.Load()
				require.ErrorIs(t, err, config.ErrDBTLSObrigatorio)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), "DB_TLS_CA")
				assert.Contains(t, err.Error(), "DB_TLS_REQUIRED=false")
				assert.Contains(t, err.Error(), host)
				assert.NotContains(t, err.Error(), "segredo-sec42", "a senha nunca vai para a mensagem")

				_, err = config.LoadSemCredenciaisDB()
				assert.ErrorIs(t, err, config.ErrDBTLSObrigatorio, "vale também para as ferramentas sem credenciais")
			})
		}
	}
}

func TestSEC42_HostLoopbackSemCA_OK(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "127.0.0.2", "::1"} {
		t.Run(host, func(t *testing.T) {
			buf := sec42CapturarLog(t)
			sec42Env(t, sp(host), nil, nil)
			cfg, err := config.Load()
			require.NoError(t, err)
			assert.False(t, cfg.DBTLS.Ativo())
			assert.NotContains(t, buf.String(), "AVISO")
		})
	}
	t.Run("DB_HOST ausente (padrão localhost)", func(t *testing.T) {
		sec42Env(t, nil, nil, nil)
		cfg, err := config.Load()
		require.NoError(t, err)
		assert.Equal(t, "localhost", cfg.DBHost)
	})
}

func TestSEC42_HostRemotoComCA_OK(t *testing.T) {
	ca := sec29PEMDaCA(t)
	for nome, required := range map[string]*string{"padrão": nil, "true": sp("true"), "false": sp("false")} {
		t.Run(nome, func(t *testing.T) {
			buf := sec42CapturarLog(t)
			sec42Env(t, sp("rotaperfumes-mariadb"), sp(ca), required)
			cfg, err := config.Load()
			require.NoError(t, err)
			assert.True(t, cfg.DBTLS.Ativo(), "DB_TLS_REQUIRED=false não desliga o TLS configurado")
			assert.Contains(t, cfg.DSN(), "&tls=custom")
			assert.NotContains(t, buf.String(), "AVISO")
		})
	}
}

func TestSEC42_FalseExplicito_LiberaComAviso(t *testing.T) {
	for _, v := range []string{"false", " false\t"} {
		t.Run(v, func(t *testing.T) {
			buf := sec42CapturarLog(t)
			sec42Env(t, sp("192.168.168.106"), nil, sp(v))
			cfg, err := config.Load()
			require.NoError(t, err)
			assert.False(t, cfg.DBTLS.Ativo())
			assert.NotContains(t, cfg.DSN(), "tls=")
			assert.Contains(t, buf.String(), "AVISO")
			assert.Contains(t, buf.String(), "DB_TLS_REQUIRED=false")
			assert.Contains(t, buf.String(), "192.168.168.106")
			assert.NotContains(t, buf.String(), "segredo-sec42", "a senha nunca vai para o log")
		})
	}
}

func TestSEC42_ValorInvalido_Erro(t *testing.T) {
	for _, v := range []string{"0", "1", "no", "TRUE", "False", "sim", "nao", "off", "truee"} {
		for _, host := range []string{"localhost", "192.168.168.106"} {
			t.Run(v+"/"+host, func(t *testing.T) {
				sec42Env(t, sp(host), nil, sp(v))
				cfg, err := config.Load()
				require.ErrorIs(t, err, config.ErrDBTLSRequired)
				assert.Nil(t, cfg)
				assert.Contains(t, err.Error(), "DB_TLS_REQUIRED")
				assert.Contains(t, err.Error(), v)
			})
		}
	}
}

func TestSEC42_ValidarDBTLSObrigatorio_Direto(t *testing.T) {
	t.Setenv("DB_TLS_REQUIRED", "")
	assert.ErrorIs(t, config.ValidarDBTLSObrigatorio("10.1.1.1", config.DBTLS{}), config.ErrDBTLSObrigatorio)
	assert.NoError(t, config.ValidarDBTLSObrigatorio("10.1.1.1", config.DBTLS{CAFile: "/etc/mysql/ssl/ca.pem"}))
	assert.NoError(t, config.ValidarDBTLSObrigatorio("127.0.0.1", config.DBTLS{}))
}

func TestTST12_DescricaoMinTLS13(t *testing.T) {
	d := config.DBTLS{CAFile: "/etc/mysql/ssl/ca.pem", ServerName: "rotaperfumes-mariadb"}.Descricao()
	assert.Contains(t, d, "min=TLS1.3")
	assert.NotContains(t, d, "TLS1.2")
}
