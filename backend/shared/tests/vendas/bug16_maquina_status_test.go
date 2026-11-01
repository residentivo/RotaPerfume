package vendas_test

// BUG-16 / BUG-20 (Lote 25, ADENDO do MegaBrain): máquina de status do
// pedido no domínio. Faturado só segue para Entregue; Entregue é terminal;
// Entregue só a partir de Faturado (no PUT e na criação). Reenviar o mesmo
// status é idempotente. A checagem de transição (400) vem ANTES da de itens
// (409). Exclusão: pagamentos vinculados primeiro, depois Faturado/Entregue.

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
)

const (
	bug16Cancelado = "Cancelado"
	bug16Separacao = "Em separação"
	bug16Entregue  = "Entregue"
	bug16Faturado  = "Faturado"
)

var bug16Status = []string{bug16Cancelado, bug16Separacao, bug16Entregue, bug16Faturado}

func bug16Itens() []vendas.ItemPedido {
	return []vendas.ItemPedido{{ProdutoID: 2, Quantidade: 4, PrecoPraticado: 50, DescontoPct: 5}}
}

// bug16Esperado é a matriz completa do ADENDO: o erro (nil = permitida) e o
// plano para itens iguais aos atuais.
func bug16Esperado(atual, destino string) (error, vendas.PlanoAtualizacao) {
	posFaturamento := atual == bug16Faturado || atual == bug16Entregue
	switch {
	case atual == destino:
		return nil, vendas.PlanoAtualizacao{ReescreverItens: !posFaturamento}
	case atual == bug16Faturado && destino != bug16Entregue:
		return vendas.ErrPedidoFaturadoSoPodeIrParaEntregue, vendas.PlanoAtualizacao{}
	case atual == bug16Entregue:
		return vendas.ErrPedidoEntregueNaoPodeMudarStatus, vendas.PlanoAtualizacao{}
	case destino == bug16Entregue && atual != bug16Faturado:
		return vendas.ErrPedidoEntregueSemFaturamento, vendas.PlanoAtualizacao{}
	}
	return nil, vendas.PlanoAtualizacao{ReescreverItens: !posFaturamento, BaixarEstoque: !posFaturamento && destino == bug16Faturado}
}

// Matriz 4x4 de transições com itens iguais (o resultado depende só do status).
func TestBUG16_PlanejarAtualizacao_MatrizDeTransicoes(t *testing.T) {
	for _, atual := range bug16Status {
		for _, destino := range bug16Status {
			t.Run(atual+"→"+destino, func(t *testing.T) {
				wantErr, wantPlano := bug16Esperado(atual, destino)
				plano, err := vendas.PlanejarAtualizacao(atual, bug16Itens(), &vendas.Pedido{Status: destino, Itens: bug16Itens()})
				if wantErr != nil {
					assert.ErrorIs(t, err, wantErr)
					assert.Equal(t, vendas.PlanoAtualizacao{}, plano)
					return
				}
				require.NoError(t, err)
				assert.Equal(t, wantPlano, plano)
			})
		}
	}
}

// As recusas de transição são 400 (KindInvalido) com o texto exato; a de
// itens continua 409 com o texto legado (contrato, CHORE-07).
func TestBUG16_Erros_KindEMensagem(t *testing.T) {
	casos := []struct {
		err  error
		kind kernel.Kind
		msg  string
	}{
		{vendas.ErrPedidoFaturadoSoPodeIrParaEntregue, kernel.KindInvalido, "pedido faturado só pode ter o status alterado para Entregue"},
		{vendas.ErrPedidoEntregueNaoPodeMudarStatus, kernel.KindInvalido, "pedido entregue não pode ter o status alterado"},
		{vendas.ErrPedidoEntregueSemFaturamento, kernel.KindInvalido, "pedido só pode ser marcado como Entregue depois de Faturado"},
		{vendas.ErrPedidoJaFaturadoNaoPodeAlterarItens, kernel.KindConflito, "pedido já faturado: não é possível alterar os itens, apenas o status"},
		{vendas.ErrPedidoEntregueNaoPodeSerExcluido, kernel.KindConflito, "pedido entregue não pode ser excluído"},
		{vendas.ErrPedidoFaturadoNaoPodeSerExcluido, kernel.KindConflito, "pedido faturado não pode ser excluído, apenas ter o status alterado"},
		{vendas.ErrPedidoPossuiPagamentosVinculados, kernel.KindConflito, "pedido possui pagamentos vinculados"},
	}
	for _, c := range casos {
		t.Run(c.msg, func(t *testing.T) {
			var e *kernel.Erro
			require.True(t, errors.As(c.err, &e))
			assert.Equal(t, c.kind, e.Kind())
			assert.Equal(t, c.msg, c.err.Error())
		})
	}
}

