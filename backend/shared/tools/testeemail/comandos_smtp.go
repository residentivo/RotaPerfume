package testeemail

// Comandos SMTP, autenticação e transcrição do diagnóstico (ver diagnostico.go).

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Comandos SMTP
// ---------------------------------------------------------------------------

func (d *diagnostico) ehloFinal(c *smtp.Client) error {
	return d.ehlo(c, "EHLO")
}

func (d *diagnostico) ehlo(c *smtp.Client, descricao string) error {
	d.iniciar(descricao)
	d.prazo()
	d.logf("enviando EHLO %s", d.opts.ehlo)
	if err := c.Hello(d.opts.ehlo); err != nil {
		return d.falha("EHLO", err)
	}
	d.logf("EHLO aceito; extensões verificadas (lista completa nas linhas S: acima):")
	for _, ext := range []string{"STARTTLS", "AUTH", "SIZE", "8BITMIME", "SMTPUTF8", "PIPELINING", "ENHANCEDSTATUSCODES", "CHUNKING", "DSN"} {
		ok, param := c.Extension(ext)
		d.logf("  extensão %-20s anunciada=%-5t parâmetros=%q", ext, ok, param)
	}
	return nil
}

func (d *diagnostico) autenticar(c *smtp.Client) error {
	d.iniciar("AUTH")
	if d.cfg.usuario == "" || d.cfg.senha == "" {
		d.logf("AVISO: SMTP_USER/SMTP_PASSWORD vazios — AUTH PULADO")
		return nil
	}
	ok, mecanismos := c.Extension("AUTH")
	d.logf("servidor anunciou AUTH? %t mecanismos=%q", ok, mecanismos)
	if !ok {
		d.logf("AVISO: AUTH não anunciado — PULANDO a autenticação (a API, desde o MAIL-01, ABORTA o envio nesse caso; o diagnóstico segue só para observar o servidor)")
		if !d.usaTLS {
			d.dica("muitos servidores só anunciam AUTH depois do TLS")
		}
		return nil
	}
	if d.opts.inseguro {
		// ARQ-07 NB-E: com o certificado sem validação, o servidor pode ser
		// qualquer um; a senha (PLAIN ou LOGIN) não sai.
		d.dica("rode sem SMTP_USER/SMTP_PASSWORD para observar o resto do fluxo com -inseguro, ou corrija a validação do certificado para testar o AUTH")
		return d.falha("AUTH", errAuthInseguro)
	}
	auth, nome, err := d.escolherAuth(mecanismos)
	if err != nil {
		return d.falha("AUTH", err)
	}
	d.logf("usando AUTH %s com usuário %q (senha de %d caracteres, não logada), host=%q, TLS=%t",
		nome, d.cfg.usuario, len([]rune(d.cfg.senha)), d.serverName, d.usaTLS)
	d.prazo()
	d.transcrito.ocultarEscrita = "credenciais"
	err = c.Auth(auth)
	d.transcrito.ocultarEscrita = ""
	if err != nil {
		d.dicaAuth(err)
		return d.falha("AUTH", err)
	}
	d.logf("AUTH aceito")
	return nil
}

// errAuthInseguro: AUTH recusado com -inseguro (ARQ-07 NB-E).
var errAuthInseguro = errors.New("AUTH recusado pelo diagnóstico: -inseguro ativo (certificado não validado), a senha não é enviada")

// escolherAuth prefere PLAIN (o que a API usa) e cai para LOGIN se for o único.
func (d *diagnostico) escolherAuth(mecanismos string) (smtp.Auth, string, error) {
	lista := strings.Fields(strings.ToUpper(mecanismos))
	tem := func(m string) bool {
		for _, x := range lista {
			if x == m {
				return true
			}
		}
		return false
	}
	switch {
	case tem("PLAIN"):
		base := smtp.PlainAuth("", d.cfg.usuario, d.cfg.senha, d.serverName)
		return &authComEstadoTLS{Auth: base, tls: d.usaTLS, d: d}, "PLAIN", nil
	case tem("LOGIN"):
		d.logf("AVISO: servidor NÃO oferece PLAIN; testando LOGIN (a API também cai para LOGIN)")
		return &authLogin{usuario: d.cfg.usuario, senha: d.cfg.senha, host: d.serverName, tls: d.usaTLS}, "LOGIN", nil
	default:
		return nil, "", fmt.Errorf("nenhum mecanismo suportado (PLAIN/LOGIN) em %q", mecanismos)
	}
}

