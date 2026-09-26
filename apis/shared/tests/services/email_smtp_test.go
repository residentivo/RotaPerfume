package services_test

// SMTPEmailService.EnviarSenhaInicial contra um servidor SMTP falso local
// (127.0.0.1, porta efêmera) que fala o protocolo de verdade, inclusive o
// STARTTLS. O certificado TLS é o do httptest (válido para 127.0.0.1), e a CA
// dele é injetada via NewSMTPEmailServiceWithTLSConfig. Nenhum servidor real
// é contatado. Cada caso faz o servidor falhar em um passo do handshake para
// cobrir todos os caminhos de erro de enviar().

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"net"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/services"
)

// smtpTLS devolve o certificado de servidor do httptest (válido para
// 127.0.0.1) e um pool com a CA correspondente, para o cliente confiar nele.
func smtpTLS(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	srv.StartTLS()
	defer srv.Close()
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return srv.TLS.Certificates[0], pool
}

// fakeSMTP é um servidor SMTP mínimo. falha indica o passo que responde erro.
type fakeSMTP struct {
	falha string
	cert  tls.Certificate

	mu       sync.Mutex
	authResp string
	mailFrom string
	rcptTo   string
	data     string
	usouTLS  bool
}

func (f *fakeSMTP) start(t *testing.T) (host, port string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(conn)
		}
	}()
	h, p, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	return h, p
}

func (f *fakeSMTP) serve(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	reply := func(s string) {
		_, _ = w.WriteString(s + "\r\n")
		_ = w.Flush()
	}

	if f.falha == "greeting" {
		reply("554 servico indisponivel")
		return
	}
	reply("220 fake ESMTP")

	emTLS := false
	for {
		linha, err := r.ReadString('\n')
		if err != nil {
			return
		}
		linha = strings.TrimRight(linha, "\r\n")
		cmd := strings.ToUpper(linha)
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			ext := []string{"250-fake"}
			if !emTLS && f.falha != "sem-starttls" {
				ext = append(ext, "250-STARTTLS")
			}
			if emTLS && f.falha != "sem-auth" {
				ext = append(ext, "250-AUTH PLAIN")
			}
			ext = append(ext, "250 8BITMIME")
			reply(strings.Join(ext, "\r\n"))
		case cmd == "STARTTLS":
			if f.falha == "starttls" {
				reply("454 TLS indisponivel")
				continue
			}
			reply("220 pronto para TLS")
			tc := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{f.cert}})
			if err := tc.Handshake(); err != nil {
				return
			}
			conn = tc
			r = bufio.NewReader(tc)
			w = bufio.NewWriter(tc)
			emTLS = true
			f.mu.Lock()
			f.usouTLS = true
			f.mu.Unlock()
		case strings.HasPrefix(cmd, "AUTH"):
			partes := strings.Fields(linha)
			f.mu.Lock()
			if len(partes) == 3 {
				f.authResp = partes[2]
			}
			f.mu.Unlock()
			if f.falha == "auth" {
				reply("535 credenciais invalidas")
				continue
			}
			reply("235 autenticado")
		case strings.HasPrefix(cmd, "MAIL FROM:"):
			f.mu.Lock()
			f.mailFrom = linha[len("MAIL FROM:"):]
			f.mu.Unlock()
			if f.falha == "mail" {
				reply("550 remetente recusado")
				continue
			}
			reply("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO:"):
			f.mu.Lock()
			f.rcptTo = linha[len("RCPT TO:"):]
			f.mu.Unlock()
			if f.falha == "rcpt" {
				reply("550 destinatario recusado")
				continue
			}
			reply("250 ok")
		case cmd == "DATA":
			if f.falha == "data" {
				reply("554 DATA recusado")
				continue
			}
			reply("354 manda")
			var sb strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				sb.WriteString(l)
			}
			f.mu.Lock()
			f.data = sb.String()
			f.mu.Unlock()
			if f.falha == "data-fim" {
				reply("554 mensagem recusada")
				continue
			}
			reply("250 enfileirada")
		case cmd == "QUIT":
			if f.falha == "quit" {
				reply("500 erro no quit")
				return
			}
			reply("221 tchau")
			return
		case cmd == "RSET", cmd == "NOOP":
			reply("250 ok")
		default:
			reply("502 comando desconhecido")
		}
	}
}

func smtpCfg(host, port string) *config.Config {
	return &config.Config{
		SMTPHost:     host,
		SMTPPort:     port,
		SMTPUser:     "user@teste.com",
		SMTPPassword: "senha-de-app",
		SMTPFrom:     "no-reply@rotaperfumes.com",
	}
}

