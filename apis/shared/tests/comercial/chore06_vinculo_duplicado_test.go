package comercial_test

// CHORE-06 (Lote 25): a corrida no vínculo do dia (1062 em carteiras) era
// 500; passa a 409 "vínculo entre cliente e vendedor já cadastrado nesta
// data", sem o erro do driver e com o log da recusa sem valores.

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/kernel"
)

func TestCHORE06_ErrVinculoDuplicado_409(t *testing.T) {
	var e *kernel.Erro
	require.True(t, errors.As(comercial.ErrVinculoDuplicado, &e))
	assert.Equal(t, kernel.KindConflito, e.Kind())
	assert.Equal(t, "vínculo entre cliente e vendedor já cadastrado nesta data", comercial.ErrVinculoDuplicado.Error())
}

func TestCHORE06_VincularCliente_CorridaNoInsert_409SemValor(t *testing.T) {
	ctx := context.Background()
	out := &bytes.Buffer{}
	prev := log.Writer()
	log.SetOutput(out)
	t.Cleanup(func() { log.SetOutput(prev) })

	pool, mock := novoMock(t)
	esperarVincularAte(mock)
	mock.ExpectQuery(reAtivoDoCliente).WillReturnRows(sqlmock.NewRows(colsCarteira))
	mock.ExpectQuery(reDoDia).WillReturnRows(sqlmock.NewRows(colsCarteira))
	mock.ExpectExec(`INSERT INTO carteiras`).WillReturnError(&mysql.MySQLError{Number: 1062,
		Message: "Duplicate entry '2-1-2026-05-06' for key 'carteiras.uq_carteiras_cliente_vendedor_data'"})

	r, err := novosCasos(pool, vinculoFake{}).VincularCliente(ctx, principal(t, kernel.RoleAdmin), 1, 2)
	assert.Nil(t, r)
	assert.Same(t, comercial.ErrVinculoDuplicado, err)
	var me *mysql.MySQLError
	assert.False(t, errors.As(err, &me), "o erro do driver não é embrulhado")
	assert.NoError(t, mock.ExpectationsWereMet())

	assert.Contains(t, out.String(), "[carteiras] vinculo duplicado recusado")
	for _, valor := range []string{"2-1-2026-05-06", "Duplicate entry"} {
		assert.NotContains(t, out.String(), valor)
	}
}

// Erro de infra no INSERT (não 1062) não é a recusa de duplicidade.
func TestCHORE06_VincularCliente_FalhaDeInfraNaoLogaRecusa(t *testing.T) {
	out := &bytes.Buffer{}
	prev := log.Writer()
	log.SetOutput(out)
	t.Cleanup(func() { log.SetOutput(prev) })

	pool, mock := novoMock(t)
	esperarVincularAte(mock)
	mock.ExpectQuery(reAtivoDoCliente).WillReturnRows(sqlmock.NewRows(colsCarteira))
	mock.ExpectQuery(reDoDia).WillReturnRows(sqlmock.NewRows(colsCarteira))
	mock.ExpectExec(`INSERT INTO carteiras`).WillReturnError(errInfra)

	_, err := novosCasos(pool, vinculoFake{}).VincularCliente(context.Background(), principal(t, kernel.RoleAdmin), 1, 2)
	assert.ErrorIs(t, err, errInfra)
	var e *kernel.Erro
	assert.False(t, errors.As(err, &e))
	assert.NotContains(t, out.String(), "vinculo duplicado recusado")
	assert.NoError(t, mock.ExpectationsWereMet())
}
