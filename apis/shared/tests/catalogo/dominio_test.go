package catalogo_test

// ARQ-02: regras do domínio Catálogo (puro, sem banco): fábricas com
// validação, VOs e query objects. Casos parametrizados; a ordem das
// validações é a do legado (produto_service.go/estoque_service.go).

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/kernel"
)

func dadosProdutoValidos() catalogo.DadosProduto {
	return catalogo.DadosProduto{
		SKU: " SKU-1 ", Descricao: " Perfume ", Categoria: "EDP", Marca: "M", NotaOlfativa: " Cítrico ",
		PrecoTabela: 10, CustoUnitario: 4, Unidade: "UN", DataLancamento: "2024-03-10",
	}
}

func TestNovoSKU(t *testing.T) {
	casos := []struct {
		in   string
		want catalogo.SKU
		err  error
	}{
		{"SKU-1", "SKU-1", nil},
		{"  SKU-2\t", "SKU-2", nil},
		{"", "", catalogo.ErrSKUObrigatorio},
		{"   ", "", catalogo.ErrSKUObrigatorio},
	}
	for _, c := range casos {
		t.Run(c.in, func(t *testing.T) {
			got, err := catalogo.NovoSKU(c.in)
			assert.ErrorIs(t, err, c.err)
			assert.Equal(t, c.want, got)
			assert.Equal(t, string(c.want), got.String())
		})
	}
}

func TestNovoProduto_Validacoes(t *testing.T) {
	casos := []struct {
		nome  string
		muda  func(*catalogo.DadosProduto)
		err   error
		campo string // mensagem pública esperada
	}{
		{"sku vazio", func(d *catalogo.DadosProduto) { d.SKU = " " }, catalogo.ErrSKUObrigatorio, "sku é obrigatório"},
		{"sku antes da descrição", func(d *catalogo.DadosProduto) { d.SKU, d.Descricao = "", "" }, catalogo.ErrSKUObrigatorio, "sku é obrigatório"},
		{"descrição vazia", func(d *catalogo.DadosProduto) { d.Descricao = "" }, catalogo.ErrDescricaoObrigatoria, "descrição é obrigatória"},
		{"categoria vazia", func(d *catalogo.DadosProduto) { d.Categoria = "  " }, catalogo.ErrCategoriaObrigatoria, "categoria é obrigatória"},
		{"marca vazia", func(d *catalogo.DadosProduto) { d.Marca = "" }, catalogo.ErrMarcaObrigatoria, "marca é obrigatória"},
		{"unidade vazia", func(d *catalogo.DadosProduto) { d.Unidade = "" }, catalogo.ErrUnidadeObrigatoria, "unidade é obrigatória"},
		{"preço negativo", func(d *catalogo.DadosProduto) { d.PrecoTabela = -0.01 }, catalogo.ErrPrecoTabelaInvalido, "preco_tabela deve ser maior ou igual a zero"},
		{"custo negativo", func(d *catalogo.DadosProduto) { d.CustoUnitario = -1 }, catalogo.ErrCustoUnitarioInvalido, "custo_unitario deve ser maior ou igual a zero"},
		{"preço antes do custo", func(d *catalogo.DadosProduto) { d.PrecoTabela, d.CustoUnitario = -1, -1 }, catalogo.ErrPrecoTabelaInvalido, "preco_tabela deve ser maior ou igual a zero"},
		{"data malformada", func(d *catalogo.DadosProduto) { d.DataLancamento = "10/03/2024" }, catalogo.ErrDataLancamentoInvalida, "data_lancamento inválida (use o formato AAAA-MM-DD)"},
		{"data inexistente", func(d *catalogo.DadosProduto) { d.DataLancamento = "2024-02-30" }, catalogo.ErrDataLancamentoInvalida, "data_lancamento inválida (use o formato AAAA-MM-DD)"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d := dadosProdutoValidos()
			c.muda(&d)
			p, err := catalogo.NovoProduto(d)
			assert.Nil(t, p)
			require.ErrorIs(t, err, c.err)
			assert.Equal(t, c.campo, err.Error())
			var de *kernel.Erro
			require.ErrorAs(t, err, &de)
			assert.Equal(t, kernel.KindInvalido, de.Kind(), "validação é 400")
		})
	}
}

