package exportdados_test

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/tools/exportdados"
)

// Campo maior que o buffer do csv.Writer (4 KiB) chega ao destino na hora:
// a falha de escrita aparece no header ou na linha, com o contexto certo
// (ARQ-06: ramos sem teste).
func TestWriteCSV_FalhaDeEscritaComCampoLongo(t *testing.T) {
	longo := strings.Repeat("x", 5000)

	db, mock := novoMock(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow(1))
	rows, err := db.Query("SELECT a")
	require.NoError(t, err)
	defer rows.Close()
	_, err = exportdados.WriteCSV(escritorQuebrado{}, []string{longo}, rows, 1)
	assert.ErrorIs(t, err, errDisco)
	assert.Contains(t, err.Error(), "escrevendo header")

	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow(longo))
	rows2, err := db.Query("SELECT a")
	require.NoError(t, err)
	defer rows2.Close()
	n, err := exportdados.WriteCSV(escritorQuebrado{}, []string{"a"}, rows2, 1)
	assert.ErrorIs(t, err, errDisco)
	assert.Contains(t, err.Error(), "escrevendo linha")
	assert.Zero(t, n)
}

// linhasFechadas devolve o resultado da query já fechado: rows.Columns()
// falha e o ExportTable não cria o arquivo.
type linhasFechadas struct{ db exportdados.Querier }

func (q linhasFechadas) Query(sqlTexto string, args ...any) (*sql.Rows, error) {
	rows, err := q.db.Query(sqlTexto, args...)
	if err == nil {
		_ = rows.Close()
	}
	return rows, err
}

func TestExportTable_FalhaAoLerColunas(t *testing.T) {
	db, mock := novoMock(t)
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"a"}).AddRow(1))
	destino := filepath.Join(t.TempDir(), "sub", "x.csv")
	n, err := exportdados.ExportTable(linhasFechadas{db}, exportdados.TableExport{Query: "SELECT a", Header: []string{"a"}}, destino)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "columns")
	assert.Zero(t, n)
	assert.NoFileExists(t, destino)
}
