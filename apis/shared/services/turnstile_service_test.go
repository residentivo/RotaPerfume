package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

// withTurnstileTestServer troca temporariamente turnstileVerifyURL para
// apontar para um httptest.Server controlado pelo teste, restaurando o valor
// original ao final. Como turnstileVerifyURL é uma var de pacote (só para
// testabilidade — ver comentário na declaração), os testes deste arquivo não
// podem rodar em paralelo entre si (t.Parallel não é usado aqui de propósito).
func withTurnstileTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	original := turnstileVerifyURL
	turnstileVerifyURL = srv.URL
	t.Cleanup(func() {
		srv.Close()
		turnstileVerifyURL = original
	})
	return srv
}

func newTestTurnstileService(secretKey string) *TurnstileService {
	return &TurnstileService{
		secretKey: secretKey,
		client:    &http.Client{Timeout: 2 * time.Second},
	}
}

func TestTurnstileService_Verify_TokenAusente(t *testing.T) {
	casos := []struct {
		nome  string
		token string
	}{
		{"vazio", ""},
		{"somente espacos", "   "},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			called := false
			withTurnstileTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			svc := newTestTurnstileService("secret-teste")
			err := svc.Verify(context.Background(), c.token, "1.2.3.4")

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrCaptchaTokenAusente)
			assert.False(t, called, "não deve chamar o Cloudflare quando o token está ausente")
		})
	}
}

func TestTurnstileService_Verify_SecretAusente(t *testing.T) {
	called := false
	withTurnstileTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	svc := newTestTurnstileService("")
	err := svc.Verify(context.Background(), "token-valido", "1.2.3.4")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaptchaInvalido)
	assert.False(t, called, "não deve chamar o Cloudflare quando a secret não está configurada")
}

func TestTurnstileService_Verify_Sucesso(t *testing.T) {
	withTurnstileTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "secret-teste", r.FormValue("secret"))
		assert.Equal(t, "token-valido", r.FormValue("response"))
		assert.Equal(t, "9.9.9.9", r.FormValue("remoteip"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(turnstileSiteverifyResponse{Success: true})
	})

	svc := newTestTurnstileService("secret-teste")
	err := svc.Verify(context.Background(), "token-valido", "9.9.9.9")

	assert.NoError(t, err)
}

func TestTurnstileService_Verify_SemRemoteIP(t *testing.T) {
	withTurnstileTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Empty(t, r.FormValue("remoteip"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(turnstileSiteverifyResponse{Success: true})
	})

	svc := newTestTurnstileService("secret-teste")
	err := svc.Verify(context.Background(), "token-valido", "")

	assert.NoError(t, err)
}

func TestTurnstileService_Verify_SuccessFalse(t *testing.T) {
	withTurnstileTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(turnstileSiteverifyResponse{
			Success:    false,
			ErrorCodes: []string{"invalid-input-response"},
		})
	})

	svc := newTestTurnstileService("secret-teste")
	err := svc.Verify(context.Background(), "token-invalido", "1.2.3.4")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaptchaInvalido)
}

func TestTurnstileService_Verify_StatusHTTPNaoSucesso(t *testing.T) {
	casos := []int{http.StatusBadRequest, http.StatusInternalServerError, http.StatusServiceUnavailable}

	for _, status := range casos {
		t.Run(http.StatusText(status), func(t *testing.T) {
			withTurnstileTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			})

			svc := newTestTurnstileService("secret-teste")
			err := svc.Verify(context.Background(), "token-qualquer", "1.2.3.4")

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrCaptchaInvalido)
		})
	}
}

func TestTurnstileService_Verify_JSONMalformado(t *testing.T) {
	withTurnstileTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{isso nao e json valido"))
	})

	svc := newTestTurnstileService("secret-teste")
	err := svc.Verify(context.Background(), "token-qualquer", "1.2.3.4")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaptchaInvalido)
}

func TestTurnstileService_Verify_TimeoutRedeErro(t *testing.T) {
	// Servidor que nunca responde dentro do timeout do client — simula
	// indisponibilidade/timeout de rede do Cloudflare. IMPORTANTE: o release
	// precisa ser fechado ANTES de srv.Close() (cleanups rodam em ordem LIFO),
	// senão srv.Close() trava esperando a conexão do handler ainda bloqueado.
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	original := turnstileVerifyURL
	turnstileVerifyURL = srv.URL
	t.Cleanup(func() {
		srv.Close()
		turnstileVerifyURL = original
	})
	t.Cleanup(func() { close(release) })

	svc := &TurnstileService{
		secretKey: "secret-teste",
		client:    &http.Client{Timeout: 50 * time.Millisecond},
	}

	err := svc.Verify(context.Background(), "token-qualquer", "1.2.3.4")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaptchaInvalido)
}

func TestTurnstileService_Verify_ConexaoRecusada(t *testing.T) {
	// URL para uma porta em que ninguém está escutando: simula erro de rede
	// (conexão recusada), diferente de timeout.
	original := turnstileVerifyURL
	turnstileVerifyURL = "http://127.0.0.1:1"
	t.Cleanup(func() { turnstileVerifyURL = original })

	svc := newTestTurnstileService("secret-teste")
	err := svc.Verify(context.Background(), "token-qualquer", "1.2.3.4")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaptchaInvalido)
}

func TestTurnstileService_Verify_ContextoJaCancelado(t *testing.T) {
	withTurnstileTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	svc := newTestTurnstileService("secret-teste")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.Verify(ctx, "token-qualquer", "1.2.3.4")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaptchaInvalido)
	assert.False(t, errors.Is(err, ErrCaptchaTokenAusente))
}

func TestNewTurnstileService_UsaSecretDaConfig(t *testing.T) {
	// NewTurnstileService deve propagar a secret da config e configurar um
	// client com timeout — validado indiretamente: sem secret, falha fechada
	// antes mesmo de contatar a rede.
	svc := &TurnstileService{secretKey: "", client: &http.Client{}}
	err := svc.Verify(context.Background(), "token", "1.2.3.4")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCaptchaInvalido)
}

func TestNewTurnstileService_Constructor(t *testing.T) {
	withTurnstileTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "minha-secret", r.FormValue("secret"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(turnstileSiteverifyResponse{Success: true})
	})

	cfg := &config.Config{TurnstileSecretKey: "minha-secret"}
	svc := NewTurnstileService(cfg)

	err := svc.Verify(context.Background(), "token-valido", "1.2.3.4")
	assert.NoError(t, err)
}
