package notificacaosmtp

import (
	"errors"
	"fmt"
	"net/smtp"
	"strings"
)

// escolherAuth prefere PLAIN e cai para LOGIN se for o único oferecido.
func escolherAuth(mecanismos, usuario, senha, host string) (smtp.Auth, string, error) {
	oferecidos := strings.Fields(strings.ToUpper(mecanismos))
	switch {
	case contem(oferecidos, "PLAIN"):
		// smtp.PlainAuth recusa por conta própria conexão sem TLS e host divergente.
		return smtp.PlainAuth("", usuario, senha, host), "PLAIN", nil
	case contem(oferecidos, "LOGIN"):
		return &authLogin{usuario: usuario, senha: senha, host: host}, "LOGIN", nil
	default:
		return nil, "", fmt.Errorf("%w (anunciados: %q)", ErrMecanismoAuth, mecanismos)
	}
}

func contem(lista []string, alvo string) bool {
	for _, item := range lista {
		if item == alvo {
			return true
		}
	}
	return false
}

// authLogin implementa AUTH LOGIN (não existe no net/smtp), com as mesmas
// proteções do PlainAuth: só sobre TLS e só para o host esperado.
type authLogin struct {
	usuario, senha, host string
}

func (a *authLogin) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS {
		return "", nil, ErrAuthSemTLS
	}
	if server.Name != a.host {
		return "", nil, errors.New("wrong host name")
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
