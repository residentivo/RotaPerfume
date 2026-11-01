package httpx_test

// SEC-23 (Lote 28): o Autenticador impõe deve_trocar_senha. Ordem: os 401 e
// o 500 atuais (token, inexistente, inativo, corte) → troca pendente (403
// "troca de senha obrigatória", qualquer papel) → AdminOnly (403). Só as
// rotas com Rota.LiberaTrocaSenhaPendente deixam passar; em rota Public o
// campo dá panic na montagem.

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/kernel"
)

const msgTrocaSenhaObrigatoria = "troca de senha obrigatória"

// checkerSEC23 devolve sempre o mesmo status (ou erro).
type checkerSEC23 struct {
	st  identidade.StatusAcesso
	err error
}

func (c checkerSEC23) StatusParaAutenticacao(context.Context, int64) (identidade.StatusAcesso, error) {
	return c.st, c.err
}

// routerSEC23 registra a rota com a política e a liberação informadas.
func routerSEC23(checker httpx.ConsultaStatus, p httpx.Policy, libera bool) http.Handler {
	r := novoRouter(checker)
	rt := rotaValida()
	rt.Policy, rt.LiberaTrocaSenhaPendente = p, libera
	r.Handle(rt)
	return r.Mux()
}

func TestSEC23_TrocaPendente_403EmQualquerPapel(t *testing.T) {
	for _, c := range []struct {
		nome   string
		policy httpx.Policy
		uid    int64
		role   string
	}{
		{"normal em Authenticated", httpx.Authenticated, 2, "normal"},
		{"admin em Authenticated", httpx.Authenticated, 1, "admin"},
		{"admin em AdminOnly", httpx.AdminOnly, 1, "admin"},
		// A troca vem antes do AdminOnly: o normal vê o 403 da troca.
		{"normal em AdminOnly", httpx.AdminOnly, 2, "normal"},
	} {
		t.Run(c.nome, func(t *testing.T) {
			out := capturarLog(t, false)
			st := identidade.StatusAcesso{Ativo: true, Role: kernel.RoleDoBanco(c.role), DeveTrocarSenha: true}
			h := routerSEC23(checkerSEC23{st: st}, c.policy, false)

			w := servir(h, http.MethodGet, "/api/x", token(t, c.uid, c.role))

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.JSONEq(t, `{"success":false,"error":"`+msgTrocaSenhaObrigatoria+`"}`, w.Body.String())
			assert.Contains(t, out.String(), "[auth] acesso negado: user_id=")
			assert.Contains(t, out.String(), "troca de senha pendente GET /api/x")
		})
	}
}

func TestSEC23_TrocaPendente_RotaLiberadaPassa(t *testing.T) {
	for _, p := range []httpx.Policy{httpx.Authenticated, httpx.AdminOnly} {
		t.Run(p.String(), func(t *testing.T) {
			st := identidade.StatusAcesso{Ativo: true, Role: "admin", DeveTrocarSenha: true}
			w := servir(routerSEC23(checkerSEC23{st: st}, p, true), http.MethodGet, "/api/x", token(t, 1, "admin"))
			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
	t.Run("liberada continua exigindo admin em AdminOnly", func(t *testing.T) {
		st := identidade.StatusAcesso{Ativo: true, Role: "normal", DeveTrocarSenha: true}
		w := servir(routerSEC23(checkerSEC23{st: st}, httpx.AdminOnly, true), http.MethodGet, "/api/x", token(t, 2, "normal"))
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "acesso restrito a administradores")
	})
}

func TestSEC23_SemTrocaPendente_NadaMuda(t *testing.T) {
	st := identidade.StatusAcesso{Ativo: true, Role: "normal"}
	w := servir(routerSEC23(checkerSEC23{st: st}, httpx.Authenticated, false), http.MethodGet, "/api/x", token(t, 2, "normal"))
	assert.Equal(t, http.StatusOK, w.Code)
}

// Os 401 e o 500 vêm antes da troca pendente.
func TestSEC23_OrdemDasRecusas(t *testing.T) {
	futuro := time.Now().Add(time.Hour)
	casos := []struct {
		nome    string
		checker checkerSEC23
		status  int
		msg     string
	}{
		{"inativo + troca = 401", checkerSEC23{st: identidade.StatusAcesso{Ativo: false, Role: "normal", DeveTrocarSenha: true}},
			http.StatusUnauthorized, "usuário inativo"},
		{"corte de sessão + troca = 401", checkerSEC23{st: identidade.StatusAcesso{Ativo: true, Role: "normal", TokensValidosDesde: &futuro, DeveTrocarSenha: true}},
			http.StatusUnauthorized, "sessão encerrada — faça login novamente"},
		{"inexistente = 401", checkerSEC23{err: identidade.ErrUsuarioNaoEncontrado}, http.StatusUnauthorized, "usuário inativo"},
		{"erro do checker = 500", checkerSEC23{err: errors.New("db down")}, http.StatusInternalServerError, "erro interno"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			capturarLog(t, false)
			for _, libera := range []bool{false, true} {
				w := servir(routerSEC23(c.checker, httpx.Authenticated, libera), http.MethodGet, "/api/x", token(t, 2, "normal"))
				assert.Equal(t, c.status, w.Code, "libera=%t", libera)
				assert.JSONEq(t, `{"success":false,"error":"`+c.msg+`"}`, w.Body.String(), "libera=%t", libera)
			}
		})
	}
}

func TestSEC23_Handle_PanicaComLiberacaoEmRotaPublica(t *testing.T) {
	r := novoRouter(checkerFake{})
	rt := rotaValida()
	rt.Policy, rt.LiberaTrocaSenhaPendente = httpx.Public, true
	assert.Panics(t, func() { r.Handle(rt) })
	assert.Empty(t, r.Rotas(), "rota inválida não pode ficar registrada")
}

func TestSEC23_ExigirLiberandoTrocaPendente_PanicaComPoliticaInvalida(t *testing.T) {
	a := httpx.NovoAutenticador(identidadecrypto.NovoJWT(testCfg()), checkerFake{})
	for _, p := range []httpx.Policy{httpx.Policy(0), httpx.Public, httpx.Policy(99)} {
		assert.Panics(t, func() { a.ExigirLiberandoTrocaPendente(p, http.HandlerFunc(handlerOK)) }, "policy=%d", p)
	}
	require.NotPanics(t, func() { a.ExigirLiberandoTrocaPendente(httpx.Authenticated, http.HandlerFunc(handlerOK)) })
}
