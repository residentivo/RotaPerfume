// Diagnóstico de envio de e-mail (cmd/testeemail): envia UMA mensagem de
// teste com as mesmas variáveis SMTP_* da API e loga TODOS os passos (.env,
// DNS, TCP, TLS/certificado, banner, EHLO, STARTTLS, AUTH, MAIL FROM, RCPT
// TO, DATA, QUIT) com a transcrição SMTP; credenciais do AUTH e o corpo
// nunca são logados. ARQ-07: a lógica saiu do main para cá (testável); o
// main é só a composition root.
package testeemail

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
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
)

// Valores padrão do diagnóstico.
const (
	remetentePadrao    = "henrique.rodrigues@rotaperfumes.com.br"
	destinatarioPadrao = "admin@rotaperfumes.com.br"
	hostPadrao         = "smtp.gmail.com" // mesmo padrão de config.go
	portaPadrao        = "587"            // mesmo padrão de config.go
)

// variaveisSMTP são as variáveis lidas pela API (config.go).
var variaveisSMTP = []string{"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASSWORD", "SMTP_FROM",
	"SMTP_TLS_MODE", "SMTP_TLS_SERVER_NAME", "SMTP_TLS_CA_FILE"}

// opcoes reúne as flags da linha de comando.
type opcoes struct {
	de            string
	para          string
	modo          string
	tlsServerName string
	tlsCAFile     string
	salvarCert    string
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
	raizes     *x509.CertPool // nil = raízes do sistema
	de, para   string
	tcp        net.Conn
	transcrito *conexaoLogada
	usaTLS     bool
	// raizProjeto localiza a raiz do repositório (o .env); injetável nos testes.
	raizProjeto func() (string, error)
}

// CLI é o comando cmd/testeemail: o log é sempre ligado (não depende de
// VERBOSE) e vai para onde o main apontar o log padrão (stdout). Falha →
// resumo "RESUMO: FALHA no passo NN" e código de saída 1.
var CLI = NovoCLI(cmdutil.FindProjectRoot)

// NovoCLI monta o comando com a busca da raiz do repositório (onde fica o
// .env) injetada.
func NovoCLI(raizProjeto func() (string, error)) cmdutil.Comando {
	return func(args []string, amb cmdutil.Ambiente) error {
		opts, err := lerFlags(args, amb)
		if err != nil {
			return err
		}
		d := &diagnostico{opts: opts, raizProjeto: raizProjeto}
		return d.rodar()
	}
}

// rodar executa o diagnóstico e loga o resumo.
func (d *diagnostico) rodar() error {
	err := d.executar()
	if d.tcp != nil {
		_ = d.tcp.Close()
	}
	if err != nil {
		log.Printf("==================================================================")
		log.Printf("RESUMO: FALHA no passo %02d (%s): %v", d.passo, d.descricao, err)
		log.Printf("==================================================================")
		return cmdutil.ErrJaRelatado
	}
	log.Printf("==================================================================")
	log.Printf("RESUMO: SUCESSO — mensagem aceita pelo servidor %s:%s (modo %s); confira a caixa de %s",
		d.cfg.host, d.cfg.porta, d.modo, d.para)
	log.Printf("==================================================================")
	return nil
}

