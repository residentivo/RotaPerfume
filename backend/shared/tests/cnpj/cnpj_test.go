package cnpj_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/cnpj"
)

// NEG-02: exemplo oficial da Receita Federal para o CNPJ alfanumérico.
const alfanumericoOficial = "12ABC34501DE35"

// validarEntrada aplica o fluxo completo usado pela API: normalizar e validar.
func validarEntrada(raw string) bool {
	n, ok := cnpj.Normalizar(raw)
	return ok && cnpj.Valido(n)
}

func TestValidarEntrada(t *testing.T) {
	cases := []struct {
		nome string
		in   string
		want bool
	}{
		{"numérico válido", "11222333000181", true},
		{"numérico válido 2", "11444777000161", true},
		{"numérico válido com zeros à esquerda", "00000000000191", true},
		{"numérico válido com máscara", "11.222.333/0001-81", true},
		{"alfanumérico válido (exemplo oficial)", alfanumericoOficial, true},
		{"alfanumérico com máscara", "12.ABC.345/01DE-35", true},
		{"alfanumérico em minúsculas é normalizado", "12abc34501de35", true},
		{"alfanumérico minúsculo com máscara e espaços", " 12.abc.345/01de-35 ", true},
		{"numérico com 1º DV errado", "11222333000191", false},
		{"numérico com 2º DV errado", "11222333000182", false},
		{"alfanumérico com 1º DV errado", "12ABC34501DE45", false},
		{"alfanumérico com 2º DV errado", "12ABC34501DE36", false},
		{"letra na posição do 1º DV", "12ABC34501DEA5", false},
		{"letra na posição do 2º DV", "12ABC34501DE3A", false},
		{"13 caracteres", "12ABC34501DE3", false},
		{"15 caracteres", "12ABC34501DE350", false},
		{"13 dígitos", "1122233300018", false},
		{"vazio", "", false},
		{"todos zeros", "00000000000000", false},
		{"todos iguais numérico", "11.111.111/1111-11", false},
		{"todos iguais letras", "AAAAAAAAAAAAAA", false},
		{"caractere fora do alfabeto", "12ABC34501DE3#", false},
		{"letra acentuada", "12ÁBC34501DE35", false},
		{"fictício do seed (DV inválido)", "12345678000199", false},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			assert.Equal(t, tc.want, validarEntrada(tc.in))
		})
	}
}

func TestNormalizar(t *testing.T) {
	cases := []struct {
		nome, in, want string
		ok             bool
	}{
		{"só dígitos", "11222333000181", "11222333000181", true},
		{"máscara numérica", "11.222.333/0001-81", "11222333000181", true},
		{"espaços", " 11 222 333 0001 81 ", "11222333000181", true},
		{"alfanumérico maiúsculo", "12ABC34501DE35", "12ABC34501DE35", true},
		{"minúsculas viram maiúsculas", "12.abc.345/01de-35", "12ABC34501DE35", true},
		{"outro símbolo é inválido", "11222333000181#", "", false},
		{"letra não ASCII é inválida", "12ÇBC34501DE35", "", false},
		{"só máscara", "../-", "", true},
		{"vazio", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			got, ok := cnpj.Normalizar(tc.in)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFormatoValido(t *testing.T) {
	cases := []struct {
		nome string
		in   string
		want bool
	}{
		{"numérico", "11222333000181", true},
		{"alfanumérico", alfanumericoOficial, true},
		{"DV errado ainda tem formato válido", "12ABC34501DE99", true},
		{"letra no DV", "12ABC34501DEA5", false},
		{"minúscula (não normalizado)", "12abc34501de35", false},
		{"13 caracteres", "12ABC34501DE3", false},
		{"15 caracteres", "12ABC34501DE355", false},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			assert.Equal(t, tc.want, cnpj.FormatoValido(tc.in))
		})
	}
}

func TestDigitosVerificadores(t *testing.T) {
	cases := []struct {
		base     string
		dv1, dv2 int
	}{
		{"12ABC34501DE", 3, 5}, // exemplo oficial da Receita
		{"112223330001", 8, 1},
		{"000000000001", 9, 1},
	}
	for _, tc := range cases {
		t.Run(tc.base, func(t *testing.T) {
			dv1, dv2 := cnpj.DigitosVerificadores(tc.base)
			assert.Equal(t, tc.dv1, dv1)
			assert.Equal(t, tc.dv2, dv2)
		})
	}
}
