package services_test

// MAIL-01: seleção do modo TLS (SMTP_TLS_MODE ou porta), ServerName
// configurável (SMTP_TLS_SERVER_NAME), CA extra (SMTP_TLS_CA_FILE), TLS
// implícito (SMTPS) e AUTH obrigatório (falha clara, nada enviado, quando o
// servidor não anuncia AUTH). Usa o fakeSMTP de email_smtp_test.go; o
// certificado do httptest vale para 127.0.0.1, ::1 e example.com.

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/notificacao"
	"github.com/rotaperfumes/shared/notificacao/notificacaosmtp"
)

// lerAuthLogin conduz o diálogo AUTH LOGIN do lado do servidor fake.
func lerAuthLogin(r *bufio.Reader, reply func(string)) (user, pass string, ok bool) {
	ler := func(prompt string) (string, bool) {
		reply("334 " + base64.StdEncoding.EncodeToString([]byte(prompt)))
		linha, err := r.ReadString('\n')
		if err != nil {
			return "", false
		}
		raw, err := base64.StdEncoding.DecodeString(strings.TrimRight(linha, "\r\n"))
		return string(raw), err == nil
	}
	if user, ok = ler("Username:"); !ok {
		return "", "", false
	}
	pass, ok = ler("Password:")
	return user, pass, ok
}

func TestMAIL01_ResolverModoTLSSMTP(t *testing.T) {
	casos := []struct {
		modo, porta string
		want        notificacao.ModoTLS
	}{
		{"", "465", notificacao.ModoTLSImplicito},
		{"", " 465 ", notificacao.ModoTLSImplicito},
		{"", "587", notificacao.ModoSTARTTLS},
		{"", "25", notificacao.ModoSTARTTLS},
		{"auto", "465", notificacao.ModoTLSImplicito},
		{"implicit", "587", notificacao.ModoTLSImplicito},
		{" IMPLICIT ", "25", notificacao.ModoTLSImplicito},
		{"starttls", "465", notificacao.ModoSTARTTLS},
		{"STARTTLS", "587", notificacao.ModoSTARTTLS},
	}
	for _, c := range casos {
		got, err := notificacao.ResolverModoTLS(c.modo, c.porta)
		require.NoError(t, err, "modo=%q porta=%q", c.modo, c.porta)
		assert.Equal(t, c.want, got, "modo=%q porta=%q", c.modo, c.porta)
	}

	for _, invalido := range []string{"ssl", "plain", "none", "tls"} {
		_, err := notificacao.ResolverModoTLS(invalido, "465")
		assert.ErrorIs(t, err, notificacao.ErrModoTLSInvalido, "modo=%q", invalido)
	}
}

func TestMAIL01_ResolverServerNameTLSSMTP(t *testing.T) {
	assert.Equal(t, "192.168.168.106", notificacaosmtp.ResolverServerName("192.168.168.106", ""))
	assert.Equal(t, "192.168.168.106", notificacaosmtp.ResolverServerName("192.168.168.106", "   "))
	assert.Equal(t, "mail.exemplo.com", notificacaosmtp.ResolverServerName("192.168.168.106", " mail.exemplo.com "))
}

func TestMAIL01_Construtor_ModoEServerName(t *testing.T) {
	t.Run("porta 465 sem SMTP_TLS_MODE vira implicit; ServerName = host", func(t *testing.T) {
		svc, err := notificacaosmtp.Novo(smtpCfg("192.168.168.106", "465"))
		require.NoError(t, err)
		assert.Equal(t, notificacao.ModoTLSImplicito, svc.ModoTLS())
		assert.Equal(t, "192.168.168.106", svc.ServerNameTLS())
	})
	t.Run("SMTP_TLS_MODE e SMTP_TLS_SERVER_NAME explícitos", func(t *testing.T) {
		cfg := smtpCfg("192.168.168.106", "465")
		cfg.SMTPTLSMode = "starttls"
		cfg.SMTPTLSServerName = "mail.exemplo.com"
		svc, err := notificacaosmtp.Novo(cfg)
		require.NoError(t, err)
		assert.Equal(t, notificacao.ModoSTARTTLS, svc.ModoTLS())
		assert.Equal(t, "mail.exemplo.com", svc.ServerNameTLS())
	})
	t.Run("SMTP_TLS_MODE inválido é erro de configuração (não ErrSMTPNaoConfigurado)", func(t *testing.T) {
		cfg := smtpCfg("smtp.exemplo.com", "587")
		cfg.SMTPTLSMode = "plain"
		svc, err := notificacaosmtp.Novo(cfg)
		assert.Nil(t, svc)
		assert.ErrorIs(t, err, notificacao.ErrModoTLSInvalido)
		assert.False(t, errors.Is(err, notificacao.ErrSMTPNaoConfigurado))
	})
	t.Run("SMTP_TLS_CA_FILE inexistente", func(t *testing.T) {
		cfg := smtpCfg("smtp.exemplo.com", "587")
		cfg.SMTPTLSCAFile = filepath.Join(t.TempDir(), "nao-existe.pem")
		_, err := notificacaosmtp.Novo(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "SMTP_TLS_CA_FILE")
	})
	t.Run("SMTP_TLS_CA_FILE sem PEM válido", func(t *testing.T) {
		caminho := filepath.Join(t.TempDir(), "lixo.pem")
		require.NoError(t, os.WriteFile(caminho, []byte("não é um certificado"), 0o600))
		cfg := smtpCfg("smtp.exemplo.com", "587")
		cfg.SMTPTLSCAFile = caminho
		_, err := notificacaosmtp.Novo(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "nenhum certificado PEM válido")
	})
}

