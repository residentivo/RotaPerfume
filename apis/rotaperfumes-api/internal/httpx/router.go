package httpx

import (
	"net/http"
	"slices"
	"strings"
)

// Policy é a política de acesso de uma rota. Não existe default público: o
// zero value é inválido e dá panic na montagem (C4, O2).
type Policy uint8

// Políticas.
const (
	policyInvalida Policy = iota
	Public
	Authenticated
	AdminOnly
)

// String devolve o nome da política (usado em mensagens e na matriz ARQ-00).
func (p Policy) String() string {
	switch p {
	case Public:
		return "Public"
	case Authenticated:
		return "Authenticated"
	case AdminOnly:
		return "AdminOnly"
	default:
		return "Invalida"
	}
}

// Rota é a declaração de uma rota. Método e política são obrigatórios
// (exceção X1); Arquivo/Funcao são o rótulo do vlog de rota (nível 1).
type Rota struct {
	Metodo  string
	Padrao  string
	Policy  Policy
	Arquivo string // ex.: "produto_handler.go"
	Funcao  string // ex.: "ProdutoHandler.ListProdutos"
	Handler http.HandlerFunc
}

// Router registra rotas num http.ServeMux aplicando, por rota, a ordem
// efetiva Exigir(policy)(Trace(handler)) (O9).
type Router struct {
	mux   *http.ServeMux
	auth  *Autenticador
	rotas []Rota
}

var metodosValidos = map[string]bool{
	http.MethodGet: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true,
}

// NovoRouter exige o Autenticador (checker obrigatório, I-03).
func NovoRouter(auth *Autenticador) *Router {
	if auth == nil {
		panic("httpx: NovoRouter exige Autenticador não nulo")
	}
	return &Router{mux: http.NewServeMux(), auth: auth}
}

// Handle registra a rota. Qualquer dado inválido é erro de programação:
// panic na montagem (no startup e nos testes), nunca em runtime (O2).
func (r *Router) Handle(rt Rota) {
	switch rt.Policy {
	case Public, Authenticated, AdminOnly:
	default:
		panic("httpx: rota " + rt.Padrao + " com política inválida")
	}
	if !metodosValidos[rt.Metodo] {
		panic("httpx: rota " + rt.Padrao + " com método inválido: " + rt.Metodo)
	}
	if !strings.HasPrefix(rt.Padrao, "/") || strings.ContainsAny(rt.Padrao, " \t") {
		panic("httpx: padrão inválido: " + rt.Padrao)
	}
	if rt.Handler == nil || rt.Arquivo == "" || rt.Funcao == "" {
		panic("httpx: rota " + rt.Padrao + " sem handler ou rótulo vlog")
	}

	var h http.Handler = Trace(rt.Arquivo, rt.Funcao, rt.Handler)
	if rt.Policy != Public {
		h = r.auth.Exigir(rt.Policy, h)
	}
	r.mux.Handle(rt.Metodo+" "+rt.Padrao, h)
	r.rotas = append(r.rotas, rt)
}

// Mux devolve o handler com todas as rotas (método errado → 405 com Allow;
// GET também atende HEAD).
func (r *Router) Mux() http.Handler { return r.mux }

// Rotas expõe a tabela registrada (introspecção para a matriz ARQ-00).
func (r *Router) Rotas() []Rota { return slices.Clone(r.rotas) }
