package resetpassword_test

// SEC-49 (Lote 34): o DSN() do resetpassword aplica a mesma regra do SEC-42
// (config.ValidarDBTLSObrigatorio) da API e das demais CLIs: DB_HOST fora do
// loopback sem DB_TLS_CA é recusado, salvo DB_TLS_REQUIRED=false (com AVISO).

import (
	"bytes"
	"log"
	"testing"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tools/resetpassword"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// envSEC49 limpa as variáveis do banco e define credenciais válidas.
func envSEC49(t *testing.T) {
	t.Helper()
	for _, k := range []string{"DB_HOST", "DB_PORT", "DB_NAME", "DB_TLS_CA", "DB_TLS_SERVER_NAME", "DB_TLS_REQUIRED"} {
		unsetEnv(t, k)
	}
	t.Setenv("DB_USUARIO", "u")
	t.Setenv("DB_SENHA", "s")
}

// capturarLog redireciona o log padrão para um buffer até o fim do teste.
func capturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	saida, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	t.Cleanup(func() {
		log.SetOutput(saida)
		log.SetFlags(flags)
	})
	return &buf
}

func TestSEC49_DSN_HostRemotoSemCARecusado(t *testing.T) {
	envSEC49(t)
	t.Setenv("DB_HOST", "10.0.0.5")

	dsn, err := resetpassword.DSN()
	require.Error(t, err)
	assert.ErrorIs(t, err, config.ErrDBTLSObrigatorio)
	assert.Contains(t, err.Error(), `DB_HOST="10.0.0.5"`)
	assert.Empty(t, dsn, "sem DSN: a senha não sai em texto claro")
	assert.NotContains(t, err.Error(), "s@tcp", "a mensagem não traz a senha")
}

func TestSEC49_DSN_HostRemotoComRequiredFalseAceitaComAviso(t *testing.T) {
	envSEC49(t)
	t.Setenv("DB_HOST", "10.0.0.5")
	t.Setenv("DB_TLS_REQUIRED", "false")
	logs := capturarLog(t)

	dsn, err := resetpassword.DSN()
	require.NoError(t, err)
	assert.Contains(t, dsn, "u:s@tcp(10.0.0.5:3306)/rotaperfumes?")
	assert.NotContains(t, dsn, "tls=", "TLS desligado de propósito")
	assert.Contains(t, logs.String(), "AVISO: DB_TLS_REQUIRED=false")
	assert.Contains(t, logs.String(), `DB_HOST="10.0.0.5"`)
}

func TestSEC49_DSN_RequiredInvalidoRecusadoMesmoNoLoopback(t *testing.T) {
	envSEC49(t)
	t.Setenv("DB_HOST", "localhost")
	t.Setenv("DB_TLS_REQUIRED", "talvez")

	dsn, err := resetpassword.DSN()
	assert.ErrorIs(t, err, config.ErrDBTLSRequired)
	assert.Empty(t, dsn)
}

func TestSEC49_DSN_LoopbackSemCAAceito(t *testing.T) {
	for _, host := range []string{"", "localhost", "127.0.0.1", "127.1.2.3", "::1"} {
		t.Run("host="+host, func(t *testing.T) {
			envSEC49(t)
			if host != "" {
				t.Setenv("DB_HOST", host)
			}
			logs := capturarLog(t)

			dsn, err := resetpassword.DSN()
			require.NoError(t, err)
			assert.NotEmpty(t, dsn)
			assert.NotContains(t, logs.String(), "AVISO: DB_TLS_REQUIRED=false", "loopback não gera aviso")
		})
	}
}