// enviarTeste envia uma senha inicial e devolve o erro.
func enviarTeste(t *testing.T, svc *notificacaosmtp.SMTP) error {
	t.Helper()
	return svc.EnviarSenhaInicial(context.Background(), "cliente@exemplo.com", "Maria", "S3nh@Forte!")
}

func TestMAIL01_TLSImplicito_Sucesso(t *testing.T) {
	cert, pool := smtpTLS(t)
	f := &fakeSMTP{cert: cert, implicito: true}
	host, port := f.start(t)

	cfg := smtpCfg(host, port)
	cfg.SMTPTLSMode = "implicit" // porta efêmera: o modo precisa ser explícito
	svc, err := notificacaosmtp.NovoComTLS(cfg, &tls.Config{RootCAs: pool})
	require.NoError(t, err)
	require.NoError(t, enviarTeste(t, svc))

	f.mu.Lock()
	defer f.mu.Unlock()
	assert.True(t, f.usouTLS)
	raw, err := base64.StdEncoding.DecodeString(f.authResp)
	require.NoError(t, err)
	assert.Equal(t, "\x00user@teste.com\x00senha-de-app", string(raw))
	assert.Equal(t, "<cliente@exemplo.com>", f.rcptTo)
	assert.Contains(t, f.data, "S3nh@Forte!")
}

func TestMAIL01_TLSImplicito_ServidorEmTextoPuro(t *testing.T) {
	cert, pool := smtpTLS(t)
	f := &fakeSMTP{cert: cert} // fala texto puro (STARTTLS), não SMTPS
	host, port := f.start(t)

	cfg := smtpCfg(host, port)
	cfg.SMTPTLSMode = "implicit"
	svc, err := notificacaosmtp.NovoComTLS(cfg, &tls.Config{RootCAs: pool})
	require.NoError(t, err)
	err = enviarTeste(t, svc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "smtp tls implícito")

	f.mu.Lock()
	defer f.mu.Unlock()
	assert.Empty(t, f.authResp, "nenhuma credencial pode ter sido enviada")
}

func TestMAIL01_ServerName(t *testing.T) {
	cert, pool := smtpTLS(t)

	casos := []struct {
		nome       string
		host       string
		serverName string
		wantErr    string
	}{
		{"host fora do certificado, sem SMTP_TLS_SERVER_NAME: falha na verificação", "localhost", "", "certificate"},
		{"host fora do certificado, SMTP_TLS_SERVER_NAME válido: envia", "localhost", "example.com", ""},
		{"SMTP_TLS_SERVER_NAME fora do certificado: falha", "127.0.0.1", "mail.errado.invalid", "certificate"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			f := &fakeSMTP{cert: cert, implicito: true}
			_, port := f.start(t)

			cfg := smtpCfg(c.host, port)
			cfg.SMTPTLSMode = "implicit"
			cfg.SMTPTLSServerName = c.serverName
			svc, err := notificacaosmtp.NovoComTLS(cfg, &tls.Config{RootCAs: pool})
			require.NoError(t, err)

			err = enviarTeste(t, svc)
			f.mu.Lock()
			defer f.mu.Unlock()
			if c.wantErr == "" {
				require.NoError(t, err)
				assert.NotEmpty(t, f.authResp, "PlainAuth deve usar o mesmo ServerName do TLS")
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantErr)
			assert.Empty(t, f.authResp, "nenhuma credencial pode ter sido enviada")
		})
	}
}

