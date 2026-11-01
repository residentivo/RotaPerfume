package testeemail

// Modos de conexão e TLS do diagnóstico (ver diagnostico.go).

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"
)

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
		if d.cfg.porta == PortaSMTPS {
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
		if d.cfg.porta == PortaSMTPS {
			d.dica("porta 465 normalmente é TLS implícito: tente -modo ssl")
		}
		d.dica("a API em modo starttls exige STARTTLS e falharia aqui; para testar sem TLS use -modo plain")
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
	d.salvarCertificado(cs)
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
		Roots:         d.raizes,
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
	d.logf("cert[%d] SHA-256: %s", i, FingerprintSHA256(cert.Raw))
}

// salvarCertificado grava o certificado folha em PEM quando -salvar-cert é
// usado. Confira o SHA-256 com o administrador do servidor ANTES de usar o
// arquivo em SMTP_TLS_CA_FILE (confiar sem conferir aceita um MITM).
func (d *diagnostico) salvarCertificado(cs tls.ConnectionState) {
	if d.opts.salvarCert == "" || len(cs.PeerCertificates) == 0 {
		return
	}
	bloco := &pem.Block{Type: "CERTIFICATE", Bytes: cs.PeerCertificates[0].Raw}
	if err := os.WriteFile(d.opts.salvarCert, pem.EncodeToMemory(bloco), 0o644); err != nil {
		d.logf("ERRO ao salvar o certificado em %s: %v", d.opts.salvarCert, err)
		return
	}
	d.logf("certificado do servidor salvo em %s (confira o SHA-256 acima antes de confiar nele)", d.opts.salvarCert)
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
