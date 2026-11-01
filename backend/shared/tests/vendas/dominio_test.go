package vendas_test

// ARQ-05: domínio de Vendas (puro). Validação na ordem do legado, cálculo
// de valor_bruto/valor_total no backend (V-03), travas da exclusão (V-01),
// regras do PUT de pedido faturado e N-9 (estouro do float64).

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
)

func pedidoValido() vendas.DadosPedido {
	return vendas.DadosPedido{ClienteID: 1, VendedorID: 2, DataPedido: " 2024-01-15 ", Canal: " App ", Status: " Faturado ",
		Itens: []vendas.DadosItem{
			{ProdutoID: 10, Quantidade: 2, PrecoPraticado: 100, DescontoPct: 10},
			{ProdutoID: 11, Quantidade: 1, PrecoPraticado: 50},
		}}
}

func TestVendas_NovoPedido_CalculaValoresNoBackend(t *testing.T) {
	p, err := vendas.NovoPedido(pedidoValido())
	require.NoError(t, err)
	assert.Equal(t, "App", p.Canal)
	assert.Equal(t, "Faturado", p.Status)
	assert.Equal(t, time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local), p.DataPedido, "meia-noite local (BUG-01)")
	require.Len(t, p.Itens, 2)
	assert.Equal(t, 180.0, p.Itens[0].ValorBruto, "2 * 100 * (1 - 10/100)")
	assert.Equal(t, 50.0, p.Itens[1].ValorBruto)
	assert.Equal(t, 230.0, p.ValorTotal, "V-03: soma calculada, nunca do payload")
	assert.Zero(t, p.PedidoIDOrigem)
}

func TestVendas_NovoPedido_ValidacaoNaOrdemDoLegado(t *testing.T) {
	casos := []struct {
		nome  string
		mudar func(d *vendas.DadosPedido)
		want  error
	}{
		{"cliente antes de tudo", func(d *vendas.DadosPedido) { d.ClienteID = 0; d.VendedorID = 0 }, vendas.ErrClienteIDObrigatorio},
		{"vendedor", func(d *vendas.DadosPedido) { d.VendedorID = -1; d.DataPedido = "" }, vendas.ErrVendedorIDObrigatorio},
		{"data vazia", func(d *vendas.DadosPedido) { d.DataPedido = "  "; d.Canal = "x" }, vendas.ErrDataPedidoInvalida},
		{"data inválida", func(d *vendas.DadosPedido) { d.DataPedido = "15/01/2024" }, vendas.ErrDataPedidoInvalida},
		{"canal", func(d *vendas.DadosPedido) { d.Canal = "Email"; d.Status = "x" }, vendas.ErrCanalInvalido},
		{"status", func(d *vendas.DadosPedido) { d.Status = "Pendente"; d.Itens = nil }, vendas.ErrStatusInvalido},
		{"sem itens", func(d *vendas.DadosPedido) { d.Itens = nil }, vendas.ErrItensObrigatorios},
		{"produto", func(d *vendas.DadosPedido) { d.Itens[1].ProdutoID = 0; d.Itens[1].Quantidade = 0 }, vendas.ErrProdutoIDObrigatorio},
		{"quantidade", func(d *vendas.DadosPedido) { d.Itens[0].Quantidade = 0; d.Itens[0].PrecoPraticado = -1 }, vendas.ErrQuantidadeInvalida},
		{"preço", func(d *vendas.DadosPedido) { d.Itens[0].PrecoPraticado = -0.01; d.Itens[0].DescontoPct = 101 }, vendas.ErrPrecoPraticadoInvalido},
		{"desconto negativo", func(d *vendas.DadosPedido) { d.Itens[0].DescontoPct = -1 }, vendas.ErrDescontoPctInvalido},
		{"desconto acima de 100", func(d *vendas.DadosPedido) { d.Itens[1].DescontoPct = 100.5 }, vendas.ErrDescontoPctInvalido},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d := pedidoValido()
			c.mudar(&d)
			p, err := vendas.NovoPedido(d)
			assert.Nil(t, p)
			assert.ErrorIs(t, err, c.want)
		})
	}
}

