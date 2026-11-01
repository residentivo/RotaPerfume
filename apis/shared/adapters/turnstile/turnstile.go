// Package turnstile verifica o token do Cloudflare Turnstile (Adapter da
// porta identidadeapp.VerificadorCaptcha, ARQ-03). Fail-closed (I-13):
// token ausente, secret ausente, erro de rede/timeout, status não 2xx,
// payload inesperado ou success=false recusam. Token e secret nunca são
// logados.
//
// SEC-19: depois do success, o hostname da resposta precisa estar no
// conjunto configurado (sem diferenciar maiúsculas; conjunto vazio recusa
// tudo) e a action precisa ser exatamente a esperada pelo fluxo. Com uma
// secret de teste da Cloudflare (SecretDeTeste) hostname e action não são
// conferidos.
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

// secretsDeTeste são as secrets de teste documentadas pela Cloudflare
// (sempre passa, sempre falha, token já usado). docs/deploy-servidor.md
// documenta a primeira para a rede interna.
var secretsDeTeste = map[string]bool{
	"1x0000000000000000000000000000000AA": true,
	"2x0000000000000000000000000000000AA": true,
	"3x0000000000000000000000000000000AA": true,
}

// SecretDeTeste informa se a secret é uma das secrets de teste da
// Cloudflare. Com ela o Verificador não confere hostname nem action (o
// bootstrap avisa no log de startup).
func SecretDeTeste(secret string) bool { return secretsDeTeste[secret] }

// Verificador chama o siteverify com a secret da config.
type Verificador struct {
	secret    string
	url       string
	client    *http.Client
	hostnames map[string]bool
	modoTeste bool
}

// Novo usa o endpoint oficial e um client com o timeout padrão.
func Novo(secret string, hostnames []string) *Verificador {
	return NovoComEndpoint(secret, URLVerificacao, hostnames, nil)
}

// NovoComEndpoint injeta o endpoint e o client (testes com httptest).
// client nil usa um com o timeout padrão. hostnames são os hosts aceitos na
// resposta (comparados em minúsculas); lista vazia recusa todo token.
func NovoComEndpoint(secret, endpoint string, hostnames []string, client *http.Client) *Verificador {
	if client == nil {
		client = &http.Client{Timeout: timeoutVerificacao}
	}
	return &Verificador{
		secret: secret, url: endpoint, client: client,
		hostnames: conjuntoHostnames(hostnames), modoTeste: SecretDeTeste(secret),
	}
}

func conjuntoHostnames(hostnames []string) map[string]bool {
	conjunto := make(map[string]bool, len(hostnames))
	for _, h := range hostnames {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			conjunto[h] = true
		}
	}
	return conjunto
}

type respostaSiteverify struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
	Hostname   string   `json:"hostname"`
	Action     string   `json:"action"`
}

// Verificar confere o token do cliente junto com o IP de origem e a action
// esperada pelo fluxo (identidade.AcaoCaptchaLogin/AcaoCaptchaResetSenha).
func (v *Verificador) Verificar(ctx context.Context, token, ip, acaoEsperada string) error {
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
	return v.conferirOrigem(r, acaoEsperada)
}

// conferirOrigem confere hostname e action depois do success (SEC-19). Os
// erros não levam os valores recebidos.
func (v *Verificador) conferirOrigem(r respostaSiteverify, acaoEsperada string) error {
	if v.modoTeste {
		return nil
	}
	if r.Hostname == "" || !v.hostnames[strings.ToLower(r.Hostname)] {
		return fmt.Errorf("%w: hostname inesperado", ErrCaptchaInvalido)
	}
	if r.Action == "" || r.Action != acaoEsperada {
		return fmt.Errorf("%w: action inesperada", ErrCaptchaInvalido)
	}
	return nil
}
