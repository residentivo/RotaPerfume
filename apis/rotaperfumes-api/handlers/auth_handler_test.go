// Package handlers_test contém testes de integração dos handlers HTTP.
package handlers_test

import (
	"bytes"
	"context"
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

// fakeEmailService é um EmailService fake para testes — nunca faz chamadas
// de rede reais, apenas registra que foi chamado.
type fakeEmailService struct {
	chamadas int
}

func (f *fakeEmailService) EnviarSenhaInicial(ctx context.Context, destinatario, nomeUsuario, senha string) error {
	f.chamadas++
	return nil
}

// fakeCaptchaVerifier é um CaptchaVerifier fake para testes — nunca faz
// chamadas de rede reais. Reproduz o comportamento fail-closed do
// TurnstileService real para token vazio (ErrCaptchaTokenAusente), e para
// qualquer outro token retorna o erro configurado em err (nil == captcha
// válido).
type fakeCaptchaVerifier struct {
	err error
}

func (f *fakeCaptchaVerifier) Verify(ctx context.Context, token, remoteIP string) error {
	if token == "" {
		return services.ErrCaptchaTokenAusente
	}
	return f.err
}

// validCaptchaBody mescla um body de requisição com um captchaToken válido —
// atalho usado pela maioria dos testes de Login/ResetPassword, já que o
// campo passou a ser obrigatório.
func validCaptchaBody(body map[string]any) map[string]any {
	merged := make(map[string]any, len(body)+1)
	for k, v := range body {
		merged[k] = v
	}
	merged["captchaToken"] = "token-valido-de-teste"
	return merged
}

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
// e handlers reais, injetando um DB mockado via sqlmock. Usado pela maioria
// dos testes de handlers deste pacote, que não precisam manipular o
// CaptchaVerifier do AuthHandler diretamente.
func setupTestServer(t *testing.T) (*httptest.Server, *sql.DB, sqlmock.Sqlmock) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	return server, db, mock
}

// setupTestServerWithAuthHandler é como setupTestServer, mas também retorna
// o *handlers.AuthHandler real usado pelo router — permite que os testes de
// captcha (Login/ResetPassword) substituam o CaptchaVerifier via
// AuthHandler.SetCaptchaVerifier para simular sucesso/falha do Turnstile sem
// chamadas de rede reais.
func setupTestServerWithAuthHandler(t *testing.T) (*httptest.Server, *sql.DB, sqlmock.Sqlmock, *handlers.AuthHandler) {
	cfg := testCfg()

	// sqlmock para DB
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	// Handlers reais (recebem *sql.DB injetado)
	authHandler := handlers.NewAuthHandler(db, cfg)
	// Por padrão, injeta um CaptchaVerifier fake que sempre aprova tokens
	// não vazios (mantendo o fail-closed real para token vazio) — evita
	// chamadas de rede reais ao Cloudflare nos testes. Testes que queiram
	// simular captcha inválido chamam authHandler.SetCaptchaVerifier de novo.
	authHandler.SetCaptchaVerifier(&fakeCaptchaVerifier{err: nil})
	userHandler := handlers.NewUsuarioHandler(db, cfg, &fakeEmailService{})
	dashboardHandler := handlers.NewDashboardHandler(db, cfg)
	senhaHandler := handlers.NewSenhaHistoricoHandler(db)
	vendedorHandler := handlers.NewVendedorHandler(db, cfg)
	clienteHandler := handlers.NewClienteHandler(db, cfg)
	produtoHandler := handlers.NewProdutoHandler(db, cfg)
	pedidoHandler := handlers.NewPedidoHandler(db, cfg)
	pagamentoHandler := handlers.NewPagamentoHandler(db, cfg)
	oportunidadeHandler := handlers.NewOportunidadeHandler(db, cfg)
	visitaHandler := handlers.NewVisitaHandler(db, cfg)
	estoqueHandler := handlers.NewEstoqueHandler(db, cfg)

	// Router real com middlewares corretos
	mux := routes.NewMux(cfg, authHandler, userHandler, dashboardHandler, senhaHandler, vendedorHandler, clienteHandler, produtoHandler, pedidoHandler, pagamentoHandler, oportunidadeHandler, visitaHandler, estoqueHandler)

	server := httptest.NewServer(mux)
	return server, db, mock, authHandler
}

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

