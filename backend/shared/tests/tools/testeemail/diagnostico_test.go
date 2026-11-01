package testeemail_test

// ARQ-07: o diagnóstico de e-mail (cmd/testeemail) saiu do main para
// tools/testeemail. Estes testes rodam o comando inteiro (testeemail.NovoCLI)
// contra um servidor SMTP falso local (127.0.0.1, porta efêmera) que fala o
// protocolo de verdade: texto puro, STARTTLS e TLS implícito, AUTH PLAIN e
// LOGIN, e uma falha programável em cada passo. O certificado é o do
// httptest (válido para 127.0.0.1). Nenhum servidor real é contatado.

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/tools/testeemail"
)

const (
	diagUsuario = "diag@exemplo.com"
	diagSenha   = "S3nh@Diagnostico!"
)

// certificadoTeste devolve o certificado do httptest (válido para
// 127.0.0.1) e o PEM dele, para SMTP_TLS_CA_FILE.
func certificadoTeste(t *testing.T) (tls.Certificate, []byte) {
	t.Helper()
	srv := httptest.NewUnstartedServer(nil)
	srv.StartTLS()
	defer srv.Close()
	return srv.TLS.Certificates[0], pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
}

// smtpFalso é um servidor SMTP mínimo e programável.
type smtpFalso struct {
	cert       tls.Certificate
	implicito  bool   // TLS desde o connect
	mecanismos string // anunciados em AUTH ("" = sem AUTH)
	authSemTLS bool   // anuncia AUTH também em texto puro
	falha      string // passo que responde erro

	mu        sync.Mutex
	recebidos []string
}

func (f *smtpFalso) anotar(s string) {
	f.mu.Lock()
	f.recebidos = append(f.recebidos, s)
	f.mu.Unlock()
}

func (f *smtpFalso) conversa() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.recebidos, "\n")
}

func (f *smtpFalso) iniciar(t *testing.T) string {
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
			go f.atender(conn)
		}
	}()
	_, porta, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	return porta
}

func (f *smtpFalso) atender(conn net.Conn) {
	defer func() { conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	emTLS := false
	if f.implicito {
		tc := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{f.cert}})
		if err := tc.Handshake(); err != nil {
			return
		}
		conn, emTLS = tc, true
	}
	r, w := bufio.NewReader(conn), bufio.NewWriter(conn)
	responder := func(linhas ...string) {
		for _, l := range linhas {
			_, _ = w.WriteString(l + "\r\n")
		}
		_ = w.Flush()
	}
	if f.falha == "banner" {
		responder("554 servico indisponivel")
		return
	}
	responder("220 smtp-falso ESMTP")
	for {
		linha, err := r.ReadString('\n')
		if err != nil {
			return
		}
		linha = strings.TrimRight(linha, "\r\n")
		cmd := strings.ToUpper(linha)
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			if f.falha == "ehlo" {
				responder("550 ehlo recusado")
				continue
			}
			ext := []string{"250-smtp-falso"}
			if !emTLS && f.falha != "sem-starttls" {
				ext = append(ext, "250-STARTTLS")
			}
			if f.mecanismos != "" && (emTLS || f.authSemTLS) {
				ext = append(ext, "250-AUTH "+f.mecanismos)
			}
			responder(append(ext, "250 SIZE 1000")...)
		case cmd == "STARTTLS":
			if f.falha == "starttls" {
				responder("454 TLS indisponivel")
				continue
			}
			responder("220 pronto para TLS")
			tc := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{f.cert}})
			if err := tc.Handshake(); err != nil {
				return
			}
			conn, emTLS = tc, true
			r, w = bufio.NewReader(tc), bufio.NewWriter(tc)
		case strings.HasPrefix(cmd, "AUTH PLAIN"):
			f.autenticarPlain(linha, responder)
		case strings.HasPrefix(cmd, "AUTH LOGIN"):
			f.autenticarLogin(r, responder)
		case strings.HasPrefix(cmd, "MAIL FROM"):
			f.anotar(linha)
			f.passo("mail", "550 remetente recusado", "250 ok", responder)
		case strings.HasPrefix(cmd, "RCPT TO"):
			f.anotar(linha)
			f.passo("rcpt", "550 5.1.1 caixa inexistente", "250 ok", responder)
		case cmd == "DATA":
			if f.falha == "data" {
				responder("554 data recusado")
				continue
			}
			responder("354 envie")
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
			}
			f.passo("ponto", "554 mensagem recusada", "250 2.0.0 aceita", responder)
		case cmd == "QUIT":
			f.passo("quit", "500 quit estranho", "221 tchau", responder)
			return
		default:
			responder("502 comando desconhecido")
		}
	}
}