// N-9 (ARQ-01, revalidado no ARQ-05): Dinheiro.Somar não checa estouro; no
// uso real (valor_total) o domínio recusa o +Inf/NaN com 500 "erro interno",
// o mesmo status que o banco (DECIMAL(15,2)) dava ao legado.
func TestVendas_N9_ValorForaDoLimite(t *testing.T) {
	casos := map[string][]vendas.DadosItem{
		"valor_bruto estoura (+Inf)":        {{ProdutoID: 1, Quantidade: 2, PrecoPraticado: math.MaxFloat64}},
		"desconto de 100% sobre +Inf (NaN)": {{ProdutoID: 1, Quantidade: 2, PrecoPraticado: math.MaxFloat64, DescontoPct: 100}},
		"soma estoura (+Inf)": {
			{ProdutoID: 1, Quantidade: 1, PrecoPraticado: math.MaxFloat64},
			{ProdutoID: 2, Quantidade: 1, PrecoPraticado: math.MaxFloat64},
		},
	}
	for nome, itens := range casos {
		t.Run(nome, func(t *testing.T) {
			d := pedidoValido()
			d.Itens = itens
			p, err := vendas.NovoPedido(d)
			assert.Nil(t, p)
			assert.ErrorIs(t, err, vendas.ErrValorForaDoLimite)
			var e *kernel.Erro
			require.ErrorAs(t, err, &e)
			assert.Equal(t, kernel.KindInterno, e.Kind(), "500 como no legado")
		})
	}
}

func TestVendas_DescontoPct(t *testing.T) {
	for _, v := range []float64{0, 37.5, 100} {
		d, err := vendas.NovoDescontoPct(v)
		require.NoError(t, err)
		assert.Equal(t, v, d.Float64())
	}
	for _, v := range []float64{-0.01, 100.01} {
		_, err := vendas.NovoDescontoPct(v)
		assert.ErrorIs(t, err, vendas.ErrDescontoPctInvalido)
	}
}

// V-01: com pagamentos vinculados, 409 (antes de olhar o status); Faturado
// ou Entregue (BUG-16), 409; os demais status podem ser excluídos.
func TestVendas_PedidoPodeExcluir(t *testing.T) {
	assert.ErrorIs(t, (&vendas.Pedido{Status: "Faturado"}).PodeExcluir(true), vendas.ErrPedidoPossuiPagamentosVinculados)
	assert.ErrorIs(t, (&vendas.Pedido{Status: "Faturado"}).PodeExcluir(false), vendas.ErrPedidoFaturadoNaoPodeSerExcluido)
	assert.ErrorIs(t, (&vendas.Pedido{Status: "Entregue"}).PodeExcluir(false), vendas.ErrPedidoEntregueNaoPodeSerExcluido)
	for _, s := range []string{"Cancelado", "Em separação", "Pendente"} {
		assert.NoError(t, (&vendas.Pedido{Status: s}).PodeExcluir(false), s)
	}
}

func TestVendas_PlanejarAtualizacao(t *testing.T) {
	itens := []vendas.ItemPedido{{ProdutoID: 2, Quantidade: 4, PrecoPraticado: 50}}
	novo := func(status string, it []vendas.ItemPedido) *vendas.Pedido {
		return &vendas.Pedido{Status: status, Itens: it}
	}

	_, err := vendas.PlanejarAtualizacao("Faturado", itens, novo("Entregue", []vendas.ItemPedido{{ProdutoID: 2, Quantidade: 5, PrecoPraticado: 50}}))
	assert.ErrorIs(t, err, vendas.ErrPedidoJaFaturadoNaoPodeAlterarItens)

	plano, err := vendas.PlanejarAtualizacao("Faturado", itens, novo("Faturado", itens))
	require.NoError(t, err)
	assert.Equal(t, vendas.PlanoAtualizacao{}, plano, "reenvio idempotente: sem reescrita nem nova baixa")

	plano, err = vendas.PlanejarAtualizacao("Pendente", nil, novo("Faturado", itens))
	require.NoError(t, err)
	assert.Equal(t, vendas.PlanoAtualizacao{ReescreverItens: true, BaixarEstoque: true}, plano, "transição para Faturado")

	_, err = vendas.PlanejarAtualizacao("Em separação", itens, novo("Entregue", nil))
	assert.ErrorIs(t, err, vendas.ErrPedidoEntregueSemFaturamento, "Entregue só a partir de Faturado (BUG-16/BUG-20)")

	plano, err = vendas.PlanejarAtualizacao("Em separação", itens, novo("Cancelado", nil))
	require.NoError(t, err)
	assert.Equal(t, vendas.PlanoAtualizacao{ReescreverItens: true}, plano)
}