func TestLogin_Success(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()

	// Mock: busca usuário admin@test.com
	// Tabela usuarios: id, nome, email, password_hash, role, id_vendedor, ativo, created_at, updated_at, ultimo_login_at
	hash, err := services.NewAuthService().HashPassword(cfg, "senha-correta")
	require.NoError(t, err)

	mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.email\s+=\s+\?\s+LIMIT\s+1`).
		WithArgs("admin@test.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(int64(1), "Admin User", "admin@test.com", hash, "admin", nil, true, false, time.Now(), time.Now(), nil, nil))

	// Mock: insert refresh_token
	mock.ExpectExec(`INSERT INTO refresh_tokens`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock: atualiza ultimo_login_at
	mock.ExpectExec(`UPDATE usuarios SET ultimo_login_at = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(map[string]string{"email": "admin@test.com", "password": "senha-correta", "captchaToken": "token-valido-de-teste"}))
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
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	hash, err := services.NewAuthService().HashPassword(cfg, "senha-correta")
	require.NoError(t, err)

	// Usuário existe, mas senha errada
	mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.email\s+=\s+\?\s+LIMIT\s+1`).
		WithArgs("admin@test.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(int64(1), "Admin User", "admin@test.com", hash, "admin", nil, true, false, time.Now(), time.Now(), nil, nil))

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(map[string]string{"email": "admin@test.com", "password": "senha-errada", "captchaToken": "token-valido-de-teste"}))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.False(t, body["success"].(bool))
	assert.Equal(t, "credenciais inválidas", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin_UsuarioNaoExiste(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	// Usuário não encontrado
	mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.email\s+=\s+\?\s+LIMIT\s+1`).
		WithArgs("naoexiste@test.com").
		WillReturnError(sql.ErrNoRows)

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(map[string]string{"email": "naoexiste@test.com", "password": "qualquer", "captchaToken": "token-valido-de-teste"}))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.False(t, body["success"].(bool))
	assert.Equal(t, "credenciais inválidas", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin_UsuarioInativo(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	hash, err := services.NewAuthService().HashPassword(cfg, "senha-correta")
	require.NoError(t, err)

	// Usuário existe mas inativo
	mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.email\s+=\s+\?\s+LIMIT\s+1`).
		WithArgs("inativo@test.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(int64(1), "Inativo User", "inativo@test.com", hash, "normal", nil, false, false, time.Now(), time.Now(), nil, nil))

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(map[string]string{"email": "inativo@test.com", "password": "qualquer", "captchaToken": "token-valido-de-teste"}))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "usuário inativo", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestLogin_RateLimited_AposVariasFalhas garante que, após várias tentativas
// de login com credenciais erradas, o endpoint passa a responder 429 (rate
// limit anti-bruteforce) em vez de continuar consultando o banco.
func TestLogin_RateLimited_AposVariasFalhas(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	hash, err := services.NewAuthService().HashPassword(cfg, "senha-correta")
	require.NoError(t, err)

	const maxFailures = 5 // deve bater com loginMaxFailures em auth_handler.go

	// As primeiras maxFailures tentativas consultam o banco normalmente
	// (senha incorreta) e vão incrementando o contador de falhas.
	for i := 0; i < maxFailures; i++ {
		mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.email\s+=\s+\?\s+LIMIT\s+1`).
			WithArgs("bruteforce@test.com").
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
				AddRow(int64(1), "Admin User", "bruteforce@test.com", hash, "admin", nil, true, false, time.Now(), time.Now(), nil, nil))
	}

	// Usa um único *http.Client dedicado, com o corpo da resposta sempre
	// totalmente lido antes de fechar, para reusar a mesma conexão TCP
	// (keep-alive) entre as tentativas. O rate limit por IP também funciona
	// com conexões novas a cada tentativa — ver
	// TestLogin_RateLimited_QuebraComConexoesNovasPorTentativa, que cobre
	// esse caso (mais realista de uma ferramenta de bruteforce).
	client := &http.Client{}
	for i := 0; i < maxFailures; i++ {
		resp, err := client.Post(server.URL+"/api/auth/login", "application/json",
			makeJSON(map[string]string{"email": "bruteforce@test.com", "password": "senha-errada", "captchaToken": "token-valido-de-teste"}))
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "tentativa %d deve ser 401 (credenciais inválidas)", i+1)
		readBody(t, resp) // drena o corpo para permitir reuso da conexão (keep-alive)
		resp.Body.Close()
	}

	// A tentativa seguinte deve ser bloqueada por rate limit, SEM consultar o banco de novo.
	resp, err := client.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(map[string]string{"email": "bruteforce@test.com", "password": "senha-errada", "captchaToken": "token-valido-de-teste"}))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NotEmpty(t, resp.Header.Get("Retry-After"), "resposta 429 deve incluir Retry-After")
	body := decodeResponse(t, readBody(t, resp))
	assert.Contains(t, body["error"], "muitas tentativas")

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestLogin_RateLimited_SucessoResetaContador garante que um login bem-sucedido
// limpa o contador de falhas, permitindo novas tentativas normalmente.
func TestLogin_RateLimited_SucessoResetaContador(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	hash, err := services.NewAuthService().HashPassword(cfg, "senha-correta")
	require.NoError(t, err)

	rowsCorretas := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(int64(1), "Admin User", "reset@test.com", hash, "admin", nil, true, false, time.Now(), time.Now(), nil, nil)
	}

	// Duas falhas (abaixo do limite de 5).
	for i := 0; i < 2; i++ {
		mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.email\s+=\s+\?\s+LIMIT\s+1`).
			WithArgs("reset@test.com").
			WillReturnRows(rowsCorretas())
	}
	for i := 0; i < 2; i++ {
		resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
			makeJSON(map[string]string{"email": "reset@test.com", "password": "senha-errada", "captchaToken": "token-valido-de-teste"}))
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		resp.Body.Close()
	}

	// Login bem-sucedido: reseta o contador.
	mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.email\s+=\s+\?\s+LIMIT\s+1`).
		WithArgs("reset@test.com").
		WillReturnRows(rowsCorretas())
	mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE usuarios SET ultimo_login_at = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(map[string]string{"email": "reset@test.com", "password": "senha-correta", "captchaToken": "token-valido-de-teste"}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	// Mais duas falhas (novo ciclo) ainda não devem bloquear (contador foi resetado).
	for i := 0; i < 2; i++ {
		mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.email\s+=\s+\?\s+LIMIT\s+1`).
			WithArgs("reset@test.com").
			WillReturnRows(rowsCorretas())
	}
	for i := 0; i < 2; i++ {
		resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
			makeJSON(map[string]string{"email": "reset@test.com", "password": "senha-errada", "captchaToken": "token-valido-de-teste"}))
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "não deveria estar bloqueado logo após reset do contador")
		resp.Body.Close()
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestLogin_RateLimited_QuebraComConexoesNovasPorTentativa garante que o rate
// limit por IP funciona mesmo quando cada tentativa abre uma conexão TCP nova
// (comportamento padrão de ferramentas de bruteforce reais, que não reusam
// conexões). getClientIP() normaliza r.RemoteAddr com net.SplitHostPort antes
// de montar a chave, então a porta efêmera de cada conexão nova não afeta o
// bloqueio: após maxFailures tentativas o mesmo e-mail/IP deve levar 429.
func TestLogin_RateLimited_QuebraComConexoesNovasPorTentativa(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	hash, err := services.NewAuthService().HashPassword(cfg, "senha-correta")
	require.NoError(t, err)

	const maxFailures = 5 // deve bater com loginMaxFailures em auth_handler.go

	// Uma conexão TCP nova (porta efêmera diferente) por tentativa, simulando
	// uma ferramenta de bruteforce real que não reaproveita conexões.
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}

	// Apenas as primeiras maxFailures tentativas consultam o banco; a partir
	// daí o rate limit bloqueia antes de qualquer consulta.
	for i := 0; i < maxFailures; i++ {
		mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.email\s+=\s+\?\s+LIMIT\s+1`).
			WithArgs("bruteforce-newconn@test.com").
			WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
				AddRow(int64(1), "Admin User", "bruteforce-newconn@test.com", hash, "admin", nil, true, false, time.Now(), time.Now(), nil, nil))
	}

	for i := 0; i < maxFailures+3; i++ {
		req, _ := http.NewRequest("POST", server.URL+"/api/auth/login", makeJSON(map[string]string{
			"email": "bruteforce-newconn@test.com", "password": "senha-errada", "captchaToken": "token-valido-de-teste",
		}))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)

		if i < maxFailures {
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"tentativa %d deve ser 401 (credenciais inválidas)", i+1)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode,
				"tentativa %d deveria ser 429 (rate limit), mesmo com conexão TCP nova a cada tentativa", i+1)
		}
		resp.Body.Close()
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestLogin_EmptyBody(t *testing.T) {
	server, db, _, _ := setupTestServerWithAuthHandler(t)
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
	server, db, _, _ := setupTestServerWithAuthHandler(t)
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
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	normalToken := generateToken(t, cfg, 2, "normal")

	hash, _ := services.NewAuthService().HashPassword(cfg, "senha-atual")
	// Mock: busca usuário (valida senha_atual)
	mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.id\s+=\s+\?\s+LIMIT\s+1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(int64(2), "User 2", "user2@test.com", hash, "normal", nil, true, true, time.Now(), time.Now(), nil, nil))

	// Mock: bloqueio de reuso — busca os 2 registros mais recentes do
	// histórico de senhas (nenhum registro existente neste caso).
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`SELECT sh\.id, sh\.usuario_id, sh\.resetado_por_id, sh\.senha_hash_anterior, sh\.ip_origem, sh\.user_agent, sh\.tipo_reset, sh\.created_at, u1\.nome AS usuario_nome, u2\.nome AS resetado_por_nome\s+FROM senha_historico sh\s+LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id\s+LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id\s+WHERE sh\.usuario_id = \?\s+ORDER BY sh\.id DESC\s+LIMIT \? OFFSET \?`).
		WithArgs(int64(2), 2, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "usuario_id", "resetado_por_id", "senha_hash_anterior", "ip_origem", "user_agent", "tipo_reset", "created_at", "usuario_nome", "resetado_por_nome"}))

	// Mock: insert senha_historico
	mock.ExpectExec(`INSERT INTO senha_historico`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Mock: update password_hash + deve_trocar_senha (troca voluntária → false)
	mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), false, int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock: revoga todos os refresh tokens
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// O endpoint /api/auth/reset-password usa senha_atual (não é admin-only).
	req, _ := http.NewRequest("POST", server.URL+"/api/auth/reset-password",
		makeJSON(map[string]any{"senha_atual": "senha-atual", "nova_senha": "nova-senha-123", "captchaToken": "token-valido-de-teste"}))
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
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// Mock: busca usuário alvo para capturar hash anterior (GetByID #1,
	// chamado pelo handler antes de invocar o service).
	hash, _ := services.NewAuthService().HashPassword(cfg, "senha-antiga")
	usuarioRowsFn := func() *sqlmock.Rows {
		return sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(int64(5), "User 5", "user5@test.com", hash, "normal", nil, true, false, time.Now(), time.Now(), nil, nil)
	}
	mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.id\s+=\s+\?\s+LIMIT\s+1`).
		WithArgs(int64(5)).
		WillReturnRows(usuarioRowsFn())

	// Mock: busca usuário alvo de novo (GetByID #2, chamado internamente por
	// UsuarioService.AdminResetPassword para comparar a senha gerada com o
	// hash atual e evitar colisão — defesa em profundidade).
	mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.id\s+=\s+\?\s+LIMIT\s+1`).
		WithArgs(int64(5)).
		WillReturnRows(usuarioRowsFn())

	// Mock: update password_hash + deve_trocar_senha (senha aleatória gerada pelo admin)
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