func (d *diagnostico) dicaAuth(err error) {
	if strings.Contains(err.Error(), "unencrypted connection") {
		d.dica("o net/smtp RECUSA enviar a senha (PlainAuth) sem TLS fora de localhost. Use -modo ssl ou -modo starttls")
		return
	}
	var tp *textproto.Error
	if !errors.As(err, &tp) {
		return
	}
	switch tp.Code {
	case 535:
		d.dica("credenciais rejeitadas (535): confira SMTP_USER e SMTP_PASSWORD no .env (e se a variável do ambiente não está sobrepondo o .env)")
	case 530, 538:
		d.dica("o servidor exige TLS/criptografia antes do AUTH (%d)", tp.Code)
	case 534:
		d.dica("o servidor exige outro mecanismo/política de autenticação (534)")
	}
}

func (d *diagnostico) mailFrom(c *smtp.Client) error {
	d.iniciar("MAIL FROM")
	d.prazo()
	if err := c.Mail(d.de); err != nil {
		d.dicaRemetente()
		return d.falha("MAIL FROM", err)
	}
	d.logf("MAIL FROM:<%s> aceito", d.de)
	return nil
}

func (d *diagnostico) rcptTo(c *smtp.Client) error {
	d.iniciar("RCPT TO")
	d.prazo()
	if err := c.Rcpt(d.para); err != nil {
		var tp *textproto.Error
		if errors.As(err, &tp) && (tp.Code == 550 || tp.Code == 554 || tp.Code == 551) {
			d.dica("destinatário recusado (%d): relay negado/caixa inexistente; sem AUTH o servidor costuma negar relay", tp.Code)
		}
		d.dicaRemetente()
		return d.falha("RCPT TO", err)
	}
	d.logf("RCPT TO:<%s> aceito", d.para)
	return nil
}

func (d *diagnostico) dados(c *smtp.Client) error {
	d.iniciar("DATA")
	agora := time.Now()
	msg, err := MontarMensagem(Mensagem{
		De:        d.de,
		Para:      d.para,
		Assunto:   AssuntoPadrao,
		Corpo:     CorpoPadrao(agora, d.cfg.host, d.cfg.porta, d.modo, d.serverName),
		Data:      agora,
		MessageID: GerarMessageID(agora, Dominio(d.de)),
	})
	if err != nil {
		return d.falha("montagem da mensagem", err)
	}
	d.logf("mensagem montada: %d bytes", len(msg))
	d.prazo()
	w, err := c.Data()
	if err != nil {
		d.dicaRemetente()
		return d.falha("DATA", err)
	}
	d.transcrito.ocultarEscrita = "corpo"
	_, errEscrita := w.Write(msg)
	errFechar := w.Close()
	d.transcrito.ocultarEscrita = ""
	if errEscrita != nil {
		return d.falha("escrita do corpo", errEscrita)
	}
	if errFechar != nil {
		d.dicaRemetente()
		return d.falha("fim do DATA (resposta ao '.')", errFechar)
	}
	d.logf("mensagem ACEITA pelo servidor (ver resposta 250 nas linhas S: acima)")
	return nil
}

func (d *diagnostico) quit(c *smtp.Client) {
	d.iniciar("QUIT")
	d.prazo()
	if err := c.Quit(); err != nil {
		d.logf("AVISO: QUIT falhou (a mensagem já foi aceita no DATA): %v", err)
		return
	}
	d.logf("QUIT ok")
}

// dicaRemetente aponta a causa mais comum de rejeição pós-AUTH.
func (d *diagnostico) dicaRemetente() {
	if d.cfg.usuario != "" && !strings.EqualFold(d.de, d.cfg.usuario) {
		d.dica("o remetente %s é diferente do usuário autenticado %s; o servidor pode exigir From = SMTP_USER: tente -from %s",
			d.de, d.cfg.usuario, d.cfg.usuario)
	}
}

// ---------------------------------------------------------------------------
// Autenticação
// ---------------------------------------------------------------------------

