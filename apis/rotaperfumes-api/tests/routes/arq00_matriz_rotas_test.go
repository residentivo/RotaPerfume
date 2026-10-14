package routes_test

// ARQ-00 / R2 (SecBrain): matriz de rotas — pré-requisito da refatoração DDD.
//
// Cada rota registrada no router (bootstrap.NovoRouter) aparece aqui com o
// método e a política de acesso esperada (Public / Authenticated / AdminOnly).
// Os testes exercitam o handler devolvido por bootstrap.HandlerHTTP, que é a
// cadeia CORS → SecurityHeaders → mux → Exigir(policy) → Trace → handler (só o
// httpx.Logging de bootstrap.Montar fica de fora; ele apenas loga e não altera
// status/headers).
//
// ARQ-01: a fonte é router.Rotas() (padrão E política registrados em runtime).
// Se o router ganhar (ou perder) uma rota, ou registrar uma política diferente
// da matriz, TestARQ00_Matriz_CobreTodasAsRotasDeRoutesGo falha — rota nova
// nunca nasce sem política. Exceção X1: os 13 padrões sem método passaram ao
// método documentado; método errado → 405 sem consultar o checker.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/identidade"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

type politicaRota int

const (
	politicaPublic politicaRota = iota + 1
	politicaAuthenticated
	politicaAdminOnly
)

func (p politicaRota) String() string {
	switch p {
	case politicaPublic:
		return "Public"
	case politicaAuthenticated:
		return "Authenticated"
	case politicaAdminOnly:
		return "AdminOnly"
	}
	return "INDEFINIDA"
}

// politicaDe traduz a política registrada no router para a da matriz.
func politicaDe(p httpx.Policy) politicaRota {
	switch p {
	case httpx.Public:
		return politicaPublic
	case httpx.Authenticated:
		return politicaAuthenticated
	case httpx.AdminOnly:
		return politicaAdminOnly
	}
	return 0
}

// todosMetodos é o universo de métodos que o router aceita registrar. Desde o
// ARQ-01 (X1) todo padrão fixa o método; os demais métodos do mesmo path
// recebem 405 do ServeMux (TestARQ00_MetodoNaoRegistrado_405SemChecker).
var todosMetodos = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

type rotaMatriz struct {
	// padrao é "MÉTODO /caminho", exatamente Rota.Metodo+" "+Rota.Padrao.
	padrao   string
	politica politicaRota
	// paths concretos usados nos requests (path params preenchidos).
	paths []string
}

// metodos devolve os métodos a exercitar: o do padrão, ou todos se o padrão
// não fixa método.
func (r rotaMatriz) metodos() []string {
	if i := strings.Index(r.padrao, " "); i > 0 {
		return []string{r.padrao[:i]}
	}
	return todosMetodos
}

func p(paths ...string) []string { return paths }

