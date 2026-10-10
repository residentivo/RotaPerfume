// Command testeemail é um DIAGNÓSTICO de envio de e-mail: envia UMA mensagem
// de teste usando as mesmas variáveis SMTP_* da API e loga TODOS os passos
// (carga do .env, DNS, TCP, TLS/certificado, banner, EHLO, STARTTLS, AUTH,
// MAIL FROM, RCPT TO, DATA, QUIT) com a transcrição SMTP completa — a senha e
// as credenciais do AUTH nunca são logadas. O log é sempre ligado (não depende
// de VERBOSE). As funções puras ficam em tools/testeemail.
//
// Uso (PowerShell, a partir de apis/shared):
//
//	& "C:\Program Files\Go\bin\go.exe" run ./cmd/testeemail
//	& "C:\Program Files\Go\bin\go.exe" run ./cmd/testeemail -to admin@rotaperfumes.com.br -modo ssl
//	& "C:\Program Files\Go\bin\go.exe" run ./cmd/testeemail -tls-server-name mail.rotaperfumes.com.br
//	& "C:\Program Files\Go\bin\go.exe" run ./cmd/testeemail -inseguro -from admin@rotaperfumes.com.br
//	& "C:\Program Files\Go\bin\go.exe" run ./cmd/testeemail -h
//
// ATENÇÃO (PowerShell 5.1): NÃO use -flag=valor quando o valor tem ponto
// (ex.: -to=admin@x.com.br). O PS 5.1 quebra o argumento no primeiro ponto e
// o programa recebe "-to=admin@x" e ".com.br" separados. Use espaço
// (-to admin@x.com.br) ou aspas ("-to=admin@x.com.br").
//
// Flags: -from, -to, -modo auto|ssl|starttls|plain, -tls-server-name,
// -inseguro, -timeout, -ehlo. Sai com código 1 em caso de falha.
package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/tools/testeemail"
	// Fixa time.Local em -03:00 para o cabeçalho Date e o corpo do e-mail.
	_ "github.com/rotaperfumes/shared/tz"
)

// Valores padrão do diagnóstico.
const (
	remetentePadrao    = "henrique.rodrigues@rotaperfumes.com.br"
	destinatarioPadrao = "admin@rotaperfumes.com.br"
	hostPadrao         = "smtp.gmail.com" // mesmo padrão de config.go
	portaPadrao        = "587"            // mesmo padrão de config.go
)

// variaveisSMTP são as variáveis lidas pela API (config.go).
var variaveisSMTP = []string{"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASSWORD", "SMTP_FROM"}

// opcoes reúne as flags da linha de comando.
type opcoes struct {
	de            string
	para          string
	modo          string
	tlsServerName string
	inseguro      bool
	timeout       time.Duration
	ehlo          string
}

// configSMTP são os valores lidos do ambiente (.env + processo).
type configSMTP struct {
	host, porta, usuario, senha, from string
}

// diagnostico guarda o estado da execução e o passo atual (para o resumo).
type diagnostico struct {
	opts       opcoes
	cfg        configSMTP
	passo      int
	descricao  string
	modo       string
	serverName string
	de, para   string
	tcp        net.Conn
	transcrito *conexaoLogada
	usaTLS     bool
}

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	opts := lerFlags()
	d := &diagnostico{opts: opts}
	err := d.executar()
	if d.tcp != nil {
		_ = d.tcp.Close()
	}
	if err != nil {
		log.Printf("==================================================================")
		log.Printf("RESUMO: FALHA no passo %02d (%s): %v", d.passo, d.descricao, err)
		log.Printf("==================================================================")
		os.Exit(1)
	}
	log.Printf("==================================================================")
	log.Printf("RESUMO: SUCESSO — mensagem aceita pelo servidor %s:%s (modo %s); confira a caixa de %s",
		d.cfg.host, d.cfg.porta, d.modo, d.para)
	log.Printf("==================================================================")
}

