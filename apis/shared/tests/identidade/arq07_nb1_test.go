package identidade_test

// ARQ-07 / NB-1 (🟣 SecBrain, ARQ-03): a falha do envio da senha gerada é
// logada sem o e-mail do usuário. Servidores SMTP ecoam o RCPT nas respostas
// 550; o log leva só o e-mail mascarado e o erro com o endereço ocultado.

import (
	"bytes"
	"errors"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/kernel"
)

func TestARQ07_NB1_FalhaDoEmailNaoLogaOEndereco(t *testing.T) {
	const email = "maria.silva@exemplo.com"
	var buf bytes.Buffer
	anterior := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(anterior) })

	a := novoAmbiente(usuarioAtivo(5, email, "Gerada#1234567", kernel.RoleNormal))
	a.senhas = &geradorSenhaFake{senhas: []string{"Outra#1234567"}}
	a.email = emailFake{err: errors.New("smtp rcpt to: 550 5.1.1 <" + email + ">: mailbox " + email + " unavailable")}

	enviado, err := a.casos().AdminResetarSenha(ctx, principal(1, kernel.RoleAdmin), identidadeapp.ComandoAdminResetarSenha{UsuarioID: 5})
	require.NoError(t, err)
	assert.False(t, enviado)

	saida := buf.String()
	assert.Contains(t, saida, "falha ao enviar email de senha inicial para m***@exemplo.com: smtp rcpt to: 550 5.1.1 <[email-usuario]>: mailbox [email-usuario] unavailable")
	assert.NotContains(t, saida, email)
	assert.NotContains(t, saida, "Outra#1234567", "a senha gerada nunca vai para o log (M-02)")
}
