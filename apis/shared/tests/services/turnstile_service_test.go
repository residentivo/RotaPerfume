package services_test

// Testes do TurnstileService pela API pública. O endpoint siteverify e o
// *http.Client são injetados via NewTurnstileServiceWithEndpoint, apontando
// para um httptest.Server controlado pelo teste (sem estado global, então os
// testes podem rodar em paralelo).

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/services"
)

// siteverifyResp espelha o payload do Cloudflare (success + error-codes).
type siteverifyResp struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

func writeSiteverify(w http.ResponseWriter, r siteverifyResp) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(r)
}

// newTurnstileServer sobe um httptest.Server e devolve um TurnstileService
// apontado para ele, com a secret informada.
func newTurnstileServer(t *testing.T, secret string, handler http.HandlerFunc) *services.TurnstileService {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cfg := &config.Config{TurnstileSecretKey: secret}
	return services.NewTurnstileServiceWithEndpoint(cfg, srv.URL, &http.Client{Timeout: 2 * time.Second})
}

func TestTurnstileService_Verify_FalhaAntesDeChamarRede(t *testing.T) {
	casos := []struct {
		nome    string
		secret  string
		token   string
		wantErr error
	}{
		{"token vazio", "secret-teste", "", services.ErrCaptchaTokenAusente},
		{"token só com espaços", "secret-teste", "   ", services.ErrCaptchaTokenAusente},
		{"secret não configurada", "", "token-valido", services.ErrCaptchaInvalido},
		{"token vazio tem precedência sobre secret ausente", "", "", services.ErrCaptchaTokenAusente},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			var chamadas atomic.Int32
			svc := newTurnstileServer(t, c.secret, func(w http.ResponseWriter, r *http.Request) {
				chamadas.Add(1)
				w.WriteHeader(http.StatusOK)
			})

			err := svc.Verify(context.Background(), c.token, "1.2.3.4")

			require.Error(t, err)
			assert.ErrorIs(t, err, c.wantErr)
			assert.Zero(t, chamadas.Load(), "não deve chamar o Cloudflare")
		})
	}
}

func TestTurnstileService_Verify_Sucesso(t *testing.T) {
	casos := []struct {
		nome         string
		remoteIP     string
		wantRemoteIP string
	}{
		{"com remoteip", "9.9.9.9", "9.9.9.9"},
		{"sem remoteip (campo omitido)", "", ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			svc := newTurnstileServer(t, "secret-teste", func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
				assert.NoError(t, r.ParseForm())
				assert.Equal(t, "secret-teste", r.FormValue("secret"))
				assert.Equal(t, "token-valido", r.FormValue("response"))
				assert.Equal(t, c.wantRemoteIP, r.FormValue("remoteip"))
				_, temRemoteIP := r.PostForm["remoteip"]
				assert.Equal(t, c.remoteIP != "", temRemoteIP)
				writeSiteverify(w, siteverifyResp{Success: true})
			})

			assert.NoError(t, svc.Verify(context.Background(), "token-valido", c.remoteIP))
		})
	}
}

func TestTurnstileService_Verify_RespostasDeFalha(t *testing.T) {
	casos := []struct {
		nome    string
		handler http.HandlerFunc
	}{
		{"success=false", func(w http.ResponseWriter, r *http.Request) {
			writeSiteverify(w, siteverifyResp{Success: false, ErrorCodes: []string{"invalid-input-response"}})
		}},
		{"HTTP 400", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadRequest) }},
		{"HTTP 500", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }},
		{"HTTP 503", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }},
		{"HTTP 304 (não-2xx)", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotModified) }},
		{"JSON malformado", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{isso nao e json valido"))
		}},
		{"corpo vazio", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			svc := newTurnstileServer(t, "secret-teste", c.handler)

			err := svc.Verify(context.Background(), "token-qualquer", "1.2.3.4")

			require.Error(t, err)
			assert.ErrorIs(t, err, services.ErrCaptchaInvalido)
			assert.False(t, errors.Is(err, services.ErrCaptchaTokenAusente))
		})
	}
}

func TestTurnstileService_Verify_ErrosDeRede(t *testing.T) {
	cfg := &config.Config{TurnstileSecretKey: "secret-teste"}

	t.Run("timeout do client", func(t *testing.T) {
		// O release é fechado ANTES de srv.Close() (cleanups em ordem LIFO),
		// senão srv.Close() trava esperando o handler ainda bloqueado.
		release := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
		t.Cleanup(srv.Close)
		t.Cleanup(func() { close(release) })

		svc := services.NewTurnstileServiceWithEndpoint(cfg, srv.URL, &http.Client{Timeout: 50 * time.Millisecond})
		err := svc.Verify(context.Background(), "token-qualquer", "1.2.3.4")
		assert.ErrorIs(t, err, services.ErrCaptchaInvalido)
	})

	t.Run("conexão recusada", func(t *testing.T) {
		svc := services.NewTurnstileServiceWithEndpoint(cfg, "http://127.0.0.1:1", nil)
		err := svc.Verify(context.Background(), "token-qualquer", "1.2.3.4")
		assert.ErrorIs(t, err, services.ErrCaptchaInvalido)
	})

	t.Run("URL inválida falha ao montar a requisição", func(t *testing.T) {
		svc := services.NewTurnstileServiceWithEndpoint(cfg, "http://[::1", nil)
		err := svc.Verify(context.Background(), "token-qualquer", "1.2.3.4")
		require.Error(t, err)
		assert.ErrorIs(t, err, services.ErrCaptchaInvalido)
		assert.Contains(t, err.Error(), "montar requisição")
	})

	t.Run("contexto já cancelado", func(t *testing.T) {
		svc := newTurnstileServer(t, "secret-teste", func(w http.ResponseWriter, r *http.Request) {
			writeSiteverify(w, siteverifyResp{Success: true})
		})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := svc.Verify(ctx, "token-qualquer", "1.2.3.4")
		require.Error(t, err)
		assert.ErrorIs(t, err, services.ErrCaptchaInvalido)
		assert.False(t, errors.Is(err, services.ErrCaptchaTokenAusente))
	})
}

func TestNewTurnstileService_UsaSecretDaConfig(t *testing.T) {
	// NewTurnstileService aponta para o Cloudflare real; sem secret ele falha
	// fechado antes de qualquer acesso à rede, então é seguro testá-lo aqui.
	svc := services.NewTurnstileService(&config.Config{TurnstileSecretKey: ""})
	require.NotNil(t, svc)
	err := svc.Verify(context.Background(), "token", "1.2.3.4")
	assert.ErrorIs(t, err, services.ErrCaptchaInvalido)

	var _ services.CaptchaVerifier = svc
}

func TestNewTurnstileServiceWithEndpoint_PropagaSecretDaConfig(t *testing.T) {
	svc := newTurnstileServer(t, "minha-secret", func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "minha-secret", r.FormValue("secret"))
		writeSiteverify(w, siteverifyResp{Success: true})
	})
	assert.NoError(t, svc.Verify(context.Background(), "token-valido", "1.2.3.4"))
}

func TestNewTurnstileServiceWithEndpoint_ClientNilUsaDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSiteverify(w, siteverifyResp{Success: true})
	}))
	t.Cleanup(srv.Close)

	svc := services.NewTurnstileServiceWithEndpoint(&config.Config{TurnstileSecretKey: "s"}, srv.URL, nil)
	assert.NoError(t, svc.Verify(context.Background(), "token-valido", ""))
}