func lerFlags() opcoes {
	var o opcoes
	flag.StringVar(&o.de, "from", remetentePadrao, "remetente (cabeçalho From e MAIL FROM)")
	flag.StringVar(&o.para, "to", destinatarioPadrao, "destinatário (cabeçalho To e RCPT TO)")
	flag.StringVar(&o.modo, "modo", testeemail.ModoAuto, "conexão: auto (465=ssl, demais=starttls) | ssl | starttls | plain")
	flag.StringVar(&o.tlsServerName, "tls-server-name", "", "nome esperado no certificado TLS (padrão: SMTP_HOST), ex.: mail.rotaperfumes.com.br")
	flag.BoolVar(&o.inseguro, "inseguro", false, "NÃO valida o certificado TLS (InsecureSkipVerify) — SÓ para diagnóstico")
	flag.DurationVar(&o.timeout, "timeout", 20*time.Second, "prazo máximo de cada etapa de rede (ex.: 20s, 1m)")
	flag.StringVar(&o.ehlo, "ehlo", "localhost", "nome enviado no EHLO (net/smtp e a API usam localhost)")
	flag.Parse()
	if flag.NArg() > 0 {
		log.Printf("AVISO: argumentos não reconhecidos %q — no PowerShell 5.1 use -to admin@x.com.br (com espaço), não -to=admin@x.com.br", flag.Args())
	}
	return o
}

// ---------------------------------------------------------------------------
// Log por passo
// ---------------------------------------------------------------------------

func (d *diagnostico) iniciar(descricao string) {
	d.passo++
	d.descricao = descricao
	d.logf("========== %s ==========", descricao)
}

func (d *diagnostico) logf(formato string, args ...any) {
	log.Printf(fmt.Sprintf("[passo %02d] ", d.passo)+formato, args...)
}

func (d *diagnostico) dica(formato string, args ...any) {
	d.logf("DICA: "+formato, args...)
}

// falha loga o erro completo (com código SMTP, se houver) e o devolve embrulhado.
func (d *diagnostico) falha(contexto string, err error) error {
	d.logf("ERRO em %s: %v", contexto, err)
	d.logf("ERRO tipo Go: %T", err)
	var tp *textproto.Error
	if errors.As(err, &tp) {
		d.logf("ERRO resposta SMTP do servidor: código=%d mensagem=%q", tp.Code, tp.Msg)
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		d.logf("ERRO é TIMEOUT: o servidor não respondeu em %s", d.opts.timeout)
	}
	return fmt.Errorf("%s: %w", contexto, err)
}

// prazo renova o deadline do socket para a próxima etapa de rede.
func (d *diagnostico) prazo() {
	if d.tcp == nil {
		return
	}
	limite := time.Now().Add(d.opts.timeout)
	if err := d.tcp.SetDeadline(limite); err != nil {
		d.logf("AVISO: SetDeadline falhou: %v", err)
		return
	}
	d.logf("deadline de rede definido para %s (timeout %s)", limite.Format("15:04:05.000000"), d.opts.timeout)
}

// ---------------------------------------------------------------------------
// Fluxo principal
// ---------------------------------------------------------------------------

func (d *diagnostico) executar() error {
	etapas := []func() error{
		d.carregarEnv,
		d.lerConfig,
		d.validarEnderecos,
		d.decidirModo,
		d.resolverDNS,
		d.discarTCP,
	}
	for _, etapa := range etapas {
		if err := etapa(); err != nil {
			return err
		}
	}
	cliente, err := d.conectarSMTP()
	if err != nil {
		return err
	}
	defer cliente.Close()
	return d.enviar(cliente)
}

func (d *diagnostico) conectarSMTP() (*smtp.Client, error) {
	switch d.modo {
	case testeemail.ModoSSL:
		return d.conectarSSL()
	case testeemail.ModoStartTLS:
		return d.conectarStartTLS()
	default:
		return d.conectarPlain()
	}
}

