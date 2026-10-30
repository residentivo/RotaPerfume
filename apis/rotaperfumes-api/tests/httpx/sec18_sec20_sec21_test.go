package httpx_test

// Lote 24 (TestBrain):
//   SEC-18: Bearer com issuer errado / sem exp / sem iss / alg=none → 401
//   "token inválido ou expirado" pelo Autenticador real, sem consultar o
//   status do usuário (calls == 0).
//   SEC-20: IPDoCliente — do XFF vale só o item mais à direita da última
//   linha, validado e normalizado; vazio/inválido cai no RemoteAddr (sem
//   recuar para o anterior); X-Real-IP nunca é lido; sem proxy confiável os
//   headers são ignorados.
//   SEC-21: Set-Cookie cru de DefinirCookiesSessao/LimparCookiesSessao com
//   Secure = parâmetro (nunca o Host) e os atributos da criação na limpeza.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
)

// ---------------------------------------------------------------- SEC-18

func sec18Token(t *testing.T, m jwt.SigningMethod, chave any, mutar func(*jwt.RegisteredClaims)) string {
	t.Helper()
	agora := time.Now()
	rc := jwt.RegisteredClaims{
		Issuer:    testCfg().JWTIssuer,
		IssuedAt:  jwt.NewNumericDate(agora.Add(-time.Minute)),
		NotBefore: jwt.NewNumericDate(agora.Add(-time.Minute)),
		ExpiresAt: jwt.NewNumericDate(agora.Add(time.Hour)),
	}
	if mutar != nil {
		mutar(&rc)
	}
	tok, err := jwt.NewWithClaims(m, identidadecrypto.Claims{UserID: 2, Role: "normal", RegisteredClaims: rc}).SignedString(chave)
	require.NoError(t, err)
	return tok
}

func TestSEC18_Bearer_RecusadoSemConsultarStatus(t *testing.T) {
	segredo := []byte(testCfg().JWTSecret)
	casos := []struct {
		nome       string
		tok        func(t *testing.T) string
		wantStatus int
	}{
		{"válido (sanidade)", func(t *testing.T) string { return sec18Token(t, jwt.SigningMethodHS256, segredo, nil) }, http.StatusOK},
		{"issuer errado", func(t *testing.T) string {
			return sec18Token(t, jwt.SigningMethodHS256, segredo, func(rc *jwt.RegisteredClaims) { rc.Issuer = "atacante" })
		}, http.StatusUnauthorized},
		{"sem iss", func(t *testing.T) string {
			return sec18Token(t, jwt.SigningMethodHS256, segredo, func(rc *jwt.RegisteredClaims) { rc.Issuer = "" })
		}, http.StatusUnauthorized},
		{"sem exp", func(t *testing.T) string {
			return sec18Token(t, jwt.SigningMethodHS256, segredo, func(rc *jwt.RegisteredClaims) { rc.ExpiresAt = nil })
		}, http.StatusUnauthorized},
		{"iat no futuro", func(t *testing.T) string {
			return sec18Token(t, jwt.SigningMethodHS256, segredo, func(rc *jwt.RegisteredClaims) {
				rc.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Hour))
			})
		}, http.StatusUnauthorized},
		{"alg=none", func(t *testing.T) string {
			return sec18Token(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, nil)
		}, http.StatusUnauthorized},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			calls := 0
			a := httpx.NovoAutenticador(identidadecrypto.NovoJWT(testCfg()), checkerFake{calls: &calls})
			w := servir(a.Exigir(httpx.Authenticated, http.HandlerFunc(handlerOK)), "GET", "/x", c.tok(t))
			assert.Equal(t, c.wantStatus, w.Code)
			if c.wantStatus == http.StatusOK {
				assert.Equal(t, 1, calls, "token válido consulta o status")
				return
			}
			assert.Contains(t, w.Body.String(), "token inválido ou expirado")
			assert.Zero(t, calls, "token recusado não consulta o banco")
		})
	}
}

// ---------------------------------------------------------------- SEC-20

