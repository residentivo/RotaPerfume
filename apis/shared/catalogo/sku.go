package catalogo

import "strings"

// SKU é o código do produto: obrigatório, sem espaços nas pontas, imutável
// depois de criado (o PUT de produto não o altera). É a chave de negócio que
// liga Produto e RegistroEstoque.
type SKU string

// NovoSKU apara os espaços e exige valor (ErrSKUObrigatorio).
func NovoSKU(s string) (SKU, error) {
	v := strings.TrimSpace(s)
	if v == "" {
		return "", ErrSKUObrigatorio
	}
	return SKU(v), nil
}

// String devolve o código.
func (s SKU) String() string { return string(s) }