func (d *diagnostico) enviar(c *smtp.Client) error {
	etapas := []func(*smtp.Client) error{
		d.ehloFinal,
		d.autenticar,
		d.mailFrom,
		d.rcptTo,
		d.dados,
	}
	for _, etapa := range etapas {
		if err := etapa(c); err != nil {
			return err
		}
	}
	d.quit(c)
	return nil
}

// ---------------------------------------------------------------------------
// Ambiente e configuração
// ---------------------------------------------------------------------------

func (d *diagnostico) carregarEnv() error {
	d.iniciar("carregar .env da raiz do repositório")
	if wd, err := os.Getwd(); err == nil {
		d.logf("diretório atual: %s", wd)
	}
	jaNoAmbiente := map[string]bool{}
	for _, nome := range variaveisSMTP {
		_, existe := os.LookupEnv(nome)
		jaNoAmbiente[nome] = existe
		d.logf("antes do .env: %s já existe no ambiente do processo? %t", nome, existe)
	}
	d.logf("AVISO: godotenv.Load NÃO sobrescreve variáveis que já existem no ambiente — as marcadas 'true' acima prevalecem sobre o .env")

	raiz, err := cmdutil.FindProjectRoot()
	if err != nil {
		d.logf("ERRO ao localizar a raiz do repositório: %v", err)
		d.logf("seguindo só com as variáveis do ambiente do processo")
		return nil
	}
	caminho := filepath.Join(raiz, ".env")
	d.logf("raiz do repositório: %s", raiz)
	d.logf("carregando %s", caminho)
	if err := godotenv.Load(caminho); err != nil {
		d.logf("ERRO ao carregar %s: %v", caminho, err)
		d.logf("seguindo só com as variáveis do ambiente do processo")
		return nil
	}
	d.logf(".env carregado com sucesso: %s", caminho)
	for _, nome := range variaveisSMTP {
		d.logf("origem de %s: %s", nome, origemVariavel(nome, jaNoAmbiente[nome]))
	}
	return nil
}

func origemVariavel(nome string, jaNoAmbiente bool) string {
	if jaNoAmbiente {
		return "ambiente do processo (o .env foi IGNORADO para ela)"
	}
	if _, existe := os.LookupEnv(nome); existe {
		return ".env"
	}
	return "ausente"
}

func (d *diagnostico) lerConfig() error {
	d.iniciar("ler configuração SMTP_*")
	d.cfg = configSMTP{
		host:    d.envComPadrao("SMTP_HOST", hostPadrao),
		porta:   d.envComPadrao("SMTP_PORT", portaPadrao),
		usuario: os.Getenv("SMTP_USER"),
		senha:   os.Getenv("SMTP_PASSWORD"),
		from:    os.Getenv("SMTP_FROM"),
	}
	d.logf("SMTP_HOST=%q", d.cfg.host)
	d.logf("SMTP_PORT=%q", d.cfg.porta)
	d.logf("SMTP_USER=%q", d.cfg.usuario)
	d.logf("SMTP_FROM=%q (a API usa este como remetente; este diagnóstico usa -from)", d.cfg.from)
	d.logf("SMTP_PASSWORD preenchida=%t tamanho=%d caracteres (valor NUNCA logado)", d.cfg.senha != "", len([]rune(d.cfg.senha)))
	if d.cfg.senha != strings.TrimSpace(d.cfg.senha) {
		d.logf("AVISO: SMTP_PASSWORD tem espaço/tab no início ou no fim — confira o .env")
	}
	if strings.TrimSpace(d.cfg.host) == "" || strings.TrimSpace(d.cfg.porta) == "" {
		return d.falha("configuração", errors.New("SMTP_HOST/SMTP_PORT vazios"))
	}
	if d.cfg.usuario == "" || d.cfg.senha == "" {
		d.logf("AVISO: SMTP_USER ou SMTP_PASSWORD vazios — o AUTH será pulado (a API usaria o EmailService noop)")
	}
	return nil
}

