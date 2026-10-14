package routes_test

// ARQ-01: casos novos da cadeia (§5.8) e da exceção X1 (§7.3/§7.4), sobre o
// mesmo router real da matriz ARQ-00:
//   - método não registrado → 405 com Allow e security headers, com ou sem
//     token, SEM consultar o checker (O3, R-b);
//   - HEAD em rota GET protegida sem token → 401; HEAD /health → 200 (R-b);
//   - OPTIONS sem Origin → 204 sem ACAO, nunca 405 (O3);
//   - 401 não emite o rótulo vlog do handler (auth por fora do Trace, O9);
//   - GET /api/senha-historico/{user_id...} com parse estrito (R-f).

import (
	"bytes"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/vlog"
)

// metodosPorPath agrupa os métodos registrados por path concreto da matriz.
func metodosPorPath() map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, r := range matrizRotas {
		for _, m := range r.metodos() {
			for _, pth := range r.paths {
				if out[pth] == nil {
					out[pth] = map[string]bool{}
				}
				out[pth][m] = true
			}
		}
	}
	return out
}

func pathsOrdenados(m map[string]map[string]bool) []string {
	out := make([]string, 0, len(m))
	for pth := range m {
		out = append(out, pth)
	}
	sort.Strings(out)
	return out
}

// Método errado (X1) → 405 do ServeMux, antes da autenticação: com ou sem
// token, o checker não é consultado e o banco não é tocado.
func TestARQ00_MetodoNaoRegistrado_405SemChecker(t *testing.T) {
	porPath := metodosPorPath()
	tokens := map[string]func(t *testing.T) string{
		"sem token":    func(*testing.T) string { return "" },
		"token admin":  func(t *testing.T) string { return token(t, 1, "admin") },
		"token normal": func(t *testing.T) string { return token(t, 2, "normal") },
	}
	for _, pth := range pathsOrdenados(porPath) {
		registrados := porPath[pth]
		for _, m := range todosMetodos {
			if registrados[m] {
				continue
			}
			for nomeTok, tokFn := range tokens {
				t.Run(m+" "+pth+" | "+nomeTok, func(t *testing.T) {
					calls := 0
					h, mock := arq00Router(t, fakeUserStatusChecker{calls: &calls})
					w := arq00Serve(h, m, pth, tokFn(t), arq00OrigemPermitida)

					assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
					assert.NotEmpty(t, w.Header().Get("Allow"), "405 deve informar os métodos permitidos")
					for metodo := range registrados {
						assert.Contains(t, w.Header().Get("Allow"), metodo)
					}
					assert.Equal(t, 0, calls, "405 sai antes da autenticação: checker não pode ser consultado")
					assertSecurityHeaders(t, w)
					assert.Equal(t, arq00OrigemPermitida, w.Header().Get("Access-Control-Allow-Origin"))
					assert.NoError(t, mock.ExpectationsWereMet())
				})
			}
		}
	}
}

