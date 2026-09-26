package handlers_test

// SEC-08: no reuso de refresh token rotacionado fora da janela (SEC-07), o
// handler, além da revogação em massa, grava o corte de sessão do usuário
// (UsuarioRepository.InvalidarSessoes) para derrubar os access tokens já
// emitidos. Falha nessa gravação só gera log [auth][seguranca]; a resposta
// continua 401 "refresh token revogado", sem emissão de tokens.

import (
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

const invalidarSessoesSQL = `UPDATE usuarios SET tokens_validos_desde = \? WHERE id = \?`

// corteDeSessaoArg casa com um time.Time sem fração de segundo gerado
// durante o teste.
type corteDeSessaoArg struct{ antes time.Time }

func (c corteDeSessaoArg) Match(v driver.Value) bool {
	t, ok := v.(time.Time)
	return ok && t.Nanosecond() == 0 && !t.Before(c.antes.Truncate(time.Second)) && !t.After(time.Now())
}

func TestSEC08_ReusoDeRotacao_InvalidaSessoes(t *testing.T) {
	casos := []struct {
		nome          string
		revokeErr     error
		invalidarRes  driver.Result
		invalidarErr  error
		wantLog       []string
		wantSemLogFal bool
	}{
		{nome: "sucesso: grava corte, sem log de falha",
			invalidarRes: sqlmock.NewResult(0, 1), wantSemLogFal: true},
		{nome: "erro no UPDATE: loga e responde 401",
			invalidarErr: sqlmock.ErrCancelled,
			wantLog:      []string{"[auth][seguranca] refresh: falha ao invalidar access tokens após reuso: user_id=5"}},
		{nome: "usuário sumiu (0 linhas): loga ErrNotFound e responde 401",
			invalidarRes: sqlmock.NewResult(0, 0),
			wantLog:      []string{"falha ao invalidar access tokens após reuso: user_id=5"}},
		{nome: "revogação em massa falha: ainda grava o corte",
			revokeErr: sqlmock.ErrCancelled, invalidarRes: sqlmock.NewResult(0, 1),
			wantLog:       []string{"falha na revogação em massa após reuso: user_id=5"},
			wantSemLogFal: true},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			logs := capturarLog(t)

			expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "rotacao")
			rev := mock.ExpectExec(revokeAllUsuarioSQL).WithArgs(sqlmock.AnyArg(), "revogacao_massa", int64(5))
			if tc.revokeErr != nil {
				rev.WillReturnError(tc.revokeErr)
			} else {
				rev.WillReturnResult(sqlmock.NewResult(0, 2))
			}
			inv := mock.ExpectExec(invalidarSessoesSQL).WithArgs(corteDeSessaoArg{antes: time.Now()}, int64(5))
			if tc.invalidarErr != nil {
				inv.WillReturnError(tc.invalidarErr)
			} else {
				inv.WillReturnResult(tc.invalidarRes)
			}

			assertRefreshRevogado401(t, server.URL+"/api/auth/refresh")

			assert.NoError(t, mock.ExpectationsWereMet(), "revogação em massa seguida do corte de sessão")
			out := logs.String()
			for _, l := range tc.wantLog {
				assert.Contains(t, out, l)
			}
			if tc.wantSemLogFal {
				assert.NotContains(t, out, "falha ao invalidar access tokens")
			}
		})
	}
}

// Motivos que não são reuso de rotação, e o reuso dentro da janela, não
// gravam corte de sessão (nenhum UPDATE em usuarios é esperado no mock).
func TestSEC08_SemReusoDeRotacao_NaoInvalidaSessoes(t *testing.T) {
	casos := []struct {
		nome      string
		revokedAt time.Duration
		motivo    any
	}{
		{"logout fora da janela", -time.Minute, "logout"},
		{"senha fora da janela", -time.Minute, "senha"},
		{"inativacao fora da janela", -time.Minute, "inativacao"},
		{"revogacao_massa fora da janela", -time.Minute, "revogacao_massa"},
		{"NULL (legado) fora da janela", -time.Minute, nil},
		{"rotacao dentro da janela", -5 * time.Second, "rotacao"},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			logs := capturarLog(t)

			expectRefreshRevogadoComMotivo(mock, time.Now().Add(tc.revokedAt), tc.motivo)
			assertRefreshRevogado401(t, server.URL+"/api/auth/refresh")

			assert.NoError(t, mock.ExpectationsWereMet())
			assert.NotContains(t, logs.String(), "invalidar access tokens")
		})
	}
}