func TestNovoProduto_Sucesso(t *testing.T) {
	p, err := catalogo.NovoProduto(dadosProdutoValidos())
	require.NoError(t, err)
	assert.Equal(t, catalogo.SKU("SKU-1"), p.SKU)
	assert.Equal(t, "Perfume", p.Descricao, "campos aparados")
	assert.Equal(t, "Cítrico", p.NotaOlfativa)
	assert.True(t, p.Ativo, "todo produto nasce ativo")
	require.NotNil(t, p.DataLancamento)
	assert.True(t, p.DataLancamento.Equal(time.Date(2024, 3, 10, 0, 0, 0, 0, time.Local)), "meia-noite local (BUG-01)")
	assert.Zero(t, p.ID)

	d := dadosProdutoValidos()
	d.DataLancamento, d.PrecoTabela, d.CustoUnitario = "  ", 0, 0
	p, err = catalogo.NovoProduto(d)
	require.NoError(t, err)
	assert.Nil(t, p.DataLancamento, "data em branco é opcional")
}

func TestNovosCamposProduto_IgnoraSKU(t *testing.T) {
	d := dadosProdutoValidos()
	d.SKU = ""
	c, err := catalogo.NovosCamposProduto(d)
	require.NoError(t, err, "na edição o SKU não é validado nem alterado")
	assert.Equal(t, "Perfume", c.Descricao)
}

func TestProduto_DefinirAtivo(t *testing.T) {
	sim, nao := true, false
	casos := []struct {
		nome     string
		inicial  bool
		ativo    *bool
		esperado bool
	}{
		{"toggle de ativo", true, nil, false},
		{"toggle de inativo", false, nil, true},
		{"define true", false, &sim, true},
		{"define false", true, &nao, false},
		{"idempotente", true, &sim, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p := &catalogo.Produto{Ativo: c.inicial}
			p.DefinirAtivo(c.ativo)
			assert.Equal(t, c.esperado, p.Ativo)
		})
	}
}

func TestNovoRegistroEstoque(t *testing.T) {
	agora := time.Date(2024, 6, 10, 15, 0, 0, 0, time.Local)
	casos := []struct {
		nome    string
		d       catalogo.DadosEstoque
		err     error
		ruptura bool
	}{
		{"válido", catalogo.DadosEstoque{SKU: "S", DataSnapshot: "2024-06-01", Saldo: 3}, nil, false},
		{"hoje não é futuro", catalogo.DadosEstoque{SKU: "S", DataSnapshot: "2024-06-10", Saldo: 3}, nil, false},
		{"saldo zero é ruptura", catalogo.DadosEstoque{SKU: "S", DataSnapshot: "2024-06-01"}, nil, true},
		{"sku vazio", catalogo.DadosEstoque{SKU: " ", DataSnapshot: "2024-06-01"}, catalogo.ErrSKUObrigatorio, false},
		{"sku antes da data", catalogo.DadosEstoque{DataSnapshot: "x"}, catalogo.ErrSKUObrigatorio, false},
		{"data vazia", catalogo.DadosEstoque{SKU: "S", DataSnapshot: "  "}, catalogo.ErrEstoqueDataObrigatoria, false},
		{"data malformada", catalogo.DadosEstoque{SKU: "S", DataSnapshot: "01/06/2024"}, catalogo.ErrEstoqueDataInvalida, false},
		{"data futura", catalogo.DadosEstoque{SKU: "S", DataSnapshot: "2024-06-11"}, catalogo.ErrEstoqueDataFutura, false},
		{"data antes do saldo", catalogo.DadosEstoque{SKU: "S", DataSnapshot: "2024-06-11", Saldo: -1}, catalogo.ErrEstoqueDataFutura, false},
		{"saldo negativo", catalogo.DadosEstoque{SKU: "S", DataSnapshot: "2024-06-01", Saldo: -1}, catalogo.ErrEstoqueSaldoInvalido, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e, err := catalogo.NovoRegistroEstoque(c.d, agora)
			if c.err != nil {
				assert.Nil(t, e)
				assert.ErrorIs(t, err, c.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, catalogo.SKU("S"), e.SKU)
			assert.Equal(t, c.ruptura, e.Ruptura, "ruptura derivada do saldo")
			assert.Equal(t, time.Local, e.DataSnapshot.Location(), "meia-noite local (BUG-01)")
		})
	}
}

