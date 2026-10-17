package notificacao

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/rotaperfumes/shared/tz"
)

// Mensagem é o assunto e o corpo (texto puro, linhas com CRLF) de um e-mail.
type Mensagem struct {
	Assunto string
	Corpo   string
}

// assuntoAlertaSeguranca é fixo: nunca contém dados do usuário/requisição.
const assuntoAlertaSeguranca = "Alerta de segurança — RotaPerfumes"

// assuntoSenhaInicial é o assunto do e-mail com a senha gerada.
const assuntoSenhaInicial = "Sua senha de acesso — RotaPerfumes"

// maxUserAgentAlerta é o tamanho máximo (em runes) do User-Agent no alerta.
const maxUserAgentAlerta = 120

// MensagemSenhaInicial é o e-mail com a senha gerada pelo sistema (criação de
// conta ou reset administrativo). A senha só existe aqui e no corpo enviado:
// nunca em log nem em resposta (M-02).
func MensagemSenhaInicial(nomeUsuario, senha string) Mensagem {
	return Mensagem{
		Assunto: assuntoSenhaInicial,
		Corpo: fmt.Sprintf(
			"Olá, %s.\r\n\r\n"+
				"Sua senha de acesso ao sistema RotaPerfumes foi gerada:\r\n\r\n"+
				"    %s\r\n\r\n"+
				"Por segurança, recomendamos trocar essa senha assim que possível após o login.\r\n\r\n"+
				"Se você não solicitou isso, contate o administrador do sistema.\r\n",
			nomeUsuario, senha,
		),
	}
}

// MensagemAlertaReuso é o alerta de reuso de refresh token (SEC-09). Nome, IP
// e User-Agent são saneados (sem caracteres de controle) e o User-Agent é
// truncado. Os identificadores internos (user_id, e-mail do usuário,
// token_id) só vão no e-mail aos administradores.
func MensagemAlertaReuso(nomeUsuario string, a AlertaReuso, paraAdmin bool) Mensagem {
	nome := removerControles(nomeUsuario)
	if nome == "" {
		nome = "usuário"
	}
	ip := removerControles(a.IP)
	ua := truncarRunes(removerControles(a.UserAgent), maxUserAgentAlerta)
	quando := a.Quando.In(tz.Local).Format("02/01/2006 15:04:05") + " (horário de Brasília)"

	var sb strings.Builder
	if paraAdmin {
		sb.WriteString("Alerta de segurança: reuso de sessão já encerrada (possível roubo de token).\r\n\r\n")
	} else {
		sb.WriteString("Olá, " + nome + ".\r\n\r\n")
		sb.WriteString("Detectamos o uso de uma credencial de sessão já substituída na sua conta RotaPerfumes, o que pode indicar acesso indevido.\r\n\r\n")
	}
	sb.WriteString("Data/hora: " + quando + "\r\n")
	sb.WriteString("IP de origem: " + ip + "\r\n")
	sb.WriteString("Navegador/dispositivo: " + ua + "\r\n")
	if paraAdmin {
		sb.WriteString(fmt.Sprintf("Usuário: %s (user_id=%d, e-mail=%s)\r\n", nome, a.UsuarioID, removerControles(a.EmailUsuario)))
		sb.WriteString(fmt.Sprintf("token_id: %d\r\n", a.TokenID))
		sb.WriteString("\r\nTodas as sessões do usuário foram encerradas automaticamente.\r\n")
		return Mensagem{Assunto: assuntoAlertaSeguranca, Corpo: sb.String()}
	}
	sb.WriteString("\r\nPor segurança, encerramos todas as suas sessões; faça login novamente.\r\n")
	sb.WriteString("Se você não reconhece esse acesso, troque a senha e contate o administrador do sistema.\r\n")
	return Mensagem{Assunto: assuntoAlertaSeguranca, Corpo: sb.String()}
}

// removerControles remove caracteres de controle (CR, LF, TAB, NUL etc.) e os
// separadores de linha/parágrafo Unicode, evitando quebra/injeção no corpo.
func removerControles(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return -1
		}
		return r
	}, s)
}

// truncarRunes limita s a max runes (sem cortar caracteres UTF-8 ao meio).
func truncarRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
