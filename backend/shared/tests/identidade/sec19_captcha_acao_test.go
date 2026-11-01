package identidade_test

// SEC-19: cada fluxo pede ao VerificadorCaptcha a sua action do Turnstile,
// para que um token de um fluxo não valha no outro.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/kernel"
)

func TestSEC19_ActionsDoCaptchaSaoDistintas(t *testing.T) {
	assert.Equal(t, "login", identidade.AcaoCaptchaLogin)
	assert.Equal(t, "reset-senha", identidade.AcaoCaptchaResetSenha)
}

func TestSEC19_LoginPedeActionLogin(t *testing.T) {
	a := novoAmbiente(usuarioAtivo(1, "a@b.c", "s", kernel.RoleNormal))
	_, err := a.casos().Login(ctx, identidadeapp.ComandoLogin{
		Email: "a@b.c", Senha: identidade.NovaSenha("s"), Captcha: "ok", Origem: identidadeapp.Origem{IP: "1.2.3.4"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{identidade.AcaoCaptchaLogin}, a.acoesCaptcha)
}

func TestSEC19_TrocarSenhaPedeActionResetSenha(t *testing.T) {
	a := novoAmbiente(usuarioAtivo(2, "a@b.c", "Atual#123", kernel.RoleNormal))
	require.NoError(t, a.casos().TrocarSenha(ctx, principal(2, kernel.RoleNormal), bug15CmdTroca()))
	assert.Equal(t, []string{identidade.AcaoCaptchaResetSenha}, a.acoesCaptcha)
}