func (f *smtpFalso) passo(nome, erro, ok string, responder func(...string)) {
	if f.falha == nome {
		responder(erro)
		return
	}
	responder(ok)
}

func (f *smtpFalso) autenticarPlain(linha string, responder func(...string)) {
	partes := strings.Fields(linha)
	b, _ := base64.StdEncoding.DecodeString(partes[len(partes)-1])
	f.anotar("AUTH PLAIN " + string(bytes.ReplaceAll(b, []byte{0}, []byte{'|'})))
	f.passo("auth", "535 5.7.8 credenciais invalidas", "235 autenticado", responder)
}

func (f *smtpFalso) autenticarLogin(r *bufio.Reader, responder func(...string)) {
	responder("334 VXNlcm5hbWU6")
	u, _ := r.ReadString('\n')
	responder("334 UGFzc3dvcmQ6")
	p, _ := r.ReadString('\n')
	du, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(u))
	dp, _ := base64.StdEncoding.DecodeString(strings.TrimSpace(p))
	f.anotar("AUTH LOGIN " + string(du) + "|" + string(dp))
	f.passo("auth", "535 5.7.8 credenciais invalidas", "235 autenticado", responder)
}

// ambienteDiag prepara as variáveis SMTP_* (t.Setenv: o .env nunca as
// sobrescreve) e um .env próprio numa raiz temporária.
type ambienteDiag struct {
	raiz   string
	caFile string
	logs   *bytes.Buffer
}

func novoAmbienteDiag(t *testing.T, porta string, pemCA []byte) *ambienteDiag {
	t.Helper()
	a := &ambienteDiag{raiz: t.TempDir()}
	a.caFile = filepath.Join(a.raiz, "ca.pem")
	require.NoError(t, os.WriteFile(a.caFile, pemCA, 0o600))
	for chave, valor := range map[string]string{
		"SMTP_HOST": "127.0.0.1", "SMTP_PORT": porta, "SMTP_USER": diagUsuario, "SMTP_PASSWORD": diagSenha,
		"SMTP_FROM": diagUsuario, "SMTP_TLS_MODE": "", "SMTP_TLS_SERVER_NAME": "", "SMTP_TLS_CA_FILE": a.caFile,
	} {
		t.Setenv(chave, valor)
	}
	a.logs = &bytes.Buffer{}
	anterior, flags := log.Writer(), log.Flags()
	log.SetOutput(a.logs)
	t.Cleanup(func() { log.SetOutput(anterior); log.SetFlags(flags) })
	return a
}

func (a *ambienteDiag) rodar(t *testing.T, args ...string) int {
	t.Helper()
	amb := cmdutil.Ambiente{Entrada: strings.NewReader(""), Saida: io.Discard, Erro: io.Discard}
	raiz := func() (string, error) { return a.raiz, nil }
	return cmdutil.Executar("testeemail", testeemail.NovoCLI(raiz), append([]string{"-timeout", "5s", "-from", diagUsuario, "-to", "dest@exemplo.com"}, args...), amb)
}

func TestARQ07_Diagnostico_STARTTLSComAuthPlain_Sucesso(t *testing.T) {
	cert, pemCA := certificadoTeste(t)
	srv := &smtpFalso{cert: cert, mecanismos: "PLAIN LOGIN"}
	a := novoAmbienteDiag(t, srv.iniciar(t), pemCA)

	require.Equal(t, cmdutil.SaidaOK, a.rodar(t))
	out := a.logs.String()
	assert.Contains(t, out, "RESUMO: SUCESSO — mensagem aceita pelo servidor 127.0.0.1:")
	assert.Contains(t, out, "modo efetivo: starttls")
	assert.Contains(t, out, "STARTTLS aceito: 220")
	assert.Contains(t, out, "verificação do certificado: OK")
	assert.Contains(t, out, "AUTH aceito")
	assert.Contains(t, out, "bytes do AUTH ocultados (credenciais)")
	assert.NotContains(t, out, diagSenha, "a senha nunca vai para o log")
	assert.NotContains(t, out, base64.StdEncoding.EncodeToString([]byte("\x00"+diagUsuario+"\x00"+diagSenha)))
	assert.Contains(t, srv.conversa(), "AUTH PLAIN |"+diagUsuario+"|"+diagSenha)
	assert.Contains(t, srv.conversa(), "RCPT TO:<dest@exemplo.com>")
}

