package httpx_test

// httpx.Router (ARQ-01, §5.9): política e método obrigatórios, validações com
// panic na montagem (O2) e ordem efetiva Exigir(policy)(Trace(handler)) (O9).

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
)

func rotaValida() httpx.Rota {
	return httpx.Rota{Metodo: "GET", Padrao: "/api/x", Policy: httpx.Authenticated,
		Arquivo: "x_handler.go", Funcao: "XHandler.Get", Handler: handlerOK}
}

func TestRouter_Handle_PanicaComDadoInvalido(t *testing.T) {
	casos := []struct {
		nome  string
		mudar func(*httpx.Rota)
	}{
		{"Policy(0)", func(r *httpx.Rota) { r.Policy = httpx.Policy(0) }},
		{"Policy(99)", func(r *httpx.Rota) { r.Policy = httpx.Policy(99) }},
		{"método vazio", func(r *httpx.Rota) { r.Metodo = "" }},
		{"método OPTIONS", func(r *httpx.Rota) { r.Metodo = "OPTIONS" }},
		{"método HEAD", func(r *httpx.Rota) { r.Metodo = "HEAD" }},
		{"método minúsculo com espaço", func(r *httpx.Rota) { r.Metodo = "get " }},
		{"método minúsculo", func(r *httpx.Rota) { r.Metodo = "get" }},
		{"padrão sem barra inicial", func(r *httpx.Rota) { r.Padrao = "api/x" }},
		{"padrão vazio", func(r *httpx.Rota) { r.Padrao = "" }},
		{"padrão com espaço", func(r *httpx.Rota) { r.Padrao = "/api /x" }},
		{"padrão com método embutido", func(r *httpx.Rota) { r.Padrao = "GET /api/x" }},
		{"padrão com tab", func(r *httpx.Rota) { r.Padrao = "/api/x\t" }},
		{"handler nil", func(r *httpx.Rota) { r.Handler = nil }},
		{"arquivo vazio", func(r *httpx.Rota) { r.Arquivo = "" }},
		{"função vazia", func(r *httpx.Rota) { r.Funcao = "" }},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			rt := rotaValida()
			c.mudar(&rt)
			r := novoRouter(checkerFake{})
			assert.Panics(t, func() { r.Handle(rt) })
			assert.Empty(t, r.Rotas(), "rota inválida não pode ficar registrada")
		})
	}
}

func TestRouter_Handle_AceitaTodasAsCombinacoesValidas(t *testing.T) {
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		for _, p := range []httpx.Policy{httpx.Public, httpx.Authenticated, httpx.AdminOnly} {
			t.Run(m+" "+p.String(), func(t *testing.T) {
				r := novoRouter(checkerFake{})
				rt := rotaValida()
				rt.Metodo, rt.Policy = m, p
				assert.NotPanics(t, func() { r.Handle(rt) })
				require.Len(t, r.Rotas(), 1)
				assert.Equal(t, m, r.Rotas()[0].Metodo)
				assert.Equal(t, p, r.Rotas()[0].Policy)
			})
		}
	}
}

func TestRouter_PadraoDuplicadoPanica(t *testing.T) {
	r := novoRouter(checkerFake{})
	r.Handle(rotaValida())
	assert.Panics(t, func() { r.Handle(rotaValida()) })
}

func TestRouter_Rotas_DevolveCopia(t *testing.T) {
	r := novoRouter(checkerFake{})
	r.Handle(rotaValida())
	copia := r.Rotas()
	copia[0].Policy = httpx.Public
	assert.Equal(t, httpx.Authenticated, r.Rotas()[0].Policy)
}

func TestPolicy_String(t *testing.T) {
	assert.Equal(t, "Public", httpx.Public.String())
	assert.Equal(t, "Authenticated", httpx.Authenticated.String())
	assert.Equal(t, "AdminOnly", httpx.AdminOnly.String())
	assert.Equal(t, "Invalida", httpx.Policy(0).String())
	assert.Equal(t, "Invalida", httpx.Policy(99).String())
}

func TestNovoRouter_E_Autenticador_DependenciaNilPanica(t *testing.T) {
	assert.Panics(t, func() { httpx.NovoRouter(nil) })
	assert.Panics(t, func() { httpx.NovoAutenticador(nil, checkerFake{}) })
	assert.Panics(t, func() { httpx.NovoAutenticador(identidadecrypto.NovoJWT(testCfg()), nil) })
}

