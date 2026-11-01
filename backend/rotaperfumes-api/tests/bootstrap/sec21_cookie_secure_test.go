package bootstrap_test

// SEC-21 (Lote 24, TestBrain): a montagem de produção (bootstrap.Montar)
// liga cfg.CookieSecure ao AuthHandler e avisa no log quando está false.
// Verificado pelo logout (não exige captcha nem banco): os cookies limpos
// trazem Secure exatamente quando COOKIE_SECURE=true.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
)

const sec21Aviso = "[config] COOKIE_SECURE=false: cookies de sessão sem Secure (somente dev em HTTP)"

func TestSEC21_Montar_CookieSecureEAviso(t *testing.T) {
	for _, seguro := range []bool{true, false} {
		t.Run(map[bool]string{true: "COOKIE_SECURE=true", false: "COOKIE_SECURE=false"}[seguro], func(t *testing.T) {
			logs := capturarLog(t)
			cfg := testCfg()
			cfg.CookieSecure = seguro
			pool, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			t.Cleanup(func() { pool.Close() })
			h := bootstrap.Montar(cfg, pool).Handler

			if seguro {
				assert.NotContains(t, logs.String(), "COOKIE_SECURE=false")
			} else {
				assert.Contains(t, logs.String(), sec21Aviso)
			}

			req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/api/auth/logout", strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", "http://app.test")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			scs := w.Result().Header.Values("Set-Cookie")
			require.Len(t, scs, 2)
			for _, sc := range scs {
				assert.Equal(t, seguro, strings.Contains(sc, "; Secure"), sc)
			}
		})
	}
}