func (d *diagnostico) envComPadrao(nome, padrao string) string {
	if v := os.Getenv(nome); v != "" {
		return v
	}
	d.logf("%s vazio: usando o padrão da API %q", nome, padrao)
	return padrao
}

func (d *diagnostico) validarEnderecos() error {
	d.iniciar("validar remetente e destinatário")
	var err error
	if d.de, err = testeemail.ValidarEndereco(d.opts.de); err != nil {
		return d.falha("remetente (-from)", err)
	}
	if d.para, err = testeemail.ValidarEndereco(d.opts.para); err != nil {
		return d.falha("destinatário (-to)", err)
	}
	d.logf("remetente (From/MAIL FROM): %s", d.de)
	d.logf("destinatário (To/RCPT TO): %s", d.para)
	if d.cfg.usuario != "" && !strings.EqualFold(d.de, d.cfg.usuario) {
		d.logf("AVISO: remetente %s difere do usuário autenticado SMTP_USER=%s — muitos servidores rejeitam isso", d.de, d.cfg.usuario)
	}
	return nil
}

func (d *diagnostico) decidirModo() error {
	d.iniciar("escolher modo de conexão")
	modo, err := testeemail.EscolherModo(d.opts.modo, d.cfg.porta)
	if err != nil {
		return d.falha("flag -modo", err)
	}
	d.modo = modo
	d.logf("flag -modo=%q, porta=%s => modo efetivo: %s", d.opts.modo, d.cfg.porta, modo)
	switch {
	case modo == testeemail.ModoStartTLS && d.cfg.porta == testeemail.PortaSMTPS:
		d.logf("AVISO: porta 465 costuma ser TLS implícito (ssl). Em starttls o cliente espera o banner em texto puro e tende a dar TIMEOUT. É exatamente o que o email_service.go faz hoje.")
	case modo == testeemail.ModoSSL && d.cfg.porta != testeemail.PortaSMTPS:
		d.logf("AVISO: modo ssl em porta %s (não 465): se o servidor falar texto puro, o handshake TLS falha", d.cfg.porta)
	case modo == testeemail.ModoPlain:
		d.logf("AVISO: modo plain = SEM TLS. O net/smtp (PlainAuth) recusa enviar senha sem TLS fora de localhost.")
	}
	d.serverName = d.cfg.host
	if d.opts.tlsServerName != "" {
		d.serverName = d.opts.tlsServerName
		d.logf("ServerName TLS sobrescrito por -tls-server-name: %s", d.serverName)
	}
	d.logf("ServerName (TLS e host do PlainAuth): %s", d.serverName)
	return nil
}

// ---------------------------------------------------------------------------
// Rede: DNS e TCP
// ---------------------------------------------------------------------------

func (d *diagnostico) resolverDNS() error {
	d.iniciar("resolver DNS do host")
	if ip := net.ParseIP(d.cfg.host); ip != nil {
		d.logf("%s é um IP literal (o LookupHost devolve o próprio IP)", d.cfg.host)
	}
	ctx, cancel := context.WithTimeout(context.Background(), d.opts.timeout)
	defer cancel()
	inicio := time.Now()
	ips, err := net.DefaultResolver.LookupHost(ctx, d.cfg.host)
	d.logf("LookupHost(%q) levou %s", d.cfg.host, time.Since(inicio))
	if err != nil {
		return d.falha("resolução DNS", err)
	}
	d.logf("endereços resolvidos: %s", strings.Join(ips, ", "))
	return nil
}