func TestSEC20_IPDoCliente(t *testing.T) {
	const remoto = "10.0.0.1:5555"
	casos := []struct {
		nome   string
		remoto string
		xff    []string // uma entrada por linha do header
		real   string
		confia bool
		want   string
	}{
		{"confia: item mais à direita", remoto, []string{"6.6.6.6, 203.0.113.5"}, "", true, "203.0.113.5"},
		{"confia: valor único", remoto, []string{"203.0.113.5"}, "", true, "203.0.113.5"},
		{"confia: espaços em volta", remoto, []string{"  203.0.113.5  "}, "", true, "203.0.113.5"},
		{"confia: duas linhas, vale a última", remoto, []string{"6.6.6.6", "203.0.113.5"}, "", true, "203.0.113.5"},
		{"confia: duas linhas com listas", remoto, []string{"1.1.1.1, 6.6.6.6", "7.7.7.7, 203.0.113.5"}, "", true, "203.0.113.5"},
		{"confia: último item vazio não recua", remoto, []string{"1.1.1.1, "}, "", true, "10.0.0.1"},
		{"confia: só vírgula", remoto, []string{","}, "", true, "10.0.0.1"},
		{"confia: header vazio", remoto, []string{""}, "", true, "10.0.0.1"},
		{"confia: última linha inválida não recua para a anterior", remoto, []string{"203.0.113.5", "abc"}, "", true, "10.0.0.1"},
		{"confia: abc", remoto, []string{"abc"}, "", true, "10.0.0.1"},
		{"confia: 999.1.1.1", remoto, []string{"999.1.1.1"}, "", true, "10.0.0.1"},
		{"confia: com porta 1.2.3.4:80", remoto, []string{"1.2.3.4:80"}, "", true, "10.0.0.1"},
		{"confia: IPv6 com colchetes e porta", remoto, []string{"[2001:db8::1]:443"}, "", true, "10.0.0.1"},
		{"confia: 4 KB de lixo", remoto, []string{strings.Repeat("1", 4096)}, "", true, "10.0.0.1"},
		{"confia: unknown", remoto, []string{"unknown"}, "", true, "10.0.0.1"},
		{"confia: válido à esquerda, inválido à direita", remoto, []string{"203.0.113.5, unknown"}, "", true, "10.0.0.1"},
		{"confia: IPv6 em maiúsculas é normalizado", remoto, []string{"2001:DB8::1"}, "", true, "2001:db8::1"},
		{"confia: IPv6 não comprimido é normalizado", remoto, []string{"2001:0db8:0000:0000:0000:0000:0000:0001"}, "", true, "2001:db8::1"},
		{"confia: IPv4 mapeado vira IPv4", remoto, []string{"::ffff:1.2.3.4"}, "", true, "1.2.3.4"},
		{"confia: zona descartada", remoto, []string{"fe80::1%eth0"}, "", true, "fe80::1"},
		{"confia: só X-Real-IP → RemoteAddr", remoto, nil, "9.9.9.9", true, "10.0.0.1"},
		{"confia: XFF inválido + X-Real-IP → RemoteAddr", remoto, []string{"abc"}, "9.9.9.9", true, "10.0.0.1"},
		{"não confia: XFF ignorado", remoto, []string{"203.0.113.5"}, "", false, "10.0.0.1"},
		{"não confia: X-Real-IP ignorado", remoto, nil, "9.9.9.9", false, "10.0.0.1"},
		{"não confia: RemoteAddr IPv6 [::1]:5555", "[::1]:5555", []string{"203.0.113.5"}, "9.9.9.9", false, "::1"},
		{"não confia: RemoteAddr IPv4 mapeado", "[::ffff:10.0.0.2]:80", nil, "", false, "10.0.0.2"},
		{"não confia: RemoteAddr com zona", "[fe80::1%eth0]:80", nil, "", false, "fe80::1"},
		{"não confia: RemoteAddr não-IP fica como veio", "@", nil, "", false, "@"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = c.remoto
			for _, l := range c.xff {
				r.Header.Add("X-Forwarded-For", l)
			}
			if c.real != "" {
				r.Header.Set("X-Real-IP", c.real)
			}
			assert.Equal(t, c.want, httpx.IPDoCliente(r, c.confia))
		})
	}
}

// ---------------------------------------------------------------- SEC-21

func setCookiesCrus(t *testing.T, w *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	crus := map[string]string{}
	for _, sc := range w.Result().Header.Values("Set-Cookie") {
		nome, _, _ := strings.Cut(sc, "=")
		crus[nome] = sc
	}
	require.Len(t, crus, 2, "access_token e refresh_token")
	return crus
}

func atributos(sc string) map[string]bool {
	m := map[string]bool{}
	for _, p := range strings.Split(sc, ";")[1:] {
		m[strings.TrimSpace(p)] = true
	}
	return m
}

func TestSEC21_DefinirCookiesSessao_SecureDoParametro(t *testing.T) {
	for _, seguro := range []bool{true, false} {
		t.Run(map[bool]string{true: "seguro", false: "inseguro"}[seguro], func(t *testing.T) {
			w := httptest.NewRecorder()
			httpx.DefinirCookiesSessao(w, "acesso-x", "refresh-y", 90*time.Minute, seguro)
			crus := setCookiesCrus(t, w)

			acc, ref := atributos(crus[httpx.CookieAcesso]), atributos(crus[httpx.CookieRefresh])
			assert.True(t, strings.HasPrefix(crus[httpx.CookieAcesso], "access_token=acesso-x;"))
			assert.True(t, strings.HasPrefix(crus[httpx.CookieRefresh], "refresh_token=refresh-y;"))
			for nome, a := range map[string]map[string]bool{"access": acc, "refresh": ref} {
				assert.True(t, a["Path=/"], nome)
				assert.True(t, a["HttpOnly"], nome)
				assert.Equal(t, seguro, a["Secure"], "%s: Secure segue o parâmetro", nome)
			}
			assert.True(t, acc["SameSite=Strict"])
			assert.True(t, acc["Max-Age=5400"], "MaxAge = TTL do JWT: %s", crus[httpx.CookieAcesso])
			assert.True(t, ref["SameSite=Lax"])
			assert.True(t, ref["Max-Age=604800"], "refresh: 7 dias: %s", crus[httpx.CookieRefresh])
		})
	}
}

func TestSEC21_LimparCookiesSessao_MesmosAtributosDaCriacao(t *testing.T) {
	for _, seguro := range []bool{true, false} {
		t.Run(map[bool]string{true: "seguro", false: "inseguro"}[seguro], func(t *testing.T) {
			w := httptest.NewRecorder()
			httpx.LimparCookiesSessao(w, seguro)
			crus := setCookiesCrus(t, w)

			for nome, sameSite := range map[string]string{httpx.CookieAcesso: "SameSite=Strict", httpx.CookieRefresh: "SameSite=Lax"} {
				sc := crus[nome]
				a := atributos(sc)
				assert.True(t, strings.HasPrefix(sc, nome+"=;"), "valor vazio: %s", sc)
				assert.True(t, a["Path=/"], sc)
				assert.True(t, a["HttpOnly"], sc)
				assert.Equal(t, seguro, a["Secure"], sc)
				assert.True(t, a[sameSite], sc)
				assert.True(t, a["Max-Age=0"], sc)
				assert.True(t, a["Expires=Thu, 01 Jan 1970 00:00:00 GMT"], sc)
			}
		})
	}
}
