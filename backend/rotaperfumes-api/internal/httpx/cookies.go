package httpx

import (
	"net/http"
	"time"
)

// NomesCookies é o par de nomes dos cookies de sessão (I-15), escolhido pelo
// modo (SEC-33).
type NomesCookies struct {
	Acesso, Refresh string
}

// Pares de nomes (SEC-33). Com COOKIE_SECURE=true os cookies usam o prefixo
// __Host- (o navegador exige Secure, Path=/ e ausência de Domain, o que
// impede cookie tossing por subdomínio ou por HTTP). Com COOKIE_SECURE=false
// (dev em HTTP) ficam os nomes antigos, sem Secure: o prefixo não funciona
// sem HTTPS.
var (
	CookiesSeguros = NomesCookies{Acesso: "__Host-access_token", Refresh: "__Host-refresh_token"}
	CookiesDev     = NomesCookies{Acesso: "access_token", Refresh: "refresh_token"}
)

// Nomes antigos (dev, e os que a transição do SEC-33 expira no modo
// seguro).
const (
	CookieAcesso  = "access_token"
	CookieRefresh = "refresh_token"
	// maxAgeRefresh: 7 dias (o TTL do refresh token).
	maxAgeRefresh = 60 * 60 * 24 * 7
)

// NomesCookiesPara devolve o par do modo: seguro → __Host-; dev → antigos.
func NomesCookiesPara(seguro bool) NomesCookies {
	if seguro {
		return CookiesSeguros
	}
	return CookiesDev
}

// DefinirCookiesSessao grava os tokens só em cookies HttpOnly (nunca no
// corpo, I-15): acesso SameSite=Strict com MaxAge = TTL do JWT; refresh
// SameSite=Lax (permite o refresh em navegação top-level) por 7 dias, os
// dois com Path=/ e sem Domain. Secure vem de COOKIE_SECURE (seguro), nunca
// do Host da requisição (SEC-21); no modo seguro os nomes têm __Host- e os
// cookies antigos são expirados (transição do SEC-33).
func DefinirCookiesSessao(w http.ResponseWriter, acesso, refresh string, ttlAcesso time.Duration, seguro bool) {
	n := NomesCookiesPara(seguro)
	http.SetCookie(w, &http.Cookie{
		Name: n.Acesso, Value: acesso, Path: "/", MaxAge: int(ttlAcesso.Seconds()),
		HttpOnly: true, Secure: seguro, SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: n.Refresh, Value: refresh, Path: "/", MaxAge: maxAgeRefresh,
		HttpOnly: true, Secure: seguro, SameSite: http.SameSiteLaxMode,
	})
	if seguro {
		expirarCookiesAntigos(w)
	}
}

// LimparCookiesSessao expira os dois cookies (logout e troca de senha, I-9)
// com os mesmos atributos da criação (Path, HttpOnly, Secure, SameSite),
// MaxAge=-1 e Expires em 1970 (SEC-21). No modo seguro expira também os
// nomes antigos (SEC-33).
func LimparCookiesSessao(w http.ResponseWriter, seguro bool) {
	n := NomesCookiesPara(seguro)
	expirar(w, n.Acesso, seguro, http.SameSiteStrictMode)
	expirar(w, n.Refresh, seguro, http.SameSiteLaxMode)
	if seguro {
		expirarCookiesAntigos(w)
	}
}

// expirarCookiesAntigos apaga access_token/refresh_token de antes do SEC-33
// (Path=/, Secure, MaxAge=-1).
func expirarCookiesAntigos(w http.ResponseWriter) {
	expirar(w, CookieAcesso, true, http.SameSiteStrictMode)
	expirar(w, CookieRefresh, true, http.SameSiteLaxMode)
}

func expirar(w http.ResponseWriter, nome string, seguro bool, sameSite http.SameSite) {
	http.SetCookie(w, &http.Cookie{
		Name: nome, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(0, 0),
		HttpOnly: true, Secure: seguro, SameSite: sameSite,
	})
}

// CookieRefreshDe lê o refresh token do cookie do par n ("" se ausente).
// No modo seguro só o nome __Host- vale: nunca cai para o antigo (SEC-33).
func CookieRefreshDe(r *http.Request, n NomesCookies) string {
	if c, err := r.Cookie(n.Refresh); err == nil {
		return c.Value
	}
	return ""
}