// A transição é checada ANTES dos itens: transição proibida com itens
// diferentes dá o 400 da transição, não o 409 dos itens.
func TestBUG16_PlanejarAtualizacao_TransicaoAntesDosItens(t *testing.T) {
	outros := []vendas.ItemPedido{{ProdutoID: 3, Quantidade: 1, PrecoPraticado: 10}}
	casos := []struct {
		atual, destino string
		want           error
	}{
		{bug16Faturado, bug16Cancelado, vendas.ErrPedidoFaturadoSoPodeIrParaEntregue},
		{bug16Faturado, bug16Separacao, vendas.ErrPedidoFaturadoSoPodeIrParaEntregue},
		{bug16Entregue, bug16Faturado, vendas.ErrPedidoEntregueNaoPodeMudarStatus},
		{bug16Entregue, bug16Cancelado, vendas.ErrPedidoEntregueNaoPodeMudarStatus},
		{bug16Separacao, bug16Entregue, vendas.ErrPedidoEntregueSemFaturamento},
		{bug16Cancelado, bug16Entregue, vendas.ErrPedidoEntregueSemFaturamento},
		// Transições permitidas com itens diferentes: a regra dos itens decide.
		{bug16Faturado, bug16Entregue, vendas.ErrPedidoJaFaturadoNaoPodeAlterarItens},
		{bug16Faturado, bug16Faturado, vendas.ErrPedidoJaFaturadoNaoPodeAlterarItens},
		{bug16Entregue, bug16Entregue, vendas.ErrPedidoJaFaturadoNaoPodeAlterarItens},
	}
	for _, c := range casos {
		t.Run(c.atual+"→"+c.destino, func(t *testing.T) {
			_, err := vendas.PlanejarAtualizacao(c.atual, bug16Itens(), &vendas.Pedido{Status: c.destino, Itens: outros})
			assert.ErrorIs(t, err, c.want)
		})
	}

	// Pedido não faturado troca os itens livremente.
	for _, atual := range []string{bug16Cancelado, bug16Separacao} {
		plano, err := vendas.PlanejarAtualizacao(atual, bug16Itens(), &vendas.Pedido{Status: bug16Separacao, Itens: outros})
		require.NoError(t, err, atual)
		assert.True(t, plano.ReescreverItens, atual)
	}
}

// Status atual fora do ENUM (lido do banco) segue a regra dos não faturados.
func TestBUG16_PlanejarAtualizacao_StatusAtualDesconhecido(t *testing.T) {
	plano, err := vendas.PlanejarAtualizacao("Pendente", nil, &vendas.Pedido{Status: bug16Faturado, Itens: bug16Itens()})
	require.NoError(t, err)
	assert.Equal(t, vendas.PlanoAtualizacao{ReescreverItens: true, BaixarEstoque: true}, plano)

	_, err = vendas.PlanejarAtualizacao("Pendente", nil, &vendas.Pedido{Status: bug16Entregue})
	assert.ErrorIs(t, err, vendas.ErrPedidoEntregueSemFaturamento)
}

// BUG-20: a criação já Faturado baixa o estoque; Entregue é recusado (400);
// os demais status gravam sem baixa.
func TestBUG20_PlanejarCriacao(t *testing.T) {
	casos := []struct {
		status string
		baixar bool
		want   error
	}{
		{bug16Faturado, true, nil},
		{bug16Entregue, false, vendas.ErrPedidoEntregueSemFaturamento},
		{bug16Separacao, false, nil},
		{bug16Cancelado, false, nil},
	}
	for _, c := range casos {
		t.Run(c.status, func(t *testing.T) {
			d := pedidoValido()
			d.Status = " " + c.status + " " // normalizado por NovoStatusPedido
			p, err := vendas.NovoPedido(d)
			require.NoError(t, err)
			baixar, err := vendas.PlanejarCriacao(p)
			assert.ErrorIs(t, err, c.want)
			if c.want == nil {
				assert.NoError(t, err)
			}
			assert.Equal(t, c.baixar, baixar)
		})
	}
}

// Ordem das travas: pagamentos vinculados primeiro, depois o status.
func TestBUG16_PodeExcluir_Matriz(t *testing.T) {
	casos := []struct {
		status    string
		pagamento bool
		want      error
	}{
		{bug16Entregue, true, vendas.ErrPedidoPossuiPagamentosVinculados},
		{bug16Faturado, true, vendas.ErrPedidoPossuiPagamentosVinculados},
		{bug16Separacao, true, vendas.ErrPedidoPossuiPagamentosVinculados},
		{bug16Entregue, false, vendas.ErrPedidoEntregueNaoPodeSerExcluido},
		{bug16Faturado, false, vendas.ErrPedidoFaturadoNaoPodeSerExcluido},
		{bug16Separacao, false, nil},
		{bug16Cancelado, false, nil},
	}
	for _, c := range casos {
		t.Run(c.status, func(t *testing.T) {
			err := (&vendas.Pedido{Status: c.status}).PodeExcluir(c.pagamento)
			if c.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, c.want)
		})
	}
}
