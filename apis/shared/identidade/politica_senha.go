package identidade

import (
	"strings"
	"unicode"
)

// Política de senha forte (I-12), inalterada desde o legado.
const (
	// SenhaMinRunes é o mínimo de caracteres, contados por rune (um caractere
	// multi-byte não "vale" mais de um).
	SenhaMinRunes = 8
	// SenhaMaxBytes é o teto em bytes (herdado do bcrypt; mantido para não
	// mudar a política nem a mensagem do frontend). Nunca trunca: rejeita.
	SenhaMaxBytes = 72
	// SenhaMinClasses de 4 (minúscula, maiúscula, dígito, símbolo).
	SenhaMinClasses = 3
	// SimbolosSenha é o conjunto de símbolos da política e do gerador de
	// senha aleatória (o mesmo nos dois lados).
	SimbolosSenha = "!@#$%&*+-=?"
)

// ValidarForca aplica a política de senha forte e devolve ErrSenhaMuitoLonga,
// ErrSenhaCurta ou ErrSenhaFraca (mensagens fixas, sem detalhes).
func ValidarForca(s Senha) error {
	texto := s.Revelar()
	if len(texto) > SenhaMaxBytes {
		return ErrSenhaMuitoLonga
	}
	if len([]rune(texto)) < SenhaMinRunes {
		return ErrSenhaCurta
	}
	if classesDe(texto) < SenhaMinClasses {
		return ErrSenhaFraca
	}
	return nil
}

// classesDe conta as classes de caractere presentes.
func classesDe(texto string) int {
	var minuscula, maiuscula, digito, simbolo bool
	for _, r := range texto {
		switch {
		case unicode.IsLower(r):
			minuscula = true
		case unicode.IsUpper(r):
			maiuscula = true
		case unicode.IsDigit(r):
			digito = true
		case strings.ContainsRune(SimbolosSenha, r):
			simbolo = true
		}
	}
	total := 0
	for _, presente := range [...]bool{minuscula, maiuscula, digito, simbolo} {
		if presente {
			total++
		}
	}
	return total
}