func (d *diagnostico) discarTCP() error {
	d.iniciar("conectar TCP")
	endereco := net.JoinHostPort(d.cfg.host, d.cfg.porta)
	discador := net.Dialer{Timeout: d.opts.timeout}
	d.logf("discando tcp %s (timeout %s)", endereco, d.opts.timeout)
	inicio := time.Now()
	conn, err := discador.Dial("tcp", endereco)
	d.logf("dial levou %s", time.Since(inicio))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "refused") {
			d.dica("conexão RECUSADA: nada escutando em %s ou firewall rejeitando; confira SMTP_PORT (465/587/25)", endereco)
		}
		return d.falha("dial TCP", err)
	}
	d.tcp = conn
	d.logf("conectado: local=%s remoto=%s", conn.LocalAddr(), conn.RemoteAddr())
	return nil
}

// ---------------------------------------------------------------------------
// Modos de conexão
// ---------------------------------------------------------------------------

// conectarSSL: TLS implícito (SMTPS, porta 465). Equivale a tls.DialWithDialer,
// mas com dial e handshake separados para medir e logar cada um.
func (d *diagnostico) conectarSSL() (*smtp.Client, error) {
	tlsConn, err := d.handshakeTLS()
	if err != nil {
		return nil, err
	}
	d.usaTLS = true
	return d.novoCliente(d.logar(tlsConn, "tls"), "banner SMTP (greeting) sobre TLS")
}

// conectarStartTLS: TCP puro, EHLO, STARTTLS, handshake e novo cliente sobre TLS.
func (d *diagnostico) conectarStartTLS() (*smtp.Client, error) {
	textoPuro, err := d.novoCliente(d.logar(d.tcp, "tcp"), "banner SMTP (greeting) em texto puro")
	if err != nil {
		if d.cfg.porta == testeemail.PortaSMTPS {
			d.dica("na porta 465 o servidor espera TLS logo de cara e não manda banner em texto puro: tente -modo ssl")
		}
		return nil, err
	}
	if err := d.ehlo(textoPuro, "EHLO em texto puro (antes do STARTTLS)"); err != nil {
		return nil, err
	}
	if err := d.comandoStartTLS(textoPuro); err != nil {
		return nil, err
	}
	tlsConn, err := d.handshakeTLS()
	if err != nil {
		return nil, err
	}
	d.usaTLS = true
	// Após o STARTTLS o servidor NÃO manda novo banner, mas smtp.NewClient
	// sempre lê um 220. Injetamos um 220 sintético (não vem do servidor).
	logada := d.logar(tlsConn, "tls")
	comBanner := &conexaoComBanner{
		Conn:   logada,
		leitor: io.MultiReader(strings.NewReader("220 banner-sintetico-pos-starttls\r\n"), logada),
	}
	d.logf("cliente SMTP sobre TLS criado com um 220 SINTÉTICO local (após STARTTLS o servidor não reenvia banner)")
	return d.novoCliente(comBanner, "cliente SMTP sobre TLS (pós-STARTTLS)")
}

// conectarPlain: sem TLS, só diagnóstico.
func (d *diagnostico) conectarPlain() (*smtp.Client, error) {
	d.logf("AVISO: modo plain — tudo trafega em texto puro")
	return d.novoCliente(d.logar(d.tcp, "tcp"), "banner SMTP (greeting) em texto puro")
}

func (d *diagnostico) comandoStartTLS(c *smtp.Client) error {
	d.iniciar("STARTTLS")
	ok, _ := c.Extension("STARTTLS")
	d.logf("servidor anunciou STARTTLS? %t", ok)
	if !ok {
		if d.cfg.porta == testeemail.PortaSMTPS {
			d.dica("porta 465 normalmente é TLS implícito: tente -modo ssl")
		}
		d.dica("o email_service.go exige STARTTLS e falharia aqui; para testar sem TLS use -modo plain")
		return d.falha("STARTTLS", errors.New("servidor não anunciou STARTTLS no EHLO"))
	}
	d.prazo()
	id, err := c.Text.Cmd("STARTTLS")
	if err != nil {
		return d.falha("envio do comando STARTTLS", err)
	}
	c.Text.StartResponse(id)
	codigo, msg, err := c.Text.ReadResponse(220)
	c.Text.EndResponse(id)
	if err != nil {
		return d.falha("resposta do STARTTLS", err)
	}
	d.logf("STARTTLS aceito: %d %s", codigo, msg)
	return nil
}

