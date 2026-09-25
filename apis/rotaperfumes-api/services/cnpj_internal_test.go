package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizarCNPJ(t *testing.T) {
	cases := []struct {
		nome, in, want string
		ok             bool
	}{
		{"só dígitos", "11222333000181", "11222333000181", true},
		{"com máscara", "11.222.333/0001-81", "11222333000181", true},
		{"com espaços", " 11 222 333 0001 81 ", "11222333000181", true},
		{"letra é inválida", "11.222.333/0001-8A", "", false},
		{"outro símbolo é inválido", "11222333000181#", "", false},
		{"vazio", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			got, ok := normalizarCNPJ(tc.in)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestCNPJDigitosValidos(t *testing.T) {
	cases := []struct {
		nome string
		in   string
		want bool
	}{
		{"válido 1", "11222333000181", true},
		{"válido 2", "11444777000161", true},
		{"válido (Banco do Brasil)", "00000000000191", true},
		{"1º DV errado", "11222333000191", false},
		{"2º DV errado", "11222333000182", false},
		{"todos iguais", "11111111111111", false},
		{"zeros", "00000000000000", false},
		{"13 dígitos", "1122233300018", false},
		{"15 dígitos", "112223330001810", false},
		{"fictício do seed", "12345678000199", false},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			assert.Equal(t, tc.want, cnpjDigitosValidos(tc.in))
		})
	}
}
