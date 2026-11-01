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

	"github.com/rotaperfumes/shared/adapters/turnstile"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
)

// siteverifyResp espelha o payload do Cloudflare (success + error-codes +
// hostname/action, SEC-19).
type siteverifyResp struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
	Hostname   string   `json:"hostname,omitempty"`
	Action     string   `json:"action,omitempty"`
}

// Hostname e action válidos dos stubs (SEC-19).
const (
	hostTeste = "rotaperfumes.local"
	acaoTeste = identidade.AcaoCaptchaLogin
)

var hostsTeste = []string{hostTeste, "127.0.0.1"}

// respOK é o success com hostname e action válidos.
func respOK() siteverifyResp {
	return siteverifyResp{Success: true, Hostname: hostTeste, Action: acaoTeste}
}

func writeSiteverify(w http.ResponseWriter, r siteverifyResp) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(r)
}

// newTurnstileServer sobe um httptest.Server e devolve um TurnstileService
// apontado para ele, com a secret informada.
func newTurnstileServer(t *testing.T, secret string, handler http.HandlerFunc) *turnstile.Verificador {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cfg := &config.Config{TurnstileSecretKey: secret}
	return turnstile.NovoComEndpoint(cfg.TurnstileSecretKey, srv.URL, hostsTeste, &http.Client{Timeout: 2 * time.Second})
}

func TestTurnstileService_Verify_FalhaAntesDeChamarRede(t *testing.T) {
	casos := []struct {
		nome    string
		secret  string
		token   string
		wantErr error
	}{
		{"token vazio", "secret-teste", "", turnstile.ErrCaptchaTokenAusente},
		{"token só com espaços", "secret-teste", "   ", turnstile.ErrCaptchaTokenAusente},
		{"secret não configurada", "", "token-valido", turnstile.ErrCaptchaInvalido},
		{"token vazio tem precedência sobre secret ausente", "", "", turnstile.ErrCaptchaTokenAusente},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			var chamadas atomic.Int32
			svc := newTurnstileServer(t, c.secret, func(w http.ResponseWriter, r *http.Request) {
				chamadas.Add(1)
				w.WriteHeader(http.StatusOK)
			})

			err := svc.Verificar(context.Background(), c.token, "1.2.3.4", acaoTeste)

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
				writeSiteverify(w, respOK())
			})

			assert.NoError(t, svc.Verificar(context.Background(), "token-valido", c.remoteIP, acaoTeste))
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

			err := svc.Verificar(context.Background(), "token-qualquer", "1.2.3.4", acaoTeste)

			require.Error(t, err)
			assert.ErrorIs(t, err, turnstile.ErrCaptchaInvalido)
			assert.False(t, errors.Is(err, turnstile.ErrCaptchaTokenAusente))
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

		svc := turnstile.NovoComEndpoint(cfg.TurnstileSecretKey, srv.URL, hostsTeste, &http.Client{Timeout: 50 * time.Millisecond})
		err := svc.Verificar(context.Background(), "token-qualquer", "1.2.3.4", acaoTeste)
		assert.ErrorIs(t, err, turnstile.ErrCaptchaInvalido)
	})

	t.Run("conexão recusada", func(t *testing.T) {
		svc := turnstile.NovoComEndpoint(cfg.TurnstileSecretKey, "http://127.0.0.1:1", hostsTeste, nil)
		err := svc.Verificar(context.Background(), "token-qualquer", "1.2.3.4", acaoTeste)
		assert.ErrorIs(t, err, turnstile.ErrCaptchaInvalido)
	})

	t.Run("URL inválida falha ao montar a requisição", func(t *testing.T) {
		svc := turnstile.NovoComEndpoint(cfg.TurnstileSecretKey, "http://[::1", hostsTeste, nil)
		err := svc.Verificar(context.Background(), "token-qualquer", "1.2.3.4", acaoTeste)
		require.Error(t, err)
		assert.ErrorIs(t, err, turnstile.ErrCaptchaInvalido)
		assert.Contains(t, err.Error(), "montar requisição")
	})

	t.Run("contexto já cancelado", func(t *testing.T) {
		svc := newTurnstileServer(t, "secret-teste", func(w http.ResponseWriter, r *http.Request) {
			writeSiteverify(w, respOK())
		})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := svc.Verificar(ctx, "token-qualquer", "1.2.3.4", acaoTeste)
		require.Error(t, err)
		assert.ErrorIs(t, err, turnstile.ErrCaptchaInvalido)
		assert.False(t, errors.Is(err, turnstile.ErrCaptchaTokenAusente))
	})
}

func TestNewTurnstileService_UsaSecretDaConfig(t *testing.T) {
	// NewTurnstileService aponta para o Cloudflare real; sem secret ele falha
	// fechado antes de qualquer acesso à rede, então é seguro testá-lo aqui.
	svc := turnstile.Novo((&config.Config{TurnstileSecretKey: ""}).TurnstileSecretKey, hostsTeste)
	require.NotNil(t, svc)
	err := svc.Verificar(context.Background(), "token", "1.2.3.4", acaoTeste)
	assert.ErrorIs(t, err, turnstile.ErrCaptchaInvalido)

	var _ identidadeapp.VerificadorCaptcha = svc
}

