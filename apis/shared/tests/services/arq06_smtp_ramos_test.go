package services_test

// ARQ-06 (portão de cobertura, lição 3 do ARQ-05): ramos do SMTP que nenhum
// teste exercitava — o prazo do contexto que fecha a conexão, o pool de CAs
// do SMTP_TLS_CA_FILE somado às RootCAs do tls.Config e o desafio AUTH LOGIN
// inesperado. Só somam: os servidores fake abaixo são próprios (as suítes
// MAIL01/SEC10 não mudam).

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/services"
)

// escutar aceita conexões e entrega cada uma a atender (em goroutine).
func escutar(t *testing.T, atender func(net.Conn)) (host, port string) {
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
			go atender(conn)
		}
	}()
	h, p, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	return h, p
}

// Servidor que aceita e nunca responde: o prazo do ctx (menor que o da
// sessão) fecha a conexão e o envio falha, sem pendurar o chamador.
func TestARQ06_SMTP_PrazoDoContextoFechaAConexao(t *testing.T) {
	liberar := make(chan struct{})
	t.Cleanup(func() { close(liberar) })
	host, port := escutar(t, func(c net.Conn) {
		defer c.Close()
		<-liberar
	})
	svc, err := services.NewSMTPEmailServiceWithTLSConfig(smtpCfg(host, port), &tls.Config{})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	inicio := time.Now()
	err = svc.EnviarSenhaInicial(ctx, "cliente@exemplo.com", "Maria", "S3nh@Forte!")
	require.Error(t, err)
	assert.Less(t, time.Since(inicio), 5*time.Second, "o prazo do ctx manda, não o da sessão")
}

// SMTP_TLS_CA_FILE com RootCAs no tls.Config: o pool é uma cópia das RootCAs
// acrescida do arquivo (o tls.Config do chamador não é alterado).
func TestARQ06_SMTP_CAFileSomaAsRootCAsDoConfig(t *testing.T) {
	srv := httptest.NewUnstartedServer(nil)
	srv.StartTLS()
	defer srv.Close()
	caminho := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(caminho, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw}), 0o600))

	cfg := smtpCfg("smtp.exemplo.com", "587")
	cfg.SMTPTLSCAFile = caminho
	base := x509.NewCertPool()
	_, err := services.NewSMTPEmailServiceWithTLSConfig(cfg, &tls.Config{RootCAs: base})
	require.NoError(t, err)
	assert.True(t, base.Equal(x509.NewCertPool()), "as RootCAs do chamador não mudam (o pool é um Clone)")
}

// AUTH LOGIN com um desafio que não pede usuário nem senha: o cliente aborta
// a autenticação e não envia a mensagem.
func TestARQ06_SMTP_AuthLoginDesafioInesperado(t *testing.T) {
	cert, pool := smtpTLS(t)
	enviouMail := make(chan bool, 1)
	host, port := escutar(t, func(raw net.Conn) {
		defer raw.Close()
		_ = raw.SetDeadline(time.Now().Add(5 * time.Second))
		c := tls.Server(raw, &tls.Config{Certificates: []tls.Certificate{cert}})
		if c.Handshake() != nil {
			return
		}
		r, w := bufio.NewReader(c), bufio.NewWriter(c)
		reply := func(s string) { _, _ = w.WriteString(s + "\r\n"); _ = w.Flush() }
		reply("220 fake ESMTP")
		mail := false
		defer func() { enviouMail <- mail }()
		for {
			linha, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.TrimRight(linha, "\r\n"))
			switch {
			case strings.HasPrefix(cmd, "EHLO"):
				reply("250-fake\r\n250 AUTH LOGIN")
			case cmd == "AUTH LOGIN":
				reply("334 " + base64.StdEncoding.EncodeToString([]byte("Token:")))
			case strings.HasPrefix(cmd, "MAIL"):
				mail = true
				reply("250 ok")
			case cmd == "QUIT":
				reply("221 tchau")
				return
			default:
				reply("501 cancelado")
			}
		}
	})
	cfg := smtpCfg(host, port)
	cfg.SMTPTLSMode = "implicit"
	svc, err := services.NewSMTPEmailServiceWithTLSConfig(cfg, &tls.Config{RootCAs: pool})
	require.NoError(t, err)

	err = enviarTeste(t, svc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "desafio AUTH LOGIN inesperado")
	select {
	case mail := <-enviouMail:
		assert.False(t, mail, "sem autenticar, nada de MAIL FROM")
	case <-time.After(5 * time.Second):
		t.Fatal("o servidor fake não terminou")
	}
}

// Cancelar o ctx durante o DATA (servidor que parou de ler, escrita
// bloqueada pela janela TCP) fecha a conexão pelo vigia: a escrita da
// mensagem falha com o rótulo da operação, sem pendurar.
func TestARQ06_SMTP_CancelarDuranteODataFalhaAEscrita(t *testing.T) {
	cert, pool := smtpTLS(t)
	recebeuData, liberar := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(liberar) })
	host, port := escutar(t, func(raw net.Conn) {
		defer raw.Close()
		_ = raw.SetDeadline(time.Now().Add(10 * time.Second))
		c := tls.Server(raw, &tls.Config{Certificates: []tls.Certificate{cert}})
		if c.Handshake() != nil {
			return
		}
		r, w := bufio.NewReader(c), bufio.NewWriter(c)
		reply := func(s string) { _, _ = w.WriteString(s + "\r\n"); _ = w.Flush() }
		reply("220 fake ESMTP")
		for {
			linha, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.TrimRight(linha, "\r\n"))
			switch {
			case strings.HasPrefix(cmd, "EHLO"):
				reply("250-fake\r\n250 AUTH PLAIN")
			case strings.HasPrefix(cmd, "AUTH"):
				reply("235 autenticado")
			case cmd == "DATA":
				reply("354 manda")
				close(recebeuData)
				<-liberar // para de ler: a escrita do cliente enche a janela e bloqueia
				return
			default:
				reply("250 ok")
			}
		}
	})
	cfg := smtpCfg(host, port)
	cfg.SMTPTLSMode = "implicit"
	svc, err := services.NewSMTPEmailServiceWithTLSConfig(cfg, &tls.Config{RootCAs: pool})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-recebeuData
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()
	nomeEnorme := strings.Repeat("n", 32<<20) // maior que qualquer buffer de socket
	err = svc.EnviarSenhaInicial(ctx, "cliente@exemplo.com", nomeEnorme, "S3nh@Forte!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "services: smtp write")
}
