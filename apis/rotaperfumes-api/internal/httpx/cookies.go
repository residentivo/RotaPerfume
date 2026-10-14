package httpx

import (
	"net/http"
	"strings"
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
// 7 dias. Secure é desligado só para host local (SEC-21, GAP preservado:
// decidido por r.Host).
func DefinirCookiesSessao(w http.ResponseWriter, r *http.Request, acesso, refresh string, ttlAcesso time.Duration) {
	seguro := !hostLocalCookie(r.Host)
	http.SetCookie(w, &http.Cookie{
		Name: CookieAcesso, Value: acesso, Path: "/", MaxAge: int(ttlAcesso.Seconds()),
		HttpOnly: true, Secure: seguro, SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: CookieRefresh, Value: refresh, Path: "/", MaxAge: maxAgeRefresh,
		HttpOnly: true, Secure: seguro, SameSite: http.SameSiteLaxMode,
	})
}

// LimparCookiesSessao expira os dois cookies (logout e troca de senha, I-9).
// Sem atributos além de Path, como no legado (SEC-21).
func LimparCookiesSessao(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: CookieAcesso, Value: "", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: CookieRefresh, Value: "", Path: "/", MaxAge: -1})
}

// CookieRefreshDe lê o refresh token do cookie ("" se ausente).
func CookieRefreshDe(r *http.Request) string {
	if c, err := r.Cookie(CookieRefresh); err == nil {
		return c.Value
	}
	return ""
}

// hostLocalCookie: o critério do legado para cookie Secure (prefixo do Host;
// diferente do hostLocal dos headers de segurança, F-03).
func hostLocalCookie(host string) bool {
	return strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") || strings.HasPrefix(host, "0.0.0.0")
}
