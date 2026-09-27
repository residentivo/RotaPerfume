package services_test

// SEC-10: NewSMTPEmailServiceWithTLSConfig impõe, sobre um CLONE da
// tls.Config recebida, InsecureSkipVerify=false e MinVersion >= TLS 1.2
// (TLS 1.3 é mantido). Reaproveita smtpTLS/fakeSMTP/smtpCfg de
// email_smtp_test.go e usa um servidor SMTP mínimo próprio (servidorTLSVersao)
// quando é preciso controlar a versão TLS do lado do servidor e observar a
// versão negociada.

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/services"
)

// servidorTLSVersao é um SMTP mínimo (sem AUTH) cujo STARTTLS usa srvTLS.
// Registra a versão TLS negociada e o erro de handshake do lado servidor.
type servidorTLSVersao struct {
	srvTLS *tls.Config

	mu        sync.Mutex
	versao    uint16
	errHS     error
	entregues int
}

func (s *servidorTLSVersao) start(t *testing.T) (string, string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c)
		}
	}()
	h, p, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	return h, p
}

func (s *servidorTLSVersao) serve(conn net.Conn) {
	defer func() { conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	r, w := bufio.NewReader(conn), bufio.NewWriter(conn)
	reply := func(l string) { _, _ = w.WriteString(l + "\r\n"); _ = w.Flush() }
	reply("220 fake")
	emTLS := false
	for {
		linha, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimRight(linha, "\r\n"))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			if emTLS {
				reply("250 fake")
			} else {
				reply("250-fake\r\n250 STARTTLS")
			}
		case cmd == "STARTTLS":
			reply("220 pronto")
			tc := tls.Server(conn, s.srvTLS)
			err := tc.Handshake()
			s.mu.Lock()
			s.errHS = err
			if err == nil {
				s.versao = tc.ConnectionState().Version
			}
			s.mu.Unlock()
			if err != nil {
				return
			}
			conn, r, w, emTLS = tc, bufio.NewReader(tc), bufio.NewWriter(tc), true
		case strings.HasPrefix(cmd, "MAIL FROM:"), strings.HasPrefix(cmd, "RCPT TO:"):
			reply("250 ok")
		case cmd == "DATA":
			reply("354 manda")
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
			}
			s.mu.Lock()
			s.entregues++
			s.mu.Unlock()
			reply("250 ok")
		case cmd == "QUIT":
			reply("221 tchau")
			return
		default:
			reply("250 ok")
		}
	}
}

// TestSEC10_TLS_InsecureSkipVerifyIgnorado: mesmo com InsecureSkipVerify=true
// o certificado não confiável é rejeitado, e a config do chamador não muda.
func TestSEC10_TLS_InsecureSkipVerifyIgnorado(t *testing.T) {
	cert, pool := smtpTLS(t)
	casos := []struct {
		nome  string
		entra *tls.Config
	}{
		{"InsecureSkipVerify sem RootCAs", &tls.Config{InsecureSkipVerify: true}},
		{"InsecureSkipVerify + MinVersion TLS1.3", &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13}},
		{"InsecureSkipVerify + pool vazio", &tls.Config{InsecureSkipVerify: true, RootCAs: x509.NewCertPool()}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			f := &fakeSMTP{cert: cert}
			host, port := f.start(t)
			minAntes := c.entra.MinVersion

			svc, err := services.NewSMTPEmailServiceWithTLSConfig(smtpCfg(host, port), c.entra)
			require.NoError(t, err)
			err = svc.EnviarSenhaInicial(context.Background(), "cliente@exemplo.com", "Maria", "S3nh@Forte!")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "smtp starttls")
			var certErr *tls.CertificateVerificationError
			assert.ErrorAs(t, err, &certErr, "falha deve ser de verificação do certificado")
			assert.NotContains(t, err.Error(), "S3nh@Forte!")

			// Clone: a config do chamador permanece como veio.
			assert.True(t, c.entra.InsecureSkipVerify, "config original não pode ser alterada")
			assert.Equal(t, minAntes, c.entra.MinVersion)
			assert.Empty(t, c.entra.ServerName)
			f.mu.Lock()
			defer f.mu.Unlock()
			assert.Empty(t, f.data, "nada é entregue")
		})
	}
	// Sanidade: com a CA certa, InsecureSkipVerify=true não atrapalha.
	t.Run("InsecureSkipVerify com a CA correta envia", func(t *testing.T) {
		f := &fakeSMTP{cert: cert}
		host, port := f.start(t)
		svc, err := services.NewSMTPEmailServiceWithTLSConfig(smtpCfg(host, port), &tls.Config{InsecureSkipVerify: true, RootCAs: pool})
		require.NoError(t, err)
		require.NoError(t, svc.EnviarSenhaInicial(context.Background(), "cliente@exemplo.com", "Maria", "x"))
	})
}