var matrizRotas = []rotaMatriz{
	// Auth (X1: método fixo)
	{"POST /api/auth/login", politicaPublic, p("/api/auth/login")},
	{"POST /api/auth/refresh", politicaPublic, p("/api/auth/refresh")},
	{"POST /api/auth/logout", politicaPublic, p("/api/auth/logout")},
	{"POST /api/auth/reset-password", politicaAuthenticated, p("/api/auth/reset-password")},
	{"GET /api/auth/me", politicaAuthenticated, p("/api/auth/me")},
	// Usuários / admin
	{"GET /api/usuarios", politicaAdminOnly, p("/api/usuarios")},
	{"POST /api/usuarios", politicaAdminOnly, p("/api/usuarios")},
	{"PUT /api/usuarios/{id}", politicaAdminOnly, p("/api/usuarios/2")},
	{"PATCH /api/usuarios/{id}/inativar", politicaAdminOnly, p("/api/usuarios/2/inativar")},
	{"POST /api/admin/reset-password", politicaAdminOnly, p("/api/admin/reset-password")},
	// Histórico de senhas (wildcard de resto continua admin; X1/R-f)
	{"GET /api/senha-historico/{user_id...}", politicaAdminOnly, p("/api/senha-historico/2", "/api/senha-historico/", "/api/senha-historico/2/extra")},
	{"GET /api/senha-historico", politicaAdminOnly, p("/api/senha-historico")},
	// Dashboard (X1: método fixo)
	{"GET /api/dashboard/metrics", politicaAuthenticated, p("/api/dashboard/metrics")},
	{"GET /api/dashboard/vendas", politicaAuthenticated, p("/api/dashboard/vendas")},
	{"GET /api/dashboard/vendedores", politicaAuthenticated, p("/api/dashboard/vendedores")},
	{"GET /api/dashboard/clientes", politicaAuthenticated, p("/api/dashboard/clientes")},
	// Vendedores
	{"GET /api/vendedores", politicaAuthenticated, p("/api/vendedores")},
	{"POST /api/vendedores", politicaAdminOnly, p("/api/vendedores")},
	{"GET /api/vendedores/{id}", politicaAdminOnly, p("/api/vendedores/1")},
	{"PUT /api/vendedores/{id}", politicaAdminOnly, p("/api/vendedores/1")},
	{"DELETE /api/vendedores/{id}", politicaAdminOnly, p("/api/vendedores/1")},
	{"POST /api/vendedores/{id}/reativar", politicaAdminOnly, p("/api/vendedores/1/reativar")},
	{"POST /api/vendedores/{id}/clientes", politicaAdminOnly, p("/api/vendedores/1/clientes")},
	{"DELETE /api/vendedores/{id}/clientes/{clienteId}", politicaAdminOnly, p("/api/vendedores/1/clientes/10")},
	{"GET /api/vendedores/{id}/clientes", politicaAuthenticated, p("/api/vendedores/1/clientes")},
	// Clientes
	{"GET /api/clientes", politicaAuthenticated, p("/api/clientes")},
	{"POST /api/clientes", politicaAuthenticated, p("/api/clientes")},
	{"GET /api/clientes/{id}", politicaAuthenticated, p("/api/clientes/1")},
	{"PUT /api/clientes/{id}", politicaAuthenticated, p("/api/clientes/1")},
	{"PATCH /api/clientes/{id}/inativar", politicaAuthenticated, p("/api/clientes/1/inativar")},
	// Produtos
	{"GET /api/produtos", politicaAuthenticated, p("/api/produtos")},
	{"POST /api/produtos", politicaAdminOnly, p("/api/produtos")},
	{"GET /api/produtos/{id}", politicaAuthenticated, p("/api/produtos/1")},
	{"PUT /api/produtos/{id}", politicaAdminOnly, p("/api/produtos/1")},
	{"PATCH /api/produtos/{id}/inativar", politicaAdminOnly, p("/api/produtos/1/inativar")},
	// Pedidos
	{"GET /api/pedidos", politicaAuthenticated, p("/api/pedidos")},
	{"POST /api/pedidos", politicaAuthenticated, p("/api/pedidos")},
	{"GET /api/pedidos/{id}", politicaAuthenticated, p("/api/pedidos/1")},
	{"PUT /api/pedidos/{id}", politicaAuthenticated, p("/api/pedidos/1")},
	{"DELETE /api/pedidos/{id}", politicaAuthenticated, p("/api/pedidos/1")},
	// Pagamentos
	{"GET /api/pagamentos", politicaAuthenticated, p("/api/pagamentos")},
	{"POST /api/pagamentos", politicaAuthenticated, p("/api/pagamentos")},
	{"GET /api/pagamentos/{id}", politicaAuthenticated, p("/api/pagamentos/1")},
	{"PUT /api/pagamentos/{id}", politicaAuthenticated, p("/api/pagamentos/1")},
	{"DELETE /api/pagamentos/{id}", politicaAuthenticated, p("/api/pagamentos/1")},
	// Oportunidades
	{"GET /api/oportunidades", politicaAuthenticated, p("/api/oportunidades")},
	{"POST /api/oportunidades", politicaAuthenticated, p("/api/oportunidades")},
	{"GET /api/oportunidades/{id}", politicaAuthenticated, p("/api/oportunidades/1")},
	{"PUT /api/oportunidades/{id}", politicaAuthenticated, p("/api/oportunidades/1")},
	{"DELETE /api/oportunidades/{id}", politicaAuthenticated, p("/api/oportunidades/1")},
	// Visitas
	{"GET /api/visitas", politicaAuthenticated, p("/api/visitas")},
	{"POST /api/visitas", politicaAuthenticated, p("/api/visitas")},
	{"GET /api/visitas/{id}", politicaAuthenticated, p("/api/visitas/1")},
	{"PUT /api/visitas/{id}", politicaAuthenticated, p("/api/visitas/1")},
	{"DELETE /api/visitas/{id}", politicaAuthenticated, p("/api/visitas/1")},
	// Estoque (tudo admin)
	{"GET /api/estoque", politicaAdminOnly, p("/api/estoque")},
	{"POST /api/estoque", politicaAdminOnly, p("/api/estoque")},
	{"GET /api/estoque/{id}", politicaAdminOnly, p("/api/estoque/1")},
	{"PUT /api/estoque/{id}", politicaAdminOnly, p("/api/estoque/1")},
	// Healthcheck (X1: GET, que também atende HEAD)
	{"GET /health", politicaPublic, p("/health")},
}