func TestARQ07_Diagnostico_TLSImplicitoComAuthLogin_SalvaCertificado(t *testing.T) {
	cert, pemCA := certificadoTeste(t)
	srv := &smtpFalso{cert: cert, implicito: true, mecanismos: "LOGIN"}
	a := novoAmbienteDiag(t, srv.iniciar(t), pemCA)
	salvo := filepath.Join(a.raiz, "servidor.pem")

	require.Equal(t, cmdutil.SaidaOK, a.rodar(t, "-modo", "ssl", "-salvar-cert", salvo, "-ehlo", "cliente.teste"))
	out := a.logs.String()
	assert.Contains(t, out, "AVISO: modo ssl em porta")
	assert.Contains(t, out, "AVISO: servidor NÃO oferece PLAIN; testando LOGIN")
	assert.Contains(t, out, "certificado do servidor salvo em")
	assert.Contains(t, out, "enviando EHLO cliente.teste")
	assert.Contains(t, srv.conversa(), "AUTH LOGIN "+diagUsuario+"|"+diagSenha)
	conteudo, err := os.ReadFile(salvo)
	require.NoError(t, err)
	assert.Contains(t, string(conteudo), "BEGIN CERTIFICATE")
	assert.NotContains(t, out, diagSenha)
}

func TestARQ07_Diagnostico_ModoPlain(t *testing.T) {
	cert, pemCA := certificadoTeste(t)
	t.Run("localhost: o PlainAuth aceita e o envio sai", func(t *testing.T) {
		srv := &smtpFalso{cert: cert, mecanismos: "PLAIN", authSemTLS: true}
		a := novoAmbienteDiag(t, srv.iniciar(t), pemCA)
		require.Equal(t, cmdutil.SaidaOK, a.rodar(t, "-modo", "plain"))
		assert.Contains(t, a.logs.String(), "AVISO: modo plain = SEM TLS")
	})
	t.Run("nome fora de localhost: o PlainAuth recusa sem TLS (dica)", func(t *testing.T) {
		srv := &smtpFalso{cert: cert, mecanismos: "PLAIN", authSemTLS: true}
		a := novoAmbienteDiag(t, srv.iniciar(t), pemCA)
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t, "-modo", "plain", "-tls-server-name", "smtp.exemplo.com"))
		out := a.logs.String()
		assert.Contains(t, out, "RESUMO: FALHA no passo")
		assert.Contains(t, out, "o net/smtp RECUSA enviar a senha (PlainAuth) sem TLS")
		assert.NotContains(t, srv.conversa(), "AUTH", "nada de credencial em texto puro")
	})
	t.Run("AUTH LOGIN sem TLS é recusado pelo próprio diagnóstico", func(t *testing.T) {
		srv := &smtpFalso{cert: cert, mecanismos: "LOGIN", authSemTLS: true}
		a := novoAmbienteDiag(t, srv.iniciar(t), pemCA)
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t, "-modo", "plain"))
		assert.Contains(t, a.logs.String(), "AUTH LOGIN sem TLS recusado pelo diagnóstico")
	})
	t.Run("sem AUTH anunciado e sem credenciais: segue sem autenticar", func(t *testing.T) {
		srv := &smtpFalso{cert: cert}
		a := novoAmbienteDiag(t, srv.iniciar(t), pemCA)
		require.Equal(t, cmdutil.SaidaOK, a.rodar(t, "-modo", "plain"))
		assert.Contains(t, a.logs.String(), "AUTH não anunciado — PULANDO a autenticação")
		assert.Contains(t, a.logs.String(), "muitos servidores só anunciam AUTH depois do TLS")

		t.Setenv("SMTP_PASSWORD", "")
		require.Equal(t, cmdutil.SaidaOK, a.rodar(t, "-modo", "plain"))
		assert.Contains(t, a.logs.String(), "AUTH PULADO")
	})
}

func TestARQ07_Diagnostico_FalhaEmCadaPasso(t *testing.T) {
	cert, pemCA := certificadoTeste(t)
	casos := []struct {
		falha, modo, quer string
		codigo            int
	}{
		{"banner", "starttls", "banner SMTP", cmdutil.SaidaErro},
		{"ehlo", "starttls", "EHLO", cmdutil.SaidaErro},
		{"sem-starttls", "starttls", "servidor não anunciou STARTTLS no EHLO", cmdutil.SaidaErro},
		{"starttls", "starttls", "resposta do STARTTLS", cmdutil.SaidaErro},
		{"auth", "starttls", "credenciais rejeitadas (535)", cmdutil.SaidaErro},
		{"mail", "starttls", "MAIL FROM", cmdutil.SaidaErro},
		{"rcpt", "starttls", "destinatário recusado (550)", cmdutil.SaidaErro},
		{"data", "starttls", "DATA", cmdutil.SaidaErro},
		{"ponto", "starttls", "fim do DATA (resposta ao '.')", cmdutil.SaidaErro},
		{"quit", "starttls", "AVISO: QUIT falhou", cmdutil.SaidaOK},
	}
	for _, c := range casos {
		t.Run(c.falha, func(t *testing.T) {
			srv := &smtpFalso{cert: cert, mecanismos: "PLAIN", falha: c.falha}
			a := novoAmbienteDiag(t, srv.iniciar(t), pemCA)
			assert.Equal(t, c.codigo, a.rodar(t, "-modo", c.modo))
			assert.Contains(t, a.logs.String(), c.quer)
			assert.NotContains(t, a.logs.String(), diagSenha)
		})
	}
}

