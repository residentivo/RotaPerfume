// Package handlers_test contém testes de integração dos handlers HTTP.
package handlers_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/routes"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/services"
)

// config de teste.
func testCfg() *config.Config {
	return &config.Config{
		JWTSecret:  "test-secret-auth-handler",
		JWTIssuer:  "rotaperfumes-test",
		JWTTTL:     24 * time.Hour,
		BCryptCost: 4,
	}
}

// generateToken cria um token JWT para testes.
func generateToken(t *testing.T, cfg *config.Config, uid int64, role string) string {
	auth := services.NewAuthService()
	tok, err := auth.GenerateJWT(cfg, uid, role)
	require.NoError(t, err)
	return tok
}

// makeJSON serializa body para *bytes.Buffer.
func makeJSON(body any) *bytes.Buffer {
	data, _ := json.Marshal(body)
	return bytes.NewBuffer(data)
}

func decodeResponse(t *testing.T, body []byte) map[string]any {
	var v map[string]any
	require.NoError(t, json.Unmarshal(body, &v))
	return v
}

func readBody(t *testing.T, resp *http.Response) []byte {
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(resp.Body)
	require.NoError(t, err)
	return buf.Bytes()
}

// setupTestServer cria um httptest.Server com o router real (routes.NewMux)
// e handlers reais, injetando um DB mockado via sqlmock.
func setupTestServer(t *testing.T) (*httptest.Server, *sql.DB, sqlmock.Sqlmock) {
	cfg := testCfg()

	// sqlmock para DB
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	// Handlers reais (recebem *sql.DB injetado)
	authHandler := handlers.NewAuthHandler(db, cfg)
	userHandler := handlers.NewUsuarioHandler(db, cfg)
	dashboardHandler := handlers.NewDashboardHandler(db, cfg)
	senhaHandler := handlers.NewSenhaHistoricoHandler(db)

	// Router real com middlewares corretos
	mux := routes.NewMux(cfg, authHandler, userHandler, dashboardHandler, senhaHandler)

	server := httptest.NewServer(mux)
	return server, db, mock
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

func TestLogin_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()

	// Mock: busca usuário admin@test.com
	// Tabela usuarios: id, nome, email, password_hash, role, id_vendedor, ativo, created_at, updated_at, ultimo_login_at
	hash, err := services.NewAuthService().HashPassword(cfg, "senha-correta")
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT id, nome, email, password_hash, role, id_vendedor, ativo, created_at, updated_at, ultimo_login_at FROM usuarios WHERE email = \? LIMIT 1`).
		WithArgs("admin@test.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "created_at", "updated_at", "ultimo_login_at"}).
			AddRow(int64(1), "Admin User", "admin@test.com", hash, "admin", nil, true, time.Now(), time.Now(), nil))

	// Mock: insert refresh_token
	mock.ExpectExec(`INSERT INTO refresh_tokens`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock: atualiza ultimo_login_at
	mock.ExpectExec(`UPDATE usuarios SET ultimo_login_at = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(map[string]string{"email": "admin@test.com", "password": "senha-correta"}))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)

	// Tokens agora vao via Set-Cookie (vulnerabilidade #1 mitigada).
	var foundAccess, foundRefresh bool
	for _, c := range resp.Cookies() {
		if c.Name == "access_token" && c.Value != "" {
			foundAccess = true
			assert.True(t, c.HttpOnly, "access_token cookie deve ser HttpOnly")
		}
		if c.Name == "refresh_token" && c.Value != "" {
			foundRefresh = true
			assert.True(t, c.HttpOnly, "refresh_token cookie deve ser HttpOnly")
		}
	}
	assert.True(t, foundAccess, "Set-Cookie access_token deve estar presente")
	assert.True(t, foundRefresh, "Set-Cookie refresh_token deve estar presente")

	// Garante que tokens NAO estao mais no body JSON.
	_, hasAccessBody := data["access_token"]
	_, hasRefreshBody := data["refresh_token"]
	assert.False(t, hasAccessBody, "access_token nao deve estar no body")
	assert.False(t, hasRefreshBody, "refresh_token nao deve estar no body")

	assert.Equal(t, float64(1), data["user"].(map[string]any)["id"])
	assert.Equal(t, "admin", data["user"].(map[string]any)["role"])

	// Verifica que todos os mocks foram chamados
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin_InvalidCredentials(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	hash, err := services.NewAuthService().HashPassword(cfg, "senha-correta")
	require.NoError(t, err)

	// Usuário existe, mas senha errada
	mock.ExpectQuery(`SELECT id, nome, email, password_hash, role, id_vendedor, ativo, created_at, updated_at, ultimo_login_at FROM usuarios WHERE email = \? LIMIT 1`).
		WithArgs("admin@test.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "created_at", "updated_at", "ultimo_login_at"}).
			AddRow(int64(1), "Admin User", "admin@test.com", hash, "admin", nil, true, time.Now(), time.Now(), nil))

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(map[string]string{"email": "admin@test.com", "password": "senha-errada"}))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.False(t, body["success"].(bool))
	assert.Equal(t, "credenciais inválidas", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin_UsuarioNaoExiste(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	// Usuário não encontrado
	mock.ExpectQuery(`SELECT id, nome, email, password_hash, role, id_vendedor, ativo, created_at, updated_at, ultimo_login_at FROM usuarios WHERE email = \? LIMIT 1`).
		WithArgs("naoexiste@test.com").
		WillReturnError(sql.ErrNoRows)

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(map[string]string{"email": "naoexiste@test.com", "password": "qualquer"}))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.False(t, body["success"].(bool))
	assert.Equal(t, "credenciais inválidas", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin_UsuarioInativo(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	hash, err := services.NewAuthService().HashPassword(cfg, "senha-correta")
	require.NoError(t, err)

	// Usuário existe mas inativo
	mock.ExpectQuery(`SELECT id, nome, email, password_hash, role, id_vendedor, ativo, created_at, updated_at, ultimo_login_at FROM usuarios WHERE email = \? LIMIT 1`).
		WithArgs("inativo@test.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "created_at", "updated_at", "ultimo_login_at"}).
			AddRow(int64(1), "Inativo User", "inativo@test.com", hash, "normal", nil, false, time.Now(), time.Now(), nil))

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(map[string]string{"email": "inativo@test.com", "password": "qualquer"}))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "usuário inativo", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin_EmptyBody(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		bytes.NewBufferString("{}"))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Contains(t, body["error"], "obrigatórios")
}

func TestLogin_InvalidJSON(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		bytes.NewBufferString("não é json"))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// ResetPassword
// ---------------------------------------------------------------------------

func TestResetPassword_UsuarioNormal_TrocaPropriaSenha(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	normalToken := generateToken(t, cfg, 2, "normal")

	hash, _ := services.NewAuthService().HashPassword(cfg, "senha-atual")
	// Mock: busca usuário (valida senha_atual)
	mock.ExpectQuery(`SELECT id, nome, email, password_hash, role, id_vendedor, ativo, created_at, updated_at, ultimo_login_at FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "created_at", "updated_at", "ultimo_login_at"}).
			AddRow(int64(2), "User 2", "user2@test.com", hash, "normal", nil, true, time.Now(), time.Now(), nil))

	// Mock: insert senha_historico
	mock.ExpectExec(`INSERT INTO senha_historico`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock: update password_hash
	mock.ExpectExec(`UPDATE usuarios SET password_hash = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock: revoga todos os refresh tokens
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// O endpoint /api/auth/reset-password usa senha_atual (não é admin-only).
	req, _ := http.NewRequest("POST", server.URL+"/api/auth/reset-password",
		makeJSON(map[string]any{"senha_atual": "senha-atual", "nova_senha": "nova-senha-123"}))
	req.Header.Set("Authorization", "Bearer "+normalToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAdminResetPassword_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// Mock: busca usuário alvo para capturar hash anterior
	hash, _ := services.NewAuthService().HashPassword(cfg, "senha-antiga")
	mock.ExpectQuery(`SELECT id, nome, email, password_hash, role, id_vendedor, ativo, created_at, updated_at, ultimo_login_at FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "created_at", "updated_at", "ultimo_login_at"}).
			AddRow(int64(5), "User 5", "user5@test.com", hash, "normal", nil, true, time.Now(), time.Now(), nil))

	// Mock: update password_hash (seta senha padrão "Mudar@123")
	mock.ExpectExec(`UPDATE usuarios SET password_hash`).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock: insert senha_historico
	mock.ExpectExec(`INSERT INTO senha_historico`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock: revoga todos os refresh tokens
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Admin reset: POST /api/admin/reset-password
	req, _ := http.NewRequest("POST", server.URL+"/api/admin/reset-password",
		makeJSON(map[string]any{"usuario_id": 5}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestResetPassword_SenhaCurta(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// senha_atual fornecida, mas nova_senha é curta.
	req, _ := http.NewRequest("POST", server.URL+"/api/auth/reset-password",
		makeJSON(map[string]any{"senha_atual": "senha-valida", "nova_senha": "abc"}))
	req.Header.Set("Authorization", "Bearer "+userToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Contains(t, body["error"], "pelo menos 6 caracteres")
}

// ---------------------------------------------------------------------------
// Me
// ---------------------------------------------------------------------------

func TestMe_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 3, "normal")

	now := time.Now()
	mock.ExpectQuery(`SELECT id, nome, email, password_hash, role, id_vendedor, ativo, created_at, updated_at, ultimo_login_at FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "created_at", "updated_at", "ultimo_login_at"}).
			AddRow(int64(3), "João Silva", "joao@test.com", "hash", "normal", nil, true, now, now, &now))

	req, _ := http.NewRequest("GET", server.URL+"/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(3), data["id"])
	assert.Equal(t, "João Silva", data["nome"])
	assert.Equal(t, "normal", data["role"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMe_NaoAutenticado(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	req, _ := http.NewRequest("GET", server.URL+"/api/auth/me", nil)
	// Sem Authorization header.

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Contains(t, body["error"], "authorization header ausente")
}