// §7.3: as rotas de auth que antes aceitavam qualquer método.
func TestARQ01_X1_AuthLogin_MetodoErrado_405(t *testing.T) {
	for _, m := range []string{http.MethodGet, http.MethodDelete, http.MethodPut} {
		t.Run(m, func(t *testing.T) {
			calls := 0
			h, mock := arq00Router(t, fakeUserStatusChecker{calls: &calls})
			w := arq00Serve(h, m, "/api/auth/login", "", "")
			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
			assert.Contains(t, w.Header().Get("Allow"), http.MethodPost)
			assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
			assert.Equal(t, 0, calls)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// R-b: o padrão GET também atende HEAD, e HEAD não fura a autenticação.
func TestARQ00_HEAD_RotaGETProtegida_SemToken_401(t *testing.T) {
	for _, c := range casosDaMatriz(protegida) {
		if c.method != http.MethodGet {
			continue
		}
		t.Run("HEAD "+c.path, func(t *testing.T) {
			calls := 0
			h, mock := arq00Router(t, fakeUserStatusChecker{calls: &calls})
			w := arq00Serve(h, http.MethodHead, c.path, "", "")
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Equal(t, 0, calls)
			assertSecurityHeaders(t, w)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestARQ00_HEAD_Health_200(t *testing.T) {
	calls := 0
	h, _ := arq00Router(t, fakeUserStatusChecker{calls: &calls})
	w := arq00Serve(h, http.MethodHead, "/health", "", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 0, calls)
	assertSecurityHeaders(t, w)
}

// O3: todo OPTIONS → 204 no CORS, antes do mux, mesmo sem Origin e mesmo em
// rota de método fixo (nunca 405) — sem headers Access-Control-Allow-*.
func TestARQ00_Preflight_SemOrigin_204SemACAO(t *testing.T) {
	for _, r := range matrizRotas {
		for _, pth := range r.paths {
			t.Run(r.padrao+" | OPTIONS "+pth, func(t *testing.T) {
				calls := 0
				h, mock := arq00Router(t, fakeUserStatusChecker{calls: &calls})
				w := arq00Serve(h, http.MethodOptions, pth, "", "")

				assert.Equal(t, http.StatusNoContent, w.Code)
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
				assert.Empty(t, w.Header().Get("Allow"), "preflight não chega ao mux")
				assert.Equal(t, 0, calls)
				assert.Empty(t, w.Body.String())
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

// arq01Log captura a saída do log com o vlog ligado.
type arq01Log struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *arq01Log) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *arq01Log) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func capturarVlog(t *testing.T) *arq01Log {
	t.Helper()
	buf := &arq01Log{}
	prevOut, prevFlags, prevEnabled := log.Writer(), log.Flags(), vlog.Enabled()
	log.SetOutput(buf)
	log.SetFlags(0)
	vlog.SetEnabled(true)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
		vlog.SetEnabled(prevEnabled)
	})
	return buf
}

// O9: a autenticação fica POR FORA do Trace. Um 401/403 não emite o rótulo
// do handler; um request aprovado emite início/fim com o rótulo.
func TestARQ01_Trace_PorDentroDaAutenticacao(t *testing.T) {
	const rotuloMe = "[auth_handler.go] [AuthHandler.Me]"
	const rotuloUsuarios = "[usuario_handler.go] [UsuarioHandler.ListUsuarios]"

	t.Run("GET /api/auth/me sem token: 401 sem rótulo", func(t *testing.T) {
		out := capturarVlog(t)
		h, _ := arq00Router(t, fakeUserStatusChecker{})
		w := arq00Serve(h, http.MethodGet, "/api/auth/me", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		assert.NotContains(t, out.String(), rotuloMe)
	})
	t.Run("AdminOnly com normal: 403 sem rótulo", func(t *testing.T) {
		out := capturarVlog(t)
		h, _ := arq00Router(t, fakeUserStatusChecker{})
		w := arq00Serve(h, http.MethodGet, "/api/usuarios", token(t, 2, "normal"), "")
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.NotContains(t, out.String(), rotuloUsuarios)
	})
	t.Run("aprovado: rótulo com início e fim", func(t *testing.T) {
		out := capturarVlog(t)
		h, _ := arq00Router(t, fakeUserStatusChecker{})
		tok := token(t, 1, "admin")
		w := arq00Serve(h, http.MethodGet, "/api/usuarios?q=busca-secreta", tok, "")
		assert.NotEqual(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, out.String(), rotuloUsuarios+" início GET\n")
		assert.Contains(t, out.String(), rotuloUsuarios+" fim status=500\n")
		assert.NotContains(t, out.String(), tok, "o nível de rota nunca loga headers")
		assert.NotContains(t, out.String(), "busca-secreta", "o nível de rota nunca loga a query string")
	})
}

// R-f: GET /api/senha-historico/{user_id...} com parse estrito. Só 400 (ou
// 404, se o próprio ServeMux recusar o caminho); nunca chega ao banco.
func TestARQ01_SenhaHistorico_ParseEstrito(t *testing.T) {
	casos := []struct {
		path    string
		status  []int
		mensage string
	}{
		{"/api/senha-historico/", []int{http.StatusBadRequest}, "usuario_id é obrigatório na URL"},
		{"/api/senha-historico/2/extra", []int{http.StatusBadRequest}, "usuario_id inválido"},
		{"/api/senha-historico/0", []int{http.StatusBadRequest}, "usuario_id inválido"},
		{"/api/senha-historico/-1", []int{http.StatusBadRequest}, "usuario_id inválido"},
		{"/api/senha-historico/abc", []int{http.StatusBadRequest}, "usuario_id inválido"},
		{"/api/senha-historico/+2", []int{http.StatusBadRequest}, "usuario_id inválido"},
		{"/api/senha-historico/%202", []int{http.StatusBadRequest}, "usuario_id inválido"},
		{"/api/senha-historico/+2", []int{http.StatusBadRequest}, "usuario_id inválido"},
		{"/api/senha-historico/%202", []int{http.StatusBadRequest}, "usuario_id inválido"},
		{"/api/senha-historico/..%2F2", []int{http.StatusBadRequest, http.StatusNotFound}, ""},
		{"/api/senha-historico/2%2F3", []int{http.StatusBadRequest, http.StatusNotFound}, ""},
	}
	for _, c := range casos {
		t.Run(c.path, func(t *testing.T) {
			h, mock := arq00Router(t, fakeUserStatusChecker{})
			w := arq00Serve(h, http.MethodGet, c.path, token(t, 1, "admin"), "")

			assert.Contains(t, c.status, w.Code, "body: %s", w.Body.String())
			if c.mensage != "" {
				assert.Contains(t, w.Body.String(), c.mensage)
			}
			assert.NoError(t, mock.ExpectationsWereMet(), "o parse falho não pode consultar o banco")
		})
	}
}

// A política do wildcard de resto continua AdminOnly em qualquer forma de path.
func TestARQ01_SenhaHistorico_Wildcard_ContinuaAdmin(t *testing.T) {
	for _, pth := range []string{"/api/senha-historico/2/extra", "/api/senha-historico/0", "/api/senha-historico/..%2F2"} {
		t.Run(pth, func(t *testing.T) {
			h, _ := arq00Router(t, fakeUserStatusChecker{})
			sem := arq00Serve(h, http.MethodGet, pth, "", "")
			normal := arq00Serve(h, http.MethodGet, pth, token(t, 2, "normal"), "")
			if sem.Code == http.StatusNotFound || sem.Code == http.StatusMovedPermanently {
				return // o ServeMux recusou o caminho antes de qualquer rota
			}
			assert.Equal(t, http.StatusUnauthorized, sem.Code)
			assert.Equal(t, http.StatusForbidden, normal.Code)
			assert.True(t, strings.Contains(normal.Body.String(), msgAdminRestrito))
		})
	}
}

func TestARQ01_RotasRegistradas_TodasComRotuloVlog(t *testing.T) {
	for _, rt := range arq00RotasRegistradas(t) {
		require.NotEmpty(t, rt.Arquivo, rt.Padrao)
		assert.True(t, strings.HasSuffix(rt.Arquivo, ".go"), "%s: rótulo de arquivo %q", rt.Padrao, rt.Arquivo)
		assert.NotContains(t, rt.Funcao, " ", "%s: rótulo de função %q", rt.Padrao, rt.Funcao)
	}
}