const (
	arq00OrigemPermitida = "http://app.arq00.test"
	arq00OrigemNegada    = "http://evil.arq00.test"
	msgAdminRestrito     = "acesso restrito a administradores"
)

// arq00Router monta o router real (CORS→SecurityHeaders→mux→Exigir→Trace→handler)
// sobre um DB sqlmock sem expectativas: o que chega ao handler falha no banco
// (nunca com as mensagens do middleware) e o mock permite provar que um
// request barrado no middleware não tocou o banco.
func arq00Router(t *testing.T, checker httpx.ConsultaStatus) (http.Handler, sqlmock.Sqlmock) {
	t.Helper()
	cfg := testCfg()
	cfg.CORSAllowedOrigins = []string{arq00OrigemPermitida}
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	h := bootstrap.HandlerHTTP(cfg, checker, bootstrap.NovosHandlers(db, cfg, sharedsvc.NewNoopEmailService()))
	return h, mock
}

// arq00RotasRegistradas devolve a tabela registrada em runtime (ARQ-01).
func arq00RotasRegistradas(t *testing.T) []httpx.Rota {
	t.Helper()
	cfg := testCfg()
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return bootstrap.NovoRouter(cfg, fakeUserStatusChecker{}, bootstrap.NovosHandlers(db, cfg, sharedsvc.NewNoopEmailService())).Rotas()
}