func TestSMTPEmailService_EnviarSenhaInicial_Sucesso(t *testing.T) {
	cert, pool := smtpTLS(t)

	casos := []struct {
		nome     string
		falha    string
		wantAuth bool
	}{
		{"com AUTH anunciado", "", true},
		{"servidor sem AUTH (pula autenticação)", "sem-auth", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			f := &fakeSMTP{falha: c.falha, cert: cert}
			host, port := f.start(t)

			// ServerName errado de propósito: o construtor força o host configurado.
			svc, err := services.NewSMTPEmailServiceWithTLSConfig(smtpCfg(host, port), &tls.Config{RootCAs: pool, ServerName: "host-errado.invalid"})
			require.NoError(t, err)

			err = svc.EnviarSenhaInicial(context.Background(), "cliente@exemplo.com", "Maria", "S3nh@Forte!")
			require.NoError(t, err)

			f.mu.Lock()
			defer f.mu.Unlock()
			assert.True(t, f.usouTLS, "deve fazer STARTTLS antes de enviar")
			assert.Equal(t, "<no-reply@rotaperfumes.com>", f.mailFrom[:len("<no-reply@rotaperfumes.com>")])
			assert.Equal(t, "<cliente@exemplo.com>", f.rcptTo)

			if c.wantAuth {
				raw, err := base64.StdEncoding.DecodeString(f.authResp)
				require.NoError(t, err)
				assert.Equal(t, "\x00user@teste.com\x00senha-de-app", string(raw))
			} else {
				assert.Empty(t, f.authResp)
			}

			assert.Contains(t, f.data, "From: no-reply@rotaperfumes.com\r\n")
			assert.Contains(t, f.data, "To: cliente@exemplo.com\r\n")
			assert.Contains(t, f.data, "Subject: Sua senha de acesso")
			assert.Contains(t, f.data, "MIME-Version: 1.0\r\n")
			assert.Contains(t, f.data, "Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n")
			assert.Contains(t, f.data, "Olá, Maria.")
			assert.Contains(t, f.data, "S3nh@Forte!")
		})
	}
}

func TestSMTPEmailService_EnviarSenhaInicial_Erros(t *testing.T) {
	cert, pool := smtpTLS(t)

	casos := []struct {
		nome    string
		falha   string
		semCA   bool // cliente sem a CA de teste: certificado não confiável
		wantErr string
	}{
		{"saudação recusada", "greeting", false, "smtp client"},
		{"servidor sem STARTTLS", "sem-starttls", false, "smtp starttls: not supported"},
		{"STARTTLS recusado", "starttls", false, "smtp starttls"},
		{"certificado não confiável", "", true, "smtp starttls"},
		{"AUTH recusado", "auth", false, "smtp auth"},
		{"MAIL FROM recusado", "mail", false, "smtp mail from"},
		{"RCPT TO recusado", "rcpt", false, "smtp rcpt to"},
		{"DATA recusado", "data", false, "smtp data"},
		{"mensagem recusada no fim do DATA", "data-fim", false, "smtp close"},
		{"QUIT com erro", "quit", false, "500"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			f := &fakeSMTP{falha: c.falha, cert: cert}
			host, port := f.start(t)

			tlsCfg := &tls.Config{RootCAs: pool}
			if c.semCA {
				tlsCfg = nil
			}
			svc, err := services.NewSMTPEmailServiceWithTLSConfig(smtpCfg(host, port), tlsCfg)
			require.NoError(t, err)

			err = svc.EnviarSenhaInicial(context.Background(), "cliente@exemplo.com", "Maria", "S3nh@Forte!")
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantErr)
			assert.NotContains(t, err.Error(), "S3nh@Forte!", "a senha nunca deve vazar no erro")
		})
	}
}

func TestSMTPEmailService_EnviarSenhaInicial_ErroDeConexao(t *testing.T) {
	casos := []struct {
		nome string
		ctx  func() context.Context
		port string
	}{
		{"porta sem ninguém escutando", context.Background, "1"},
		{"contexto já cancelado", func() context.Context {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx
		}, "25"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			svc, err := services.NewSMTPEmailService(smtpCfg("127.0.0.1", c.port))
			require.NoError(t, err)
			err = svc.EnviarSenhaInicial(c.ctx(), "cliente@exemplo.com", "Maria", "x")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "smtp dial")
		})
	}
}

func TestNewSMTPEmailServiceWithTLSConfig(t *testing.T) {
	t.Run("config incompleta", func(t *testing.T) {
		cfg := smtpCfg("smtp.exemplo.com", "587")
		cfg.SMTPFrom = ""
		svc, err := services.NewSMTPEmailServiceWithTLSConfig(cfg, &tls.Config{})
		assert.Nil(t, svc)
		assert.ErrorIs(t, err, services.ErrSMTPNaoConfigurado)
	})

	t.Run("não altera a tls.Config recebida (clona)", func(t *testing.T) {
		orig := &tls.Config{ServerName: "outro-host"}
		svc, err := services.NewSMTPEmailServiceWithTLSConfig(smtpCfg("smtp.exemplo.com", "587"), orig)
		require.NoError(t, err)
		require.NotNil(t, svc)
		assert.Equal(t, "outro-host", orig.ServerName)
	})
}
