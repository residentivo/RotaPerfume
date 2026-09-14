// Testes de EmailService.
//
// SMTPEmailService.enviar() faz handshake SMTP real via rede (net.Dialer +
// net/smtp) e a função buildMessage() é não-exportada — nenhuma delas é
// testável isoladamente a partir deste pacote externo (services_test) sem
// golpear uma rede/servidor real ou duplicar acesso interno ao pacote.
// Por isso, cobrimos aqui apenas a parte pura/testável sem rede:
//   - validação de configuração em NewSMTPEmailService (sucesso/erro sem I/O)
//   - comportamento do NoopEmailService (não faz I/O, apenas loga)
//   - conformidade de ambas implementações com a interface EmailService
package services_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/services"
)

func fullSMTPConfig() *config.Config {
	return &config.Config{
		SMTPHost:     "smtp.gmail.com",
		SMTPPort:     "587",
		SMTPUser:     "user@gmail.com",
		SMTPPassword: "app-password",
		SMTPFrom:     "no-reply@rotaperfumes.com",
	}
}

// ---------------------------------------------------------------------------
// NewSMTPEmailService — validação de configuração (sem I/O de rede)
// ---------------------------------------------------------------------------

func TestNewSMTPEmailService(t *testing.T) {
	testCases := []struct {
		nome    string
		cfg     func() *config.Config
		wantErr bool
	}{
		{
			nome:    "configuração completa - sucesso",
			cfg:     fullSMTPConfig,
			wantErr: false,
		},
		{
			nome: "host ausente",
			cfg: func() *config.Config {
				c := fullSMTPConfig()
				c.SMTPHost = ""
				return c
			},
			wantErr: true,
		},
		{
			nome: "user ausente",
			cfg: func() *config.Config {
				c := fullSMTPConfig()
				c.SMTPUser = ""
				return c
			},
			wantErr: true,
		},
		{
			nome: "password ausente",
			cfg: func() *config.Config {
				c := fullSMTPConfig()
				c.SMTPPassword = ""
				return c
			},
			wantErr: true,
		},
		{
			nome: "from ausente",
			cfg: func() *config.Config {
				c := fullSMTPConfig()
				c.SMTPFrom = ""
				return c
			},
			wantErr: true,
		},
		{
			nome:    "config totalmente vazia",
			cfg:     func() *config.Config { return &config.Config{} },
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			svc, err := services.NewSMTPEmailService(tc.cfg())
			if tc.wantErr {
				assert.Nil(t, svc)
				assert.ErrorIs(t, err, services.ErrSMTPNaoConfigurado)
			} else {
				require.NoError(t, err)
				require.NotNil(t, svc)
				// Confirma que a implementação satisfaz a interface EmailService.
				var _ services.EmailService = svc
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NoopEmailService — não realiza I/O, apenas loga
// ---------------------------------------------------------------------------

func TestNoopEmailService_EnviarSenhaInicial(t *testing.T) {
	testCases := []struct {
		nome         string
		destinatario string
		nomeUsuario  string
		senha        string
	}{
		{"caso padrão", "user@test.com", "Fulano", "senha-gerada-123"},
		{"destinatario vazio ainda assim não retorna erro", "", "Fulano", "senha-gerada-123"},
		{"nome vazio ainda assim não retorna erro", "user@test.com", "", "senha-gerada-123"},
	}

	svc := services.NewNoopEmailService()
	var _ services.EmailService = svc

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			err := svc.EnviarSenhaInicial(context.Background(), tc.destinatario, tc.nomeUsuario, tc.senha)
			assert.NoError(t, err, "NoopEmailService nunca deve falhar (não faz I/O real)")
		})
	}
}
