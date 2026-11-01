package handlers_test

// SEC-19 (Lote 28): pelas rotas HTTP reais, o login pede ao verificador a
// action "login" e a troca de senha a action "reset-senha". Recusa do
// captcha continua 400 "captcha inválido ou ausente".

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/adapters/turnstile"
	"github.com/rotaperfumes/shared/identidade"
)

func TestSEC19_Login_PedeActionLogin(t *testing.T) {
	server, db, _, authHandler := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()
	captcha := &fakeCaptchaVerifier{err: turnstile.ErrCaptchaInvalido}
	authHandler.captcha.trocar(captcha)

	resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
		makeJSON(validCaptchaBody(map[string]any{"email": "a@test.com", "password": "x"})))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "captcha inválido ou ausente", decodeResponse(t, readBody(t, resp))["error"])
	assert.Equal(t, []string{identidade.AcaoCaptchaLogin}, captcha.acoesPedidas())
}

func TestSEC19_ResetPassword_PedeActionResetSenha(t *testing.T) {
	server, db, _, authHandler := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()
	captcha := &fakeCaptchaVerifier{err: turnstile.ErrCaptchaInvalido}
	authHandler.captcha.trocar(captcha)

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/auth/reset-password",
		makeJSON(validCaptchaBody(map[string]any{"senha_atual": "senha-atual", "nova_senha": "nova-senha-123"})))
	req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 2, "normal"))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "captcha inválido ou ausente", decodeResponse(t, readBody(t, resp))["error"])
	assert.Equal(t, []string{identidade.AcaoCaptchaResetSenha}, captcha.acoesPedidas())
}
