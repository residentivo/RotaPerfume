package visitas_test

import (
	"bytes"
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
)

var errBanco = errors.New("falha simulada no banco")

// escreverArquivo grava conteudo em dir/nome e devolve o caminho.
func escreverArquivo(t *testing.T, dir, nome, conteudo string) string {
	t.Helper()
	path := filepath.Join(dir, nome)
	require.NoError(t, os.WriteFile(path, []byte(conteudo), 0o600))
	return path
}

// novoMock cria um sqlmock com matcher por regex e monitoramento de Ping.
func novoMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp),
		sqlmock.MonitorPingsOption(true),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

// openerDe devolve um Opener que entrega db e conta as chamadas.
func openerDe(db *sql.DB, chamadas *int) cmdutil.Opener {
	return func() (cmdutil.Conn, error) {
		*chamadas++
		return db, nil
	}
}

// openerProibido falha o teste se o Run tentar abrir o banco.
func openerProibido(t *testing.T) cmdutil.Opener {
	return func() (cmdutil.Conn, error) {
		t.Fatalf("o banco não deveria ser aberto")
		return nil, nil
	}
}

// capturarLog redireciona o log padrão para um buffer até o fim do teste.
func capturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	saida, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(saida)
		log.SetFlags(flags)
	})
	return &buf
}

// chdir muda o diretório de trabalho até o fim do teste (não paralelo).
func chdir(t *testing.T, dir string) {
	t.Helper()
	antigo, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(antigo) })
}
