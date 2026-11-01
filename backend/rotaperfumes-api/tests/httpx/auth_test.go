package httpx_test

// httpx.Autenticador na transição (O4): delega ao middleware legado e deriva
// o Principal internamente, com o role do BANCO; falha fechado se não
// conseguir; role desconhecido continua restrito com log (R-a).

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
)

// capturaPrincipal registra uma rota que guarda o Principal visto pelo handler.
func capturaPrincipal(t *testing.T, policy httpx.Policy, checker httpx.ConsultaStatus) (http.Handler, *kernel.Principal, *bool) {
	t.Helper()
	var p kernel.Principal
	var ok bool
	r := novoRouter(checker)
	rt := rotaValida()
	rt.Policy = policy
	rt.Handler = func(w http.ResponseWriter, req *http.Request) {
		p, ok = httpx.PrincipalFrom(req)
		w.WriteHeader(http.StatusOK)
	}
	r.Handle(rt)
	return r.Mux(), &p, &ok
}

func TestAutenticador_DerivaPrincipalComRoleDoBanco(t *testing.T) {
	casos := []struct {
		nome      string
		policy    httpx.Policy
		uid       int64
		roleToken string
		status    map[int64]identidade.StatusAcesso
		wantCode  int
		wantRole  kernel.Role
		wantAdmin bool
	}{
		{"normal autenticado", httpx.Authenticated, 2, "normal", nil, 200, kernel.RoleNormal, false},
		{"admin em AdminOnly", httpx.AdminOnly, 1, "admin", nil, 200, kernel.RoleAdmin, true},
		{"token admin, banco normal: role do banco", httpx.Authenticated, 1, "admin",
			map[int64]identidade.StatusAcesso{1: {Ativo: true, Role: "normal"}}, 200, kernel.RoleNormal, false},
		{"token normal, banco admin: role do banco", httpx.AdminOnly, 2, "normal",
			map[int64]identidade.StatusAcesso{2: {Ativo: true, Role: "admin"}}, 200, kernel.RoleAdmin, true},
		{"role desconhecido é restrito (R-a)", httpx.Authenticated, 3, "normal",
			map[int64]identidade.StatusAcesso{3: {Ativo: true, Role: "gerente"}}, 200, kernel.RoleDoBanco("gerente"), false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			h, p, ok := capturaPrincipal(t, c.policy, checkerFake{status: c.status})
			w := servir(h, "GET", "/api/x", token(t, c.uid, c.roleToken))
			assert.Equal(t, c.wantCode, w.Code)
			assert.True(t, *ok)
			assert.Equal(t, c.uid, p.UserID())
			assert.Equal(t, c.wantRole, p.Role())
			assert.Equal(t, c.wantAdmin, p.EhAdmin())
		})
	}
}

// R-a: role desconhecido não ganha AdminOnly e gera o log operacional.
func TestAutenticador_RoleDesconhecido_LogERestrito(t *testing.T) {
	out := capturarLog(t, false)
	st := map[int64]identidade.StatusAcesso{3: {Ativo: true, Role: "gerente"}}

	h, _, _ := capturaPrincipal(t, httpx.Authenticated, checkerFake{status: st})
	w := servir(h, "GET", "/api/x", token(t, 3, "admin"))
	assert.Equal(t, http.StatusOK, w.Code, "nenhum 401/500 novo para role desconhecido")
	assert.Contains(t, out.String(), "[auth] role desconhecido user_id=3")

	hAdmin, _, _ := capturaPrincipal(t, httpx.AdminOnly, checkerFake{status: st})
	assert.Equal(t, http.StatusForbidden, servir(hAdmin, "GET", "/api/x", token(t, 3, "admin")).Code)
}

// Fail-closed: o legado aprovou, mas o Principal seria inválido (role vazio
// no banco) → 500 "erro interno" sem chamar o handler.
func TestAutenticador_PrincipalInvalido_500SemHandler(t *testing.T) {
	out := capturarLog(t, false)
	chamado := false
	r := novoRouter(checkerFake{status: map[int64]identidade.StatusAcesso{2: {Ativo: true, Role: ""}}})
	rt := rotaValida()
	rt.Handler = func(http.ResponseWriter, *http.Request) { chamado = true }
	r.Handle(rt)

	w := servir(r.Mux(), "GET", "/api/x", token(t, 2, "normal"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"success":false,"error":"erro interno"}`, w.Body.String())
	assert.False(t, chamado)
	assert.Contains(t, out.String(), "[auth] principal invalido user_id=2")
}

func TestPrincipalFrom_SemAutenticacao(t *testing.T) {
	req, _ := http.NewRequest("GET", "/x", nil)
	p, ok := httpx.PrincipalFrom(req)
	assert.False(t, ok)
	assert.False(t, p.Valido())
}
