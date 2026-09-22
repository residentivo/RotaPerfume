package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/config"
)

// turnstileVerifyURL é o endpoint oficial de verificação do Cloudflare Turnstile.
// É uma var (e não const) apenas para permitir que os testes deste pacote
// apontem para um httptest.Server local — em produção seu valor nunca muda.
var turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

// turnstileVerifyTimeout limita o tempo de espera pela resposta do Cloudflare
// para não travar requisições de login/reset em caso de indisponibilidade.
const turnstileVerifyTimeout = 4 * time.Second

// ErrCaptchaTokenAusente é retornado quando o token do Turnstile não foi
// enviado pelo cliente — falha fechada, sem sequer chamar o Cloudflare.
var ErrCaptchaTokenAusente = errors.New("services: token de captcha ausente")

// ErrCaptchaInvalido é retornado quando o Cloudflare rejeita o token
// (success=false) ou quando a verificação não pode ser concluída com
// confiança (erro de rede, timeout, resposta inesperada). Em qualquer um
// desses casos o chamador deve recusar a operação (fail-closed).
var ErrCaptchaInvalido = errors.New("services: captcha inválido ou não verificável")

// CaptchaVerifier abstrai a verificação de CAPTCHA para permitir mocks em testes.
type CaptchaVerifier interface {
	// Verify valida o token do Turnstile enviado pelo cliente (campo
	// "captchaToken" no JSON de login/reset-password), junto com o IP de
	// origem da requisição. Retorna ErrCaptchaTokenAusente se token=="" e
	// ErrCaptchaInvalido para qualquer falha de verificação (fail-closed).
	Verify(ctx context.Context, token, remoteIP string) error
}

// TurnstileService implementa CaptchaVerifier usando a API siteverify do
// Cloudflare Turnstile.
type TurnstileService struct {
	secretKey string
	client    *http.Client
}

// NewTurnstileService cria um TurnstileService a partir da configuração.
// secretKey vem de cfg.TurnstileSecretKey (env TURNSTILE_SECRET_KEY) —
// nunca deve ser logada.
func NewTurnstileService(cfg *config.Config) *TurnstileService {
	return &TurnstileService{
		secretKey: cfg.TurnstileSecretKey,
		client: &http.Client{
			Timeout: turnstileVerifyTimeout,
		},
	}
}

// turnstileSiteverifyResponse é o payload retornado pelo Cloudflare.
type turnstileSiteverifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify chama o endpoint siteverify do Cloudflare Turnstile. Comportamento
// fail-closed: qualquer ausência de token, erro de rede/timeout, resposta
// HTTP não-2xx, payload inesperado, ou success=false resulta em erro — nunca
// deixa passar silenciosamente. O token e a secret key nunca são logados.
func (s *TurnstileService) Verify(ctx context.Context, token, remoteIP string) error {
	if strings.TrimSpace(token) == "" {
		return ErrCaptchaTokenAusente
	}
	if s.secretKey == "" {
		// Falha fechada: sem secret configurada não há como verificar — recusa.
		return fmt.Errorf("%w: TURNSTILE_SECRET_KEY não configurada", ErrCaptchaInvalido)
	}

	ctx, cancel := context.WithTimeout(ctx, turnstileVerifyTimeout)
	defer cancel()

	form := url.Values{}
	form.Set("secret", s.secretKey)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("%w: erro ao montar requisição: %v", ErrCaptchaInvalido, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: falha ao contatar Cloudflare: %v", ErrCaptchaInvalido, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: status HTTP inesperado do Cloudflare: %d", ErrCaptchaInvalido, resp.StatusCode)
	}

	var result turnstileSiteverifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("%w: resposta inesperada do Cloudflare: %v", ErrCaptchaInvalido, err)
	}

	if !result.Success {
		return fmt.Errorf("%w: %v", ErrCaptchaInvalido, result.ErrorCodes)
	}

	return nil
}
