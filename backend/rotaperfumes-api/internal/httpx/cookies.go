package httpx

import (
	"net/http"
	"time"
)

// Nomes dos cookies de sessão (I-15).
const (
	CookieAcesso  = "access_token"
	CookieRefresh = "refresh_token"
	// maxAgeRefresh: 7 dias (o TTL do refresh token).
	maxAgeRefresh = 60 * 60 * 24 * 7
)

// DefinirCookiesSessao grava os tokens só em cookies HttpOnly (nunca no
// corpo, I-15): access_token SameSite=Strict com MaxAge = TTL do JWT;
// refresh_token SameSite=Lax (permite o refresh em navegação top-level) por
// 7 dias. Secure vem de COOKIE_SECURE (seguro), nunca do Host da requisição
// (SEC-21).
func DefinirCookiesSessao(w http.ResponseWriter, acesso, refresh string, ttlAcesso time.Duration, seguro bool) {
	http.SetCookie(w, &http.Cookie{
		Name: CookieAcesso, Value: acesso, Path: "/", MaxAge: int(ttlAcesso.Seconds()),
		HttpOnly: true, Secure: seguro, SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: CookieRefresh, Value: refresh, Path: "/", MaxAge: maxAgeRefresh,
		HttpOnly: true, Secure: seguro, SameSite: http.SameSiteLaxMode,
	})
}

// LimparCookiesSessao expira os dois cookies (logout e troca de senha, I-9)
// com os mesmos atributos da criação (Path, HttpOnly, Secure, SameSite),
// MaxAge=-1 e Expires em 1970 (SEC-21).
func LimparCookiesSessao(w http.ResponseWriter, seguro bool) {
	expirado := time.Unix(0, 0)
	http.SetCookie(w, &http.Cookie{
		Name: CookieAcesso, Value: "", Path: "/", MaxAge: -1, Expires: expirado,
		HttpOnly: true, Secure: seguro, SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: CookieRefresh, Value: "", Path: "/", MaxAge: -1, Expires: expirado,
		HttpOnly: true, Secure: seguro, SameSite: http.SameSiteLaxMode,
	})
}

// CookieRefreshDe lê o refresh token do cookie ("" se ausente).
func CookieRefreshDe(r *http.Request) string {
	if c, err := r.Cookie(CookieRefresh); err == nil {
		return c.Value
	}
	return ""
}