func TestRegistroEstoque_AjustarSaldo(t *testing.T) {
	casos := []struct {
		saldo   int
		ruptura bool
		err     error
	}{
		{10, false, nil},
		{1, false, nil},
		{0, true, nil},
		{-1, true, catalogo.ErrEstoqueSaldoInvalido},
	}
	for _, c := range casos {
		e := &catalogo.RegistroEstoque{Saldo: 7, Ruptura: true}
		err := e.AjustarSaldo(c.saldo)
		assert.ErrorIs(t, err, c.err)
		if c.err != nil {
			assert.Equal(t, 7, e.Saldo, "saldo inválido não altera o registro")
			continue
		}
		assert.Equal(t, c.saldo, e.Saldo)
		assert.Equal(t, c.ruptura, e.Ruptura)
	}
}

func TestNovoFiltroEstoque(t *testing.T) {
	sim := true
	o := catalogo.OrdenacaoEstoquePadrao()

	f, err := catalogo.NovoFiltroEstoque(" SKU-1 ", " 2024-01-01 ", "2024-01-31", &sim, o)
	require.NoError(t, err)
	assert.Equal(t, "SKU-1", f.SKU)
	require.NotNil(t, f.De)
	require.NotNil(t, f.Ate)
	assert.Equal(t, "2024-01-01", f.De.String())
	assert.Equal(t, "2024-01-31", f.Ate.String())
	assert.Same(t, &sim, f.Ruptura)
	assert.Equal(t, o, f.Ordenacao)

	f, err = catalogo.NovoFiltroEstoque("", "", "", nil, o)
	require.NoError(t, err)
	assert.Nil(t, f.De, "datas vazias não filtram")
	assert.Nil(t, f.Ate)

	for _, de := range []string{"31/12/2024", "2024-13-01"} {
		_, err = catalogo.NovoFiltroEstoque("", de, "", nil, o)
		assert.ErrorIs(t, err, catalogo.ErrEstoqueDataInvalida, "data_de %q", de)
		_, err = catalogo.NovoFiltroEstoque("", "", de, nil, o)
		assert.ErrorIs(t, err, catalogo.ErrEstoqueDataInvalida, "data_ate %q", de)
	}
}

// TestOrdenacao_EnumsCatalogo: as whitelists do domínio (C-02) e os padrões
// de cada entidade, com o parse do kernel caindo no padrão fora do enum.
func TestOrdenacao_EnumsCatalogo(t *testing.T) {
	assert.Len(t, catalogo.CamposOrdenacaoProduto(), 11)
	assert.Len(t, catalogo.CamposOrdenacaoEstoque(), 7)

	pp := catalogo.OrdenacaoProdutoPadrao()
	assert.Equal(t, catalogo.OrdemProdutoID, pp.Campo())
	assert.Equal(t, kernel.Asc, pp.Direcao())
	pe := catalogo.OrdenacaoEstoquePadrao()
	assert.Equal(t, catalogo.OrdemEstoqueDataSnapshot, pe.Campo())
	assert.Equal(t, kernel.Desc, pe.Direcao())

	for _, c := range catalogo.CamposOrdenacaoProduto() {
		o := kernel.ParseOrdenacao(string(c), "", catalogo.CamposOrdenacaoProduto(), pp)
		assert.Equal(t, c, o.Campo())
	}
	for _, c := range catalogo.CamposOrdenacaoEstoque() {
		o := kernel.ParseOrdenacao(string(c), "", catalogo.CamposOrdenacaoEstoque(), pe)
		assert.Equal(t, c, o.Campo())
	}
	assert.Equal(t, catalogo.OrdemEstoqueDataSnapshot,
		kernel.ParseOrdenacao("preco_tabela", "", catalogo.CamposOrdenacaoEstoque(), pe).Campo(),
		"campo de produto não vale para estoque")
}
