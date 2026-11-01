package testeemail_test

// ARQ-07 NB-E: com -inseguro (certificado não validado) o diagnóstico não
// envia credenciais, nem por PLAIN nem por LOGIN.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
)

func TestARQ07_NBE_InseguroRecusaAuth(t *testing.T) {
	cert, pemCA := certificadoTeste(t)
	for _, c := range []struct{ mecanismos, modo string }{
		{"PLAIN LOGIN", "starttls"},
		{"LOGIN", "ssl"},
	} {
		t.Run(c.mecanismos+"/"+c.modo, func(t *testing.T) {
			srv := &smtpFalso{cert: cert, implicito: c.modo == "ssl", mecanismos: c.mecanismos}
			a := novoAmbienteDiag(t, srv.iniciar(t), pemCA)

			require.Equal(t, cmdutil.SaidaErro, a.rodar(t, "-modo", c.modo, "-inseguro"))
			out := a.logs.String()
			assert.Contains(t, out, "AUTH recusado pelo diagnóstico: -inseguro ativo")
			assert.Contains(t, out, "rode sem SMTP_USER/SMTP_PASSWORD")
			assert.NotContains(t, srv.conversa(), "AUTH", "nenhuma credencial vai ao servidor")
			assert.NotContains(t, out, diagSenha)
		})
	}
}