func TestVendas_MesmosItens(t *testing.T) {
	base := vendas.ItemPedido{ProdutoID: 1, Quantidade: 2, PrecoPraticado: 3, DescontoPct: 4, ItemIDOrigem: 9}
	assert.True(t, vendas.MesmosItens(nil, []vendas.ItemPedido{}))
	assert.True(t, vendas.MesmosItens([]vendas.ItemPedido{base}, []vendas.ItemPedido{{ProdutoID: 1, Quantidade: 2, PrecoPraticado: 3, DescontoPct: 4}}),
		"ids e timestamps não contam")
	assert.False(t, vendas.MesmosItens([]vendas.ItemPedido{base}, nil))
	mudancas := []func(*vendas.ItemPedido){
		func(i *vendas.ItemPedido) { i.ProdutoID = 9 },
		func(i *vendas.ItemPedido) { i.Quantidade = 9 },
		func(i *vendas.ItemPedido) { i.PrecoPraticado = 9 },
		func(i *vendas.ItemPedido) { i.DescontoPct = 9 },
	}
	for _, mudar := range mudancas {
		outro := base
		mudar(&outro)
		assert.False(t, vendas.MesmosItens([]vendas.ItemPedido{base}, []vendas.ItemPedido{outro}))
	}
}

func TestVendas_Enums(t *testing.T) {
	for _, s := range []string{"App", "Telefone", "Visita", " WhatsApp "} {
		_, err := vendas.NovoCanal(s)
		assert.NoError(t, err, s)
	}
	for _, s := range []string{"Cancelado", "Em separação", "Entregue", "Faturado"} {
		_, err := vendas.NovoStatusPedido(s)
		assert.NoError(t, err, s)
	}
	for _, s := range []string{"Boleto 14 dias", "Boleto 28 dias", "Cartão de crédito", "Cartão de débito", "Cheque a prazo", "Dinheiro", " PIX "} {
		f, err := vendas.NovaFormaPagamento(s)
		assert.NoError(t, err, s)
		assert.NotEqual(t, ' ', rune(string(f)[0]))
	}
	quitados := map[string]bool{"Em aberto": false, "Inadimplente": false, "Pago": true, "Pago com atraso": true}
	for s, quitado := range quitados {
		st, err := vendas.NovoStatusPagamento(s)
		require.NoError(t, err, s)
		assert.Equal(t, quitado, st.Quitado(), s)
		err = (&vendas.Pagamento{StatusPagamento: s}).PodeExcluir()
		if quitado {
			assert.ErrorIs(t, err, vendas.ErrPagamentoJaQuitadoNaoPodeSerExcluido, s)
		} else {
			assert.NoError(t, err, s)
		}
	}
	_, err := vendas.NovaFormaPagamento("Bitcoin")
	assert.ErrorIs(t, err, vendas.ErrFormaPagamentoInvalida)
	_, err = vendas.NovoStatusPagamento("Cancelado")
	assert.ErrorIs(t, err, vendas.ErrStatusPagamentoInvalido)
}

