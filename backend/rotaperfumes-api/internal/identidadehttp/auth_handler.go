package identidadehttp

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/kernel"
)

// AuthHandler trata /api/auth/* (login, refresh e logout públicos; me e
// reset-password autenticados). Nome do arquivo e do tipo mantidos: são o
// rótulo do vlog de rota exigido pelo TST-05.
type AuthHandler struct {
	casos        *identidadeapp.Casos
	ttlAcesso    time.Duration
	confiarProxy bool
	cookieSeguro bool
}

// NovoAuthHandler exige os casos de uso (nil: panic na montagem). ttlAcesso
// é o TTL do JWT (cookie e expires_in); confiarProxy é TRUST_PROXY_HEADERS;
// cookieSeguro é COOKIE_SECURE (atributo Secure dos cookies, SEC-21).
func NovoAuthHandler(c *identidadeapp.Casos, ttlAcesso time.Duration, confiarProxy, cookieSeguro bool) *AuthHandler {
	if c == nil {
		panic("identidadehttp: NovoAuthHandler exige os casos de uso")
	}
	return &AuthHandler{casos: c, ttlAcesso: ttlAcesso, confiarProxy: confiarProxy, cookieSeguro: cookieSeguro}
}

// Login POST /api/auth/login → tokens em cookies HttpOnly, dados no corpo.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	const tag = "[auth] Login"
	var req loginRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	s, err := h.casos.Login(r.Context(), identidadeapp.ComandoLogin{
		Email: req.Email, Senha: identidade.NovaSenha(req.Password), Captcha: req.CaptchaToken, Origem: h.origem(r),
	})
	if err != nil {
		responderFalha(w, tag, err)
		return
	}
	httpx.DefinirCookiesSessao(w, s.Acesso, s.Refresh.Revelar(), h.ttlAcesso, h.cookieSeguro)
	httpx.JSON(w, http.StatusOK, novoLoginResposta(s, h.ttlAcesso), "")
}

// Refresh POST /api/auth/refresh: refresh_token no body ou no cookie;
// rotação com tokens novos em cookies.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	s, err := h.casos.RenovarSessao(r.Context(), refreshDoRequest(r), h.origem(r))
	if err != nil {
		responderFalha(w, "[auth] Refresh", err)
		return
	}
	httpx.DefinirCookiesSessao(w, s.Acesso, s.Refresh.Revelar(), h.ttlAcesso, h.cookieSeguro)
	httpx.JSON(w, http.StatusOK, refreshResposta{ExpiresIn: int64(h.ttlAcesso.Seconds()), TokenType: tokenType}, "")
}

// Logout POST /api/auth/logout: revoga o refresh informado e limpa os
// cookies; sempre 200 (idempotente).
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.casos.Logout(r.Context(), refreshDoRequest(r))
	httpx.LimparCookiesSessao(w, h.cookieSeguro)
	log.Printf("[auth] logout OK")
	httpx.JSON(w, http.StatusOK, mensagemResposta{Mensagem: "logout realizado"}, "")
}

// ResetPassword POST /api/auth/reset-password: o usuário (do Principal,
// nunca do body) troca a própria senha; limpa os cookies (novo login).
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	const tag = "[auth] ResetPassword"
	var req resetPasswordRequest
	if err := httpx.DecodificarJSON(r, &req); err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	p, _ := httpx.PrincipalFrom(r) // inválido: o caso de uso falha fechado (500)
	err := h.casos.TrocarSenha(r.Context(), p, identidadeapp.ComandoTrocarSenha{
		SenhaAtual: identidade.NovaSenha(req.SenhaAtual), NovaSenha: identidade.NovaSenha(req.NovaSenha),
		Captcha: req.CaptchaToken, Origem: h.origem(r),
	})
	if err != nil {
		httpx.ResponderErro(w, tag, err)
		return
	}
	httpx.LimparCookiesSessao(w, h.cookieSeguro)
	httpx.JSON(w, http.StatusOK, mensagemResposta{Mensagem: "senha alterada com sucesso"}, "")
}

// Me GET /api/auth/me: dados do usuário autenticado (consulta própria).
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	p, _ := httpx.PrincipalFrom(r)
	u, desligado, err := h.casos.ObterSessao(r.Context(), p)
	if err != nil {
		httpx.ResponderErro(w, "[auth] Me", err)
		return
	}
	httpx.JSON(w, http.StatusOK, novoMeResposta(u, desligado), "")
}

func (h *AuthHandler) origem(r *http.Request) identidadeapp.Origem {
	return identidadeapp.Origem{IP: httpx.IPDoCliente(r, h.confiarProxy), UserAgent: r.UserAgent()}
}

// refreshDoRequest: body opcional (erro de decode é ignorado, como no
// legado); sem token no body, vale o cookie.
func refreshDoRequest(r *http.Request) identidade.TokenOpaco {
	var req refreshRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.RefreshToken != "" {
		return identidade.NovoTokenOpaco(req.RefreshToken)
	}
	return identidade.NovoTokenOpaco(httpx.CookieRefreshDe(r))
}

// responderFalha: as duas falhas de emissão de JWT são 500 com mensagem
// própria (legado); o resto passa pelo mapper.
func responderFalha(w http.ResponseWriter, tag string, err error) {
	var de *kernel.Erro
	if errors.As(err, &de) && (de == identidade.ErrGerarToken || de == identidade.ErrGerarAccessToken) {
		httpx.JSON(w, http.StatusInternalServerError, nil, de.Error())
		return
	}
	httpx.ResponderErro(w, tag, err)
}