func TestNewTurnstileServiceWithEndpoint_PropagaSecretDaConfig(t *testing.T) {
	svc := newTurnstileServer(t, "minha-secret", func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "minha-secret", r.FormValue("secret"))
		writeSiteverify(w, respOK())
	})
	assert.NoError(t, svc.Verificar(context.Background(), "token-valido", "1.2.3.4", acaoTeste))
}

func TestNewTurnstileServiceWithEndpoint_ClientNilUsaDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSiteverify(w, respOK())
	}))
	t.Cleanup(srv.Close)

	svc := turnstile.NovoComEndpoint("s", srv.URL, hostsTeste, nil)
	assert.NoError(t, svc.Verificar(context.Background(), "token-valido", "", acaoTeste))
}

// SEC-19: depois do success, hostname (conjunto, sem diferenciar
// maiúsculas) e action (exata) precisam casar; a recusa não leva os
// valores recebidos.
func TestSEC19_Turnstile_ConfereHostnameEAction(t *testing.T) {
	casos := []struct {
		nome       string
		resp       siteverifyResp
		hostnames  []string
		wantMotivo string // "" = aceito
	}{
		{"hostname e action válidos", respOK(), hostsTeste, ""},
		{"segundo hostname do conjunto", siteverifyResp{Success: true, Hostname: "127.0.0.1", Action: acaoTeste}, hostsTeste, ""},
		{"hostname com outras maiúsculas é aceito", siteverifyResp{Success: true, Hostname: "RotaPerfumes.LOCAL", Action: acaoTeste}, hostsTeste, ""},
		{"hostname configurado com maiúsculas é aceito", respOK(), []string{" RotaPerfumes.Local "}, ""},
		{"hostname errado", siteverifyResp{Success: true, Hostname: "evil.example.com", Action: acaoTeste}, hostsTeste, "hostname inesperado"},
		{"hostname vazio", siteverifyResp{Success: true, Action: acaoTeste}, hostsTeste, "hostname inesperado"},
		{"lista de hostnames vazia recusa", respOK(), nil, "hostname inesperado"},
		{"action de outro fluxo", siteverifyResp{Success: true, Hostname: hostTeste, Action: identidade.AcaoCaptchaResetSenha}, hostsTeste, "action inesperada"},
		{"action com outras maiúsculas", siteverifyResp{Success: true, Hostname: hostTeste, Action: "LOGIN"}, hostsTeste, "action inesperada"},
		{"action vazia", siteverifyResp{Success: true, Hostname: hostTeste}, hostsTeste, "action inesperada"},
		{"success=false vem antes do hostname", siteverifyResp{Success: false, Hostname: "evil.example.com", ErrorCodes: []string{"x"}}, hostsTeste, "[x]"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeSiteverify(w, c.resp) }))
			t.Cleanup(srv.Close)
			svc := turnstile.NovoComEndpoint("secret-teste", srv.URL, c.hostnames, nil)

			err := svc.Verificar(context.Background(), "token-valido", "1.2.3.4", acaoTeste)

			if c.wantMotivo == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, turnstile.ErrCaptchaInvalido)
			assert.Contains(t, err.Error(), c.wantMotivo)
			if c.resp.Hostname != "" {
				assert.NotContains(t, err.Error(), c.resp.Hostname, "o erro não leva o hostname recebido")
			}
			if c.resp.Action != "" {
				assert.NotContains(t, err.Error(), c.resp.Action, "o erro não leva a action recebida")
			}
		})
	}
}

// SEC-19: com uma secret de teste da Cloudflare, hostname e action não são
// conferidos (o success continua valendo).
func TestSEC19_Turnstile_SecretDeTesteNaoConfereOrigem(t *testing.T) {
	for _, secret := range []string{
		"1x0000000000000000000000000000000AA",
		"2x0000000000000000000000000000000AA",
		"3x0000000000000000000000000000000AA",
	} {
		t.Run(secret[:2], func(t *testing.T) {
			assert.True(t, turnstile.SecretDeTeste(secret))
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeSiteverify(w, siteverifyResp{Success: true, Hostname: "example.com", Action: "qualquer"})
			}))
			t.Cleanup(srv.Close)
			svc := turnstile.NovoComEndpoint(secret, srv.URL, nil, nil)
			assert.NoError(t, svc.Verificar(context.Background(), "token-valido", "", acaoTeste))
		})
	}
	t.Run("success=false continua recusado", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeSiteverify(w, siteverifyResp{Success: false})
		}))
		t.Cleanup(srv.Close)
		svc := turnstile.NovoComEndpoint("1x0000000000000000000000000000000AA", srv.URL, nil, nil)
		assert.ErrorIs(t, svc.Verificar(context.Background(), "token-valido", "", acaoTeste), turnstile.ErrCaptchaInvalido)
	})
	for _, secret := range []string{"secret-teste", "", "1x0000000000000000000000000000000aa", "1x00000000000000000000AA"} {
		assert.False(t, turnstile.SecretDeTeste(secret), "secret %q", secret)
	}
}
