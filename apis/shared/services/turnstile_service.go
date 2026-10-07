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
	"github.com/rotaperfumes/shared/vlog"
)

// turnstileVerifyURL é o endpoint oficial de verificação do Cloudflare Turnstile.
// Os testes apontam para um httptest.Server local via
// NewTurnstileServiceWithEndpoint, sem alterar estado global.
const turnstileVerifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"

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
	verifyURL string
	client    *http.Client
}

// NewTurnstileService cria um TurnstileService a partir da configuração.
// secretKey vem de cfg.TurnstileSecretKey (env TURNSTILE_SECRET_KEY) —
// nunca deve ser logada.
func NewTurnstileService(cfg *config.Config) *TurnstileService {
	return NewTurnstileServiceWithEndpoint(cfg, turnstileVerifyURL, nil)
}

// NewTurnstileServiceWithEndpoint é NewTurnstileService com o endpoint
// siteverify e o *http.Client injetáveis (usado nos testes, apontando para um
// httptest.Server). client nil usa um client com o timeout padrão de
// verificação.
func NewTurnstileServiceWithEndpoint(cfg *config.Config, verifyURL string, client *http.Client) *TurnstileService {
	vlog.Printf("turnstile_service.go", "NewTurnstileServiceWithEndpoint", "verificando se foi injetado um http.Client")
	if client == nil {
		vlog.Printf("turnstile_service.go", "NewTurnstileServiceWithEndpoint", "criando http.Client padrão com timeout %s", turnstileVerifyTimeout)
		client = &http.Client{Timeout: turnstileVerifyTimeout}
	}
	return &TurnstileService{
		secretKey: cfg.TurnstileSecretKey,
		verifyURL: verifyURL,
		client:    client,
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
	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "verificando se o token de captcha foi enviado (token não logado)")
	if strings.TrimSpace(token) == "" {
		return ErrCaptchaTokenAusente
	}
	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "verificando se a secret do Turnstile está configurada")
	if s.secretKey == "" {
		// Falha fechada: sem secret configurada não há como verificar — recusa.
		return fmt.Errorf("%w: TURNSTILE_SECRET_KEY não configurada", ErrCaptchaInvalido)
	}

	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "criando contexto com timeout de %s", turnstileVerifyTimeout)
	ctx, cancel := context.WithTimeout(ctx, turnstileVerifyTimeout)
	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "agendando cancel do contexto")
	defer cancel()

	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "criando formulário do siteverify")
	form := url.Values{}
	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "adicionando secret ao formulário (valor não logado)")
	form.Set("secret", s.secretKey)
	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "adicionando token ao formulário (valor não logado)")
	form.Set("response", token)
	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "verificando se há IP remoto para enviar (IP não logado)")
	if remoteIP != "" {
		vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "adicionando IP remoto ao formulário")
		form.Set("remoteip", remoteIP)
	}

	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "montando requisição POST para o siteverify")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.verifyURL, strings.NewReader(form.Encode()))
	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "verificando se err != nil após montar a requisição")
	if err != nil {
		return fmt.Errorf("%w: erro ao montar requisição: %v", ErrCaptchaInvalido, err)
	}
	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "definindo Content-Type form-urlencoded")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "enviando requisição ao Cloudflare")
	resp, err := s.client.Do(req)
	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "verificando se err != nil após client.Do")
	if err != nil {
		return fmt.Errorf("%w: falha ao contatar Cloudflare: %v", ErrCaptchaInvalido, err)
	}
	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "agendando fechamento do corpo da resposta")
	defer resp.Body.Close()

	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "verificando status HTTP do Cloudflare (status=%d)", resp.StatusCode)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%w: status HTTP inesperado do Cloudflare: %d", ErrCaptchaInvalido, resp.StatusCode)
	}

	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "declarando estrutura da resposta do siteverify")
	var result turnstileSiteverifyResponse
	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "decodificando JSON da resposta (conteúdo não logado)")
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("%w: resposta inesperada do Cloudflare: %v", ErrCaptchaInvalido, err)
	}

	vlog.Printf("turnstile_service.go", "TurnstileService.Verify", "verificando success=%t retornado pelo Cloudflare", result.Success)
	if !result.Success {
		return fmt.Errorf("%w: %v", ErrCaptchaInvalido, result.ErrorCodes)
	}

	return nil
}