// TestSEC10_TLS_Versoes: versão mínima TLS 1.2 imposta pelo cliente.
func TestSEC10_TLS_Versoes(t *testing.T) {
	cert, pool := smtpTLS(t)
	casos := []struct {
		nome       string
		cliente    *tls.Config
		servidor   *tls.Config
		wantErr    bool
		wantVersao uint16
	}{
		{
			nome:    "cliente MaxVersion TLS1.1: handshake falha",
			cliente: &tls.Config{RootCAs: pool, MaxVersion: tls.VersionTLS11},
			servidor: &tls.Config{Certificates: []tls.Certificate{cert},
				MinVersion: tls.VersionTLS10},
			wantErr: true,
		},
		{
			nome:    "cliente MinVersion TLS1.0 contra servidor só TLS1.1: sobe para 1.2 e falha",
			cliente: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS10},
			servidor: &tls.Config{Certificates: []tls.Certificate{cert},
				MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11},
			wantErr: true,
		},
		{
			nome:    "cliente sem MinVersion contra servidor só TLS1.0: falha",
			cliente: &tls.Config{RootCAs: pool},
			servidor: &tls.Config{Certificates: []tls.Certificate{cert},
				MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS10},
			wantErr: true,
		},
		{
			nome:       "cliente MinVersion TLS1.3: envio OK em TLS1.3",
			cliente:    &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS13},
			servidor:   &tls.Config{Certificates: []tls.Certificate{cert}},
			wantVersao: tls.VersionTLS13,
		},
		{
			nome:    "cliente MinVersion TLS1.3 contra servidor só TLS1.2: 1.3 é mantido (falha)",
			cliente: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS13},
			servidor: &tls.Config{Certificates: []tls.Certificate{cert},
				MaxVersion: tls.VersionTLS12},
			wantErr: true,
		},
		{
			nome:       "cliente MinVersion TLS1.0 contra servidor TLS1.2: OK em TLS1.2",
			cliente:    &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS10},
			servidor:   &tls.Config{Certificates: []tls.Certificate{cert}, MaxVersion: tls.VersionTLS12},
			wantVersao: tls.VersionTLS12,
		},
		{
			nome:       "cliente sem versões definidas: negocia TLS1.3",
			cliente:    &tls.Config{RootCAs: pool},
			servidor:   &tls.Config{Certificates: []tls.Certificate{cert}},
			wantVersao: tls.VersionTLS13,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			s := &servidorTLSVersao{srvTLS: c.servidor}
			host, port := s.start(t)
			minAntes, maxAntes := c.cliente.MinVersion, c.cliente.MaxVersion

			svc, err := services.NewSMTPEmailServiceWithTLSConfig(smtpCfg(host, port), c.cliente)
			require.NoError(t, err)
			err = svc.EnviarSenhaInicial(context.Background(), "cliente@exemplo.com", "Maria", "x")

			assert.Equal(t, minAntes, c.cliente.MinVersion, "clone: MinVersion original intacta")
			assert.Equal(t, maxAntes, c.cliente.MaxVersion)
			s.mu.Lock()
			defer s.mu.Unlock()
			if c.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "smtp starttls")
				assert.Zero(t, s.entregues)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantVersao, s.versao)
			assert.Equal(t, 1, s.entregues)
		})
	}
}
