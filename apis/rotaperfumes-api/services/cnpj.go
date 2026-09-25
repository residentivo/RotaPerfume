package services

import (
	"errors"
	"strings"
)

// cnpjTamanho é a quantidade de dígitos de um CNPJ (sem máscara).
const cnpjTamanho = 14

// ErrCNPJInvalido indica CNPJ fora do formato (≠ 14 dígitos, caracteres
// inválidos, todos os dígitos iguais) ou com dígito verificador inválido.
var ErrCNPJInvalido = errors.New("cnpj inválido")

// ErrCNPJDuplicado indica que já existe outro cliente com o mesmo CNPJ
// (índice UNIQUE uq_clientes_cnpj). A mensagem é genérica de propósito: não
// revela id, vendedor nem razão social do cliente existente.
var ErrCNPJDuplicado = errors.New("cnpj já cadastrado")

// normalizarCNPJ remove a máscara (".", "/", "-" e espaços) e devolve só os
// dígitos. Qualquer outro caractere torna o CNPJ inválido (ok=false).
// Não valida tamanho nem dígito verificador.
func normalizarCNPJ(cnpj string) (digitos string, ok bool) {
	var b strings.Builder
	b.Grow(cnpjTamanho)
	for _, r := range cnpj {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '/' || r == '-' || r == ' ':
			// máscara: ignora
		default:
			return "", false
		}
	}
	return b.String(), true
}

// cnpjFormatoValido reporta se digitos tem exatamente 14 dígitos.
func cnpjFormatoValido(digitos string) bool {
	return len(digitos) == cnpjTamanho
}

// cnpjDigitosValidos reporta se o CNPJ (14 dígitos, sem máscara) não é uma
// sequência de dígitos iguais e tem os dois dígitos verificadores corretos
// (módulo 11).
func cnpjDigitosValidos(digitos string) bool {
	if !cnpjFormatoValido(digitos) || todosIguais(digitos) {
		return false
	}
	dv1 := cnpjDigitoVerificador(digitos[:12])
	dv2 := cnpjDigitoVerificador(digitos[:12] + string(rune('0'+dv1)))
	return int(digitos[12]-'0') == dv1 && int(digitos[13]-'0') == dv2
}

// cnpjDigitoVerificador calcula um dígito verificador do CNPJ pelo módulo 11
// sobre base (12 dígitos para o 1º DV, 13 para o 2º). Os pesos vão de 2 a 9,
// da direita para a esquerda, reiniciando em 2.
func cnpjDigitoVerificador(base string) int {
	soma, peso := 0, 2
	for i := len(base) - 1; i >= 0; i-- {
		soma += int(base[i]-'0') * peso
		peso++
		if peso > 9 {
			peso = 2
		}
	}
	resto := soma % 11
	if resto < 2 {
		return 0
	}
	return 11 - resto
}

func todosIguais(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}
