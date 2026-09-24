package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// msgVendedorDesligadoH espelha handlers.msgVendedorDesligado (contrato SecBrain).
const msgVendedorDesligadoH = "acesso bloqueado: vendedor desligado"

// expectEscopoDesligado prepara o escopo de um usuário normal (id 2)
// vinculado a um vendedor desligado.
func expectEscopoDesligado(mock sqlmock.Sqlmock, vendedorID int64) {
	mock.ExpectQuery(reUsuarioVendedor).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(vendedorID))
	expectVendedorDesligado(mock, vendedorID, true)
}

// TestEscopo_VendedorDesligado_Bloqueia403 garante que rotas de negócio com
// escopo respondem 403 com a mensagem exata para vendedor desligado, sem
// buscar o registro nem decodificar o body.
func TestEscopo_VendedorDesligado_Bloqueia403(t *testing.T) {
	casos := []struct {
		metodo string
		path   string
	}{
		{"GET", "/api/clientes"},
		{"GET", "/api/clientes/1"},
		{"POST", "/api/clientes"},
		{"PUT", "/api/clientes/1"},
		{"PATCH", "/api/clientes/1/inativar"},
		{"GET", "/api/pedidos"},
		{"DELETE", "/api/pedidos/1"},
		{"GET", "/api/pagamentos/1"},
		{"PUT", "/api/oportunidades/1"},
		{"POST", "/api/visitas"},
		{"GET", "/api/vendedores/10/clientes"},
	}

	for _, c := range casos {
		t.Run(c.metodo+" "+c.path, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			token := generateToken(t, testCfg(), 2, "normal")
			expectEscopoDesligado(mock, 10)

			req, _ := http.NewRequest(c.metodo, server.URL+c.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusForbidden, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, false, body["success"])
			assert.Equal(t, msgVendedorDesligadoH, body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestEscopo_ErroChecagemDesligado_500 garante fail-closed: erro de banco na
// checagem de desligamento vira 500 "erro interno".
func TestEscopo_ErroChecagemDesligado_500(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	token := generateToken(t, testCfg(), 2, "normal")
	mock.ExpectQuery(reUsuarioVendedor).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(10)))
	mock.ExpectQuery(reVendedorDesligado).
		WithArgs(int64(10)).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/pedidos", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "erro interno", body["error"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestMe_VendedorDesligado garante que GET /api/auth/me expõe
// vendedor_desligado=true para usuário vinculado a vendedor desligado.
func TestMe_VendedorDesligado(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	token := generateToken(t, testCfg(), 3, "normal")
	now := time.Now()
	mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v`).
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(int64(3), "João Silva", "joao@test.com", "hash", "normal", int64(10), true, false, now, now, &now, "Vendedor X"))
	expectVendedorDesligado(mock, 10, true)

	req, _ := http.NewRequest("GET", server.URL+"/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := decodeResponse(t, readBody(t, resp))["data"].(map[string]any)
	assert.Equal(t, true, data["vendedor_desligado"])
	assert.NoError(t, mock.ExpectationsWereMet())
}
