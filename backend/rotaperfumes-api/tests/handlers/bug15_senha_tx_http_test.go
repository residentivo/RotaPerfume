package handlers_test

// BUG-15 (Lote 27): na troca de senha pelo próprio usuário, histórico,
// senha e revogação rodam numa tx. Mudança intencional de contrato: falha
// no histórico ou na revogação, que antes respondia 200, passa a 500 sem
// trocar a senha (rollback).

import (
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBUG15_HTTP_TrocaDeSenha_FalhaNaTx_500Rollback(t *testing.T) {
	errBanco := errors.New("banco fora do ar")
	etapas := []struct {
		nome    string
		prepara func(m sqlmock.Sqlmock)
	}{
		{"histórico", func(m sqlmock.Sqlmock) {
			m.ExpectExec(`INSERT INTO senha_historico`).WillReturnError(errBanco)
		}},
		{"senha", func(m sqlmock.Sqlmock) {
			m.ExpectExec(`INSERT INTO senha_historico`).WillReturnResult(sqlmock.NewResult(1, 1))
			m.ExpectExec(`UPDATE usuarios SET password_hash`).WillReturnError(errBanco)
		}},
		{"revogação", func(m sqlmock.Sqlmock) {
			m.ExpectExec(`INSERT INTO senha_historico`).WillReturnResult(sqlmock.NewResult(1, 1))
			m.ExpectExec(`UPDATE usuarios SET password_hash`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`UPDATE refresh_tokens SET revoked_at`).WillReturnError(errBanco)
		}},
	}
	for _, e := range etapas {
		t.Run(e.nome, func(t *testing.T) {
			server, db, mock, _ := setupTestServerWithAuthHandler(t)
			defer server.Close()
			defer db.Close()
			cfg := testCfg()
			hashSenhaAntiga, err := gerarHashSenha(cfg.HashSenha, "SenhaBemAntiga1!")
			require.NoError(t, err)
			mockUsuarioParaResetPassword(t, mock, cfg, 2, "SenhaAtual999!")
			mockSenhaHistoricoRecente(mock, 2, hashSenhaAntiga)
			mock.ExpectBegin()
			e.prepara(mock)
			mock.ExpectRollback()

			status, body := doReq(t, http.MethodPost, server.URL+"/api/auth/reset-password", generateToken(t, cfg, 2, "normal"),
				validCaptchaBody(map[string]any{"senha_atual": "SenhaAtual999!", "nova_senha": "SenhaNovaForte1!"}))
			assert.Equal(t, http.StatusInternalServerError, status)
			assert.Equal(t, false, body["success"])
			assert.NoError(t, mock.ExpectationsWereMet(), "rollback, sem commit")
		})
	}
}
