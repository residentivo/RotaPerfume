package httpx_test

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

func testCfg() *config.Config {
	return &config.Config{
		JWTSecret: "test-secret-httpx",
		JWTIssuer: "rotaperfumes-test",
		JWTTTL:    time.Hour,
	}
}

func token(t *testing.T, uid int64, role string) string {
	t.Helper()
	tok, err := identidadecrypto.NovoJWT(testCfg()).Emitir(uid, kernel.RoleDoBanco(role))
	require.NoError(t, err)
	return tok
}

// checkerFake: user 1 admin, demais normal, todos ativos; status sobrescreve.
type checkerFake struct {
	status map[int64]identidade.StatusAcesso
	calls  *int
}

func (f checkerFake) StatusParaAutenticacao(_ context.Context, userID int64) (identidade.StatusAcesso, error) {
	if f.calls != nil {
		*f.calls++
	}
	if st, ok := f.status[userID]; ok {
		return st, nil
	}
	if userID == 1 {
		return identidade.StatusAcesso{Ativo: true, Role: "admin"}, nil
	}
	return identidade.StatusAcesso{Ativo: true, Role: "normal"}, nil
}

func novoRouter(checker httpx.ConsultaStatus) *httpx.Router {
	return httpx.NovoRouter(httpx.NovoAutenticador(identidadecrypto.NovoJWT(testCfg()), checker))
}

func handlerOK(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }

func servir(h http.Handler, metodo, alvo, tok string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(metodo, alvo, nil)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// saidaLog captura log (e vlog, se ligado) com segurança para concorrência.
type saidaLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *saidaLog) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *saidaLog) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func capturarLog(t *testing.T, vlogLigado bool) *saidaLog {
	t.Helper()
	out := &saidaLog{}
	prevOut, prevFlags, prevVlog := log.Writer(), log.Flags(), vlog.Enabled()
	log.SetOutput(out)
	log.SetFlags(0)
	vlog.SetEnabled(vlogLigado)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
		vlog.SetEnabled(prevVlog)
	})
	return out
}