// mockSenhaHistoricoRecente configura as expectations do
// SenhaHistoricoService.ListarPorUsuario(uid, 1, 2, "id", "desc") — chamado
// pelo ResetPassword handler no bloqueio de reuso das últimas 3 senhas.
// hashesAnteriores são os senha_hash_anterior dos registros retornados (mais
// recente primeiro), até 2 registros.
func mockSenhaHistoricoRecente(mock sqlmock.Sqlmock, uid int64, hashesAnteriores ...string) {
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(uid).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(len(hashesAnteriores)))

	rows := sqlmock.NewRows([]string{"id", "usuario_id", "resetado_por_id", "senha_hash_anterior", "ip_origem", "user_agent", "tipo_reset", "created_at", "usuario_nome", "resetado_por_nome"})
	for i, h := range hashesAnteriores {
		rows.AddRow(int64(i+1), uid, nil, h, "127.0.0.1", "curl/8.0", "usuario", time.Now(), "Usuario Teste", nil)
	}
	mock.ExpectQuery(`SELECT sh\.id, sh\.usuario_id, sh\.resetado_por_id, sh\.senha_hash_anterior, sh\.ip_origem, sh\.user_agent, sh\.tipo_reset, sh\.created_at, u1\.nome AS usuario_nome, u2\.nome AS resetado_por_nome\s+FROM senha_historico sh\s+LEFT JOIN usuarios u1 ON u1\.id = sh\.usuario_id\s+LEFT JOIN usuarios u2 ON u2\.id = sh\.resetado_por_id\s+WHERE sh\.usuario_id = \?\s+ORDER BY sh\.id DESC\s+LIMIT \? OFFSET \?`).
		WithArgs(uid, 2, 0).
		WillReturnRows(rows)
}

