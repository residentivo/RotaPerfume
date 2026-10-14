package httpx

import (
	"log"
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/kernel"
)

// Autenticador aplica AuthN/AuthZ por rota e produz o kernel.Principal.
//
// Transição (ARQ-01/ARQ-02, O4): DELEGA ao middleware.JWTMiddlewareWithUserCheck
// legado, que continua sendo o ÚNICO código a gravar as chaves antigas do
// contexto, e só depois de validar o JWT e conferir ativo, corte de sessão e
// role no banco (I-03, I-04). O Principal é derivado internamente
// (derivarPrincipal, não exportado). No ARQ-03 a autenticação é reescrita
// aqui e o middleware legado sai.
type Autenticador struct {
	cfg     *config.Config
	checker middleware.UserStatusChecker
}

// NovoAutenticador exige config e checker (I-03): nil é erro de programação,
// panic na montagem.
func NovoAutenticador(cfg *config.Config, checker middleware.UserStatusChecker) *Autenticador {
	if cfg == nil || checker == nil {
		panic("httpx: NovoAutenticador exige config e UserStatusChecker não nulos")
	}
	return &Autenticador{cfg: cfg, checker: checker}
}

// Exigir envolve next com a autenticação da política. Public não passa por
// aqui (rota pública não consulta o checker); qualquer valor fora de
// {Authenticated, AdminOnly} é bug: panic na montagem (O2).
func (a *Autenticador) Exigir(p Policy, next http.Handler) http.Handler {
	switch p {
	case Authenticated:
		return a.legado(false)(derivarPrincipal(next))
	case AdminOnly:
		return a.legado(true)(derivarPrincipal(next))
	default:
		panic("httpx: Exigir com política inválida: " + p.String())
	}
}

// legado traduz a política para o par de booleanos do middleware antigo
// (Authenticated → (true,false), AdminOnly → (true,true)).
func (a *Autenticador) legado(exigeAdmin bool) Middleware {
	return middleware.JWTMiddlewareWithUserCheck(a.cfg, a.checker, true, exigeAdmin)
}

// derivarPrincipal só roda se o legado aprovou (ele chama next apenas nesse
// caso). Lê userID e role (do BANCO) gravados pelo legado e guarda o
// Principal com a chave não exportada. Falha fechado: sem dados ou Principal
// inválido → 500 sem chamar o handler.
func derivarPrincipal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, okID := middleware.GetUserID(r.Context())
		role, okRole := middleware.GetRole(r.Context())
		if !okID || !okRole {
			log.Printf("[auth] principal ausente apos autenticacao %s", r.Method)
			JSON(w, http.StatusInternalServerError, nil, msgErroInterno)
			return
		}
		p, err := kernel.NovoPrincipal(userID, kernel.RoleDoBanco(role))
		if err != nil {
			log.Printf("[auth] principal invalido user_id=%d", userID)
			JSON(w, http.StatusInternalServerError, nil, msgErroInterno)
			return
		}
		if !p.Role().Conhecido() {
			// R-a: preservado como restrito (EhAdmin falso), sem 401/500 novo.
			log.Printf("[auth] role desconhecido user_id=%d", userID)
		}
		next.ServeHTTP(w, r.WithContext(comPrincipal(r.Context(), p)))
	})
}
