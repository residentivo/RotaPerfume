package catalogo

import (
	"strings"
	"time"

	"github.com/rotaperfumes/shared/kernel"
)

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

// dataLocal converte AAAA-MM-DD na meia-noite em time.Local (o fuso do DSN
// loc=Local, fixado em -03:00 por shared/tz): o mesmo instante que o
// time.ParseInLocation legado produzia (BUG-01). Qualquer falha devolve
// errCampo.
func dataLocal(s string, errCampo error) (time.Time, error) {
	d, err := kernel.ParseData(s, errCampo)
	if err != nil {
		return time.Time{}, err
	}
	return d.MeiaNoiteEm(time.Local), nil
}