func (d *diagnostico) novoCliente(conn net.Conn, descricao string) (*smtp.Client, error) {
	d.iniciar(descricao)
	d.prazo()
	inicio := time.Now()
	c, err := smtp.NewClient(conn, d.serverName)
	d.logf("leitura do banner levou %s", time.Since(inicio))
	if err != nil {
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() && !d.usaTLS {
			d.dica("TIMEOUT esperando o banner em texto puro: o servidor provavelmente fala TLS implícito nesta porta — tente -modo ssl")
		}
		return nil, d.falha("banner SMTP", err)
	}
	d.logf("banner recebido (ver linhas S: acima)")
	return c, nil
}

// ---------------------------------------------------------------------------
// TLS
// ---------------------------------------------------------------------------

func (d *diagnostico) handshakeTLS() (*tls.Conn, error) {
	d.iniciar("handshake TLS")
	if d.opts.inseguro {
		d.logf("##################################################################")
		d.logf("WARNING: -inseguro ATIVO — o certificado do servidor NÃO será validado!")
		d.logf("WARNING: use SÓ para diagnóstico; NUNCA em produção (sujeito a MITM).")
		d.logf("##################################################################")
	}
	d.logf("ServerName=%q MinVersion=TLS1.2 verificação=%t", d.serverName, !d.opts.inseguro)
	tlsConn := tls.Client(d.tcp, d.configTLS())
	d.prazo()
	ctx, cancel := context.WithTimeout(context.Background(), d.opts.timeout)
	defer cancel()
	inicio := time.Now()
	err := tlsConn.HandshakeContext(ctx)
	d.logf("handshake levou %s", time.Since(inicio))
	if err != nil {
		var rh tls.RecordHeaderError
		if errors.As(err, &rh) {
			d.dica("o servidor respondeu algo que NÃO é TLS (%q): nesta porta ele fala SMTP em texto puro — tente -modo starttls", string(rh.RecordHeader[:]))
		}
		return nil, d.falha("handshake TLS", err)
	}
	estado := tlsConn.ConnectionState()
	d.logf("TLS estabelecido: versão=%s cipher=%s ServerName(SNI)=%q retomada=%t",
		tls.VersionName(estado.Version), tls.CipherSuiteName(estado.CipherSuite), estado.ServerName, estado.DidResume)
	return tlsConn, nil
}

// configTLS liga InsecureSkipVerify só para que a verificação seja feita em
// verificarCertificado: assim os dados do certificado são logados mesmo
// quando a validação falha. Sem -inseguro, o erro de validação aborta o
// handshake exatamente como a verificação padrão do crypto/tls.
func (d *diagnostico) configTLS() *tls.Config {
	return &tls.Config{
		ServerName:         d.serverName,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, //nolint:gosec // verificação manual em VerifyConnection
		VerifyConnection:   d.verificarCertificado,
	}
}

func (d *diagnostico) verificarCertificado(cs tls.ConnectionState) error {
	d.logf("servidor apresentou %d certificado(s)", len(cs.PeerCertificates))
	for i, cert := range cs.PeerCertificates {
		d.logarCertificado(i, cert)
	}
	err := d.validarCadeia(cs)
	if err == nil {
		d.logf("verificação do certificado: OK (válido para %q)", d.serverName)
		return nil
	}
	d.logf("verificação do certificado: FALHOU: %v", err)
	d.dicaCertificado(err, cs)
	if d.opts.inseguro {
		d.logf("WARNING: falha de verificação IGNORADA por causa de -inseguro")
		return nil
	}
	return err
}

