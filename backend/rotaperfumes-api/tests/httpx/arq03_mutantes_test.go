package httpx_test

// ARQ-03 (validação do TestBrain): mata o mutante "cache de status por
// instância do Autenticador". Os testes de SEC-06/SEC-08 montam um
// Autenticador novo a cada request, então um cache guardado na própria
// instância (pular a consulta na 2ª chamada com o mesmo user_id) passava.
// Aqui o MESMO router atende os dois requests e o estado do usuário muda no
// banco entre eles (R4: sem cache, consulta a cada request).

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/identidade"
)

func TestARQ03_Autenticador_SemCacheNaMesmaInstancia(t *testing.T) {
	corteFuturo := time.Now().Add(time.Hour)
	casos := []struct {
		nome                  string
		policy                httpx.Policy
		roleTok               string
		antes                 identidade.StatusAcesso
		depois                identidade.StatusAcesso
		wantAntes, wantDepois int
	}{
		{"inativado entre os requests → 401", httpx.Authenticated, "normal",
			identidade.StatusAcesso{Ativo: true, Role: "normal"}, identidade.StatusAcesso{Ativo: false, Role: "normal"},
			http.StatusOK, http.StatusUnauthorized},
		{"admin rebaixado entre os requests → 403 em AdminOnly", httpx.AdminOnly, "admin",
			identidade.StatusAcesso{Ativo: true, Role: "admin"}, identidade.StatusAcesso{Ativo: true, Role: "normal"},
			http.StatusOK, http.StatusForbidden},
		{"promovido entre os requests → passa em AdminOnly", httpx.AdminOnly, "normal",
			identidade.StatusAcesso{Ativo: true, Role: "normal"}, identidade.StatusAcesso{Ativo: true, Role: "admin"},
			http.StatusForbidden, http.StatusOK},
		{"corte de sessão gravado entre os requests → 401", httpx.Authenticated, "normal",
			identidade.StatusAcesso{Ativo: true, Role: "normal"},
			identidade.StatusAcesso{Ativo: true, Role: "normal", TokensValidosDesde: &corteFuturo},
			http.StatusOK, http.StatusUnauthorized},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			const uid = int64(7)
			calls := 0
			status := map[int64]identidade.StatusAcesso{uid: c.antes}
			r := novoRouter(checkerFake{status: status, calls: &calls})
			rt := rotaValida()
			rt.Policy = c.policy
			r.Handle(rt)
			h := r.Mux()
			tok := token(t, uid, c.roleTok)

			assert.Equal(t, c.wantAntes, servir(h, http.MethodGet, "/api/x", tok).Code, "1º request")
			status[uid] = c.depois // muda no "banco"
			assert.Equal(t, c.wantDepois, servir(h, http.MethodGet, "/api/x", tok).Code, "2º request (mesma instância)")
			assert.Equal(t, 2, calls, "o status é consultado a cada request, sem cache (R4)")
		})
	}
}
