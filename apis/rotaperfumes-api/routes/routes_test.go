package routes_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/routes"
	"github.com/rotaperfumes/shared/config"
	sharedsvc "github.com/rotaperfumes/shared/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCfg() *config.Config {
	return &config.Config{
		JWTSecret:  "test-secret-routes",
		JWTIssuer:  "rotaperfumes-test",
		JWTTTL:     24 * time.Hour,
		BCryptCost: 4,
	}
}

// newRouter monta o router real (routes.NewMux) com handlers reais sobre um
// DB sqlmock sem expectativas: qualquer acesso ao banco resulta em erro do
// handler (nunca 401/403), o que permite distinguir bloqueio do middleware
// de erro do handler.
func newRouter(t *testing.T) (http.Handler, sqlmock.Sqlmock) {
	t.Helper()
	cfg := testCfg()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	mux := routes.NewMux(cfg,
		handlers.NewAuthHandler(db, cfg),
		handlers.NewUsuarioHandler(db, cfg, sharedsvc.NewNoopEmailService()),
		handlers.NewDashboardHandler(db, cfg),
		handlers.NewSenhaHistoricoHandler(db),
		handlers.NewVendedorHandler(db, cfg),
		handlers.NewClienteHandler(db, cfg),
		handlers.NewProdutoHandler(db, cfg),
		handlers.NewPedidoHandler(db, cfg),
		handlers.NewPagamentoHandler(db, cfg),
		handlers.NewOportunidadeHandler(db, cfg),
		handlers.NewVisitaHandler(db, cfg),
		handlers.NewEstoqueHandler(db, cfg),
	)
	return mux, mock
}

func token(t *testing.T, uid int64, role string) string {
	t.Helper()
	tok, err := sharedsvc.NewAuthService().GenerateJWT(testCfg(), uid, role)
	require.NoError(t, err)
	return tok
}

func serve(h http.Handler, method, path, tok, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// Rotas de vendedores que passaram a ser admin-only.
var rotasVendedorAdminOnly = []struct {
	method string
	path   string
	body   string
}{
	{"POST", "/api/vendedores", `{"nome":"X","regiao":"Sul","uf":"PR","meta_mensal":1}`},
	{"GET", "/api/vendedores/1", ""},
	{"PUT", "/api/vendedores/1", `{"nome":"X","regiao":"Sul","uf":"PR","meta_mensal":1}`},
	{"DELETE", "/api/vendedores/1", ""},
	{"POST", "/api/vendedores/1/reativar", ""},
	{"POST", "/api/vendedores/1/clientes", `{"cliente_id":10}`},
	{"DELETE", "/api/vendedores/1/clientes/10", ""},
}

func TestVendedores_AdminOnly_TokenNormal_403(t *testing.T) {
	for _, rt := range rotasVendedorAdminOnly {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			h, mock := newRouter(t)
			w := serve(h, rt.method, rt.path, token(t, 2, "normal"), rt.body)

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "acesso restrito a administradores")
			// Nenhuma expectativa registrada e nenhuma pendente: o
			// middleware barrou antes de o handler tocar o banco.
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestVendedores_AdminOnly_SemToken_401(t *testing.T) {
	for _, rt := range rotasVendedorAdminOnly {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			h, _ := newRouter(t)
			w := serve(h, rt.method, rt.path, "", rt.body)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// Token admin passa pelo middleware (a resposta vem do handler; o DB mock sem
// expectativas faz o handler falhar, mas nunca com 401/403).
func TestVendedores_AdminOnly_TokenAdmin_PassaMiddleware(t *testing.T) {
	for _, rt := range rotasVendedorAdminOnly {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			h, _ := newRouter(t)
			w := serve(h, rt.method, rt.path, token(t, 1, "admin"), rt.body)
			assert.NotEqual(t, http.StatusUnauthorized, w.Code)
			assert.NotEqual(t, http.StatusForbidden, w.Code)
		})
	}
}

// Rotas de leitura de vendedores que continuam com acesso comum.
func TestVendedores_AcessoComum_TokenNormal_PassaMiddleware(t *testing.T) {
	rotas := []struct {
		method string
		path   string
	}{
		{"GET", "/api/vendedores"},
		{"GET", "/api/vendedores/1/clientes"},
	}
	for _, rt := range rotas {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			h, _ := newRouter(t)

			w := serve(h, rt.method, rt.path, token(t, 2, "normal"), "")
			assert.NotEqual(t, http.StatusUnauthorized, w.Code)
			assert.NotEqual(t, http.StatusForbidden, w.Code)

			wSem := serve(h, rt.method, rt.path, "", "")
			assert.Equal(t, http.StatusUnauthorized, wSem.Code)
		})
	}
}
