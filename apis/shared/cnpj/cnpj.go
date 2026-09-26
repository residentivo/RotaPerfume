// Package cnpj centraliza a normalização e a validação de CNPJ — numérico e
// alfanumérico (Receita Federal, vigente desde julho de 2026, NEG-02).
//
// Regra:
//   - Normalização: remove a máscara (".", "/", "-" e espaços) e converte
//     para MAIÚSCULAS. Qualquer outro caractere torna o CNPJ inválido.
//   - Formato: 14 caracteres; as 12 primeiras posições em [0-9A-Z] e as 2
//     últimas (dígitos verificadores) em [0-9].
//   - DV: módulo 11 com os pesos do CNPJ numérico. O valor de cada caractere
//     é ASCII − 48 (0–9 → 0–9, A → 17, …, Z → 42). Resto < 2 → DV 0; senão
//     DV = 11 − resto.
//   - Todos os caracteres iguais (ex.: 00000000000000) é inválido.
//
// CNPJs numéricos continuam válidos: para dígitos, ASCII − 48 é o próprio
// dígito, então o cálculo coincide com o do CNPJ numérico.
package cnpj

import "strings"

// Tamanho é a quantidade de caracteres de um CNPJ normalizado (sem máscara).
const Tamanho = 14

// tamanhoBase é a quantidade de posições antes dos dígitos verificadores.
const tamanhoBase = 12

// Normalizar remove a máscara (".", "/", "-" e espaços) e converte letras
// para maiúsculas. Qualquer caractere fora de [0-9A-Za-z] e da máscara torna
// o valor inválido (ok=false). Não valida tamanho, posições nem DV.
func Normalizar(raw string) (normalizado string, ok bool) {
	var b strings.Builder
	b.Grow(Tamanho)
	for _, r := range raw {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 'a' + 'A')
		case r == '.' || r == '/' || r == '-' || r == ' ':
			// máscara: ignora
		default:
			return "", false
		}
	}
	return b.String(), true
}

// FormatoValido reporta se s (já normalizado) tem 14 caracteres, as 12
// primeiras posições em [0-9A-Z] e as 2 últimas em [0-9]. Não checa o DV.
func FormatoValido(s string) bool {
	if len(s) != Tamanho {
		return false
	}
	for i := 0; i < Tamanho; i++ {
		if !caractereValido(s[i], i < tamanhoBase) {
			return false
		}
	}
	return true
}

// Valido reporta se s (já normalizado) tem formato válido, não é uma
// sequência de caracteres iguais e tem os dois dígitos verificadores corretos.
func Valido(s string) bool {
	if !FormatoValido(s) || todosIguais(s) {
		return false
	}
	dv1, dv2 := DigitosVerificadores(s[:tamanhoBase])
	return int(s[12]-'0') == dv1 && int(s[13]-'0') == dv2
}

// DigitosVerificadores calcula os dois DVs de uma base de 12 posições
// [0-9A-Z] (sem máscara, maiúscula). O chamador garante o formato da base.
func DigitosVerificadores(base string) (dv1, dv2 int) {
	dv1 = digitoVerificador(base)
	dv2 = digitoVerificador(base + string(rune('0'+dv1)))
	return dv1, dv2
}

// digitoVerificador aplica o módulo 11 sobre base (12 posições para o 1º DV,
// 13 para o 2º). Os pesos vão de 2 a 9, da direita para a esquerda,
// reiniciando em 2 — o que equivale a 5,4,3,2,9,8,7,6,5,4,3,2 (1º DV) e
// 6,5,4,3,2,9,8,7,6,5,4,3,2 (2º DV) da esquerda para a direita.
func digitoVerificador(base string) int {
	soma, peso := 0, 2
	for i := len(base) - 1; i >= 0; i-- {
		soma += valorCaractere(base[i]) * peso
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

// valorCaractere devolve ASCII − 48 (0–9 → 0–9, A → 17, …, Z → 42).
func valorCaractere(c byte) int {
	return int(c) - '0'
}

func caractereValido(c byte, aceitaLetra bool) bool {
	if c >= '0' && c <= '9' {
		return true
	}
	return aceitaLetra && c >= 'A' && c <= 'Z'
}

func todosIguais(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}
