package httpx

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
)

// Mensagens públicas da autenticação (as do middleware legado, inalteradas).
const (
	msgHeaderMalFormado = "authorization header mal formado"
	msgHeaderAusente    = "authorization header ausente"
	msgTokenInvalido    = "token inválido ou expirado"
	msgTokenExpirado    = "access token expirado — use /api/auth/refresh"
	msgUsuarioInativo   = "usuário inativo"
	msgSessaoEncerrada  = "sessão encerrada — faça login novamente"
	msgSoAdmin          = "acesso restrito a administradores"
)

// ValidadorAcesso valida o access token (identidadecrypto.JWT satisfaz):
// só HS256, assinatura e exp conferidos (I-01).
type ValidadorAcesso interface {
	ValidarAcesso(token string) (identidade.ClaimsAcesso, error)
}

// ConsultaStatus consulta o usuário no banco a cada request protegido, sem
// cache (SEC-06/SEC-08, R4). Inexistente: identidade.ErrUsuarioNaoEncontrado
// (identidadeapp.Casos satisfaz).
type ConsultaStatus interface {
	StatusParaAutenticacao(ctx context.Context, userID int64) (identidade.StatusAcesso, error)
}

// Autenticador aplica AuthN/AuthZ por rota e produz o kernel.Principal
// (ARQ-03, reescrito a partir do middleware JWT legado, que saiu):
//   - I-02: Bearer tem precedência; header malformado → 401 sem fallback
//     para o cookie; o cookie access_token só vale sem header;
//   - I-01: JWT inválido/expirado → 401 SEM consultar o banco;
//   - I-03: usuário consultado no banco a cada request; inexistente ou
//     inativo → 401 com a mesma mensagem; erro → 500 sem chamar o handler;
//   - I-04: token emitido até o corte de sessão → 401;
//   - I-05: AuthZ pelo role do BANCO; só "admin" exato passa em AdminOnly;
//     role desconhecido é restrito, com log (R-a).
//
// O Principal só nasce aqui (O5), depois de tudo aprovado, e vai para o
// contexto com chave de tipo não exportado (O4).
type Autenticador struct {
	tokens ValidadorAcesso
	status ConsultaStatus
	agora  func() time.Time
}

// NovoAutenticador exige o validador e a consulta de status (I-03): nil é
// erro de programação, panic na montagem.
func NovoAutenticador(tokens ValidadorAcesso, status ConsultaStatus) *Autenticador {
	if tokens == nil || status == nil {
		panic("httpx: NovoAutenticador exige ValidadorAcesso e ConsultaStatus não nulos")
	}
	return &Autenticador{tokens: tokens, status: status, agora: time.Now}
}

// Exigir envolve next com a autenticação da política. Public não passa por
// aqui (rota pública não consulta o banco); qualquer valor fora de
// {Authenticated, AdminOnly} é bug: panic na montagem (O2).
func (a *Autenticador) Exigir(p Policy, next http.Handler) http.Handler {
	switch p {
	case Authenticated:
		return a.autenticar(next, false)
	case AdminOnly:
		return a.autenticar(next, true)
	default:
		panic("httpx: Exigir com política inválida: " + p.String())
	}
}

func (a *Autenticador) autenticar(next http.Handler, exigeAdmin bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, msg := a.validarToken(r)
		if msg != "" {
			JSON(w, http.StatusUnauthorized, nil, msg)
			return
		}
		role, rec := a.conferirUsuario(r, claims)
		if rec.status != 0 {
			JSON(w, rec.status, nil, rec.msg)
			return
		}
		if exigeAdmin && !role.EhAdmin() {
			JSON(w, http.StatusForbidden, nil, msgSoAdmin)
			return
		}
		p, err := kernel.NovoPrincipal(claims.UserID, role)
		if err != nil {
			log.Printf("[auth] principal invalido user_id=%d", claims.UserID)
			JSON(w, http.StatusInternalServerError, nil, msgErroInterno)
			return
		}
		if !role.Conhecido() {
			log.Printf("[auth] role desconhecido user_id=%d", claims.UserID) // R-a: segue restrito
		}
		next.ServeHTTP(w, r.WithContext(comPrincipal(r.Context(), p)))
	})
}

// validarToken extrai (I-02) e valida (I-01) o access token. msg != "" é a
// mensagem do 401.
func (a *Autenticador) validarToken(r *http.Request) (identidade.ClaimsAcesso, string) {
	token, msg := tokenDoRequest(r)
	if msg != "" {
		return identidade.ClaimsAcesso{}, msg
	}
	claims, err := a.tokens.ValidarAcesso(token)
	if err != nil {
		return identidade.ClaimsAcesso{}, msgTokenInvalido
	}
	if claims.ExpiraEm != nil && claims.ExpiraEm.Before(a.agora()) { // checagem defensiva (I-01)
		return identidade.ClaimsAcesso{}, msgTokenExpirado
	}
	return claims, ""
}

// tokenDoRequest: Bearer tem precedência; header malformado não cai no cookie.
func tokenDoRequest(r *http.Request) (string, string) {
	if h := r.Header.Get("Authorization"); h != "" {
		partes := strings.SplitN(h, " ", 2)
		if len(partes) != 2 || !strings.EqualFold(partes[0], "Bearer") {
			return "", msgHeaderMalFormado
		}
		if partes[1] == "" {
			return "", msgHeaderAusente
		}
		return partes[1], ""
	}
	if c, err := r.Cookie(CookieAcesso); err == nil && c.Value != "" {
		return c.Value, ""
	}
	return "", msgHeaderAusente
}

// recusa é a resposta de um usuário barrado (status 0 = aprovado).
type recusa struct {
	status int
	msg    string
}

// conferirUsuario consulta o banco e aplica ativo e corte de sessão. Devolve
// o role do banco ou a recusa (401/500).
func (a *Autenticador) conferirUsuario(r *http.Request, c identidade.ClaimsAcesso) (kernel.Role, recusa) {
	st, err := a.status.StatusParaAutenticacao(r.Context(), c.UserID)
	switch {
	case errors.Is(err, identidade.ErrUsuarioNaoEncontrado):
		log.Printf("[auth] acesso negado: user_id=%d inexistente (token válido) %s %s", c.UserID, r.Method, r.URL.Path)
		return "", recusa{http.StatusUnauthorized, msgUsuarioInativo}
	case err != nil:
		log.Printf("[auth] erro ao verificar usuário user_id=%d: %v", c.UserID, err)
		return "", recusa{http.StatusInternalServerError, msgErroInterno}
	case !st.Ativo:
		log.Printf("[auth] acesso negado: user_id=%d inativo %s %s", c.UserID, r.Method, r.URL.Path)
		return "", recusa{http.StatusUnauthorized, msgUsuarioInativo}
	case st.TokenAnteriorAoCorte(c.EmitidoEm):
		log.Printf("[auth] acesso negado: user_id=%d token anterior ao corte de sessão iat=%d corte=%d",
			c.UserID, unixOuZero(c.EmitidoEm), st.TokensValidosDesde.Unix())
		return "", recusa{http.StatusUnauthorized, msgSessaoEncerrada}
	}
	if st.Role != c.Role {
		log.Printf("[auth] role do token divergente do banco: user_id=%d token=%s banco=%s (usando banco)", c.UserID, c.Role, st.Role)
	}
	return st.Role, recusa{}
}

func unixOuZero(t *time.Time) int64 {
	if t == nil {
		return 0
	}
	return t.Unix()
}
