package vendas_test

// CHORE-08 (Lote 31): NaN e ±Inf em taxa_pct são recusados no domínio com
// o mesmo 400; o vlog da recusa do PUT tem um rótulo por regra da máquina
// de status; o log da edição bloqueada diz o motivo real.

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/vendas"
)

func TestCHORE08_TaxaPct_NaNEInf_400(t *testing.T) {
	for nome, taxa := range map[string]float64{"NaN": math.NaN(), "+Inf": math.Inf(1), "-Inf": math.Inf(-1)} {
		t.Run(nome, func(t *testing.T) {
			d := pagamentoValido()
			d.TaxaPct = taxa
			_, err := d.Pagamento(1)
			assert.Same(t, vendas.ErrTaxaPctInvalida, err)
		})
	}
	d := pagamentoValido()
	d.TaxaPct = vendas.TaxaPctMaxima
	_, err := d.Pagamento(1)
	assert.NoError(t, err, "o limite continua aceito")
}

func TestCHORE08_VlogDoPUT_UmRotuloPorRegra(t *testing.T) {
	casos := []struct{ atual, destino, rotulo string }{
		{"Faturado", "Cancelado", "decisão bloqueado status_faturado_so_entregue id=1"},
		{"Entregue", "Faturado", "decisão bloqueado status_entregue_terminal id=1"},
		{"Em separação", "Entregue", "decisão bloqueado status_entregue_sem_faturamento id=1"},
	}
	for _, c := range casos {
		t.Run(c.atual+"→"+c.destino, func(t *testing.T) {
			out := bug16CapturarVlog(t)
			pool, mock := novoMock(t)
			sec26Lock(mock, 1, c.atual, 2)
			mock.ExpectQuery(bug16ItensAtuaRe).WithArgs(int64(1)).WillReturnRows(bug16ItensAtuais())
			mock.ExpectRollback()

			d := pedidoValido()
			d.Status = c.destino
			_, err := novosCasos(pool, escopoIrrestrito(t)).AtualizarPedido(context.Background(), admin(t), 1, ler(d))
			assert.Error(t, err)
			assert.Contains(t, out.String(), c.rotulo)
			assert.NotContains(t, out.String(), "bloqueado status_faturado id=", "o rótulo ambíguo antigo saiu")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCHORE08_EdicaoBloqueada_LogComMotivoReal(t *testing.T) {
	casos := []struct{ status, motivo string }{
		{"Pago", "motivo=quitado"},
		{"Em aberto", "motivo=edicao_concorrente"},
	}
	for _, c := range casos {
		t.Run(c.status, func(t *testing.T) {
			out := bug16CapturarVlog(t)
			p, e := sec26Restrito(t)
			pool, mock := novoMock(t)
			mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
			mock.ExpectQuery(arq05PedidoPorIDRe).WithArgs(int64(1)).WillReturnRows(arq05LinhaPedido(1, 2))
			mock.ExpectExec(`UPDATE pagamentos`).WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(chore08LinhaPagamentoStatus(5, 1, c.status))

			_, err := novosCasos(pool, e).AtualizarPagamento(context.Background(), p, 5, ler(pagamentoValido()))
			assert.Error(t, err)
			assert.Contains(t, out.String(), "[pagamentos] edição bloqueada: pagamento_id=5 "+c.motivo)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func chore08LinhaPagamentoStatus(id, pedidoID int64, status string) *sqlmock.Rows {
	agora := time.Now()
	return sqlmock.NewRows([]string{"pagamento_id", "pedido_id", "forma_pagamento", "parcelas", "valor", "taxa_pct",
		"valor_liquido", "data_vencimento", "data_pagamento", "status_pagamento", "created_at", "updated_at"}).
		AddRow(id, pedidoID, "PIX", uint8(1), 100.0, 0.0, 100.0, agora, nil, status, agora, agora)
}
