// Package services (shared) — política de senha forte.
package services

import (
	"errors"
	"strings"
	"unicode"
)

// Erros de validação de força de senha. Mensagens curtas e sem detalhes
// sensíveis — o handler mapeia diretamente para uma resposta 400.
var (
	// ErrSenhaCurta indica que a senha tem menos que o mínimo de caracteres
	// exigido (contado por rune, não por byte — evita bypass com
	// caracteres multi-byte que "parecem" mais longos que realmente são).
	ErrSenhaCurta = errors.New("senha deve ter pelo menos 8 caracteres")

	// ErrSenhaMuitoLonga indica que a senha excede o limite de bytes do
	// bcrypt. Nunca truncamos a senha silenciosamente — rejeitamos com erro
	// claro para o usuário escolher uma senha mais curta.
	ErrSenhaMuitoLonga = errors.New("senha excede o tamanho máximo permitido (72 bytes)")

	// ErrSenhaFraca indica que a senha não atinge o mínimo de 3 das 4
	// classes de caractere exigidas (minúscula, maiúscula, dígito, símbolo).
	ErrSenhaFraca = errors.New("senha deve conter ao menos 3 dos 4 tipos: letra minúscula, letra maiúscula, dígito e símbolo")
)

const (
	// senhaForteMinLen é o mínimo de caracteres (runes) exigido pela
	// política de senha forte.
	senhaForteMinLen = 8

	// senhaForteMaxBytes é o limite de bytes aceito pelo bcrypt
	// (golang.org/x/crypto/bcrypt trunca silenciosamente acima disso —
	// por isso rejeitamos explicitamente antes de chegar lá).
	senhaForteMaxBytes = 72

	// senhaForteMinClasses é o número mínimo de classes de caractere
	// distintas (dentre minúscula, maiúscula, dígito, símbolo) exigido.
	senhaForteMinClasses = 3
)

// ValidarForcaSenha aplica a política de senha forte do sistema:
//
//   - mínimo de senhaForteMinLen caracteres, contados por rune (não por
//     byte, para não permitir bypass usando caracteres multi-byte);
//   - máximo de senhaForteMaxBytes bytes (limite do bcrypt) — senhas mais
//     longas são rejeitadas explicitamente, nunca truncadas em silêncio;
//   - pelo menos senhaForteMinClasses das 4 classes de caractere:
//     minúscula, maiúscula, dígito, símbolo. O conjunto de símbolos
//     reconhecido é o mesmo usado por GerarSenhaAleatoria (constante
//     simbolos, em password_generator.go), para manter a política
//     consistente entre senha gerada pelo sistema e senha escolhida pelo
//     usuário.
//
// Retorna nil se a senha atende a política, ou um dos erros ErrSenha* com
// uma mensagem curta e sem detalhes sensíveis, apta a ser repassada ao
// cliente numa resposta 400.
func ValidarForcaSenha(senha string) error {
	if len([]byte(senha)) > senhaForteMaxBytes {
		return ErrSenhaMuitoLonga
	}
	if len([]rune(senha)) < senhaForteMinLen {
		return ErrSenhaCurta
	}

	var temMinuscula, temMaiuscula, temDigito, temSimbolo bool
	for _, r := range senha {
		switch {
		case unicode.IsLower(r):
			temMinuscula = true
		case unicode.IsUpper(r):
			temMaiuscula = true
		case unicode.IsDigit(r):
			temDigito = true
		case strings.ContainsRune(simbolos, r):
			temSimbolo = true
		}
	}

	classes := 0
	for _, classeOK := range [...]bool{temMinuscula, temMaiuscula, temDigito, temSimbolo} {
		if classeOK {
			classes++
		}
	}
	if classes < senhaForteMinClasses {
		return ErrSenhaFraca
	}
	return nil
}