func arq00Serve(h http.Handler, method, path, tok, origin string) *httptest.ResponseRecorder {
	var req *http.Request
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		req = httptest.NewRequest(method, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
	default:
		req = httptest.NewRequest(method, path, nil)
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// casoMatriz é um request concreto (método + path) derivado de uma entrada da matriz.
type casoMatriz struct {
	nome     string
	method   string
	path     string
	politica politicaRota
}

func casosDaMatriz(filtro func(politicaRota) bool) []casoMatriz {
	var out []casoMatriz
	for _, r := range matrizRotas {
		if filtro != nil && !filtro(r.politica) {
			continue
		}
		for _, m := range r.metodos() {
			for _, pth := range r.paths {
				out = append(out, casoMatriz{
					nome:     r.padrao + " | " + m + " " + pth,
					method:   m,
					path:     pth,
					politica: r.politica,
				})
			}
		}
	}
	return out
}

func protegida(p politicaRota) bool { return p == politicaAuthenticated || p == politicaAdminOnly }

// assertSecurityHeaders confere os headers do SecurityHeadersMiddleware que
// independem do host (HSTS fica de fora: depende de r.Host).
func assertSecurityHeaders(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"))
	assert.NotEmpty(t, w.Header().Get("Referrer-Policy"))
}

// ---------------------------------------------------------------------------
// Integridade da matriz
// ---------------------------------------------------------------------------

// ARQ-01: fonte = router.Rotas() (padrão "MÉTODO /caminho" e política
// registrados em runtime). Mais forte que a regex sobre routes.go: confere
// também que a política registrada é a da matriz.
func TestARQ00_Matriz_CobreTodasAsRotasDeRoutesGo(t *testing.T) {
	registradas := arq00RotasRegistradas(t)
	require.NotEmpty(t, registradas, "router sem rotas — a montagem quebrou?")

	naMatriz := make(map[string]politicaRota, len(matrizRotas))
	for _, r := range matrizRotas {
		naMatriz[r.padrao] = r.politica
	}
	emRoutes := make(map[string]politicaRota, len(registradas))
	for _, rt := range registradas {
		pd := rt.Metodo + " " + rt.Padrao
		_, dup := emRoutes[pd]
		assert.False(t, dup, "padrão registrado em duplicidade no router: %q", pd)
		emRoutes[pd] = politicaDe(rt.Policy)
	}

	var faltando, sobrando, divergentes []string
	for pd, pol := range emRoutes {
		esperada, ok := naMatriz[pd]
		if !ok {
			faltando = append(faltando, pd)
			continue
		}
		if pol != esperada {
			divergentes = append(divergentes, pd+": registrada "+pol.String()+", matriz "+esperada.String())
		}
	}
	for pd := range naMatriz {
		if _, ok := emRoutes[pd]; !ok {
			sobrando = append(sobrando, pd)
		}
	}
	sort.Strings(faltando)
	sort.Strings(sobrando)
	sort.Strings(divergentes)
	assert.Empty(t, faltando, "rotas registradas no router SEM política na matriz ARQ-00 — adicione-as a matrizRotas")
	assert.Empty(t, sobrando, "rotas na matriz ARQ-00 que NÃO estão mais no router — remova-as ou corrija o padrão")
	assert.Empty(t, divergentes, "política registrada no router diverge da matriz ARQ-00")
	assert.Len(t, registradas, 60, "a API registra 60 padrões (4 Public, 35 Authenticated, 21 AdminOnly)")
	porPolitica := map[politicaRota]int{}
	for _, rt := range registradas {
		porPolitica[politicaDe(rt.Policy)]++
	}
	assert.Equal(t, map[politicaRota]int{politicaPublic: 4, politicaAuthenticated: 35, politicaAdminOnly: 21}, porPolitica,
		"contagem por política registrada no router")
}

func TestARQ00_Matriz_Sanidade(t *testing.T) {
	vistos := map[string]bool{}
	for _, r := range matrizRotas {
		assert.False(t, vistos[r.padrao], "padrão duplicado na matriz: %q", r.padrao)
		vistos[r.padrao] = true
		assert.Contains(t, []politicaRota{politicaPublic, politicaAuthenticated, politicaAdminOnly}, r.politica,
			"%q sem política válida", r.padrao)
		assert.NotEmpty(t, r.paths, "%q sem path de exemplo", r.padrao)
		metodo, caminho, ok := strings.Cut(r.padrao, " ")
		assert.True(t, ok && strings.HasPrefix(caminho, "/"), "%q sem método fixo (exceção X1)", r.padrao)
		assert.Contains(t, todosMetodos, metodo, "%q com método fora de GET/POST/PUT/PATCH/DELETE", r.padrao)
	}
	// Só estas rotas podem ser públicas; qualquer nova rota pública exige
	// revisão do SecBrain e atualização explícita desta lista.
	var publicas []string
	for _, r := range matrizRotas {
		if r.politica == politicaPublic {
			publicas = append(publicas, r.padrao)
		}
	}
	sort.Strings(publicas)
	assert.Equal(t, []string{"GET /health", "POST /api/auth/login", "POST /api/auth/logout", "POST /api/auth/refresh"}, publicas)
}

// ---------------------------------------------------------------------------
// Políticas
// ---------------------------------------------------------------------------

// Rota protegida sem token → 401, sem consultar o checker nem o banco, com os
// security headers e o CORS aplicados (cadeia na ordem certa).
func TestARQ00_RotaProtegida_SemToken_401(t *testing.T) {
	for _, c := range casosDaMatriz(protegida) {
		t.Run(c.nome, func(t *testing.T) {
			calls := 0
			h, mock := arq00Router(t, fakeUserStatusChecker{calls: &calls})
			w := arq00Serve(h, c.method, c.path, "", arq00OrigemPermitida)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Body.String(), "authorization header ausente")
			assert.Equal(t, 0, calls, "checker não pode ser consultado sem token")
			assertSecurityHeaders(t, w)
			assert.Equal(t, arq00OrigemPermitida, w.Header().Get("Access-Control-Allow-Origin"),
				"CORS deve ser o middleware mais externo (headers presentes também no 401)")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Rota protegida com token inválido → 401 sem consultar o checker.
func TestARQ00_RotaProtegida_TokenInvalido_401(t *testing.T) {
	for _, c := range casosDaMatriz(protegida) {
		t.Run(c.nome, func(t *testing.T) {
			calls := 0
			h, _ := arq00Router(t, fakeUserStatusChecker{calls: &calls})
			w := arq00Serve(h, c.method, c.path, "nao.e.um.jwt", "")

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Equal(t, 0, calls)
			assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		})
	}
}

// AdminOnly com token válido de usuário normal (ativo no checker) → 403, com o
// checker consultado exatamente uma vez e sem tocar o banco do handler.
func TestARQ00_RotaAdminOnly_UsuarioNormal_403(t *testing.T) {
	for _, c := range casosDaMatriz(func(p politicaRota) bool { return p == politicaAdminOnly }) {
		t.Run(c.nome, func(t *testing.T) {
			calls := 0
			h, mock := arq00Router(t, fakeUserStatusChecker{calls: &calls})
			w := arq00Serve(h, c.method, c.path, token(t, 2, "normal"), "")

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), msgAdminRestrito)
			assert.Equal(t, 1, calls, "rota protegida com token válido deve consultar o checker")
			assertSecurityHeaders(t, w)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// AdminOnly: o role do BANCO prevalece — token "admin" de usuário rebaixado → 403.
func TestARQ00_RotaAdminOnly_TokenAdminRoleBancoNormal_403(t *testing.T) {
	for _, c := range casosDaMatriz(func(p politicaRota) bool { return p == politicaAdminOnly }) {
		t.Run(c.nome, func(t *testing.T) {
			h, _ := arq00Router(t, fakeUserStatusChecker{
				status: map[int64]identidade.StatusAcesso{1: {Ativo: true, Role: "normal"}},
			})
			w := arq00Serve(h, c.method, c.path, token(t, 1, "admin"), "")
			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), msgAdminRestrito)
		})
	}
}

// AdminOnly com admin ativo → passa o middleware (consulta o checker 1x).
func TestARQ00_RotaAdminOnly_Admin_PassaMiddleware(t *testing.T) {
	for _, c := range casosDaMatriz(func(p politicaRota) bool { return p == politicaAdminOnly }) {
		t.Run(c.nome, func(t *testing.T) {
			calls := 0
			h, _ := arq00Router(t, fakeUserStatusChecker{calls: &calls})
			w := arq00Serve(h, c.method, c.path, token(t, 1, "admin"), "")

			assert.NotEqual(t, http.StatusUnauthorized, w.Code)
			assert.NotContains(t, w.Body.String(), msgAdminRestrito)
			assert.Equal(t, 1, calls)
		})
	}
}

// Authenticated com usuário normal ativo → passa o middleware (consulta o
// checker 1x). O handler falha no banco sem expectativas; o corpo não é validado.
func TestARQ00_RotaAuthenticated_UsuarioNormal_PassaMiddleware(t *testing.T) {
	for _, c := range casosDaMatriz(func(p politicaRota) bool { return p == politicaAuthenticated }) {
		t.Run(c.nome, func(t *testing.T) {
			calls := 0
			h, _ := arq00Router(t, fakeUserStatusChecker{calls: &calls})
			w := arq00Serve(h, c.method, c.path, token(t, 2, "normal"), "")

			assert.NotEqual(t, http.StatusUnauthorized, w.Code, "body: %s", w.Body.String())
			assert.NotContains(t, w.Body.String(), msgAdminRestrito)
			assert.Equal(t, 1, calls, "rota protegida com token válido deve consultar o checker")
		})
	}
}

// Rota protegida com usuário inativo → 401 (SEC-06) em toda a matriz.
func TestARQ00_RotaProtegida_UsuarioInativo_401(t *testing.T) {
	for _, c := range casosDaMatriz(protegida) {
		t.Run(c.nome, func(t *testing.T) {
			h, mock := arq00Router(t, fakeUserStatusChecker{
				status: map[int64]identidade.StatusAcesso{2: {Ativo: false, Role: "normal"}},
			})
			w := arq00Serve(h, c.method, c.path, token(t, 2, "normal"), "")
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Rota protegida com erro no checker → 500 fail-closed, sem chamar o handler.
func TestARQ00_RotaProtegida_ErroNoChecker_500(t *testing.T) {
	for _, c := range casosDaMatriz(protegida) {
		t.Run(c.nome, func(t *testing.T) {
			h, mock := arq00Router(t, fakeUserStatusChecker{err: errors.New("db down")})
			w := arq00Serve(h, c.method, c.path, token(t, 2, "normal"), "")
			assert.Equal(t, http.StatusInternalServerError, w.Code)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Rota pública NÃO consulta o checker — nem sem token, nem com token válido,
// nem com token inválido. O checker falha se chamado, para que uma consulta
// indevida também apareça como 500.
func TestARQ00_RotaPublica_NaoConsultaChecker(t *testing.T) {
	tokens := map[string]func(t *testing.T) string{
		"sem token":      func(*testing.T) string { return "" },
		"token normal":   func(t *testing.T) string { return token(t, 2, "normal") },
		"token admin":    func(t *testing.T) string { return token(t, 1, "admin") },
		"token inválido": func(*testing.T) string { return "nao.e.um.jwt" },
	}
	for _, c := range casosDaMatriz(func(p politicaRota) bool { return p == politicaPublic }) {
		for nomeTok, tokFn := range tokens {
			t.Run(c.nome+" | "+nomeTok, func(t *testing.T) {
				calls := 0
				h, _ := arq00Router(t, fakeUserStatusChecker{err: errors.New("checker não deveria ser chamado"), calls: &calls})
				w := arq00Serve(h, c.method, c.path, tokFn(t), "")

				assert.Equal(t, 0, calls, "rota pública não pode consultar o checker")
				assert.NotContains(t, w.Body.String(), "authorization header ausente")
				assert.NotContains(t, w.Body.String(), "token inválido ou expirado")
				assertSecurityHeaders(t, w)
			})
		}
	}
}

func TestARQ00_Health_200(t *testing.T) {
	h, _ := arq00Router(t, fakeUserStatusChecker{})
	w := arq00Serve(h, http.MethodGet, "/health", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"success":true,"data":{"status":"ok"}}`, w.Body.String())
	assertSecurityHeaders(t, w)
}

// ---------------------------------------------------------------------------
// CORS → SecurityHeaders → mux
// ---------------------------------------------------------------------------

// Preflight OPTIONS sem token, com Origin permitida, em QUALQUER rota → 204
// com Access-Control-Allow-Origin, antes da autenticação (checker intocado).
func TestARQ00_Preflight_OrigemPermitida_204SemAuth(t *testing.T) {
	for _, r := range matrizRotas {
		for _, pth := range r.paths {
			t.Run(r.padrao+" | OPTIONS "+pth, func(t *testing.T) {
				calls := 0
				h, mock := arq00Router(t, fakeUserStatusChecker{calls: &calls})
				w := arq00Serve(h, http.MethodOptions, pth, "", arq00OrigemPermitida)

				assert.Equal(t, http.StatusNoContent, w.Code)
				assert.Equal(t, arq00OrigemPermitida, w.Header().Get("Access-Control-Allow-Origin"))
				assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
				assert.Contains(t, w.Header().Values("Vary"), "Origin")
				assert.Equal(t, 0, calls)
				assert.Empty(t, w.Body.String())
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

// Preflight com Origin fora da allowlist: não reflete a origem.
func TestARQ00_Preflight_OrigemNegada_SemACAO(t *testing.T) {
	for _, r := range matrizRotas {
		pth := r.paths[0]
		t.Run(r.padrao+" | OPTIONS "+pth, func(t *testing.T) {
			h, _ := arq00Router(t, fakeUserStatusChecker{})
			w := arq00Serve(h, http.MethodOptions, pth, "", arq00OrigemNegada)

			assert.Equal(t, http.StatusNoContent, w.Code)
			assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
		})
	}
}

// Request real (não preflight) com Origin negada em rota protegida sem token:
// 401 com security headers e SEM refletir a origem.
func TestARQ00_RotaProtegida_SemToken_OrigemNegada_401SemACAO(t *testing.T) {
	for _, c := range casosDaMatriz(protegida) {
		t.Run(c.nome, func(t *testing.T) {
			h, _ := arq00Router(t, fakeUserStatusChecker{})
			w := arq00Serve(h, c.method, c.path, "", arq00OrigemNegada)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			assertSecurityHeaders(t, w)
		})
	}
}
