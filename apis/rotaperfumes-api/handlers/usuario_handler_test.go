// Package handlers_test contém testes de integração dos handlers HTTP.
package handlers_test

import (
	"bytes"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// usuarioSelectRegex reflete a query usuarioSelectComVendedor do repositório.
const usuarioSelectRegex = `SELECT u\.id, u\.nome, u\.email, u\.password_hash, u\.role, u\.id_vendedor, u\.ativo,\s+u\.deve_trocar_senha, u\.created_at, u\.updated_at, u\.ultimo_login_at, v\.nome\s+FROM usuarios u\s+LEFT JOIN vendedores v ON v\.id = u\.id_vendedor`

func usuarioRowsForHandler(id int64, ativo bool) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo",
		"deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome",
	}).AddRow(id, "Usuário Teste", "usuario@test.com", "hash", "normal", nil, ativo, false, now, now, nil, nil)
}

func emptyUsuarioRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo",
		"deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome",
	})
}

// ---------------------------------------------------------------------------
// ListUsuarios GET /api/usuarios
// ---------------------------------------------------------------------------

func TestListUsuarios_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM usuarios`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(usuarioSelectRegex+`\s+ORDER BY u\.id ASC\s+LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(usuarioRowsForHandler(1, true))

	req, _ := http.NewRequest("GET", server.URL+"/api/usuarios", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].([]any)
	require.Len(t, data, 1)
	item := data[0].(map[string]any)
	assert.Equal(t, "usuario@test.com", item["email"])
	_, hasHash := item["password_hash"]
	assert.False(t, hasHash, "password_hash nunca deve ser exposto")

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListUsuarios_OrderBy(t *testing.T) {
	testCases := []struct {
		nome        string
		query       string
		orderRegexp string
	}{
		{"order_by e order_dir válidos", "order_by=nome&order_dir=desc", `ORDER BY u\.nome DESC`},
		{"order_dir inválido cai no default (asc)", "order_by=email&order_dir=sideways", `ORDER BY u\.email ASC`},
		{"order_by fora da whitelist cai no default", "order_by=password_hash&order_dir=asc", `ORDER BY u\.id ASC`},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM usuarios`).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			mock.ExpectQuery(usuarioSelectRegex+`\s+`+tc.orderRegexp+`\s+LIMIT \? OFFSET \?`).
				WithArgs(20, 0).
				WillReturnRows(usuarioRowsForHandler(1, true))

			req, _ := http.NewRequest("GET", server.URL+"/api/usuarios?"+tc.query, nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestListUsuarios_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/usuarios", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "acesso restrito a administradores", body["error"])
}

func TestListUsuarios_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM usuarios`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/usuarios", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CreateUsuario POST /api/usuarios
// ---------------------------------------------------------------------------

func validCreateUsuarioPayload() map[string]any {
	return map[string]any{
		"nome":  "Novo Usuário",
		"email": "novo@test.com",
		"role":  "normal",
	}
}

func TestCreateUsuario_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`INSERT INTO usuarios \(nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha\)`).
		WithArgs("Novo Usuário", "novo@test.com", sqlmock.AnyArg(), "normal", nil, true, true).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(usuarioRowsForHandler(1, true))

	req, _ := http.NewRequest("POST", server.URL+"/api/usuarios", makeJSON(validCreateUsuarioPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "usuario@test.com", data["email"])
	assert.Equal(t, true, data["email_enviado"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateUsuario_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("POST", server.URL+"/api/usuarios", makeJSON(validCreateUsuarioPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestCreateUsuario_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/usuarios", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestCreateUsuario_ValidacaoNegocio(t *testing.T) {
	testCases := []struct {
		nome    string
		payload func() map[string]any
		wantMsg string
	}{
		{
			nome: "nome vazio",
			payload: func() map[string]any {
				p := validCreateUsuarioPayload()
				p["nome"] = ""
				return p
			},
			wantMsg: "nome é obrigatório",
		},
		{
			nome: "email vazio",
			payload: func() map[string]any {
				p := validCreateUsuarioPayload()
				p["email"] = ""
				return p
			},
			wantMsg: "email é obrigatório",
		},
		{
			nome: "role inválido",
			payload: func() map[string]any {
				p := validCreateUsuarioPayload()
				p["role"] = "superadmin"
				return p
			},
			wantMsg: "role deve ser 'admin' ou 'normal'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, _ := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			req, _ := http.NewRequest("POST", server.URL+"/api/usuarios", makeJSON(tc.payload()))
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, tc.wantMsg, body["error"])
		})
	}
}

func TestCreateUsuario_EmailDuplicado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`INSERT INTO usuarios \(nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha\)`).
		WithArgs("Novo Usuário", "novo@test.com", sqlmock.AnyArg(), "normal", nil, true, true).
		WillReturnError(errServerf("Error 1062: Duplicate entry 'novo@test.com' for key 'email'"))

	req, _ := http.NewRequest("POST", server.URL+"/api/usuarios", makeJSON(validCreateUsuarioPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "email já cadastrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateUsuario_VendedorNaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}))

	payload := validCreateUsuarioPayload()
	payload["id_vendedor"] = 99

	req, _ := http.NewRequest("POST", server.URL+"/api/usuarios", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "vendedor não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateUsuario_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`INSERT INTO usuarios \(nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha\)`).
		WithArgs("Novo Usuário", "novo@test.com", sqlmock.AnyArg(), "normal", nil, true, true).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("POST", server.URL+"/api/usuarios", makeJSON(validCreateUsuarioPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// UpdateUsuario PUT /api/usuarios/{id}
// ---------------------------------------------------------------------------

func validUpdateUsuarioPayload() map[string]any {
	return map[string]any{
		"nome": "Usuário Atualizado",
		"role": "admin",
	}
}

func TestUpdateUsuario_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE usuarios SET nome = \?, role = \?, id_vendedor = \? WHERE id = \?`).
		WithArgs("Usuário Atualizado", "admin", nil, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(usuarioRowsForHandler(1, true))

	req, _ := http.NewRequest("PUT", server.URL+"/api/usuarios/1", makeJSON(validUpdateUsuarioPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateUsuario_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("PUT", server.URL+"/api/usuarios/1", makeJSON(validUpdateUsuarioPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestUpdateUsuario_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/usuarios/abc", makeJSON(validUpdateUsuarioPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestUpdateUsuario_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/usuarios/1", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestUpdateUsuario_ValidacaoNegocio(t *testing.T) {
	testCases := []struct {
		nome    string
		payload func() map[string]any
		wantMsg string
	}{
		{
			nome: "nome vazio",
			payload: func() map[string]any {
				p := validUpdateUsuarioPayload()
				p["nome"] = ""
				return p
			},
			wantMsg: "nome é obrigatório",
		},
		{
			nome: "role inválido",
			payload: func() map[string]any {
				p := validUpdateUsuarioPayload()
				p["role"] = "superadmin"
				return p
			},
			wantMsg: "role deve ser 'admin' ou 'normal'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, _ := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			req, _ := http.NewRequest("PUT", server.URL+"/api/usuarios/1", makeJSON(tc.payload()))
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, tc.wantMsg, body["error"])
		})
	}
}

func TestUpdateUsuario_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE usuarios SET nome = \?, role = \?, id_vendedor = \? WHERE id = \?`).
		WithArgs("Usuário Atualizado", "admin", nil, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req, _ := http.NewRequest("PUT", server.URL+"/api/usuarios/999", makeJSON(validUpdateUsuarioPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "usuário não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateUsuario_VendedorNaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}))

	payload := validUpdateUsuarioPayload()
	payload["id_vendedor"] = 99

	req, _ := http.NewRequest("PUT", server.URL+"/api/usuarios/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "vendedor não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateUsuario_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE usuarios SET nome = \?, role = \?, id_vendedor = \? WHERE id = \?`).
		WithArgs("Usuário Atualizado", "admin", nil, int64(1)).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("PUT", server.URL+"/api/usuarios/1", makeJSON(validUpdateUsuarioPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// ToggleAtivoUsuario PATCH /api/usuarios/{id}/inativar
// ---------------------------------------------------------------------------

func TestToggleAtivoUsuario_Toggle_SemBody(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(usuarioRowsForHandler(1, true))
	mock.ExpectExec(`UPDATE usuarios SET ativo = \? WHERE id = \?`).
		WithArgs(false, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// SEC-06: inativação revoga os refresh tokens do usuário.
	mock.ExpectExec(revokeAllByUserRegex).
		WithArgs(sqlmock.AnyArg(), "inativacao", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req, _ := http.NewRequest("PATCH", server.URL+"/api/usuarios/1/inativar", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].(map[string]any)
	assert.Equal(t, false, data["ativo"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestToggleAtivoUsuario_ComBodyExplicito(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(usuarioRowsForHandler(1, true))
	mock.ExpectExec(`UPDATE usuarios SET ativo = \? WHERE id = \?`).
		WithArgs(true, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req, _ := http.NewRequest("PATCH", server.URL+"/api/usuarios/1/inativar", makeJSON(map[string]any{"ativo": true}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].(map[string]any)
	assert.Equal(t, true, data["ativo"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestToggleAtivoUsuario_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnRows(emptyUsuarioRows())

	req, _ := http.NewRequest("PATCH", server.URL+"/api/usuarios/999/inativar", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "usuário não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestToggleAtivoUsuario_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PATCH", server.URL+"/api/usuarios/abc/inativar", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestToggleAtivoUsuario_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("PATCH", server.URL+"/api/usuarios/1/inativar", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestToggleAtivoUsuario_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(usuarioRowsForHandler(1, true))
	mock.ExpectExec(`UPDATE usuarios SET ativo = \? WHERE id = \?`).
		WithArgs(false, int64(1)).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("PATCH", server.URL+"/api/usuarios/1/inativar", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// AdminResetPassword POST /api/admin/reset-password
// ---------------------------------------------------------------------------

func TestAdminResetPassword_Success_ViaUsuarioHandler(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// GetByID #1: UsuarioHandler.AdminResetPassword busca o alvo (email/nome
	// para auditoria e envio de email).
	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(5)).
		WillReturnRows(usuarioRowsForHandler(5, true))
	// GetByID #2: UsuarioService.AdminResetPassword busca o mesmo usuário de
	// novo internamente, para comparar a senha aleatória gerada com o hash
	// atual e evitar colisão (defesa em profundidade).
	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(5)).
		WillReturnRows(usuarioRowsForHandler(5, true))
	mock.ExpectExec(`UPDATE usuarios SET password_hash`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO senha_historico`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE usuario_id = \?`).WithArgs(sqlmock.AnyArg(), "senha", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req, _ := http.NewRequest("POST", server.URL+"/api/admin/reset-password", makeJSON(map[string]any{"usuario_id": 5}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].(map[string]any)
	assert.Equal(t, true, data["sucesso"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminResetPassword_Forbidden_NaoAdmin(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("POST", server.URL+"/api/admin/reset-password", makeJSON(map[string]any{"usuario_id": 5}))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestAdminResetPassword_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/admin/reset-password", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestAdminResetPassword_UsuarioIDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/admin/reset-password", makeJSON(map[string]any{"usuario_id": 0}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "usuario_id é obrigatório e deve ser > 0", body["error"])
}

func TestAdminResetPassword_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnRows(emptyUsuarioRows())

	req, _ := http.NewRequest("POST", server.URL+"/api/admin/reset-password", makeJSON(map[string]any{"usuario_id": 999}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "usuário não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminResetPassword_ErroInterno_Busca(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(5)).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("POST", server.URL+"/api/admin/reset-password", makeJSON(map[string]any{"usuario_id": 5}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminResetPassword_ErroInterno_Update(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// GetByID #1: UsuarioHandler.AdminResetPassword busca o alvo (email/nome
	// para auditoria e envio de email).
	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(5)).
		WillReturnRows(usuarioRowsForHandler(5, true))
	// GetByID #2: UsuarioService.AdminResetPassword busca o mesmo usuário de
	// novo internamente, para comparar a senha aleatória gerada com o hash
	// atual e evitar colisão (defesa em profundidade).
	mock.ExpectQuery(usuarioSelectRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
		WithArgs(int64(5)).
		WillReturnRows(usuarioRowsForHandler(5, true))
	mock.ExpectExec(`UPDATE usuarios SET password_hash`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("POST", server.URL+"/api/admin/reset-password", makeJSON(map[string]any{"usuario_id": 5}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// errServerf cria um error simples a partir de uma string (evita depender de
// fmt/errors extras no topo do arquivo só para os testes de erro de driver).
func errServerf(msg string) error {
	return &driverErr{msg}
}

type driverErr struct{ msg string }

func (e *driverErr) Error() string { return e.msg }