func (d *diagnostico) validarCadeia(cs tls.ConnectionState) error {
	if len(cs.PeerCertificates) == 0 {
		return errors.New("servidor não enviou certificado")
	}
	intermediarios := x509.NewCertPool()
	for _, cert := range cs.PeerCertificates[1:] {
		intermediarios.AddCert(cert)
	}
	_, err := cs.PeerCertificates[0].Verify(x509.VerifyOptions{
		DNSName:       d.serverName,
		Intermediates: intermediarios,
	})
	return err
}

func (d *diagnostico) logarCertificado(i int, cert *x509.Certificate) {
	d.logf("cert[%d] Subject: %s", i, cert.Subject)
	d.logf("cert[%d] Issuer:  %s", i, cert.Issuer)
	d.logf("cert[%d] SANs DNS: %v", i, cert.DNSNames)
	d.logf("cert[%d] SANs IP:  %v", i, cert.IPAddresses)
	agora := time.Now()
	d.logf("cert[%d] validade: %s até %s (expirado=%t, ainda-não-válido=%t)", i,
		cert.NotBefore.Format(time.RFC3339), cert.NotAfter.Format(time.RFC3339),
		agora.After(cert.NotAfter), agora.Before(cert.NotBefore))
	d.logf("cert[%d] autoassinado (Subject==Issuer)? %t", i, bytes.Equal(cert.RawSubject, cert.RawIssuer))
}

func (d *diagnostico) dicaCertificado(err error, cs tls.ConnectionState) {
	var nomeErr x509.HostnameError
	var autErr x509.UnknownAuthorityError
	var invErr x509.CertificateInvalidError
	switch {
	case errors.As(err, &nomeErr):
		nomes := "(nenhum SAN DNS)"
		if len(cs.PeerCertificates) > 0 && len(cs.PeerCertificates[0].DNSNames) > 0 {
			nomes = strings.Join(cs.PeerCertificates[0].DNSNames, ", ")
		}
		d.dica("o certificado não vale para %q. Nomes no certificado: %s. Use -tls-server-name <um desses nomes> (ex.: -tls-server-name mail.rotaperfumes.com.br)", d.serverName, nomes)
	case errors.As(err, &autErr):
		d.dica("certificado autoassinado ou de CA não confiável neste Windows. Para diagnosticar use -inseguro; para corrigir, instale a CA ou use certificado público")
	case errors.As(err, &invErr):
		d.dica("certificado inválido (motivo %d: expirado/não autorizado). Use -inseguro só para confirmar o resto do fluxo", invErr.Reason)
	default:
		d.dica("tente -tls-server-name <nome do certificado> ou, só para diagnóstico, -inseguro")
	}
}

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
		d.logf("AVISO: AUTH não anunciado — PULANDO a autenticação (o email_service.go também pula em silêncio nesse caso; o servidor pode recusar o RCPT depois)")
		if !d.usaTLS {
			d.dica("muitos servidores só anunciam AUTH depois do TLS")
		}
		return nil
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
		d.logf("AVISO: servidor NÃO oferece PLAIN (o único que o email_service.go usa); testando LOGIN")
		return &authLogin{usuario: d.cfg.usuario, senha: d.cfg.senha, tls: d.usaTLS}, "LOGIN", nil
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
	msg, err := testeemail.MontarMensagem(testeemail.Mensagem{
		De:        d.de,
		Para:      d.para,
		Assunto:   testeemail.AssuntoPadrao,
		Corpo:     testeemail.CorpoPadrao(agora, d.cfg.host, d.cfg.porta, d.modo, d.serverName),
		Data:      agora,
		MessageID: testeemail.GerarMessageID(agora, testeemail.Dominio(d.de)),
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

// authLogin implementa AUTH LOGIN (fallback para servidores sem PLAIN).
type authLogin struct {
	usuario, senha string
	tls            bool
}

func (a *authLogin) Start(_ *smtp.ServerInfo) (string, []byte, error) {
	if !a.tls {
		return "", nil, errors.New("unencrypted connection (AUTH LOGIN sem TLS recusado pelo diagnóstico)")
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
