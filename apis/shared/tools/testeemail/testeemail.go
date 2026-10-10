// Package testeemail reúne as funções puras do diagnóstico de envio de e-mail
// (cmd/testeemail): escolha do modo de conexão, validação de endereços e
// montagem da mensagem RFC 5322. A parte de rede fica no main do comando.
package testeemail

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net/mail"
	"strings"
	"time"
)

// Modos de conexão aceitos pela flag -modo.
const (
	ModoAuto     = "auto"
	ModoSSL      = "ssl"
	ModoStartTLS = "starttls"
	ModoPlain    = "plain"
)

// PortaSMTPS é a porta do SMTP com TLS implícito (RFC 8314).
const PortaSMTPS = "465"

// ErrQuebraDeLinha indica CR ou LF em um valor que vai para cabeçalho/envelope.
var ErrQuebraDeLinha = errors.New("valor contém CR/LF (injeção de cabeçalho)")

// EscolherModo resolve o modo efetivo. Em "auto", a porta 465 vira "ssl" e
// qualquer outra vira "starttls". Modos explícitos são devolvidos como estão.
func EscolherModo(modo, porta string) (string, error) {
	modo = strings.ToLower(strings.TrimSpace(modo))
	switch modo {
	case ModoSSL, ModoStartTLS, ModoPlain:
		return modo, nil
	case ModoAuto, "":
		if strings.TrimSpace(porta) == PortaSMTPS {
			return ModoSSL, nil
		}
		return ModoStartTLS, nil
	default:
		return "", fmt.Errorf("modo inválido %q (use auto|ssl|starttls|plain)", modo)
	}
}

// ValidarEndereco rejeita CR/LF e valida o endereço com net/mail, devolvendo
// apenas o addr-spec (ex.: "admin@x.com.br"), usado no envelope e no cabeçalho.
func ValidarEndereco(valor string) (string, error) {
	if strings.ContainsAny(valor, "\r\n") {
		return "", ErrQuebraDeLinha
	}
	valor = strings.TrimSpace(valor)
	if valor == "" {
		return "", errors.New("endereço vazio")
	}
	end, err := mail.ParseAddress(valor)
	if err != nil {
		return "", fmt.Errorf("endereço inválido %q: %w", valor, err)
	}
	return end.Address, nil
}

// Dominio devolve a parte após o último "@" (ou "localhost" se não houver).
func Dominio(endereco string) string {
	i := strings.LastIndex(endereco, "@")
	if i < 0 || i == len(endereco)-1 {
		return "localhost"
	}
	return endereco[i+1:]
}

// GerarMessageID cria um Message-ID único no domínio informado.
func GerarMessageID(agora time.Time, dominio string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("<%d.testeemail@%s>", agora.UnixNano(), dominio)
	}
	return fmt.Sprintf("<%d.%s.testeemail@%s>", agora.UnixNano(), hex.EncodeToString(b), dominio)
}

// Mensagem descreve o e-mail de teste a montar.
type Mensagem struct {
	De        string
	Para      string
	Assunto   string
	Corpo     string
	Data      time.Time
	MessageID string
}

// AssuntoPadrao é o assunto do e-mail de teste.
const AssuntoPadrao = "Teste de envio de e-mail — Rota Perfumes (diagnóstico)"

// CorpoPadrao monta o corpo de teste, identificando data/hora e servidor usado.
func CorpoPadrao(agora time.Time, host, porta, modo, serverName string) string {
	linhas := []string{
		"Olá!",
		"",
		"Esta é uma mensagem de TESTE enviada pelo diagnóstico de e-mail",
		"(apis/shared/cmd/testeemail) do sistema Rota Perfumes.",
		"",
		"Enviada em: " + agora.Format("02/01/2006 15:04:05 -07:00"),
		"Servidor SMTP: " + host + ":" + porta,
		"Modo de conexão: " + modo,
		"ServerName TLS: " + serverName,
		"",
		"Se você recebeu esta mensagem, o envio de e-mail funcionou.",
		"Nenhuma ação é necessária.",
	}
	return strings.Join(linhas, "\r\n") + "\r\n"
}

// MontarMensagem gera a mensagem completa (cabeçalhos + corpo) com CRLF.
// O assunto é codificado em Q-encoding UTF-8 por causa dos acentos.
func MontarMensagem(m Mensagem) ([]byte, error) {
	for nome, valor := range map[string]string{"From": m.De, "To": m.Para, "Message-ID": m.MessageID} {
		if strings.ContainsAny(valor, "\r\n") {
			return nil, fmt.Errorf("cabeçalho %s: %w", nome, ErrQuebraDeLinha)
		}
	}
	if strings.ContainsAny(m.Assunto, "\r\n") {
		return nil, fmt.Errorf("cabeçalho Subject: %w", ErrQuebraDeLinha)
	}
	var sb strings.Builder
	escrever := func(nome, valor string) { sb.WriteString(nome + ": " + valor + "\r\n") }
	escrever("From", m.De)
	escrever("To", m.Para)
	escrever("Subject", mime.QEncoding.Encode("utf-8", m.Assunto))
	escrever("Date", m.Data.Format(time.RFC1123Z))
	escrever("Message-ID", m.MessageID)
	escrever("MIME-Version", "1.0")
	escrever("Content-Type", `text/plain; charset="utf-8"`)
	escrever("Content-Transfer-Encoding", "8bit")
	sb.WriteString("\r\n")
	sb.WriteString(normalizarCRLF(m.Corpo))
	return []byte(sb.String()), nil
}

// normalizarCRLF converte quebras de linha soltas (LF ou CR) em CRLF.
func normalizarCRLF(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.ReplaceAll(s, "\n", "\r\n")
}
