package handlers_test

// SEC-07: comportamento do POST /api/auth/refresh com token revogado, de
// acordo com refresh_tokens.revoked_reason.
//   - fora da janela de graça + motivo "rotacao": reuso de token já
//     rotacionado → alerta [auth][seguranca] + revogação de todas as sessões
//     do usuário (motivo revogacao_massa); 401 igual; conta no rate limit.
//   - fora da janela + logout/revogacao_massa/senha/inativacao/NULL: 401
//     igual, log informativo com user_id e motivo, sem alerta e sem revogação
//     em massa; conta no rate limit (como antes).
//   - dentro da janela: inalterado (SEC-04) — sem alerta, sem revogação, não
//     conta no rate limit.

import (
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const revokeAllUsuarioSQL = `UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE usuario_id = \? AND revoked_at IS NULL`

// expectRefreshRevogadoComMotivo programa a busca do token id=10 do
// usuario_id=5, revogado em revokedAt com o motivo informado (nil = NULL).
func expectRefreshRevogadoComMotivo(mock sqlmock.Sqlmock, revokedAt time.Time, motivo any) {
	mock.ExpectQuery(findRefreshSQL).
		WithArgs(hashRefresh(refreshTokenTexto)).
		WillReturnRows(sqlmock.NewRows(refreshTokenCols).
			AddRow(int64(10), int64(5), hashRefresh(refreshTokenTexto), time.Now().Add(time.Hour), revokedAt, "127.0.0.1", "go-test", motivo))
}

func TestSEC07_ReusoDeTokenRotacionado_ForaDaJanela_RevogaTudoEAlerta(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	logs := capturarLog(t)

	expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "rotacao")
	mock.ExpectExec(revokeAllUsuarioSQL).
		WithArgs(sqlmock.AnyArg(), "revogacao_massa", int64(5)).
		WillReturnResult(sqlmock.NewResult(0, 2))

	assertRefreshRevogado401(t, server.URL+"/api/auth/refresh")

	assert.NoError(t, mock.ExpectationsWereMet(), "deve revogar todas as sessões do usuário")
	out := logs.String()
	assert.Contains(t, out, "[auth][seguranca]")
	assert.Contains(t, out, "user_id=5")
	assert.Contains(t, out, "token_id=10")
	assert.Contains(t, out, "ip=")
	assert.Contains(t, out, "ua=")
}

func TestSEC07_ReusoDeTokenRotacionado_FalhaNaRevogacaoEmMassa_Loga401(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	logs := capturarLog(t)

	expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "rotacao")
	mock.ExpectExec(revokeAllUsuarioSQL).
		WithArgs(sqlmock.AnyArg(), "revogacao_massa", int64(5)).
		WillReturnError(sqlmock.ErrCancelled)

	assertRefreshRevogado401(t, server.URL+"/api/auth/refresh")

	assert.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, logs.String(), "falha na revogação em massa após reuso: user_id=5")
}

func TestSEC07_ReusoDeTokenRotacionado_ContaNoRateLimit(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	capturarLog(t)
	url := server.URL + "/api/auth/refresh"

	for i := 0; i < 10; i++ {
		expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "rotacao")
		mock.ExpectExec(revokeAllUsuarioSQL).
			WithArgs(sqlmock.AnyArg(), "revogacao_massa", int64(5)).
			WillReturnResult(sqlmock.NewResult(0, 0))
		assertRefreshRevogado401(t, url)
	}
	require.NoError(t, mock.ExpectationsWereMet())

	resp := postRefresh(t, url, map[string]string{"refresh_token": refreshTokenTexto}, "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet(), "bloqueado sem consultar o banco")
}

func TestSEC07_TokenRevogadoPorOutroMotivo_ForaDaJanela_SemAlertaNemRevogacaoEmMassa(t *testing.T) {
	casos := []struct {
		nome      string
		motivo    any
		wantNoLog string
	}{
		{"logout", "logout", "motivo=logout"},
		{"inativacao", "inativacao", "motivo=inativacao"},
		{"senha", "senha", "motivo=senha"},
		{"revogacao_massa", "revogacao_massa", "motivo=revogacao_massa"},
		{"NULL (legado, conservador)", nil, "motivo=desconhecido(legado)"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			logs := capturarLog(t)

			expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), c.motivo)
			// Sem ExpectExec: uma revogação em massa daria erro no sqlmock
			// e seria logada como [auth][seguranca].

			raw := assertRefreshRevogado401(t, server.URL+"/api/auth/refresh")

			assert.NoError(t, mock.ExpectationsWereMet())
			out := logs.String()
			assert.NotContains(t, out, "[auth][seguranca]", "sem alerta de segurança")
			assert.Contains(t, out, "user_id=5")
			assert.Contains(t, out, c.wantNoLog)
			assert.Contains(t, raw, "refresh token revogado")
		})
	}
}

func TestSEC07_TokenRevogadoPorLogout_ForaDaJanela_ContinuaContandoNoRateLimit(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	capturarLog(t)
	url := server.URL + "/api/auth/refresh"

	for i := 0; i < 10; i++ {
		expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "logout")
		assertRefreshRevogado401(t, url)
	}

	resp := postRefresh(t, url, map[string]string{"refresh_token": refreshTokenTexto}, "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC07_TokenRotacionado_DentroDaJanela_SemAlertaSemRevogacaoNaoConta(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	logs := capturarLog(t)
	url := server.URL + "/api/auth/refresh"

	for i := 0; i < 15; i++ {
		expectRefreshRevogadoComMotivo(mock, time.Now().Add(-5*time.Second), "rotacao")
		assertRefreshRevogado401(t, url)
	}
	require.NoError(t, mock.ExpectationsWereMet(), "nenhuma revogação em massa dentro da janela")
	assert.NotContains(t, logs.String(), "[auth][seguranca]")

	// Não contou no rate limit: um refresh válido em seguida recebe 200.
	expectRefreshSucesso(mock)
	assertRefreshOK(t, url)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC07_RespostaHTTPIgualEntreReusoEOutrosMotivos(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	capturarLog(t)
	url := server.URL + "/api/auth/refresh"

	expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "rotacao")
	mock.ExpectExec(revokeAllUsuarioSQL).WillReturnResult(sqlmock.NewResult(0, 1))
	statusReuso, corpoReuso := refreshSemEmissao(t, url)

	expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "logout")
	statusLogout, corpoLogout := refreshSemEmissao(t, url)

	assert.Equal(t, http.StatusUnauthorized, statusReuso)
	assert.Equal(t, statusReuso, statusLogout)
	assert.Equal(t, corpoReuso, corpoLogout, "o cliente não pode distinguir reuso de outros motivos")
	assert.NoError(t, mock.ExpectationsWereMet())
}
