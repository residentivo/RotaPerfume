package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

// NEG-01: validação e duplicidade de CNPJ no ClienteService.

const (
	reInsertClienteCNPJ = `INSERT INTO clientes \(cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo\)`
	reUpdateClienteCNPJ = `UPDATE clientes\s+SET cnpj = \?`
	reGetClienteCNPJ    = `FROM clientes WHERE cliente_id_origem = \? LIMIT 1`
)

func erroDuplicadoCNPJ() error {
	return &mysql.MySQLError{Number: 1062, Message: "Duplicate entry '11222333000181' for key 'clientes.uq_clientes_cnpj'"}
}

// clienteRowsComCNPJ devolve o cliente id=1 gravado com o CNPJ informado.
func clienteRowsComCNPJ(cnpj string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro",
		"data_cadastro", "ativo", "created_at", "updated_at",
	}).AddRow(int64(1), cnpj, "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro", now, true, now, now)
}

func TestClienteService_CreateCliente_CNPJ(t *testing.T) {
	cases := []struct {
		nome     string
		cnpj     string
		expect   func(m sqlmock.Sqlmock)
		wantErr  error
		wantCNPJ string
	}{
		{
			nome: "máscara é removida e grava só dígitos",
			cnpj: "11.222.333/0001-81",
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectExec(reInsertClienteCNPJ).
					WithArgs("11222333000181", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), true).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantCNPJ: "11222333000181",
		},
		{
			nome: "NEG-02: alfanumérico minúsculo com máscara grava sem máscara e em maiúsculas",
			cnpj: "12.abc.345/01de-35",
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectExec(reInsertClienteCNPJ).
					WithArgs("12ABC34501DE35", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), true).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			wantCNPJ: "12ABC34501DE35",
		},
		{nome: "13 dígitos", cnpj: "1122233300018", wantErr: services.ErrCNPJInvalido},
		{nome: "15 dígitos", cnpj: "112223330001810", wantErr: services.ErrCNPJInvalido},
		{nome: "alfanumérico com DV errado", cnpj: "12ABC34501DE36", wantErr: services.ErrCNPJInvalido},
		{nome: "letra na posição do DV", cnpj: "12ABC34501DEA5", wantErr: services.ErrCNPJInvalido},
		{nome: "símbolo fora da máscara", cnpj: "12ABC34501DE3#", wantErr: services.ErrCNPJInvalido},
		{nome: "todos iguais", cnpj: "11.111.111/1111-11", wantErr: services.ErrCNPJInvalido},
		{nome: "DV inválido", cnpj: "12345678000199", wantErr: services.ErrCNPJInvalido},
		{nome: "só máscara vira vazio de dígitos", cnpj: "../-", wantErr: services.ErrCNPJInvalido},
		{nome: "vazio continua obrigatório", cnpj: "  ", wantErr: services.ErrCNPJObrigatorio},
		{
			nome: "duplicado (1062 uq_clientes_cnpj) vira ErrCNPJDuplicado",
			cnpj: "11222333000181",
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectExec(reInsertClienteCNPJ).WillReturnError(erroDuplicadoCNPJ())
			},
			wantErr: services.ErrCNPJDuplicado,
		},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newClienteTestDB(t)
			if tc.expect != nil {
				tc.expect(mock)
			}
			in := validClienteInput()
			in.CNPJ = tc.cnpj

			c, err := services.NewClienteService(db, clienteTestCfg(true)).CreateCliente(context.Background(), db, in)
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, c)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.wantCNPJ, c.CNPJ)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Violação de OUTRO índice único (1062 sem uq_clientes_cnpj) não é tratada
// como CNPJ duplicado: vira erro interno.
func TestClienteService_CreateCliente_OutroDuplicadoNaoViraCNPJ(t *testing.T) {
	db, mock := newClienteTestDB(t)
	mock.ExpectExec(reInsertClienteCNPJ).
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry 'x' for key 'clientes.PRIMARY'"})

	_, err := services.NewClienteService(db, clienteTestCfg(false)).CreateCliente(context.Background(), db, validClienteInput())
	assert.Error(t, err)
	assert.NotErrorIs(t, err, services.ErrCNPJDuplicado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteService_CreateClienteNaCarteira_CNPJDuplicadoFazRollback(t *testing.T) {
	db, mock := newClienteTestDB(t)
	mock.ExpectBegin()
	mock.ExpectExec(reInsertClienteCNPJ).WillReturnError(erroDuplicadoCNPJ())
	mock.ExpectRollback()

	c, err := services.NewClienteService(db, clienteTestCfg(false)).CreateClienteNaCarteira(context.Background(), db, validClienteInput(), 10)
	assert.ErrorIs(t, err, services.ErrCNPJDuplicado)
	assert.Nil(t, c)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestClienteService_CreateClienteNaCarteira_DVInvalidoAntesDoBegin(t *testing.T) {
	db, mock := newClienteTestDB(t)
	in := validClienteInput()
	in.CNPJ = "12345678000199"

	_, err := services.NewClienteService(db, clienteTestCfg(false)).CreateClienteNaCarteira(context.Background(), db, in, 10)
	assert.ErrorIs(t, err, services.ErrCNPJInvalido)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestClienteService_UpdateCliente_CNPJ: NEG-04 — o DV é exigido em TODA
// gravação, inclusive quando o CNPJ enviado é igual ao gravado (cliente
// legado/importado com DV inválido só volta a ser salvo com o CNPJ corrigido).
// A recusa acontece antes de qualquer query; o 404 vem do próprio UPDATE
// (RowsAffected=0).
func TestClienteService_UpdateCliente_CNPJ(t *testing.T) {
	cases := []struct {
		nome      string
		cnpj      string
		expect    func(m sqlmock.Sqlmock)
		wantErr   error
		wantArgUp string // CNPJ esperado no UPDATE ("" = sem UPDATE bem-sucedido)
	}{
		{
			// Os casos sem expect e sem wantArgUp provam, via
			// ExpectationsWereMet + ErrorIs, que nenhuma query foi feita.
			nome: "mesmo CNPJ legado com DV inválido (sem máscara) é recusado sem query", cnpj: "29401965569816",
			wantErr: services.ErrCNPJInvalido,
		},
		{
			nome: "mesmo CNPJ legado com DV inválido (com máscara) é recusado sem query", cnpj: "29.401.965/5698-16",
			wantErr: services.ErrCNPJInvalido,
		},
		{
			nome: "CNPJ com DV inválido é recusado sem query", cnpj: "12345678000199",
			wantErr: services.ErrCNPJInvalido,
		},
		{
			nome: "CNPJ com DV válido (com máscara) grava só dígitos", cnpj: "11.444.777/0001-61",
			wantArgUp: "11444777000161",
		},
		{
			nome: "NEG-02: alfanumérico minúsculo grava em maiúsculas", cnpj: "12abc34501de35",
			wantArgUp: "12ABC34501DE35",
		},
		{
			nome: "NEG-02: alfanumérico com DV errado é recusado sem query", cnpj: "12.ABC.345/01DE-36",
			wantErr: services.ErrCNPJInvalido,
		},
		{
			nome: "formato inválido é recusado antes de consultar o banco", cnpj: "123",
			wantErr: services.ErrCNPJInvalido,
		},
		{
			nome: "cliente inexistente (UPDATE com 0 linhas) retorna ErrClienteNaoEncontrado", cnpj: "11222333000181",
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectExec(reUpdateClienteCNPJ).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: services.ErrClienteNaoEncontrado,
		},
		{
			nome: "erro no UPDATE é propagado", cnpj: "11222333000181",
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectExec(reUpdateClienteCNPJ).WillReturnError(errors.New("db down"))
			},
		},
		{
			nome: "duplicado no UPDATE vira ErrCNPJDuplicado", cnpj: "11222333000181",
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectExec(reUpdateClienteCNPJ).WillReturnError(erroDuplicadoCNPJ())
			},
			wantErr: services.ErrCNPJDuplicado,
		},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newClienteTestDB(t)
			switch {
			case tc.expect != nil:
				tc.expect(mock)
			case tc.wantArgUp != "":
				mock.ExpectExec(reUpdateClienteCNPJ).
					WithArgs(tc.wantArgUp, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(reGetClienteCNPJ).WithArgs(int64(1)).WillReturnRows(clienteRowsComCNPJ(tc.wantArgUp))
			}
			in := validClienteInput()
			in.CNPJ = tc.cnpj

			c, err := services.NewClienteService(db, clienteTestCfg(true)).UpdateCliente(context.Background(), db, 1, in)
			switch {
			case tc.wantErr != nil:
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, c)
			case tc.wantArgUp == "":
				assert.Error(t, err)
				assert.NotErrorIs(t, err, services.ErrCNPJDuplicado)
				assert.NotErrorIs(t, err, services.ErrClienteNaoEncontrado)
				assert.Nil(t, c)
			default:
				require.NoError(t, err)
				assert.Equal(t, tc.wantArgUp, c.CNPJ)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// NEG-04: precedência — id inexistente com DV inválido dá ErrCNPJInvalido
// (400), pois a validação ocorre antes do UPDATE que detectaria o 404.
func TestClienteService_UpdateCliente_DVInvalidoTemPrecedenciaSobre404(t *testing.T) {
	db, mock := newClienteTestDB(t)
	in := validClienteInput()
	in.CNPJ = "29401965569816"

	c, err := services.NewClienteService(db, clienteTestCfg(false)).UpdateCliente(context.Background(), db, 999999, in)
	assert.ErrorIs(t, err, services.ErrCNPJInvalido)
	assert.NotErrorIs(t, err, services.ErrClienteNaoEncontrado)
	assert.Nil(t, c)
	assert.NoError(t, mock.ExpectationsWereMet())
}
