package vendas_test

// SEC-14 (Lote 27): o pedido tem no máximo vendas.MaxItensPedido (200)
// itens, na criação e no PUT (os dois passam por vendas.NovoPedido). 201 é
// ErrItensDemais (400), checado antes da validação item a item.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
)

func sec14Itens(n int) []vendas.DadosItem {
	itens := make([]vendas.DadosItem, n)
	for i := range itens {
		itens[i] = vendas.DadosItem{ProdutoID: int64(i + 1), Quantidade: 1, PrecoPraticado: 1}
	}
	return itens
}

func TestSEC14_NovoPedido_MaxItens(t *testing.T) {
	assert.Equal(t, 200, vendas.MaxItensPedido)

	d := pedidoValido()
	d.Itens = sec14Itens(vendas.MaxItensPedido)
	p, err := vendas.NovoPedido(d)
	require.NoError(t, err, "200 itens é o teto aceito")
	assert.Len(t, p.Itens, 200)
	assert.Equal(t, 200.0, p.ValorTotal)

	d.Itens = sec14Itens(vendas.MaxItensPedido + 1)
	p, err = vendas.NovoPedido(d)
	assert.Nil(t, p)
	assert.ErrorIs(t, err, vendas.ErrItensDemais)
	assert.Equal(t, "o pedido deve ter no máximo 200 itens", err.Error())
	assert.Equal(t, kernel.KindInvalido, vendas.ErrItensDemais.Kind(), "400")
}

func TestSEC14_NovoPedido_ItensDemaisAntesDoItemInvalido(t *testing.T) {
	d := pedidoValido()
	d.Itens = sec14Itens(vendas.MaxItensPedido + 1)
	d.Itens[0].ProdutoID = 0
	_, err := vendas.NovoPedido(d)
	assert.ErrorIs(t, err, vendas.ErrItensDemais, "a contagem vem logo depois do len == 0")
}