func TestMAIL01_CAFile_ConfiaNoCertificadoDoArquivo(t *testing.T) {
	cert, _ := smtpTLS(t)
	f := &fakeSMTP{cert: cert, implicito: true}
	host, port := f.start(t)

	caminho := filepath.Join(t.TempDir(), "smtp-ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	require.NoError(t, os.WriteFile(caminho, pemBytes, 0o600))

	cfg := smtpCfg(host, port)
	cfg.SMTPTLSMode = "implicit"

	t.Run("sem SMTP_TLS_CA_FILE o certificado não é confiável", func(t *testing.T) {
		svc, err := notificacaosmtp.Novo(cfg)
		require.NoError(t, err)
		err = enviarTeste(t, svc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "smtp tls implícito")
	})
	t.Run("com SMTP_TLS_CA_FILE envia, verificação ligada", func(t *testing.T) {
		cfg.SMTPTLSCAFile = caminho
		// InsecureSkipVerify do chamador continua ignorado (SEC-10).
		svc, err := notificacaosmtp.NovoComTLS(cfg, &tls.Config{InsecureSkipVerify: true})
		require.NoError(t, err)
		require.NoError(t, enviarTeste(t, svc))
	})
}

func TestMAIL01_SemAuth_FalhaSemEnviar(t *testing.T) {
	cert, pool := smtpTLS(t)
	for _, implicito := range []bool{false, true} {
		t.Run(map[bool]string{false: "starttls", true: "implicit"}[implicito], func(t *testing.T) {
			f := &fakeSMTP{cert: cert, implicito: implicito, falha: "sem-auth"}
			host, port := f.start(t)
			cfg := smtpCfg(host, port)
			if implicito {
				cfg.SMTPTLSMode = "implicit"
			}
			svc, err := notificacaosmtp.NovoComTLS(cfg, &tls.Config{RootCAs: pool})
			require.NoError(t, err)

			err = enviarTeste(t, svc)
			require.Error(t, err)
			assert.ErrorIs(t, err, notificacaosmtp.ErrSemAuth)
			assert.Contains(t, err.Error(), "smtp auth")

			f.mu.Lock()
			defer f.mu.Unlock()
			assert.Empty(t, f.mailFrom, "não pode seguir para MAIL FROM sem autenticar")
			assert.Empty(t, f.data)
		})
	}
}

func TestMAIL01_AuthLogin_EMecanismoNaoSuportado(t *testing.T) {
	cert, pool := smtpTLS(t)

	t.Run("só LOGIN anunciado: usa AUTH LOGIN", func(t *testing.T) {
		f := &fakeSMTP{cert: cert, mecanismos: "LOGIN"}
		host, port := f.start(t)
		svc, err := notificacaosmtp.NovoComTLS(smtpCfg(host, port), &tls.Config{RootCAs: pool})
		require.NoError(t, err)
		require.NoError(t, enviarTeste(t, svc))

		f.mu.Lock()
		defer f.mu.Unlock()
		assert.Equal(t, "user@teste.com", f.loginUser)
		assert.Equal(t, "senha-de-app", f.loginPass)
		assert.Empty(t, f.authResp, "não deve usar PLAIN")
	})
	t.Run("PLAIN e LOGIN: prefere PLAIN", func(t *testing.T) {
		f := &fakeSMTP{cert: cert, mecanismos: "LOGIN PLAIN"}
		host, port := f.start(t)
		svc, err := notificacaosmtp.NovoComTLS(smtpCfg(host, port), &tls.Config{RootCAs: pool})
		require.NoError(t, err)
		require.NoError(t, enviarTeste(t, svc))

		f.mu.Lock()
		defer f.mu.Unlock()
		assert.NotEmpty(t, f.authResp)
		assert.Empty(t, f.loginUser)
	})
	t.Run("nem PLAIN nem LOGIN: erro claro", func(t *testing.T) {
		f := &fakeSMTP{cert: cert, mecanismos: "CRAM-MD5 XOAUTH2"}
		host, port := f.start(t)
		svc, err := notificacaosmtp.NovoComTLS(smtpCfg(host, port), &tls.Config{RootCAs: pool})
		require.NoError(t, err)
		err = enviarTeste(t, svc)
		assert.ErrorIs(t, err, notificacaosmtp.ErrMecanismoAuth)

		f.mu.Lock()
		defer f.mu.Unlock()
		assert.Empty(t, f.mailFrom)
	})
}
