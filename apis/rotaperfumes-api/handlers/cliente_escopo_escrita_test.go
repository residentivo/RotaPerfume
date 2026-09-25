package handlers_test

// Testes básicos do SEC-01 (IDOR em Create/Update/Toggle de clientes).
// Cobertura completa fica a cargo do 🔴 TestBrain.

import (
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const reInsertCarteira = `INSERT INTO carteiras \(cliente_id, vendedor_id, data_inicio, data_fim\)`

const reInsertCliente = `INSERT INTO clientes \(cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo\)`

// doClienteReq executa a requisição autenticada como usuário normal (id 2)
// e devolve status + body decodificado.
func doClienteReq(t *testing.T, serverURL, method, path string, payload map[string]any) (int, map[string]any) {
	t.Helper()
	token := generateToken(t, testCfg(), 2, "normal")
	var req *http.Request
	if payload != nil {
		req, _ = http.NewRequest(method, serverURL+path, makeJSON(payload))
	} else {
		req, _ = http.NewRequest(method, serverURL+path, nil)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, decodeResponse(t, readBody(t, resp))
}

func TestSEC01_UpdateToggle_ForaDaCarteira_404SemTocarNoCliente(t *testing.T) {
	casos := []struct {
		nome, method, path string
		payload            map[string]any
	}{
		{"update", "PUT", "/api/clientes/1", validClientePayload()},
		{"toggle", "PATCH", "/api/clientes/1/inativar", map[string]any{"ativo": false}},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			expectEscopoUsuarioH(mock, 2, 10)
			expectCarteiraInexistenteH(mock, 10, 1)

			status, body := doClienteReq(t, server.URL, tc.method, tc.path, tc.payload)
			assert.Equal(t, http.StatusNotFound, status)
			assert.Equal(t, "cliente não encontrado", body["error"])
			// Nenhum SELECT/UPDATE em clientes pode ter ocorrido.
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSEC01_UpdateToggle_ErroAoChecarCarteira_500(t *testing.T) {
	casos := []struct{ nome, method, path string }{
		{"update", "PUT", "/api/clientes/1"},
		{"toggle", "PATCH", "/api/clientes/1/inativar"},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			expectEscopoUsuarioH(mock, 2, 10)
			mock.ExpectQuery(carteiraAtivaRegexH).WithArgs(int64(10), int64(1)).
				WillReturnError(errors.New("db down"))

			status, body := doClienteReq(t, server.URL, tc.method, tc.path, validClientePayload())
			assert.Equal(t, http.StatusInternalServerError, status)
			assert.Equal(t, "erro interno", body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSEC01_UpdateToggle_SemVendedor_404(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{"PUT", "/api/clientes/1"},
		{"PATCH", "/api/clientes/1/inativar"},
	} {
		t.Run(tc.method, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			expectEscopoSemVendedorH(mock, 2)

			status, body := doClienteReq(t, server.URL, tc.method, tc.path, validClientePayload())
			assert.Equal(t, http.StatusNotFound, status)
			assert.Equal(t, "cliente não encontrado", body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSEC01_UpdateToggle_IDInvalido_400AntesDoEscopo(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	status, _ := doClienteReq(t, server.URL, "PUT", "/api/clientes/abc", validClientePayload())
	assert.Equal(t, http.StatusBadRequest, status)
	assert.NoError(t, mock.ExpectationsWereMet(), "id inválido não pode consultar o banco")
}

func TestSEC01_Create_SemVendedor_403(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	expectEscopoSemVendedorH(mock, 2)

	status, body := doClienteReq(t, server.URL, "POST", "/api/clientes", validClientePayload())
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, "usuário sem vendedor vinculado", body["error"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC01_Create_FalhaNaCarteira_Rollback500(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	expectEscopoUsuarioH(mock, 2, 10)
	mock.ExpectBegin()
	mock.ExpectExec(reInsertCliente).WillReturnResult(sqlmock.NewResult(101, 1))
	mock.ExpectExec(reInsertCarteira).
		WithArgs(int64(101), int64(10), sqlmock.AnyArg(), nil).
		WillReturnError(errors.New("fk violada"))
	mock.ExpectRollback()

	status, body := doClienteReq(t, server.URL, "POST", "/api/clientes", validClientePayload())
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "erro interno", body["error"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC01_Create_ValidacaoAntesDaTransacao_400(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	expectEscopoUsuarioH(mock, 2, 10)
	payload := validClientePayload()
	payload["razao_social"] = ""

	status, _ := doClienteReq(t, server.URL, "POST", "/api/clientes", payload)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.NoError(t, mock.ExpectationsWereMet(), "validação falha antes do BEGIN")
}
