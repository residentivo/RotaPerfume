package identidade_test

// LOG-03 (Lote 31): a recusa do captcha é logada com o motivo do enum fixo
// (success|hostname|action), sem o token, a secret, o hostname e a action
// recebidos. A resposta continua 400 "captcha inválido ou ausente".

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/adapters/turnstile"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/kernel"
)

const (
	log03Secret   = "secret-producao-LOG03-nao-pode-vazar"
	log03Token    = "token-LOG03-nao-pode-vazar"
	log03HostRuim = "host-recebido-LOG03.example"
	log03AcaoRuim = "acao-recebida-LOG03"
)

func log03Verificador(t *testing.T, resp map[string]any) *turnstile.Verificador {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return turnstile.NovoComEndpoint(log03Secret, srv.URL, []string{"rotaperfumes.local"}, nil)
}

func TestLOG03_RecusaDoCaptcha_LogaSoOMotivo(t *testing.T) {
	casos := []struct {
		nome, motivo string
		resp         map[string]any
	}{
		{"success=false", "success", map[string]any{"success": false, "error-codes": []string{"invalid-input-response"}, "hostname": log03HostRuim, "action": log03AcaoRuim}},
		{"hostname inesperado", "hostname", map[string]any{"success": true, "hostname": log03HostRuim, "action": identidade.AcaoCaptchaLogin}},
		{"action inesperada", "action", map[string]any{"success": true, "hostname": "rotaperfumes.local", "action": log03AcaoRuim}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			buf := capturarLogSEC32(t)
			a := novoAmbiente(usuarioAtivo(2, "a@b.c", "Atual#123", kernel.RoleNormal))
			d := a.deps()
			d.Captcha = log03Verificador(t, c.resp)

			_, err := identidadeapp.Novo(d, identidadeapp.Opcoes{}).Login(context.Background(), identidadeapp.ComandoLogin{
				Email: "a@b.c", Senha: identidade.NovaSenha("Atual#123"), Captcha: log03Token, Origem: identidadeapp.Origem{IP: "1.2.3.4"}})

			assert.Same(t, identidade.ErrCaptchaInvalido, err, "resposta HTTP inalterada (400)")
			assert.Equal(t, "captcha inválido ou ausente", err.Error())
			saida := buf.String()
			assert.Contains(t, saida, "[auth] login: captcha recusado: motivo="+c.motivo+" ")
			for _, segredo := range []string{log03Token, log03Secret, log03HostRuim, log03AcaoRuim} {
				assert.NotContains(t, saida, segredo)
			}
		})
	}
}

// Erro sem motivo (rede, secret ausente) continua com o log genérico;
// token ausente, com o log de ausente.
func TestLOG03_ErroSemMotivo_LogGenerico(t *testing.T) {
	buf := capturarLogSEC32(t)
	a := novoAmbiente(usuarioAtivo(2, "a@b.c", "Atual#123", kernel.RoleNormal))
	d := a.deps()
	d.Captcha = turnstile.NovoComEndpoint(log03Secret, "http://127.0.0.1:1", nil, nil)
	_, err := identidadeapp.Novo(d, identidadeapp.Opcoes{}).Login(context.Background(), identidadeapp.ComandoLogin{
		Email: "a@b.c", Senha: identidade.NovaSenha("Atual#123"), Captcha: log03Token, Origem: identidadeapp.Origem{IP: "1.2.3.4"}})
	assert.Same(t, identidade.ErrCaptchaInvalido, err)
	assert.Contains(t, buf.String(), "[auth] login: falha na verificação de captcha: ip=1.2.3.4")
	assert.NotContains(t, buf.String(), "motivo=")
	assert.NotContains(t, buf.String(), log03Token)
}

// motivo fora do enum (adapter futuro) não é logado.
type recusaMotivoEstranho struct{}

func (recusaMotivoEstranho) Error() string        { return "x" }
func (recusaMotivoEstranho) MotivoRecusa() string { return "valor-recebido-qualquer" }

type captchaRecusaEstranha struct{}

func (captchaRecusaEstranha) Verificar(context.Context, string, string, string) error {
	return recusaMotivoEstranho{}
}

func TestLOG03_MotivoForaDoEnum_NaoELogado(t *testing.T) {
	buf := capturarLogSEC32(t)
	a := novoAmbiente(usuarioAtivo(2, "a@b.c", "Atual#123", kernel.RoleNormal))
	d := a.deps()
	d.Captcha = captchaRecusaEstranha{}
	_, err := identidadeapp.Novo(d, identidadeapp.Opcoes{}).Login(context.Background(), identidadeapp.ComandoLogin{
		Email: "a@b.c", Senha: identidade.NovaSenha("Atual#123"), Captcha: "tok", Origem: identidadeapp.Origem{IP: "1.2.3.4"}})
	assert.Same(t, identidade.ErrCaptchaInvalido, err)
	assert.NotContains(t, buf.String(), "valor-recebido-qualquer")
}

func TestLOG03_AdapterExpoeMotivoEContinuaErrCaptchaInvalido(t *testing.T) {
	v := log03Verificador(t, map[string]any{"success": true, "hostname": log03HostRuim, "action": identidade.AcaoCaptchaLogin})
	err := v.Verificar(context.Background(), log03Token, "1.2.3.4", identidade.AcaoCaptchaLogin)
	require.ErrorIs(t, err, turnstile.ErrCaptchaInvalido)
	var m interface{ MotivoRecusa() string }
	require.True(t, errors.As(err, &m))
	assert.Equal(t, turnstile.MotivoHostname, m.MotivoRecusa())
}