// authComEstadoTLS repassa ao PlainAuth o estado TLS real. Necessário porque o
// net/smtp detecta TLS por type assertion (*tls.Conn), e aqui a conexão está
// embrulhada pela camada de transcrição. Em -modo plain, tls=false e o
// PlainAuth continua recusando (comportamento real do net/smtp).
type authComEstadoTLS struct {
	smtp.Auth
	tls bool
	d   *diagnostico
}

func (a *authComEstadoTLS) Start(server *smtp.ServerInfo) (string, []byte, error) {
	info := *server
	info.TLS = a.tls
	a.d.logf("AUTH Start: ServerInfo{Name=%q, TLS=%t, Auth=%v}", info.Name, info.TLS, info.Auth)
	mecanismo, resp, err := a.Auth.Start(&info)
	if err != nil {
		a.d.logf("PlainAuth recusou iniciar (nada foi enviado ao servidor): %v", err)
	}
	return mecanismo, resp, err
}

// authLogin implementa AUTH LOGIN (fallback para servidores sem PLAIN), com
// as proteções do PlainAuth: só sobre TLS e só para o host esperado (ARQ-07
// NB-E, como notificacaosmtp.authLogin).
type authLogin struct {
	usuario, senha, host string
	tls                  bool
}

func (a *authLogin) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !a.tls {
		return "", nil, errors.New("unencrypted connection (AUTH LOGIN sem TLS recusado pelo diagnóstico)")
	}
	if server.Name != a.host {
		return "", nil, errors.New("wrong host name (AUTH LOGIN recusado pelo diagnóstico)")
	}
	return "LOGIN", nil, nil
}

func (a *authLogin) Next(desafio []byte, mais bool) ([]byte, error) {
	if !mais {
		return nil, nil
	}
	pergunta := strings.ToLower(string(desafio))
	switch {
	case strings.Contains(pergunta, "user"):
		return []byte(a.usuario), nil
	case strings.Contains(pergunta, "pass"):
		return []byte(a.senha), nil
	default:
		return nil, fmt.Errorf("desafio AUTH LOGIN inesperado: %q", desafio)
	}
}

// ---------------------------------------------------------------------------
// Transcrição da conversa SMTP
// ---------------------------------------------------------------------------

func (d *diagnostico) logar(conn net.Conn, camada string) *conexaoLogada {
	c := &conexaoLogada{Conn: conn, camada: camada, d: d}
	d.transcrito = c
	return c
}

// conexaoLogada loga cada linha lida (S:) e escrita (C:). Credenciais do
// AUTH e o corpo da mensagem são substituídos por contagem de bytes.
type conexaoLogada struct {
	net.Conn
	camada         string
	d              *diagnostico
	pendente       []byte
	ocultarEscrita string
}

func (c *conexaoLogada) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.pendente = append(c.pendente, p[:n]...)
	for {
		i := bytes.IndexByte(c.pendente, '\n')
		if i < 0 {
			break
		}
		c.d.logf("S[%s]: %s", c.camada, strings.TrimRight(string(c.pendente[:i]), "\r"))
		c.pendente = c.pendente[i+1:]
	}
	if err != nil && err != io.EOF {
		c.d.logf("leitura [%s] falhou após %d bytes: %v", c.camada, n, err)
	}
	return n, err
}

func (c *conexaoLogada) Write(p []byte) (int, error) {
	switch c.ocultarEscrita {
	case "":
		for _, linha := range strings.Split(strings.TrimRight(string(p), "\r\n"), "\n") {
			c.d.logf("C[%s]: %s", c.camada, strings.TrimRight(linha, "\r"))
		}
	case "credenciais":
		c.d.logf("C[%s]: <%d bytes do AUTH ocultados (credenciais)>", c.camada, len(p))
	default:
		c.d.logf("C[%s]: <%d bytes do %s da mensagem>", c.camada, len(p), c.ocultarEscrita)
	}
	n, err := c.Conn.Write(p)
	if err != nil {
		c.d.logf("escrita [%s] falhou: %v", c.camada, err)
	}
	return n, err
}

// conexaoComBanner entrega primeiro um banner sintético e depois a conexão real.
type conexaoComBanner struct {
	net.Conn
	leitor io.Reader
}

func (c *conexaoComBanner) Read(p []byte) (int, error) {
	return c.leitor.Read(p)
}