func TestARQ07_Diagnostico_Certificado(t *testing.T) {
	cert, pemCA := certificadoTeste(t)
	t.Run("nome errado: dica com os SANs", func(t *testing.T) {
		srv := &smtpFalso{cert: cert, implicito: true, mecanismos: "PLAIN"}
		a := novoAmbienteDiag(t, srv.iniciar(t), pemCA)
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t, "-modo", "ssl", "-tls-server-name", "outro.exemplo"))
		assert.Contains(t, a.logs.String(), `o certificado não vale para "outro.exemplo"`)
	})
	t.Run("CA desconhecida: dica; -inseguro ignora e segue (sem credenciais, ARQ-07 NB-E)", func(t *testing.T) {
		srv := &smtpFalso{cert: cert, implicito: true, mecanismos: "PLAIN"}
		a := novoAmbienteDiag(t, srv.iniciar(t), pemCA)
		t.Setenv("SMTP_TLS_CA_FILE", "")
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t, "-modo", "ssl"))
		assert.Contains(t, a.logs.String(), "certificado autoassinado ou de CA não confiável")

		t.Setenv("SMTP_PASSWORD", "")
		require.Equal(t, cmdutil.SaidaOK, a.rodar(t, "-modo", "ssl", "-inseguro"))
		assert.Contains(t, a.logs.String(), "WARNING: -inseguro ATIVO")
		assert.Contains(t, a.logs.String(), "falha de verificação IGNORADA por causa de -inseguro")
	})
	t.Run("TLS implícito contra servidor em texto puro: dica de starttls", func(t *testing.T) {
		srv := &smtpFalso{cert: cert, mecanismos: "PLAIN"}
		a := novoAmbienteDiag(t, srv.iniciar(t), pemCA)
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t, "-modo", "ssl"))
		assert.Contains(t, a.logs.String(), "o servidor respondeu algo que NÃO é TLS")
	})
	t.Run("-tls-ca-file ilegível ou sem PEM", func(t *testing.T) {
		srv := &smtpFalso{cert: cert}
		a := novoAmbienteDiag(t, srv.iniciar(t), pemCA)
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t, "-tls-ca-file", filepath.Join(a.raiz, "nao-existe.pem")))
		assert.Contains(t, a.logs.String(), "leitura da CA extra")
		lixo := filepath.Join(a.raiz, "lixo.pem")
		require.NoError(t, os.WriteFile(lixo, []byte("não é PEM"), 0o600))
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t, "-tls-ca-file", lixo))
		assert.Contains(t, a.logs.String(), "nenhum certificado PEM válido")
	})
}

