package services_test

// BUG-08: POST /api/clientes devolvia created_at/updated_at zerados porque o
// service retornava o objeto em memória. Agora os dois fluxos de criação
// relêem o registro do banco após o INSERT (fallback: objeto em memória se a
// releitura falhar, pois o INSERT já foi confirmado).

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

const reGetClienteByID = `SELECT ` + `cliente_id_origem, cnpj, razao_social, segmento, cidade, uf, COALESCE\(bairro, ''\), data_cadastro, ativo, created_at, updated_at` + ` FROM clientes WHERE cliente_id_origem = \? LIMIT 1`

func clienteGravadoRows(id int64, criado time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro",
		"data_cadastro", "ativo", "created_at", "updated_at",
	}).AddRow(id, "11222333000181", "Empresa", "varejo", "Curitiba", "PR", "Centro",
		criado, true, criado, criado)
}

func TestBUG08_CreateCliente_DevolveTimestampsDoBanco(t *testing.T) {
	db, mock := newClienteTestDB(t)
	criado := time.Date(2026, 9, 26, 10, 30, 0, 0, time.Local)

	mock.ExpectExec(reInsertClienteS).WillReturnResult(sqlmock.NewResult(101, 1))
	mock.ExpectQuery(reGetClienteByID).WithArgs(int64(101)).WillReturnRows(clienteGravadoRows(101, criado))

	svc := services.NewClienteService(db, clienteTestCfg(false))
	c, err := svc.CreateCliente(context.Background(), db, inputClienteValido())

	require.NoError(t, err)
	assert.Equal(t, int64(101), c.ClienteIDOrigem)
	assert.False(t, c.CreatedAt.IsZero(), "created_at deve vir preenchido")
	assert.False(t, c.UpdatedAt.IsZero(), "updated_at deve vir preenchido")
	assert.True(t, c.CreatedAt.Equal(criado))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBUG08_CreateClienteNaCarteira_DevolveTimestampsDoBanco(t *testing.T) {
	db, mock := newClienteTestDB(t)
	criado := time.Date(2026, 9, 26, 11, 0, 0, 0, time.Local)

	mock.ExpectBegin()
	mock.ExpectExec(reInsertClienteS).WillReturnResult(sqlmock.NewResult(555, 1))
	mock.ExpectExec(reInsertCarteiraS).WillReturnResult(sqlmock.NewResult(77, 1))
	mock.ExpectCommit()
	// Releitura só depois do COMMIT (fora da transação).
	mock.ExpectQuery(reGetClienteByID).WithArgs(int64(555)).WillReturnRows(clienteGravadoRows(555, criado))

	svc := services.NewClienteService(db, clienteTestCfg(false))
	c, err := svc.CreateClienteNaCarteira(context.Background(), db, inputClienteValido(), 10)

	require.NoError(t, err)
	assert.Equal(t, int64(555), c.ClienteIDOrigem)
	assert.True(t, c.CreatedAt.Equal(criado))
	assert.True(t, c.UpdatedAt.Equal(criado))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBUG08_CreateCliente_FalhaNaReleitura_DevolveObjetoGravadoSemErro(t *testing.T) {
	db, mock := newClienteTestDB(t)

	mock.ExpectExec(reInsertClienteS).WillReturnResult(sqlmock.NewResult(101, 1))
	mock.ExpectQuery(reGetClienteByID).WithArgs(int64(101)).WillReturnError(errors.New("db down"))

	svc := services.NewClienteService(db, clienteTestCfg(false))
	c, err := svc.CreateCliente(context.Background(), db, inputClienteValido())

	require.NoError(t, err, "INSERT confirmado não pode virar erro por falha só na releitura")
	require.NotNil(t, c)
	assert.Equal(t, int64(101), c.ClienteIDOrigem)
	assert.Equal(t, "Empresa", c.RazaoSocial)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBUG08_UpdateCliente_DevolveTimestampsDoBanco(t *testing.T) {
	db, mock := newClienteTestDB(t)
	criado := time.Date(2026, 1, 2, 3, 4, 5, 0, time.Local)

	mock.ExpectExec(`UPDATE clientes\s+SET cnpj = \?`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(reGetClienteByID).WithArgs(int64(555)).WillReturnRows(clienteGravadoRows(555, criado))

	svc := services.NewClienteService(db, clienteTestCfg(false))
	c, err := svc.UpdateCliente(context.Background(), db, 555, inputClienteValido())

	require.NoError(t, err)
	assert.True(t, c.CreatedAt.Equal(criado))
	assert.True(t, c.UpdatedAt.Equal(criado))
	assert.NoError(t, mock.ExpectationsWereMet())
}