func lerFlags(args []string, amb cmdutil.Ambiente) (opcoes, error) {
	var o opcoes
	fs := cmdutil.NovasFlags("testeemail", amb)
	fs.StringVar(&o.de, "from", remetentePadrao, "remetente (cabeçalho From e MAIL FROM)")
	fs.StringVar(&o.para, "to", destinatarioPadrao, "destinatário (cabeçalho To e RCPT TO)")
	fs.StringVar(&o.modo, "modo", ModoAuto, "conexão: auto (465=ssl, demais=starttls) | ssl | starttls | plain")
	fs.StringVar(&o.tlsServerName, "tls-server-name", "", "nome esperado no certificado TLS (padrão: SMTP_TLS_SERVER_NAME ou SMTP_HOST), ex.: mail.rotaperfumes.com.br")
	fs.StringVar(&o.tlsCAFile, "tls-ca-file", "", "PEM com CA/certificado extra a confiar (padrão: SMTP_TLS_CA_FILE)")
	fs.StringVar(&o.salvarCert, "salvar-cert", "", "grava em PEM o certificado apresentado pelo servidor (para conferir o fingerprint e usar em SMTP_TLS_CA_FILE)")
	fs.BoolVar(&o.inseguro, "inseguro", false, "NÃO valida o certificado TLS (InsecureSkipVerify) — SÓ para diagnóstico")
	fs.DurationVar(&o.timeout, "timeout", 20*time.Second, "prazo máximo de cada etapa de rede (ex.: 20s, 1m)")
	fs.StringVar(&o.ehlo, "ehlo", "localhost", "nome enviado no EHLO (net/smtp e a API usam localhost)")
	if err := cmdutil.LerFlags(fs, args); err != nil {
		return o, err
	}
	if fs.NArg() > 0 {
		log.Printf("AVISO: argumentos não reconhecidos %q — no PowerShell 5.1 use -to admin@x.com.br (com espaço), não -to=admin@x.com.br", fs.Args())
	}
	return o, nil
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
	case ModoSSL:
		return d.conectarSSL()
	case ModoStartTLS:
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

	raiz, err := d.raizProjeto()
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
	if d.de, err = ValidarEndereco(d.opts.de); err != nil {
		return d.falha("remetente (-from)", err)
	}
	if d.para, err = ValidarEndereco(d.opts.para); err != nil {
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
	modoPedido := d.opts.modo
	if envModo := os.Getenv("SMTP_TLS_MODE"); strings.EqualFold(strings.TrimSpace(modoPedido), ModoAuto) && envModo != "" {
		convertido, err := ModoDaVariavelAPI(envModo)
		if err != nil {
			return d.falha("SMTP_TLS_MODE", err)
		}
		d.logf("-modo auto e SMTP_TLS_MODE=%q: usando modo %s (o mesmo da API)", envModo, convertido)
		modoPedido = convertido
	}
	modo, err := EscolherModo(modoPedido, d.cfg.porta)
	if err != nil {
		return d.falha("flag -modo", err)
	}
	d.modo = modo
	d.logf("flag -modo=%q, porta=%s => modo efetivo: %s", d.opts.modo, d.cfg.porta, modo)
	switch {
	case modo == ModoStartTLS && d.cfg.porta == PortaSMTPS:
		d.logf("AVISO: porta 465 costuma ser TLS implícito (ssl). Em starttls o cliente espera o banner em texto puro e tende a dar TIMEOUT. A API (email_service) infere implicit na 465; force com SMTP_TLS_MODE.")
	case modo == ModoSSL && d.cfg.porta != PortaSMTPS:
		d.logf("AVISO: modo ssl em porta %s (não 465): se o servidor falar texto puro, o handshake TLS falha", d.cfg.porta)
	case modo == ModoPlain:
		d.logf("AVISO: modo plain = SEM TLS. O net/smtp (PlainAuth) recusa enviar senha sem TLS fora de localhost.")
	}
	d.serverName = d.cfg.host
	switch {
	case d.opts.tlsServerName != "":
		d.serverName = d.opts.tlsServerName
		d.logf("ServerName TLS sobrescrito por -tls-server-name: %s", d.serverName)
	case strings.TrimSpace(os.Getenv("SMTP_TLS_SERVER_NAME")) != "":
		d.serverName = strings.TrimSpace(os.Getenv("SMTP_TLS_SERVER_NAME"))
		d.logf("ServerName TLS vindo de SMTP_TLS_SERVER_NAME: %s", d.serverName)
	}
	d.logf("ServerName (TLS e host do PlainAuth): %s", d.serverName)
	return d.carregarCA()
}

// carregarCA monta as raízes confiáveis: sistema + PEM de -tls-ca-file ou
// SMTP_TLS_CA_FILE (mesma regra da API).
func (d *diagnostico) carregarCA() error {
	caminho := d.opts.tlsCAFile
	if caminho == "" {
		caminho = os.Getenv("SMTP_TLS_CA_FILE")
	}
	if caminho == "" {
		d.logf("sem SMTP_TLS_CA_FILE/-tls-ca-file: confiando só nas raízes do sistema")
		return nil
	}
	pemBytes, err := os.ReadFile(caminho)
	if err != nil {
		return d.falha("leitura da CA extra", err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		d.logf("raízes do sistema indisponíveis (%v); usando só o arquivo", err)
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pemBytes) {
		return d.falha("leitura da CA extra", fmt.Errorf("%s: nenhum certificado PEM válido", caminho))
	}
	d.raizes = pool
	d.logf("CA extra carregada de %s (além das raízes do sistema)", caminho)
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
