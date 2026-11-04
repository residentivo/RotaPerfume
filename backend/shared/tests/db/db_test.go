package db_test

// db.Open: os caminhos de erro são testados sem banco (DSN inválido e porta
// sem MySQL). O caminho de sucesso (pool + ping + log com a senha mascarada)
// só roda com INTEGRATION=1 e o MySQL local, com as mesmas variáveis
// DB_HOST/DB_PORT/DB_NAME/DB_USUARIO/DB_SENHA de config.Load.

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/testkit/integracao"
)

func TestOpen_Erros(t *testing.T) {
	casos := []struct {
		nome    string
		dsn     string
		wantMsg string
	}{
		{"DSN sem a barra do database", "usuario:senha@tcp(127.0.0.1:3306)", "db: falha ao abrir conexão"},
		{"DSN com parâmetro inválido", "usuario:senha@tcp(127.0.0.1:3306)/x?parseTime=talvez", "db: falha ao abrir conexão"},
		{"MySQL fora do ar (porta sem servidor)", "usuario:senha-secreta@tcp(127.0.0.1:1)/rotaperfumes?timeout=2s", "db: ping falhou"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			conn, err := db.Open(c.dsn)
			require.Error(t, err)
			assert.Nil(t, conn)
			assert.Contains(t, err.Error(), c.wantMsg)
			assert.NotContains(t, err.Error(), "senha-secreta", "a senha nunca deve aparecer no erro")
		})
	}
}

func TestOpen_Integracao(t *testing.T) {
	// SEC-50: guarda (pula sem INTEGRATION=1/credenciais; recusa a produção).
	integracao.Config(t)
	t.Setenv("JWT_SECRET", "integracao-segredo-com-pelo-menos-32-bytes")
	t.Setenv("PASSWORD_PEPPER", "pepper-de-integracao-com-32-bytes-ou-mais")
	cfg, err := config.Load()
	require.NoError(t, err)

	var buf bytes.Buffer
	saida, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(saida); log.SetFlags(flags) })

	conn, err := db.Open(cfg.DSN())
	require.NoError(t, err, "MySQL local indisponível")
	t.Cleanup(func() { conn.Close() })

	var um int
	require.NoError(t, conn.QueryRow("SELECT 1").Scan(&um))
	assert.Equal(t, 1, um)
	assert.Equal(t, 25, conn.Stats().MaxOpenConnections)

	// As asserções sobre o log usam strings.Contains + mensagem fixa: com
	// assert.Contains/NotContains, uma falha imprimiria o log (e o usuário, que
	// pode conter a senha) na saída do teste.
	logado := buf.String()
	assert.True(t, strings.Contains(logado, "[db] conexão MariaDB estabelecida"), "log de conexão ausente")
	if strings.Contains(cfg.DBUsuario, cfg.DBSenha) {
		// Credencial em que a senha é parte do usuário (ex.: usuário = senha):
		// o "user:****@" ainda conteria a senha, e a proteção extra do maskDSN
		// (senhaVazou) loga o DSN inteiro como "****".
		assert.True(t, strings.Contains(logado, "host=****,"), "com a senha contida no usuário, o DSN deve sair como ****")
	} else {
		assert.True(t, strings.Contains(logado, cfg.DBUsuario+":****@tcp("), "DSN mascarado (user:****@tcp(...)) ausente no log")
	}
	assert.False(t, strings.Contains(logado, cfg.DBSenha), "a senha nunca deve ser logada")
}
