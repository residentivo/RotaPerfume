// Package turnstile verifica o token do Cloudflare Turnstile (Adapter da
// porta identidadeapp.VerificadorCaptcha, ARQ-03). Fail-closed (I-13):
// token ausente, secret ausente, erro de rede/timeout, status não 2xx,
// payload inesperado ou success=false recusam. Token e secret nunca são
// logados.
package turnstile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// URLVerificacao é o endpoint oficial de siteverify.
const URLVerificacao = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// timeoutVerificacao limita a espera pelo Cloudflare (F-04).
const timeoutVerificacao = 4 * time.Second

var (
	// ErrCaptchaTokenAusente: o cliente não enviou o token (o Cloudflare
	// nem é chamado).
	ErrCaptchaTokenAusente = errors.New("turnstile: token de captcha ausente")
	// ErrCaptchaInvalido: recusado ou não verificável com confiança.
	ErrCaptchaInvalido = errors.New("turnstile: captcha inválido ou não verificável")
)

// Verificador chama o siteverify com a secret da config.
type Verificador struct {
	secret string
	url    string
	client *http.Client
}

// Novo usa o endpoint oficial e um client com o timeout padrão.
func Novo(secret string) *Verificador { return NovoComEndpoint(secret, URLVerificacao, nil) }

// NovoComEndpoint injeta o endpoint e o client (testes com httptest).
// client nil usa um com o timeout padrão.
func NovoComEndpoint(secret, endpoint string, client *http.Client) *Verificador {
	if client == nil {
		client = &http.Client{Timeout: timeoutVerificacao}
	}
	return &Verificador{secret: secret, url: endpoint, client: client}
}

type respostaSiteverify struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verificar confere o token do cliente junto com o IP de origem.
func (v *Verificador) Verificar(ctx context.Context, token, ip string) error {
	if strings.TrimSpace(token) == "" {
		return ErrCaptchaTokenAusente
	}
	if v.secret == "" {
		return fmt.Errorf("%w: TURNSTILE_SECRET_KEY não configurada", ErrCaptchaInvalido)
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutVerificacao)
	defer cancel()

	form := url.Values{"secret": {v.secret}, "response": {token}}
	if ip != "" {
		form.Set("remoteip", ip)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.url, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("%w: erro ao montar requisição: %v", ErrCaptchaInvalido, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: falha ao contatar Cloudflare: %v", ErrCaptchaInvalido, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: status HTTP inesperado do Cloudflare: %d", ErrCaptchaInvalido, resp.StatusCode)
	}
	var r respostaSiteverify
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("%w: resposta inesperada do Cloudflare: %v", ErrCaptchaInvalido, err)
	}
	if !r.Success {
		return fmt.Errorf("%w: %v", ErrCaptchaInvalido, r.ErrorCodes)
	}
	return nil
}
