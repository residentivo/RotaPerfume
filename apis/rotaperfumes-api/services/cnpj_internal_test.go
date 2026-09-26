package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// NEG-02: validação de CNPJ numérico e alfanumérico na camada de serviço
// (normalizarCNPJ + cnpjDigitosValidos = fluxo de Create/Update).
func TestValidacaoCNPJ(t *testing.T) {
	cases := []struct {
		nome, in, normalizado string
		valido                bool
	}{
		{"numérico válido", "11222333000181", "11222333000181", true},
		{"numérico válido (Banco do Brasil)", "00000000000191", "00000000000191", true},
		{"numérico com máscara", "11.222.333/0001-81", "11222333000181", true},
		{"numérico com espaços", " 11 222 333 0001 81 ", "11222333000181", true},
		{"alfanumérico válido (exemplo oficial da Receita)", "12ABC34501DE35", "12ABC34501DE35", true},
		{"alfanumérico com máscara", "12.ABC.345/01DE-35", "12ABC34501DE35", true},
		{"minúsculas normalizadas", "12abc34501de35", "12ABC34501DE35", true},
		{"minúsculas com máscara", "12.abc.345/01de-35", "12ABC34501DE35", true},
		{"numérico com DV errado", "11222333000182", "11222333000182", false},
		{"alfanumérico com 1º DV errado", "12ABC34501DE45", "12ABC34501DE45", false},
		{"alfanumérico com 2º DV errado", "12ABC34501DE36", "12ABC34501DE36", false},
		{"fictício do seed (DV errado)", "12345678000199", "12345678000199", false},
		{"letra na posição do 1º DV", "12ABC34501DEA5", "", false},
		{"letra na posição do 2º DV", "11.222.333/0001-8A", "", false},
		{"13 caracteres", "12ABC34501DE3", "", false},
		{"15 caracteres", "112223330001810", "", false},
		{"vazio", "", "", false},
		{"só máscara", "../-", "", false},
		{"símbolo inválido", "11222333000181#", "", false},
		{"todos iguais (zeros)", "00000000000000", "00000000000000", false},
		{"todos iguais com máscara", "11.111.111/1111-11", "11111111111111", false},
		{"todos iguais letras", "AAAAAAAAAAAAAA", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			got, ok := normalizarCNPJ(tc.in)
			assert.Equal(t, tc.normalizado, got)
			assert.Equal(t, tc.normalizado != "", ok)
			assert.Equal(t, tc.valido, ok && cnpjDigitosValidos(got))
		})
	}
}

func TestTermoBuscaCNPJ(t *testing.T) {
	cases := []struct {
		nome, q, want string
	}{
		{"sem máscara usa o próprio q", "Perfumaria", ""},
		{"dígitos sem máscara usa o próprio q", "11222333", ""},
		{"máscara numérica", "11.222.333/0001-81", "11222333000181"},
		{"prefixo mascarado", "11.222", "11222"},
		{"alfanumérico minúsculo com máscara", "12.abc.345", "12ABC345"},
		{"razão social com barra e espaço", "Perfumes S/A", "PERFUMESSA"},
		{"caractere que não é de CNPJ", "Ótica S/A", ""},
		{"só máscara", "./-", ""},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			assert.Equal(t, tc.want, termoBuscaCNPJ(tc.q))
		})
	}
}
