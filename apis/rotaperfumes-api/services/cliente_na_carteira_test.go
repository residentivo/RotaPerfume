package services_test

// SEC-01: ClienteService.CreateClienteNaCarteira (cliente + vínculo de
// carteira na mesma transação). sqlmock estrito: toda falha deve terminar em
// ROLLBACK, nunca COMMIT; validação falha antes do BEGIN.

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

const (
	reInsertClienteS  = `INSERT INTO clientes \(cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo\)`
	reInsertCarteiraS = `INSERT INTO carteiras \(cliente_id, vendedor_id, data_inicio, data_fim\)`
)

// meiaNoiteDeHoje valida que o argumento data_inicio é a meia-noite local
// do dia corrente (coluna DATE, sem hora).
type meiaNoiteDeHoje struct{}

func (meiaNoiteDeHoje) Match(v driver.Value) bool {
	t, ok := v.(time.Time)
	if !ok {
		return false
	}
	t = t.In(time.Local)
	y, m, d := time.Now().Date()
	ty, tm, td := t.Date()
	return y == ty && m == tm && d == td && t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0
}

func inputClienteValido() services.ClienteInput {
	return services.ClienteInput{
		CNPJ: "12345678000199", RazaoSocial: "Empresa", Segmento: "varejo",
		Cidade: "Curitiba", UF: "pr", Bairro: "Centro", DataCadastro: "2024-02-10",
	}
}

func TestClienteService_CreateClienteNaCarteira(t *testing.T) {
	errDB := errors.New("falha simulada")
	casos := []struct {
		nome    string
		input   func() services.ClienteInput
		expect  func(m sqlmock.Sqlmock)
		wantErr error // errors.Is
		algum   bool  // qualquer erro
	}{
		{
			nome:  "sucesso grava cliente e carteira e faz commit",
			input: inputClienteValido,
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec(reInsertClienteS).
					WithArgs("12345678000199", "Empresa", "varejo", "Curitiba", "PR", "Centro", sqlmock.AnyArg(), true).
					WillReturnResult(sqlmock.NewResult(555, 1))
				m.ExpectExec(reInsertCarteiraS).
					WithArgs(int64(555), int64(10), meiaNoiteDeHoje{}, nil).
					WillReturnResult(sqlmock.NewResult(77, 1))
				m.ExpectCommit()
			},
		},
		{
			nome:    "validacao falha antes do BEGIN",
			input:   func() services.ClienteInput { in := inputClienteValido(); in.UF = "PRR"; return in },
			expect:  func(m sqlmock.Sqlmock) {},
			wantErr: services.ErrUFInvalida,
		},
		{
			nome:    "data_cadastro invalida falha antes do BEGIN",
			input:   func() services.ClienteInput { in := inputClienteValido(); in.DataCadastro = "10/02/2024"; return in },
			expect:  func(m sqlmock.Sqlmock) {},
			wantErr: services.ErrDataCadastroInvalida,
		},
		{
			nome:    "erro no BEGIN",
			input:   inputClienteValido,
			expect:  func(m sqlmock.Sqlmock) { m.ExpectBegin().WillReturnError(errDB) },
			wantErr: errDB,
		},
		{
			nome:  "erro no insert do cliente faz rollback",
			input: inputClienteValido,
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec(reInsertClienteS).WillReturnError(errDB)
				m.ExpectRollback()
			},
			wantErr: errDB,
		},
		{
			nome:  "erro no insert da carteira faz rollback (sem cliente orfao)",
			input: inputClienteValido,
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec(reInsertClienteS).WillReturnResult(sqlmock.NewResult(555, 1))
				m.ExpectExec(reInsertCarteiraS).WillReturnError(errDB)
				m.ExpectRollback()
			},
			wantErr: errDB,
		},
		{
			nome:  "erro no lastInsertId da carteira faz rollback",
			input: inputClienteValido,
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec(reInsertClienteS).WillReturnResult(sqlmock.NewResult(555, 1))
				m.ExpectExec(reInsertCarteiraS).WillReturnResult(sqlmock.NewErrorResult(errDB))
				m.ExpectRollback()
			},
			wantErr: errDB,
		},
		{
			nome:  "erro no commit",
			input: inputClienteValido,
			expect: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec(reInsertClienteS).WillReturnResult(sqlmock.NewResult(555, 1))
				m.ExpectExec(reInsertCarteiraS).WillReturnResult(sqlmock.NewResult(77, 1))
				m.ExpectCommit().WillReturnError(errDB)
			},
			wantErr: errDB,
		},
	}

	for _, tc := range casos {
		for _, verbose := range []bool{false, true} {
			t.Run(tc.nome, func(t *testing.T) {
				db, mock := newClienteTestDB(t)
				tc.expect(mock)

				svc := services.NewClienteService(db, clienteTestCfg(verbose))
				c, err := svc.CreateClienteNaCarteira(context.Background(), db, tc.input(), 10)

				switch {
				case tc.wantErr != nil:
					assert.ErrorIs(t, err, tc.wantErr)
					assert.Nil(t, c)
				case tc.algum:
					assert.Error(t, err)
				default:
					require.NoError(t, err)
					require.NotNil(t, c)
					assert.Equal(t, int64(555), c.ClienteIDOrigem)
					assert.True(t, c.Ativo)
					assert.Equal(t, "PR", c.UF)
				}
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

// Admin (CreateCliente) nunca toca em carteiras nem abre transação.
func TestClienteService_CreateCliente_SemCarteiraNemTransacao(t *testing.T) {
	db, mock := newClienteTestDB(t)
	mock.ExpectExec(reInsertClienteS).WillReturnResult(sqlmock.NewResult(556, 1))

	c, err := services.NewClienteService(db, clienteTestCfg(false)).
		CreateCliente(context.Background(), db, inputClienteValido())
	require.NoError(t, err)
	assert.Equal(t, int64(556), c.ClienteIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}