func TestARQ07_Diagnostico_ConfiguracaoEModo(t *testing.T) {
	cert, pemCA := certificadoTeste(t)
	srv := &smtpFalso{cert: cert, mecanismos: "PLAIN"}
	porta := srv.iniciar(t)

	t.Run("SMTP_TLS_MODE da API é seguido com -modo auto", func(t *testing.T) {
		a := novoAmbienteDiag(t, porta, pemCA)
		t.Setenv("SMTP_TLS_MODE", "starttls")
		t.Setenv("SMTP_TLS_SERVER_NAME", "127.0.0.1")
		require.Equal(t, cmdutil.SaidaOK, a.rodar(t))
		assert.Contains(t, a.logs.String(), `SMTP_TLS_MODE="starttls": usando modo starttls`)
		assert.Contains(t, a.logs.String(), "ServerName TLS vindo de SMTP_TLS_SERVER_NAME")
	})
	t.Run("SMTP_TLS_MODE inválido e -modo inválido", func(t *testing.T) {
		a := novoAmbienteDiag(t, porta, pemCA)
		t.Setenv("SMTP_TLS_MODE", "tls13")
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t))
		assert.Contains(t, a.logs.String(), "SMTP_TLS_MODE")
		t.Setenv("SMTP_TLS_MODE", "")
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t, "-modo", "quantico"))
		assert.Contains(t, a.logs.String(), "flag -modo")
	})
	t.Run("host/porta vazios e destinatário com CR/LF", func(t *testing.T) {
		a := novoAmbienteDiag(t, porta, pemCA)
		t.Setenv("SMTP_HOST", " ")
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t))
		assert.Contains(t, a.logs.String(), "SMTP_HOST/SMTP_PORT vazios")
		t.Setenv("SMTP_HOST", "127.0.0.1")
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t, "-to", "x@y.z\r\nBcc: a@b.c"))
		assert.Contains(t, a.logs.String(), "destinatário (-to)")
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t, "-from", ""))
		assert.Contains(t, a.logs.String(), "remetente (-from)")
	})
	t.Run("avisos: starttls na 465, remetente diferente do usuário, senha com espaço, args soltos", func(t *testing.T) {
		a := novoAmbienteDiag(t, porta, pemCA)
		t.Setenv("SMTP_PASSWORD", diagSenha+" ")
		a.rodar(t, "-from", "outro@exemplo.com", "-modo", "starttls", "sobrando")
		out := a.logs.String()
		assert.Contains(t, out, "difere do usuário autenticado")
		assert.Contains(t, out, "SMTP_PASSWORD tem espaço/tab no início ou no fim")
		assert.Contains(t, out, "argumentos não reconhecidos")
		assert.NotContains(t, out, diagSenha)
	})
	t.Run("porta fechada: conexão recusada com dica", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		_, fechada, _ := net.SplitHostPort(ln.Addr().String())
		ln.Close()
		a := novoAmbienteDiag(t, fechada, pemCA)
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t))
		assert.Contains(t, a.logs.String(), "dial TCP")
	})
	t.Run("DNS que não resolve", func(t *testing.T) {
		a := novoAmbienteDiag(t, porta, pemCA)
		t.Setenv("SMTP_HOST", "host-inexistente.invalid")
		require.Equal(t, cmdutil.SaidaErro, a.rodar(t))
		assert.Contains(t, a.logs.String(), "resolução DNS")
	})
	t.Run("flags: -h e flag desconhecida", func(t *testing.T) {
		a := novoAmbienteDiag(t, porta, pemCA)
		assert.Equal(t, cmdutil.SaidaOK, a.rodar(t, "-h"))
		assert.Equal(t, cmdutil.SaidaUso, a.rodar(t, "-nao-existe"))
	})
}

func TestARQ07_Diagnostico_ArquivoEnv(t *testing.T) {
	cert, pemCA := certificadoTeste(t)
	srv := &smtpFalso{cert: cert, mecanismos: "PLAIN"}
	porta := srv.iniciar(t)

	t.Run(".env carregado: variável ausente do processo vem dele", func(t *testing.T) {
		a := novoAmbienteDiag(t, porta, pemCA)
		require.NoError(t, os.Setenv("SMTP_TLS_SERVER_NAME", "")) // garantido pelo t.Setenv do ambiente
		require.NoError(t, os.Unsetenv("SMTP_TLS_SERVER_NAME"))
		t.Cleanup(func() { os.Unsetenv("SMTP_TLS_SERVER_NAME") })
		require.NoError(t, os.WriteFile(filepath.Join(a.raiz, ".env"), []byte("SMTP_TLS_SERVER_NAME=127.0.0.1\n"), 0o600))
		require.Equal(t, cmdutil.SaidaOK, a.rodar(t))
		out := a.logs.String()
		assert.Contains(t, out, ".env carregado com sucesso")
		assert.Contains(t, out, "origem de SMTP_TLS_SERVER_NAME: .env")
		assert.Contains(t, out, "origem de SMTP_HOST: ambiente do processo")
	})
	t.Run(".env ausente ou raiz não encontrada: segue só com o ambiente", func(t *testing.T) {
		a := novoAmbienteDiag(t, porta, pemCA)
		require.Equal(t, cmdutil.SaidaOK, a.rodar(t))
		assert.Contains(t, a.logs.String(), "seguindo só com as variáveis do ambiente do processo")

		semRaiz := testeemail.NovoCLI(func() (string, error) { return "", fmt.Errorf("sem raiz") })
		amb := cmdutil.Ambiente{Saida: io.Discard, Erro: io.Discard}
		assert.Equal(t, cmdutil.SaidaOK, cmdutil.Executar("testeemail", semRaiz, []string{"-timeout", "5s", "-from", diagUsuario}, amb))
		assert.Contains(t, a.logs.String(), "ERRO ao localizar a raiz do repositório")
	})
}
