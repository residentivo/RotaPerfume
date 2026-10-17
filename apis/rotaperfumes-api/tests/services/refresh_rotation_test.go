package services_test

// Rotação do refresh token (SEC-02, I-06). ARQ-07 (pendência a): os testes
// montavam a transação no próprio teste (BeginRotation/Issue do repassador).
// Agora, com os mesmos nomes, passam pelo caso de uso real
// identidadeapp.Casos.RenovarSessao: a revogação condicional, a emissão do
// novo token e o commit acontecem no Unit of Work de produção
// (db.Transactor[TxRotacao]) sobre os adapters reais e o db do sqlmock.

import (
	"context"
	"errors"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
)

const revokeCondicionalSQL = `UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE id = \? AND revoked_at IS NULL`

const tokenRotacao = "token-da-rotacao"

// esperarRefreshValido: o token 7 (usuário 3) é válido e o usuário está ativo.
func esperarRefreshValido(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(`FROM refresh_tokens\s+WHERE token_hash = \?`).WithArgs(hashOf(tokenRotacao)).
		WillReturnRows(sqlmock.NewRows(refreshTokenColunas()).
			AddRow(int64(7), int64(3), hashOf(tokenRotacao), time.Now().Add(time.Hour), nil, "ip", "ua", nil))
	mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows(usuarioColunasHeader()).
			AddRow(int64(3), "Nome", "n@test.com", "hash", "normal", nil, true, false, time.Now(), time.Now(), nil, nil))
}

func renovar(t *testing.T, mockSetup func(sqlmock.Sqlmock)) (*identidadeapp.SessaoEmitida, error, sqlmock.Sqlmock) {
	t.Helper()
	db, mock := newRefreshTokenTestDB(t)
	esperarRefreshValido(mock)
	mockSetup(mock)
	s, err := novosCasos(db, nil, nil, time.Now, identidadeapp.Opcoes{}).
		RenovarSessao(context.Background(), identidade.NovoTokenOpaco(tokenRotacao), identidadeapp.Origem{IP: "ip", UserAgent: "ua"})
	return s, err, mock
}

// TestRefreshTokenService_BeginRotation cobre a abertura da rotação (SEC-02):
// begin + UPDATE condicional. 0 linhas → identidade.ErrRefreshRevogado (rollback).
func TestRefreshTokenService_BeginRotation(t *testing.T) {
	cases := []struct {
		nome    string
		setup   func(mock sqlmock.Sqlmock)
		wantErr error
		wantAny bool
	}{
		{
			nome: "revoga na tx e segue para a emissão",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(revokeCondicionalSQL).WithArgs(sqlmock.AnyArg(), "rotacao", int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnResult(sqlmock.NewResult(11, 1))
				mock.ExpectCommit()
			},
		},
		{
			nome: "já revogado (0 linhas) retorna ErrRefreshTokenRevoked",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(revokeCondicionalSQL).WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectRollback()
			},
			wantErr: identidade.ErrRefreshRevogado,
		},
		{
			nome: "erro de banco no UPDATE é propagado",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec(revokeCondicionalSQL).WillReturnError(errors.New("db down"))
				mock.ExpectRollback()
			},
			wantAny: true,
		},
		{
			nome: "erro no begin é propagado",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin().WillReturnError(errors.New("sem conexão"))
			},
			wantAny: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			s, err, mock := renovar(t, tc.setup)
			switch {
			case tc.wantErr != nil:
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Nil(t, s)
			case tc.wantAny:
				assert.Error(t, err)
				assert.NotErrorIs(t, err, identidade.ErrRefreshRevogado)
				assert.Nil(t, s)
			default:
				require.NoError(t, err)
				require.NotNil(t, s)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestRefreshRotation_Issue cobre a emissão do novo token na mesma tx.
func TestRefreshRotation_Issue(t *testing.T) {
	cases := []struct {
		nome    string
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			nome: "insere e confirma",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO refresh_tokens`).WithArgs(int64(3), sqlmock.AnyArg(), sqlmock.AnyArg(), "ip", "ua").
					WillReturnResult(sqlmock.NewResult(11, 1))
				mock.ExpectCommit()
			},
		},
		{
			nome: "falha no insert desfaz a revogação",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnError(errors.New("falha insert"))
				mock.ExpectRollback()
			},
			wantErr: true,
		},
		{
			nome: "falha no commit",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnResult(sqlmock.NewResult(11, 1))
				mock.ExpectCommit().WillReturnError(errors.New("falha commit"))
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			s, err, mock := renovar(t, func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectExec(revokeCondicionalSQL).WillReturnResult(sqlmock.NewResult(0, 1))
				tc.setup(m)
			})
			if tc.wantErr {
				assert.Error(t, err)
				assert.Nil(t, s, "nenhum token sai quando a rotação falha")
			} else {
				require.NoError(t, err)
				assert.Len(t, s.Refresh.Revelar(), identidadecrypto.BytesRefresh*2)
				assert.NotEqual(t, tokenRotacao, s.Refresh.Revelar())
				assert.NotEmpty(t, s.Acesso)
			}
			assert.NoError(t, mock.ExpectationsWereMet(), "uma única tx: revogação e emissão")
		})
	}
}
