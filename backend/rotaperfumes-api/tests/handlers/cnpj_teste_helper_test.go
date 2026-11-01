package handlers_test

import (
	"fmt"
	"testing"
)

func TestCNPJValidoTesteHelper(t *testing.T) {
	if got := cnpjValidoTeste(112223330001); got != "11222333000181" {
		t.Fatalf("cnpjValidoTeste = %s", got)
	}
	if got := cnpjValidoTeste(1); got != "00000000000191" {
		t.Fatalf("cnpjValidoTeste = %s", got)
	}
}

// cnpjValidoTeste completa uma base de 12 dígitos com os dois dígitos
// verificadores (módulo 11), gerando um CNPJ aceito pela API (NEG-01).
// Usado pelos testes de integração, que criam clientes reais no banco.
func cnpjValidoTeste(base int64) string {
	s := fmt.Sprintf("%012d", base%1e12)
	s += fmt.Sprint(dvCNPJTeste(s))
	s += fmt.Sprint(dvCNPJTeste(s))
	return s
}

func dvCNPJTeste(base string) int {
	soma, peso := 0, 2
	for i := len(base) - 1; i >= 0; i-- {
		soma += int(base[i]-'0') * peso
		peso++
		if peso > 9 {
			peso = 2
		}
	}
	if r := soma % 11; r >= 2 {
		return 11 - r
	}
	return 0
}
