package bootstrap_test

// bootstrap.Montar (composition root, ARQ-01): a cadeia de produção é
// Logging → CORS → SecurityHeaders → mux → Exigir → Trace → handler (F-01),
// com o checker do banco (SEC-06) e o e-mail noop quando não há SMTP.

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/notificacao/notificacaosmtp"
)

type saida struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *saida) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *saida) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func capturarLog(t *testing.T) *saida {
	t.Helper()
	out := &saida{}
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(out)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return out
}

func testCfg() *config.Config {
	return &config.Config{
		JWTSecret:          "test-secret-bootstrap-com-pelo-menos-32-bytes",
		JWTIssuer:          "rotaperfumes-test",
		JWTTTL:             time.Hour,
		CORSAllowedOrigins: []string{"http://app.test"},
		HashSenha:          config.HashSenha{Pepper: "pepper-de-teste-com-pelo-menos-32-bytes", MemoriaKiB: 64, Iteracoes: 1, Paralelismo: 1},
	}
}

func montar(t *testing.T) (http.Handler, sqlmock.Sqlmock) {
	t.Helper()
	pool, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })
	return bootstrap.Montar(testCfg(), pool).Handler, mock
}

func TestMontar_CadeiaDeProducao(t *testing.T) {
	casos := []struct {
		nome    string
		metodo  string
		alvo    string
		origem  string
		status  int
		logHTTP string
	}{
		{"health", "GET", "/health", "", 200, "[http] GET /health -> 200 ("},
		{"HEAD health", "HEAD", "/health", "", 200, "[http] HEAD /health -> 200 ("},
		{"preflight sem Origin", "OPTIONS", "/api/auth/login", "", 204, "[http] OPTIONS /api/auth/login -> 204 ("},
		{"protegida sem token", "GET", "/api/clientes?q=secreto", "http://app.test", 401, "[http] GET /api/clientes -> 401 ("},
		{"método errado (X1)", "DELETE", "/api/auth/me", "", 405, "[http] DELETE /api/auth/me -> 405 ("},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			h, mock := montar(t)
			out := capturarLog(t)
			req := httptest.NewRequest(c.metodo, c.alvo, nil)
			if c.origem != "" {
				req.Header.Set("Origin", c.origem)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			assert.Equal(t, c.status, w.Code)
			if c.metodo != http.MethodOptions { // o preflight sai no CORS, antes dos security headers (F-01)
				assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
			}
			assert.Contains(t, out.String(), c.logHTTP, "Logging é o middleware mais externo")
			assert.NotContains(t, out.String(), "secreto", "o log de acesso não leva query string")
			if c.origem != "" {
				assert.Equal(t, c.origem, w.Header().Get("Access-Control-Allow-Origin"))
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// SEC-06: em produção o checker é o do banco — token válido consulta
// usuarios (sem cache) antes do handler.
func TestMontar_CheckerDoBanco(t *testing.T) {
	h, mock := montar(t)
	mock.ExpectQuery(`FROM usuarios`).WithArgs(int64(2)).WillReturnRows(
		sqlmock.NewRows([]string{"ativo", "role", "tokens_validos_desde"}).AddRow(false, "normal", nil))

	tok, err := identidadecrypto.NovoJWT(testCfg()).Emitir(2, kernel.RoleNormal)
	require.NoError(t, err)
	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "usuário inativo")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestNovoRouter_ContagemEMetodosDasRotas: o total de rotas registradas
// (61 desde o DASH-01, que somou GET /api/dashboard/funil) e o método de
// cada uma. O nome não carrega o número para não envelhecer a cada rota.
func TestNovoRouter_ContagemEMetodosDasRotas(t *testing.T) {
	pool, _, err := sqlmock.New()
	require.NoError(t, err)
	defer pool.Close()
	cfg := testCfg()
	hs := bootstrap.NovosHandlers(pool, cfg, notificacaosmtp.NovoNoop())
	rotas := bootstrap.NovoRouter(cfg, checkerNulo{}, hs).Rotas()
	assert.Len(t, rotas, 61) // 60 + /api/dashboard/funil (DASH-01)
	re := regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE)$`)
	funil := 0
	for _, rt := range rotas {
		assert.Regexp(t, re, rt.Metodo, rt.Padrao)
		if rt.Padrao == "/api/dashboard/funil" {
			funil++
			assert.Equal(t, "GET", rt.Metodo)
			assert.Equal(t, "DashboardHandler.GetFunil", rt.Funcao)
		}
	}
	assert.Equal(t, 1, funil, "GET /api/dashboard/funil registrada uma vez (DASH-01)")
}

func TestHandlerHTTP_CheckerNilPanica(t *testing.T) {
	pool, _, err := sqlmock.New()
	require.NoError(t, err)
	defer pool.Close()
	cfg := testCfg()
	hs := bootstrap.NovosHandlers(pool, cfg, notificacaosmtp.NovoNoop())
	assert.Panics(t, func() { bootstrap.HandlerHTTP(cfg, nil, hs) })
}

// checkerNulo satisfaz o checker só para montar a tabela de rotas.
type checkerNulo struct{}

func (checkerNulo) StatusParaAutenticacao(context.Context, int64) (identidade.StatusAcesso, error) {
	return identidade.StatusAcesso{}, nil
}

// ARQ-03 (Observer, §5.12): em produção o evento de reuso de refresh token
// chega ao notificador de alerta (SEC-09) pelo publicador montado no
// bootstrap, depois do corte de sessões; a resposta continua o 401 de sempre.
func TestMontar_ReusoDeRefresh_PublicaParaONotificador(t *testing.T) {
	logs := capturarLog(t)
	h, mock := montar(t)
	mock.MatchExpectationsInOrder(false)
	tok := "refresh-reusado"
	sum := identidadecrypto.GeradorRefresh{}.Hash(identidade.NovoTokenOpaco(tok))
	mock.ExpectQuery(`FROM refresh_tokens\s+WHERE token_hash = \?`).WithArgs(sum).
		WillReturnRows(sqlmock.NewRows([]string{"id", "usuario_id", "token_hash", "expires_at", "revoked_at", "ip_origem", "user_agent", "revoked_reason"}).
			AddRow(int64(10), int64(5), sum, time.Now().Add(time.Hour), time.Now().Add(-time.Hour), "ip", "ua", "rotacao"))
	mock.ExpectExec(`SET reuso_detectado_em = \?`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`WHERE usuario_id = \? AND revoked_at IS NULL`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE usuarios SET tokens_validos_desde`).WillReturnResult(sqlmock.NewResult(0, 1))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewBufferString(`{"refresh_token":"`+tok+`"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "refresh token revogado")
	// O notificador é assíncrono: espera a linha dele (usuário não carregado
	// no sqlmock, sem admins → nenhum destinatário).
	assert.Eventually(t, func() bool {
		return regexp.MustCompile(`\[auth\]\[seguranca\] alerta.*user_id=5`).MatchString(logs.String())
	}, 3*time.Second, 10*time.Millisecond, logs.String())
}
