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

	expectToggleUsuarioTxH(mock, 5, false, false)

	status, body := patchInativarUsuario(t, server.URL+"/api/usuarios/5/inativar", map[string]any{"ativo": false})

	assert.Equal(t, http.StatusOK, status)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "data deve ser objeto: %v", body)
	assert.Equal(t, false, data["ativo"])
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.NotContains(t, logs.String(), "falha ao revogar")
}

func TestSEC06_ToggleParaInativo_RevogaRefreshTokens(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	expectToggleUsuarioTxH(mock, 5, true, false)

	status, _ := patchInativarUsuario(t, server.URL+"/api/usuarios/5/inativar", nil)

	assert.Equal(t, http.StatusOK, status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC06_AtivarUsuario_NaoRevoga(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	logs := capturarLog(t)

	// Nenhuma expectativa de revogação: se o caso de uso tentasse revogar,
	// o sqlmock devolveria erro (rollback e 500).
	expectToggleUsuarioTxH(mock, 5, false, true)

	status, body := patchInativarUsuario(t, server.URL+"/api/usuarios/5/inativar", map[string]any{"ativo": true})

	assert.Equal(t, http.StatusOK, status)
	data, ok := body["data"].(map[string]any)
	require.True(t, ok, "data deve ser objeto: %v", body)
	assert.Equal(t, true, data["ativo"])
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.NotContains(t, logs.String(), "revogar")
}

// BUG-22 (Lote 31): a revogação entra na tx da inativação; falha desfaz
// tudo (rollback) e responde 500. Antes (SEC-06) só logava e seguia 200.
func TestSEC06_InativarUsuario_FalhaNaRevogacao_RollbackE500(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	logs := capturarLog(t)

	mock.ExpectBegin()
	expectUpdateAtivoUsuarioH(mock, 5, false, false)
	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(5)).WillReturnRows(usuarioRowsForHandler(5, false))
	mock.ExpectExec(revokeAllByUserRegex).
		WithArgs(sqlmock.AnyArg(), "inativacao", int64(5)).WillReturnError(sqlmock.ErrCancelled)
	mock.ExpectRollback()

	status, _ := patchInativarUsuario(t, server.URL+"/api/usuarios/5/inativar", map[string]any{"ativo": false})

	assert.Equal(t, http.StatusInternalServerError, status, "a inativação não fica gravada sem a revogação")
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.NotContains(t, logs.String(), "[refresh] todos tokens revogados: usuario_id=5", "log de sucesso só depois do commit")
}
