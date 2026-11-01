package clientes_test

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/importers/clientes"
)

// NEG-01: unificação de CNPJ duplicado no importador de clientes.

func linhaCliente(id int64, cnpj string) clientes.Row {
	return clientes.Row{ClienteIDOrigem: id, CNPJ: cnpj, RazaoSocial: "X", UF: "PR", DataCadastro: time.Date(2024, 1, 2, 0, 0, 0, 0, time.Local), Ativo: true}
}

func TestUnificarPorCNPJ(t *testing.T) {
	rows := []clientes.Row{
		linhaCliente(1, "11222333000181"),
		linhaCliente(2, "11444777000161"),
		linhaCliente(3001, "11222333000181"), // cópia de 1
		linhaCliente(3002, "11444777000161"), // cópia de 2
		linhaCliente(3, "12345678000199"),    // DV inválido: aceito no importador
	}
	mantidas, unificadas := clientes.UnificarPorCNPJ(rows)

	assert.Equal(t, 2, unificadas)
	ids := make([]int64, 0, len(mantidas))
	for _, r := range mantidas {
		ids = append(ids, r.ClienteIDOrigem)
	}
	assert.Equal(t, []int64{1, 2, 3}, ids, "fica a 1ª ocorrência, na ordem do CSV")
}

func TestIsDuplicateCNPJ(t *testing.T) {
	assert.True(t, clientes.IsDuplicateCNPJ(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'x' for key 'clientes.uq_clientes_cnpj'"}))
	assert.False(t, clientes.IsDuplicateCNPJ(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'x' for key 'PRIMARY'"}))
	assert.False(t, clientes.IsDuplicateCNPJ(errors.New("uq_clientes_cnpj")))
}

const reUpsertCliente = `INSERT INTO clientes`
const reDonosCNPJ = `SELECT cliente_id_origem, cnpj FROM clientes`

// TestUpsertAll_ConflitosCNPJ: linha cujo CNPJ pertence a outro cliente no
// banco é pulada (sem INSERT); 1062 residual vira conflito; outros erros
// contam como failed; nada aborta a importação.
func TestUpsertAll_ConflitosCNPJ(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(reDonosCNPJ).WillReturnRows(sqlmock.NewRows([]string{"cliente_id_origem", "cnpj"}).
		AddRow(int64(1), "11222333000181").
		AddRow(int64(9000), "11444777000161")) // criado pela API com o CNPJ da linha 2
	prep := mock.ExpectPrepare(reUpsertCliente)
	prep.ExpectExec().WithArgs(int64(1), "11222333000181", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1)) // inalterado
	// linha 2 (CNPJ do cliente 9000) é pulada sem Exec
	prep.ExpectExec().WithArgs(int64(3), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'x' for key 'clientes.uq_clientes_cnpj'"})
	prep.ExpectExec().WithArgs(int64(4), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(errors.New("db down"))
	prep.ExpectExec().WithArgs(int64(5), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 2)) // atualizado

	res, err := clientes.UpsertAll(db, []clientes.Row{
		linhaCliente(1, "11222333000181"),
		linhaCliente(2, "11444777000161"),
		linhaCliente(3, "00000000000191"),
		linhaCliente(4, "11111111000191"),
		linhaCliente(5, "22222222000191"),
	})
	require.NoError(t, err)

	assert.Equal(t, clientes.ResultadoUpsert{Inserted: 1, Updated: 1, ConflitosCNPJ: 2, Failed: 1}, res)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDonosCNPJ_TrocaDeCNPJLiberaOAntigo(t *testing.T) {
	d := clientes.NovosDonosCNPJ()
	d.Gravar(1, "A")
	d.Gravar(1, "B")
	_, ok := d.Dono("A")
	assert.False(t, ok, "CNPJ antigo do cliente 1 fica livre")
	id, ok := d.Dono("B")
	assert.True(t, ok)
	assert.Equal(t, int64(1), id)
}