func pagamentoValido() vendas.DadosPagamento {
	return vendas.DadosPagamento{FormaPagamento: " PIX ", Parcelas: 2, Valor: 100, TaxaPct: 1.5, ValorLiquido: 98.5,
		DataVencimento: "2024-02-10", DataPagamento: " 2024-02-11 ", StatusPagamento: " Pago "}
}

func TestVendas_DadosPagamento(t *testing.T) {
	p, err := pagamentoValido().Pagamento(7)
	require.NoError(t, err)
	assert.Equal(t, int64(7), p.PedidoID)
	assert.Equal(t, "PIX", p.FormaPagamento)
	assert.Equal(t, "Pago", p.StatusPagamento)
	assert.Equal(t, time.Date(2024, 2, 10, 0, 0, 0, 0, time.Local), p.DataVencimento)
	require.NotNil(t, p.DataPagamento)
	assert.Equal(t, time.Date(2024, 2, 11, 0, 0, 0, 0, time.Local), *p.DataPagamento)
	assert.Equal(t, 98.5, p.ValorLiquido, "valor_liquido explícito, não calculado")

	sem := pagamentoValido()
	sem.DataPagamento = "  "
	p, err = sem.Pagamento(7)
	require.NoError(t, err)
	assert.Nil(t, p.DataPagamento, "vazio = NULL")
}

func TestVendas_DadosPagamento_ValidacaoNaOrdemDoLegado(t *testing.T) {
	casos := []struct {
		nome  string
		mudar func(d *vendas.DadosPagamento)
		want  error
	}{
		{"forma antes de tudo", func(d *vendas.DadosPagamento) { d.FormaPagamento = "x"; d.StatusPagamento = "x" }, vendas.ErrFormaPagamentoInvalida},
		{"status", func(d *vendas.DadosPagamento) { d.StatusPagamento = "x"; d.Parcelas = 0 }, vendas.ErrStatusPagamentoInvalido},
		{"parcelas", func(d *vendas.DadosPagamento) { d.Parcelas = 0; d.Valor = -1 }, vendas.ErrParcelasInvalidas},
		{"valor", func(d *vendas.DadosPagamento) { d.Valor = -1; d.TaxaPct = -1 }, vendas.ErrValorInvalido},
		{"taxa", func(d *vendas.DadosPagamento) { d.TaxaPct = -1; d.ValorLiquido = -1 }, vendas.ErrTaxaPctInvalida},
		{"valor líquido", func(d *vendas.DadosPagamento) { d.ValorLiquido = -1; d.DataVencimento = "" }, vendas.ErrValorLiquidoInvalido},
		{"vencimento ausente", func(d *vendas.DadosPagamento) { d.DataVencimento = " "; d.DataPagamento = "x" }, vendas.ErrDataVencimentoObrigatoria},
		{"vencimento inválido", func(d *vendas.DadosPagamento) { d.DataVencimento = "10/02/2024"; d.DataPagamento = "x" }, vendas.ErrDataVencimentoInvalida},
		{"pagamento inválido", func(d *vendas.DadosPagamento) { d.DataPagamento = "11/02/2024" }, vendas.ErrDataPagamentoInvalida},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d := pagamentoValido()
			c.mudar(&d)
			p, err := d.Pagamento(1)
			assert.Nil(t, p)
			assert.ErrorIs(t, err, c.want)
		})
	}
}

func TestVendas_Ordenacao(t *testing.T) {
	assert.Equal(t, vendas.OrdemPedidoID, vendas.OrdenacaoPedidoPadrao().Campo())
	assert.Equal(t, kernel.Desc, vendas.OrdenacaoPedidoPadrao().Direcao())
	assert.Len(t, vendas.CamposOrdenacaoPedido(), 9)
	assert.Equal(t, vendas.OrdemPagamentoID, vendas.OrdenacaoPagamentoPadrao().Campo())
	assert.Equal(t, kernel.Asc, vendas.OrdenacaoPagamentoPadrao().Direcao())
	assert.Len(t, vendas.CamposOrdenacaoPagamento(), 12)
}