// mockUsuarioParaResetPassword configura o mock de GetByID (WHERE u.id = ?)
// usado pelo ResetPassword handler para validar a senha_atual, retornando um
// usuário cujo password_hash corresponde a senhaAtualPlana.
func mockUsuarioParaResetPassword(t *testing.T, mock sqlmock.Sqlmock, cfg *config.Config, uid int64, senhaAtualPlana string) {
	hash, err := services.NewAuthService().HashPassword(cfg, senhaAtualPlana)
	require.NoError(t, err)
	mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.id\s+=\s+\?\s+LIMIT\s+1`).
		WithArgs(uid).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(uid, "User", "user@test.com", hash, "normal", nil, true, false, time.Now(), time.Now(), nil, nil))
}

// TestResetPassword_SenhaFraca_NaoContaNoRateLimiter garante que uma nova
// senha que não atende a política de senha forte é rejeitada com 400 e a
// mensagem de ValidarForcaSenha — e que essa rejeição (erro de usabilidade,
// não tentativa de ataque) NÃO conta no rate limiter de reset-password:
// várias tentativas seguidas com senha fraca não devem disparar 429.
func TestResetPassword_SenhaFraca_NaoContaNoRateLimiter(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	const tentativas = 12 // > resetPasswordMaxFailures (10), para provar que não bloqueia

	for i := 0; i < tentativas; i++ {
		mockUsuarioParaResetPassword(t, mock, cfg, 2, "senha-atual")
	}

	client := &http.Client{}
	for i := 0; i < tentativas; i++ {
		req, _ := http.NewRequest("POST", server.URL+"/api/auth/reset-password",
			makeJSON(validCaptchaBody(map[string]any{"senha_atual": "senha-atual", "nova_senha": "abcdefgh"})))
		req.Header.Set("Authorization", "Bearer "+userToken)

		resp, err := client.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "tentativa %d", i+1)
		body := decodeResponse(t, readBody(t, resp))
		assert.Contains(t, body["error"], "senha deve conter ao menos 3 dos 4 tipos")
		resp.Body.Close()
	}

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestResetPassword_SenhaCurta garante que uma nova senha abaixo do mínimo de
// caracteres é rejeitada com 400 e a mensagem de ValidarForcaSenha.
func TestResetPassword_SenhaCurta(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mockUsuarioParaResetPassword(t, mock, cfg, 2, "senha-atual")

	// senha_atual correta, mas nova_senha é curta (menos de 8 caracteres).
	req, _ := http.NewRequest("POST", server.URL+"/api/auth/reset-password",
		makeJSON(validCaptchaBody(map[string]any{"senha_atual": "senha-atual", "nova_senha": "Ab1!"})))
	req.Header.Set("Authorization", "Bearer "+userToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Contains(t, body["error"], "pelo menos 8 caracteres")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestResetPassword_NovaSenhaIgualASenhaAtual garante que a troca é recusada
// (400, mensagem específica de igualdade) quando a nova senha é idêntica à
// senha atual. A checagem ocorre antes da força e do histórico, por isso não
// há expectativa de query de histórico. O caso "senha atual fraca" prova que
// o erro retornado é o de igualdade, e não o de força de senha.
func TestResetPassword_NovaSenhaIgualASenhaAtual(t *testing.T) {
	casos := []struct {
		nome  string
		senha string
	}{
		{nome: "senha atual forte", senha: "SenhaForte123!"},
		{nome: "senha atual fraca", senha: "abcdefgh"},
	}

	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock, _ := setupTestServerWithAuthHandler(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			userToken := generateToken(t, cfg, 2, "normal")

			mockUsuarioParaResetPassword(t, mock, cfg, 2, tc.senha)

			req, _ := http.NewRequest("POST", server.URL+"/api/auth/reset-password",
				makeJSON(validCaptchaBody(map[string]any{"senha_atual": tc.senha, "nova_senha": tc.senha})))
			req.Header.Set("Authorization", "Bearer "+userToken)

			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, "a nova senha não pode ser igual à senha atual", body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestResetPassword_NovaSenhaIgualAoHistorico garante que a troca é recusada
// quando a nova senha coincide com uma das 2 senhas mais recentes do
// histórico (mesmo não sendo a senha atual), com a mensagem genérica de
// reuso.
func TestResetPassword_NovaSenhaIgualAoHistorico(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	hashSenhaAntiga, err := services.NewAuthService().HashPassword(cfg, "SenhaAntiga123!")
	require.NoError(t, err)

	mockUsuarioParaResetPassword(t, mock, cfg, 2, "SenhaAtual999!")
	mockSenhaHistoricoRecente(mock, 2, hashSenhaAntiga)

	req, _ := http.NewRequest("POST", server.URL+"/api/auth/reset-password",
		makeJSON(validCaptchaBody(map[string]any{"senha_atual": "SenhaAtual999!", "nova_senha": "SenhaAntiga123!"})))
	req.Header.Set("Authorization", "Bearer "+userToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Contains(t, body["error"], "não pode ser igual a uma das últimas senhas utilizadas")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestResetPassword_ReusoConta_NoRateLimiter comprova que a rejeição por
// reuso de senha (bloco "if reutilizada") de fato invoca
// resetPasswordLimiter.RegisterFailure, e não apenas retorna 400 sem
// registrar nada. Como toda verificação de senha_atual correta chama
// RegisterSuccess (zerando o contador) ANTES do bloco de reuso, uma única
// falha de reuso nunca sobrevive à requisição seguinte com senha_atual
// correta — por isso a prova combina 1 tentativa de reuso (soma 1 falha) com
// 9 tentativas subsequentes de senha_atual INCORRETA (que não chamam
// RegisterSuccess, então acumulam sobre a falha anterior): a 10ª falha
// combinada deve disparar o bloqueio (resetPasswordMaxFailures=10) já na
// requisição seguinte.
func TestResetPassword_ReusoConta_NoRateLimiter(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	hashSenhaAntiga, err := services.NewAuthService().HashPassword(cfg, "SenhaAntiga123!")
	require.NoError(t, err)

	const resetPasswordMaxFailures = 10 // deve bater com auth_handler.go

	// 1ª requisição: senha_atual correta, nova_senha reutilizada do
	// histórico → RegisterSuccess (zera) seguido de RegisterFailure (conta=1).
	mockUsuarioParaResetPassword(t, mock, cfg, 2, "SenhaAtual999!")
	mockSenhaHistoricoRecente(mock, 2, hashSenhaAntiga)

	// Próximas (resetPasswordMaxFailures-1) requisições: senha_atual
	// INCORRETA → apenas RegisterFailure (sem RegisterSuccess), acumulando
	// sobre a falha da tentativa de reuso.
	for i := 0; i < resetPasswordMaxFailures-1; i++ {
		mockUsuarioParaResetPassword(t, mock, cfg, 2, "SenhaAtual999!")
	}

	client := &http.Client{}

	req, _ := http.NewRequest("POST", server.URL+"/api/auth/reset-password",
		makeJSON(validCaptchaBody(map[string]any{"senha_atual": "SenhaAtual999!", "nova_senha": "SenhaAntiga123!"})))
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "1ª tentativa: reuso de senha do histórico")
	resp.Body.Close()

	for i := 0; i < resetPasswordMaxFailures-1; i++ {
		req, _ := http.NewRequest("POST", server.URL+"/api/auth/reset-password",
			makeJSON(validCaptchaBody(map[string]any{"senha_atual": "senha-incorreta", "nova_senha": "QualquerSenhaForte1!"})))
		req.Header.Set("Authorization", "Bearer "+userToken)
		resp, err := client.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "tentativa de senha_atual incorreta %d", i+1)
		resp.Body.Close()
	}

	// Após a tentativa de reuso (1 falha) + resetPasswordMaxFailures-1
	// tentativas de senha_atual incorreta, o total de falhas acumuladas
	// atinge resetPasswordMaxFailures — a próxima requisição deve ser
	// bloqueada por rate limit, comprovando que a falha de reuso contou.
	req, _ = http.NewRequest("POST", server.URL+"/api/auth/reset-password",
		makeJSON(validCaptchaBody(map[string]any{"senha_atual": "senha-incorreta", "nova_senha": "QualquerSenhaForte1!"})))
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestResetPassword_NovaSenhaForte_NaoReutilizada_Sucesso garante que uma
// nova senha forte e não reutilizada (nem igual à atual, nem ao histórico) é
// aceita com 200 e o hash é atualizado.
func TestResetPassword_NovaSenhaForte_NaoReutilizada_Sucesso(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	hashSenhaAntiga, err := services.NewAuthService().HashPassword(cfg, "SenhaBemAntiga1!")
	require.NoError(t, err)

	mockUsuarioParaResetPassword(t, mock, cfg, 2, "SenhaAtual999!")
	mockSenhaHistoricoRecente(mock, 2, hashSenhaAntiga)

	mock.ExpectExec(`INSERT INTO senha_historico`).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), false, int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req, _ := http.NewRequest("POST", server.URL+"/api/auth/reset-password",
		makeJSON(validCaptchaBody(map[string]any{"senha_atual": "SenhaAtual999!", "nova_senha": "SenhaNovaForte1!"})))
	req.Header.Set("Authorization", "Bearer "+userToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestResetPassword_ListarPorUsuarioErro_500 garante que um erro ao consultar
// o histórico de senhas (usado no bloqueio de reuso) resulta em 500, sem
// travar em algum estado intermediário.
func TestResetPassword_ListarPorUsuarioErro_500(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mockUsuarioParaResetPassword(t, mock, cfg, 2, "SenhaAtual999!")
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(int64(2)).
		WillReturnError(sql.ErrConnDone)

	req, _ := http.NewRequest("POST", server.URL+"/api/auth/reset-password",
		makeJSON(validCaptchaBody(map[string]any{"senha_atual": "SenhaAtual999!", "nova_senha": "SenhaNovaForte1!"})))
	req.Header.Set("Authorization", "Bearer "+userToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// Me
// ---------------------------------------------------------------------------

func TestMe_Success(t *testing.T) {
	server, db, mock, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 3, "normal")

	now := time.Now()
	mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.id\s+=\s+\?\s+LIMIT\s+1`).
		WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(int64(3), "João Silva", "joao@test.com", "hash", "normal", nil, true, false, now, now, &now, nil))

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

