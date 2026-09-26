package handlers_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BUG-06: PATCH .../{id}/inativar com body inválido não pode alternar o
// estado. Contrato (🟣 SecBrain):
//   - vazio, null, {} ou {"ativo":null} → toggle;
//   - {"ativo":true|false} → define o valor;
//   - qualquer outro erro de decode → 400 "body JSON inválido" sem chamar o
//     service (nenhuma query/UPDATE no banco).

type inativarEndpoint struct {
	nome string
	path string
	// expectToggle registra o SELECT do estado atual (ativo=true) e o UPDATE
	// com o valor final esperado.
	expectToggle func(mock sqlmock.Sqlmock, valorFinal bool)
}

func inativarEndpoints() []inativarEndpoint {
	return []inativarEndpoint{
		{
			nome: "clientes",
			path: "/api/clientes/1/inativar",
			expectToggle: func(mock sqlmock.Sqlmock, valorFinal bool) {
				mock.ExpectQuery(`SELECT ` + clienteColunasRegex + ` FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(1)).WillReturnRows(clienteRowsForHandler())
				mock.ExpectExec(`UPDATE clientes SET ativo = \? WHERE cliente_id_origem = \?`).
					WithArgs(valorFinal, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			nome: "produtos",
			path: "/api/produtos/1/inativar",
			expectToggle: func(mock sqlmock.Sqlmock, valorFinal bool) {
				mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).WillReturnRows(produtoRowsForHandler())
				mock.ExpectExec(`UPDATE produtos SET ativo = \? WHERE id = \?`).
					WithArgs(valorFinal, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			nome: "usuarios",
			path: "/api/usuarios/1/inativar",
			expectToggle: func(mock sqlmock.Sqlmock, valorFinal bool) {
				mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
					WithArgs(int64(1)).WillReturnRows(usuarioRowsForHandler(1, true))
				mock.ExpectExec(`UPDATE usuarios SET ativo = \? WHERE id = \?`).
					WithArgs(valorFinal, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
				if !valorFinal { // SEC-06: inativação revoga os refresh tokens
					mock.ExpectExec(revokeAllByUserRegex).
						WithArgs(sqlmock.AnyArg(), "inativacao", int64(1)).WillReturnResult(sqlmock.NewResult(0, 0))
				}
			},
		},
	}
}

func doPatchInativar(t *testing.T, url, body string, semBody bool) (int, map[string]any) {
	t.Helper()
	var rdr io.Reader
	if !semBody {
		rdr = bytes.NewBufferString(body)
	}
	req, err := http.NewRequest(http.MethodPatch, url, rdr)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, decodeResponse(t, readBody(t, resp))
}

// TestInativar_BodyValido_ToggleOuDefine: bodies aceitos. O estado atual é
// ativo=true em todos os mocks: toggle → false; explícito → o valor enviado.
func TestInativar_BodyValido_ToggleOuDefine(t *testing.T) {
	bodies := []struct {
		nome      string
		body      string
		semBody   bool
		wantAtivo bool
	}{
		{nome: "sem body (nil)", semBody: true, wantAtivo: false},
		{nome: "body vazio", body: "", wantAtivo: false},
		{nome: "só espaços", body: "  \n ", wantAtivo: false},
		{nome: "null", body: "null", wantAtivo: false},
		{nome: "objeto vazio", body: "{}", wantAtivo: false},
		{nome: "ativo null", body: `{"ativo":null}`, wantAtivo: false},
		{nome: "ativo true", body: `{"ativo":true}`, wantAtivo: true},
		{nome: "ativo false", body: `{"ativo":false}`, wantAtivo: false},
	}
	for _, ep := range inativarEndpoints() {
		for _, b := range bodies {
			t.Run(ep.nome+"/"+b.nome, func(t *testing.T) {
				server, db, mock := setupTestServer(t)
				defer server.Close()
				defer db.Close()
				ep.expectToggle(mock, b.wantAtivo)

				status, body := doPatchInativar(t, server.URL+ep.path, b.body, b.semBody)

				assert.Equal(t, http.StatusOK, status)
				data := body["data"].(map[string]any)
				assert.Equal(t, b.wantAtivo, data["ativo"])
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

// TestInativar_BodyInvalido_400SemChamarService: bodies preenchidos e
// inválidos → 400 "body JSON inválido", sem nenhuma query ao banco.
func TestInativar_BodyInvalido_400SemChamarService(t *testing.T) {
	bodies := []struct{ nome, body string }{
		{"json malformado", `{"ativo":`},
		{"texto solto", `abc`},
		{"ativo string false", `{"ativo":"false"}`},
		{"ativo string x", `{"ativo":"x"}`},
		{"ativo número", `{"ativo":1}`},
		{"array", `[true]`},
		{"booleano solto", `true`},
		{"lixo após o objeto", `{"ativo":true} xyz`},
		{"dois objetos", `{} {}`},
	}
	for _, ep := range inativarEndpoints() {
		for _, b := range bodies {
			t.Run(ep.nome+"/"+b.nome, func(t *testing.T) {
				server, db, mock := setupTestServer(t)
				defer server.Close()
				defer db.Close()

				status, body := doPatchInativar(t, server.URL+ep.path, b.body, false)

				assert.Equal(t, http.StatusBadRequest, status)
				assert.Equal(t, false, body["success"])
				assert.Nil(t, body["data"])
				assert.Equal(t, "body JSON inválido", body["error"])
				assert.NoError(t, mock.ExpectationsWereMet(), "o service não pode ser chamado")
			})
		}
	}
}
