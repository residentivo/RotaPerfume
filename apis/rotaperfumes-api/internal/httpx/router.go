package httpx

import (
	"net/http"
	"slices"
	"strings"

	"github.com/rotaperfumes/shared/vlog"
)

// MaxCorpoPadrao é o teto do corpo de toda rota sem Rota.MaxCorpo (SEC-14).
const MaxCorpoPadrao int64 = 256 << 10

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
//
// MaxCorpo é o teto do corpo em bytes (SEC-14): 0 usa MaxCorpoPadrao;
// negativo dá panic na montagem. Só o upload de imagem declara um teto
// próprio (matriz ARQ-00).
//
// LiberaTrocaSenhaPendente deixa passar o usuário com deve_trocar_senha
// (SEC-23); o padrão (false) responde 403. Só a troca de senha e o /me
// declaram true; numa rota Public dá panic na montagem.
type Rota struct {
	Metodo                   string
	Padrao                   string
	Policy                   Policy
	Arquivo                  string // ex.: "produto_handler.go"
	Funcao                   string // ex.: "ProdutoHandler.ListProdutos"
	Handler                  http.HandlerFunc
	MaxCorpo                 int64
	LiberaTrocaSenhaPendente bool
}

// Router registra rotas num http.ServeMux aplicando, por rota, a ordem
// efetiva Exigir(policy) → limitarCorpo → Trace(handler) (O9, SEC-14): 401 e
// 403 vêm antes do 413.
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
	if rt.MaxCorpo < 0 {
		panic("httpx: rota " + rt.Padrao + " com MaxCorpo negativo")
	}
	if rt.Policy == Public && rt.LiberaTrocaSenhaPendente {
		panic("httpx: rota pública " + rt.Padrao + " não passa pelo Autenticador (LiberaTrocaSenhaPendente sem efeito)")
	}

	var h http.Handler = Trace(rt.Arquivo, rt.Funcao, rt.Handler)
	h = limitarCorpo(rt, h)
	switch {
	case rt.Policy == Public:
	case rt.LiberaTrocaSenhaPendente:
		h = r.auth.ExigirLiberandoTrocaPendente(rt.Policy, h)
	default:
		h = r.auth.Exigir(rt.Policy, h)
	}
	r.mux.Handle(rt.Metodo+" "+rt.Padrao, h)
	r.rotas = append(r.rotas, rt)
}

// limitarCorpo aplica o teto do corpo da rota (SEC-14) com
// http.MaxBytesReader; a leitura além do teto falha com *http.MaxBytesError,
// que DecodificarJSON/LerAtivoOpcional traduzem em 413.
//
// Com o teto padrão, Content-Length acima do teto responde 413 sem chamar o
// handler. Uma rota com teto próprio (o upload de imagem) faz essa checagem
// no handler, com a mensagem dela e depois do anti-CSRF (IMG-01); o router
// só garante o MaxBytesReader.
func limitarCorpo(rt Rota, next http.Handler) http.Handler {
	teto := rt.MaxCorpo
	tetoProprio := teto != 0
	if !tetoProprio {
		teto = MaxCorpoPadrao
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !tetoProprio && r.ContentLength > teto {
			vlog.Printf(rt.Arquivo, rt.Funcao, "decisão corpo_muito_grande limite=%d", teto)
			ResponderErro(w, rt.Funcao, ErrCorpoMuitoGrande)
			return
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, teto)
		}
		next.ServeHTTP(w, r)
	})
}

// Mux devolve o handler com todas as rotas (método errado → 405 com Allow;
// GET também atende HEAD).
func (r *Router) Mux() http.Handler { return r.mux }

// Rotas expõe a tabela registrada (introspecção para a matriz ARQ-00).
func (r *Router) Rotas() []Rota { return slices.Clone(r.rotas) }
