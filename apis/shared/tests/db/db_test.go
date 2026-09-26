package db_test

// db.Open: os caminhos de erro são testados sem banco (DSN inválido e porta
// sem MySQL). O caminho de sucesso (pool + ping + log com a senha mascarada)
// só roda com INTEGRATION=1 e o MySQL local, com as mesmas variáveis
// DB_HOST/DB_PORT/DB_NAME/DB_USUARIO/DB_SENHA de config.Load.

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/db"
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
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("teste de integração: defina INTEGRATION=1 (requer MySQL local)")
	}
	t.Setenv("JWT_SECRET", "integracao")
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

	logado := buf.String()
	assert.Contains(t, logado, "[db] conexão MySQL estabelecida")
	assert.Contains(t, logado, cfg.DBUsuario+":****@tcp(")
	if cfg.DBSenha != "" && !strings.Contains(cfg.DBUsuario, cfg.DBSenha) {
		assert.NotContains(t, logado, ":"+cfg.DBSenha+"@", "a senha nunca deve ser logada")
	}
}