// ---------------------------------------------------------------------------
// Captcha (Cloudflare Turnstile) — Login e ResetPassword
// ---------------------------------------------------------------------------

// TestLogin_CaptchaTokenAusente garante que login sem captchaToken é recusado
// com 400 antes mesmo de consultar o banco (fail-closed).
func TestLogin_CaptchaTokenAusente(t *testing.T) {
	server, db, _, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(map[string]string{"email": "qualquer@test.com", "password": "qualquer"}))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Contains(t, body["error"], "captcha")
}

// TestLogin_CaptchaInvalido garante que login com captchaToken presente mas
// rejeitado pelo verificador (ex.: Cloudflare recusou) é recusado com 400,
// sem sequer consultar o banco (nenhum mock.ExpectQuery configurado).
func TestLogin_CaptchaInvalido(t *testing.T) {
	server, db, _, authHandler := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	authHandler.SetCaptchaVerifier(&fakeCaptchaVerifier{err: services.ErrCaptchaInvalido})

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(validCaptchaBody(map[string]any{"email": "qualquer@test.com", "password": "qualquer"})))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Contains(t, body["error"], "captcha")
}

// TestLogin_CaptchaValido_ProssegueParaCredenciais garante que, com captcha
// válido, o fluxo prossegue normalmente até a validação de credenciais
// (chegando a consultar o banco).
func TestLogin_CaptchaValido_ProssegueParaCredenciais(t *testing.T) {
	server, db, mock, authHandler := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	authHandler.SetCaptchaVerifier(&fakeCaptchaVerifier{err: nil})

	mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.email\s+=\s+\?\s+LIMIT\s+1`).
		WithArgs("captcha-ok@test.com").
		WillReturnError(sql.ErrNoRows)

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(validCaptchaBody(map[string]any{"email": "captcha-ok@test.com", "password": "qualquer"})))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Chegou até a checagem de credenciais (401, não 400 de captcha).
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestResetPassword_CaptchaTokenAusente garante que reset-password sem
// captchaToken é recusado com 400 antes de consultar o banco.
func TestResetPassword_CaptchaTokenAusente(t *testing.T) {
	server, db, _, _ := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("POST", server.URL+"/api/auth/reset-password",
		makeJSON(map[string]any{"senha_atual": "senha-atual", "nova_senha": "nova-senha-123"}))
	req.Header.Set("Authorization", "Bearer "+userToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Contains(t, body["error"], "captcha")
}

// TestResetPassword_CaptchaInvalido garante que reset-password com
// captchaToken rejeitado pelo verificador é recusado com 400, sem consultar
// o banco.
func TestResetPassword_CaptchaInvalido(t *testing.T) {
	server, db, _, authHandler := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()

	authHandler.SetCaptchaVerifier(&fakeCaptchaVerifier{err: services.ErrCaptchaInvalido})

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("POST", server.URL+"/api/auth/reset-password",
		makeJSON(validCaptchaBody(map[string]any{"senha_atual": "senha-atual", "nova_senha": "nova-senha-123"})))
	req.Header.Set("Authorization", "Bearer "+userToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Contains(t, body["error"], "captcha")
}

func TestMe_NaoAutenticado(t *testing.T) {
	server, db, _, _ := setupTestServerWithAuthHandler(t)
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