func TestAutenticador_Exigir_PoliticaInvalidaPanica(t *testing.T) {
	a := httpx.NovoAutenticador(identidadecrypto.NovoJWT(testCfg()), checkerFake{})
	h := http.HandlerFunc(handlerOK)
	for _, p := range []httpx.Policy{httpx.Public, httpx.Policy(0), httpx.Policy(99)} {
		assert.Panics(t, func() { a.Exigir(p, h) }, "policy=%d", p)
	}
	assert.NotPanics(t, func() { a.Exigir(httpx.Authenticated, h) })
	assert.NotPanics(t, func() { a.Exigir(httpx.AdminOnly, h) })
}

// Política por rota em runtime: Public não consulta o checker; protegidas
// consultam 1x; AdminOnly barra normal com 403; método errado → 405 sem
// checker; GET atende HEAD.
func TestRouter_PoliticaEmRuntime(t *testing.T) {
	casos := []struct {
		nome       string
		policy     httpx.Policy
		metodo     string
		uid        int64
		role       string
		wantStatus int
		wantCalls  int
	}{
		{"public sem token", httpx.Public, "GET", 0, "", 200, 0},
		{"public com token", httpx.Public, "GET", 2, "normal", 200, 0},
		{"authenticated sem token", httpx.Authenticated, "GET", 0, "", 401, 0},
		{"authenticated normal", httpx.Authenticated, "GET", 2, "normal", 200, 1},
		{"authenticated HEAD sem token", httpx.Authenticated, "HEAD", 0, "", 401, 0},
		{"admin com normal", httpx.AdminOnly, "GET", 2, "normal", 403, 1},
		{"admin com admin", httpx.AdminOnly, "GET", 1, "admin", 200, 1},
		{"método errado com token", httpx.AdminOnly, "DELETE", 1, "admin", 405, 0},
		{"método errado sem token", httpx.Authenticated, "POST", 0, "", 405, 0},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			calls := 0
			r := novoRouter(checkerFake{calls: &calls})
			rt := rotaValida()
			rt.Policy = c.policy
			r.Handle(rt)
			tok := ""
			if c.uid > 0 {
				tok = token(t, c.uid, c.role)
			}
			w := servir(r.Mux(), c.metodo, "/api/x", tok)
			assert.Equal(t, c.wantStatus, w.Code)
			assert.Equal(t, c.wantCalls, calls)
			if c.wantStatus == http.StatusMethodNotAllowed {
				assert.Contains(t, w.Header().Get("Allow"), "GET")
			}
		})
	}
}

// O9: 401/403 não emitem o rótulo do handler; o aprovado emite início/fim.
func TestRouter_TraceFicaPorDentroDaAutenticacao(t *testing.T) {
	casos := []struct {
		nome       string
		uid        int64
		role       string
		wantRotulo bool
	}{
		{"sem token", 0, "", false},
		{"normal em admin", 2, "normal", false},
		{"admin", 1, "admin", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out := capturarLog(t, true)
			r := novoRouter(checkerFake{})
			rt := rotaValida()
			rt.Policy = httpx.AdminOnly
			r.Handle(rt)
			tok := ""
			if c.uid > 0 {
				tok = token(t, c.uid, c.role)
			}
			servir(r.Mux(), "GET", "/api/x?segredo=1", tok)
			if c.wantRotulo {
				assert.Contains(t, out.String(), "[x_handler.go] [XHandler.Get] início GET\n")
				assert.Contains(t, out.String(), "[x_handler.go] [XHandler.Get] fim status=200\n")
			} else {
				assert.NotContains(t, out.String(), "[x_handler.go] [XHandler.Get]")
			}
			assert.NotContains(t, out.String(), "segredo")
		})
	}
}

// Rota Public não tem Principal (O4, mudança interna registrada em §5.9).
func TestRouter_RotaPublica_SemPrincipal(t *testing.T) {
	var ok bool
	r := novoRouter(checkerFake{})
	rt := rotaValida()
	rt.Policy = httpx.Public
	rt.Handler = func(w http.ResponseWriter, req *http.Request) {
		// ARQ-07: as chaves antigas (middleware.GetUserID) não existem mais; a
		// rota pública segue sem Principal.
		_, ok = httpx.PrincipalFrom(req)
	}
	r.Handle(rt)
	servir(r.Mux(), "GET", "/api/x", token(t, 1, "admin"))
	assert.False(t, ok)
}
