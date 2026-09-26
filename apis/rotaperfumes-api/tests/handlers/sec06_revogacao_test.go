package handlers_test

// SEC-06: inativar um usuário (PATCH /api/usuarios/{id}/inativar) revoga
// todos os refresh tokens dele; ativar não revoga. Falha na revogação é
// logada (não engolida) e não desfaz a inativação.

import (
	"bytes"
	"log"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const revokeAllByUserRegex = `UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE usuario_id = \? AND revoked_at IS NULL`

// capturarLog redireciona o log padrão para um buffer durante o teste.
func capturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	anterior := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(anterior) })
	return &buf
}

func patchInativarUsuario(t *testing.T, url string, body map[string]any) (int, map[string]any) {
	t.Helper()
	var req *http.Request
	if body == nil {
		req, _ = http.NewRequest("PATCH", url, nil)
	} else {
		req, _ = http.NewRequest("PATCH", url, makeJSON(body))
	}
	req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, decodeResponse(t, readBody(t, resp))
}

func TestSEC06_InativarUsuario_RevogaRefreshTokens(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	logs := capturarLog(t)

	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(5)).WillReturnRows(usuarioRowsForHandler(5, true))
	mock.ExpectExec(`UPDATE usuarios SET ativo = \? WHERE id = \?`).
		WithArgs(false, int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(revokeAllByUserRegex).
		WithArgs(sqlmock.AnyArg(), "inativacao", int64(5)).WillReturnResult(sqlmock.NewResult(0, 3))

	status, body := patchInativarUsuario(t, server.URL+"/api/usuarios/5/inativar", map[string]any{"ativo": false})

	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, body["data"].(map[string]any)["ativo"])
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.NotContains(t, logs.String(), "falha ao revogar")
}

func TestSEC06_ToggleParaInativo_RevogaRefreshTokens(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(5)).WillReturnRows(usuarioRowsForHandler(5, true))
	mock.ExpectExec(`UPDATE usuarios SET ativo = \? WHERE id = \?`).
		WithArgs(false, int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(revokeAllByUserRegex).
		WithArgs(sqlmock.AnyArg(), "inativacao", int64(5)).WillReturnResult(sqlmock.NewResult(0, 0))

	status, _ := patchInativarUsuario(t, server.URL+"/api/usuarios/5/inativar", nil)

	assert.Equal(t, http.StatusOK, status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC06_AtivarUsuario_NaoRevoga(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	logs := capturarLog(t)

	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(5)).WillReturnRows(usuarioRowsForHandler(5, false))
	mock.ExpectExec(`UPDATE usuarios SET ativo = \? WHERE id = \?`).
		WithArgs(true, int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	// Nenhuma expectativa de revogação: se o handler tentasse revogar, o
	// sqlmock devolveria erro e o handler logaria "falha ao revogar".

	status, body := patchInativarUsuario(t, server.URL+"/api/usuarios/5/inativar", map[string]any{"ativo": true})

	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, body["data"].(map[string]any)["ativo"])
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.NotContains(t, logs.String(), "revogar")
}

func TestSEC06_InativarUsuario_FalhaNaRevogacao_LogaESegue200(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	logs := capturarLog(t)

	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(5)).WillReturnRows(usuarioRowsForHandler(5, true))
	mock.ExpectExec(`UPDATE usuarios SET ativo = \? WHERE id = \?`).
		WithArgs(false, int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(revokeAllByUserRegex).
		WithArgs(sqlmock.AnyArg(), "inativacao", int64(5)).WillReturnError(sqlmock.ErrCancelled)

	status, _ := patchInativarUsuario(t, server.URL+"/api/usuarios/5/inativar", map[string]any{"ativo": false})

	assert.Equal(t, http.StatusOK, status, "inativação já gravada; o middleware bloqueia o usuário inativo")
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, logs.String(), "falha ao revogar refresh tokens do usuario_id=5")
}
