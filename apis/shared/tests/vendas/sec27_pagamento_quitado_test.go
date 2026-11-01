package vendas_test

// SEC-27 (Lote 25, ADENDO): teto de taxa_pct (0..30, para todos) e
// pagamento quitado editável SÓ pelo admin. Para o perfil restrito, o
// UPDATE é condicionado a "não quitado" no próprio statement (atômico);
// 0 linhas → releitura: inexistente 404, quitado (ou editável, corrida) 409.
// Ordem: posse → decode → validação (400) → UPDATE (409/404).

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

const (
	sec27UpdateRe     = `UPDATE pagamentos\s+SET forma_pagamento = \?, parcelas = \?, valor = \?, taxa_pct = \?, valor_liquido = \?, data_vencimento = \?, data_pagamento = \?, status_pagamento = \?\s+WHERE pagamento_id = \?`
	sec27UpdateCondRe = sec27UpdateRe + ` AND status_pagamento NOT IN \(\?, \?\)`
)

func sec27LinhaPagamento(id, pedidoID int64, status string) *sqlmock.Rows {
	agora := time.Now()
	return sqlmock.NewRows([]string{"pagamento_id", "pedido_id", "forma_pagamento", "parcelas", "valor", "taxa_pct",
		"valor_liquido", "data_vencimento", "data_pagamento", "status_pagamento", "created_at", "updated_at"}).
		AddRow(id, pedidoID, "PIX", uint8(1), 100.0, 0.0, 100.0, agora, nil, status, agora, agora)
}

func TestSEC27_TaxaPctMaxima_EMensagem(t *testing.T) {
	assert.Equal(t, 30.0, vendas.TaxaPctMaxima)
	var e *kernel.Erro
	require.True(t, errors.As(vendas.ErrTaxaPctInvalida, &e))
	assert.Equal(t, kernel.KindInvalido, e.Kind())
	assert.Equal(t, "taxa_pct deve estar entre 0 e 30", vendas.ErrTaxaPctInvalida.Error())

	require.True(t, errors.As(vendas.ErrPagamentoJaQuitadoNaoPodeSerEditado, &e))
	assert.Equal(t, kernel.KindConflito, e.Kind())
	assert.Equal(t, "pagamento já quitado não pode ser editado", vendas.ErrPagamentoJaQuitadoNaoPodeSerEditado.Error())
}

func TestSEC27_DadosPagamento_LimitesDaTaxa(t *testing.T) {
	casos := []struct {
		taxa float64
		ok   bool
	}{
		{0, true}, {0.01, true}, {1.5, true}, {29.99, true}, {30, true},
		{30.000001, false}, {30.01, false}, {31, false}, {100, false}, {999.99, false},
		{-0.01, false}, {-30, false},
	}
	for _, c := range casos {
		t.Run(fmt.Sprintf("taxa=%v", c.taxa), func(t *testing.T) {
			d := pagamentoValido()
			d.TaxaPct = c.taxa
			p, err := d.Pagamento(1)
			if c.ok {
				require.NoError(t, err)
				assert.Equal(t, c.taxa, p.TaxaPct)
				return
			}
			assert.Nil(t, p)
			assert.ErrorIs(t, err, vendas.ErrTaxaPctInvalida)
		})
	}
}

func TestSEC27_PodeEditar_PorStatus(t *testing.T) {
	casos := []struct {
		status string
		want   error
	}{
		{"Em aberto", nil},
		{"Inadimplente", nil},
		{"Pago", vendas.ErrPagamentoJaQuitadoNaoPodeSerEditado},
		{"Pago com atraso", vendas.ErrPagamentoJaQuitadoNaoPodeSerEditado},
	}
	for _, c := range casos {
		t.Run(c.status, func(t *testing.T) {
			err := (&vendas.Pagamento{StatusPagamento: c.status}).PodeEditar()
			if c.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, c.want)
		})
	}
}

// O adapter condiciona o UPDATE aos dois status quitados vindos do domínio.
func TestSEC27_Adapter_AtualizarSeNaoQuitado(t *testing.T) {
	ctx := context.Background()
	pag := &vendas.Pagamento{FormaPagamento: "PIX", Parcelas: 1, Valor: 100, TaxaPct: 30, ValorLiquido: 70,
		DataVencimento: time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local), StatusPagamento: "Pago"}

	t.Run("1 linha", func(t *testing.T) {
		pool, mock := novoMock(t)
		mock.ExpectExec(sec27UpdateCondRe).
			WithArgs("PIX", uint8(1), 100.0, 30.0, 70.0, sqlmock.AnyArg(), nil, "Pago", int64(3), "Pago", "Pago com atraso").
			WillReturnResult(sqlmock.NewResult(0, 1))
		assert.NoError(t, vendasmysql.NovoPagamentos(pool).AtualizarSeNaoQuitado(ctx, 3, pag))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("0 linhas vira ErrPagamentoNaoEncontrado", func(t *testing.T) {
		pool, mock := novoMock(t)
		mock.ExpectExec(sec27UpdateCondRe).WillReturnResult(sqlmock.NewResult(0, 0))
		assert.Same(t, vendas.ErrPagamentoNaoEncontrado, vendasmysql.NovoPagamentos(pool).AtualizarSeNaoQuitado(ctx, 3, pag))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("erro do driver é infra (500)", func(t *testing.T) {
		pool, mock := novoMock(t)
		errBanco := errors.New("db down")
		mock.ExpectExec(sec27UpdateCondRe).WillReturnError(errBanco)
		err := vendasmysql.NovoPagamentos(pool).AtualizarSeNaoQuitado(ctx, 3, pag)
		assert.ErrorIs(t, err, errBanco)
		var e *kernel.Erro
		assert.False(t, errors.As(err, &e))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("o UPDATE do admin continua sem a condição", func(t *testing.T) {
		pool, mock := novoMock(t)
		mock.ExpectExec(sec27UpdateRe+`$`).
			WithArgs("PIX", uint8(1), 100.0, 30.0, 70.0, sqlmock.AnyArg(), nil, "Pago", int64(3)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		assert.NoError(t, vendasmysql.NovoPagamentos(pool).Atualizar(ctx, 3, pag))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// Casos de uso no escopo restrito (vendedor 2, dono do pedido 1).
func TestSEC27_Casos_Restrito(t *testing.T) {
	ctx := context.Background()
	posse := func(m sqlmock.Sqlmock, statusAtual string) {
		m.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(sec27LinhaPagamento(5, 1, statusAtual))
		m.ExpectQuery(arq05PedidoPorIDRe).WithArgs(int64(1)).WillReturnRows(arq05LinhaPedido(1, 2))
	}
	casos := []struct {
		nome    string
		mudar   func(d *vendas.DadosPagamento)
		prepara func(m sqlmock.Sqlmock)
		want    error
	}{
		{"Em aberto → Pago grava com o UPDATE condicional", func(d *vendas.DadosPagamento) { d.StatusPagamento = "Pago" }, func(m sqlmock.Sqlmock) {
			posse(m, "Em aberto")
			m.ExpectExec(sec27UpdateCondRe).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
				sqlmock.AnyArg(), sqlmock.AnyArg(), "Pago", int64(5), "Pago", "Pago com atraso").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(sec27LinhaPagamento(5, 1, "Pago"))
		}, nil},
		{"quitado (Pago): 0 linhas e releitura quitado → 409", nil, func(m sqlmock.Sqlmock) {
			posse(m, "Pago")
			m.ExpectExec(sec27UpdateCondRe).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(sec27LinhaPagamento(5, 1, "Pago"))
		}, vendas.ErrPagamentoJaQuitadoNaoPodeSerEditado},
		{"quitado (Pago com atraso) → 409", nil, func(m sqlmock.Sqlmock) {
			posse(m, "Pago com atraso")
			m.ExpectExec(sec27UpdateCondRe).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(sec27LinhaPagamento(5, 1, "Pago com atraso"))
		}, vendas.ErrPagamentoJaQuitadoNaoPodeSerEditado},
		{"corrida: 0 linhas e releitura editável → 409", nil, func(m sqlmock.Sqlmock) {
			posse(m, "Em aberto")
			m.ExpectExec(sec27UpdateCondRe).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(sec27LinhaPagamento(5, 1, "Em aberto"))
		}, vendas.ErrPagamentoJaQuitadoNaoPodeSerEditado},
		{"excluído entre a posse e o UPDATE → 404", nil, func(m sqlmock.Sqlmock) {
			posse(m, "Em aberto")
			m.ExpectExec(sec27UpdateCondRe).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"pagamento_id"}))
		}, vendas.ErrPagamentoNaoEncontrado},
		{"erro na releitura → 500 (erro de infra)", nil, func(m sqlmock.Sqlmock) {
			posse(m, "Pago")
			m.ExpectExec(sec27UpdateCondRe).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnError(errors.New("db down"))
		}, errSec27Infra},
		{"erro no UPDATE condicional → 500 sem releitura", nil, func(m sqlmock.Sqlmock) {
			posse(m, "Pago")
			m.ExpectExec(sec27UpdateCondRe).WillReturnError(errors.New("db down"))
		}, errSec27Infra},
		{"validação (taxa 31) vem antes do 409: 400 sem UPDATE", func(d *vendas.DadosPagamento) { d.TaxaPct = 31 }, func(m sqlmock.Sqlmock) {
			posse(m, "Pago")
		}, vendas.ErrTaxaPctInvalida},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			pool, mock := novoMock(t)
			c.prepara(mock)
			d := pagamentoValido()
			d.StatusPagamento = "Em aberto"
			d.DataPagamento = ""
			if c.mudar != nil {
				c.mudar(&d)
			}
			pag, err := novosCasos(pool, doVendedor(t, 2)).AtualizarPagamento(ctx, admin(t), 5, ler(d))
			switch {
			case c.want == nil:
				require.NoError(t, err)
				assert.Equal(t, "Pago", pag.StatusPagamento)
			case c.want == errSec27Infra:
				require.Error(t, err)
				var e *kernel.Erro
				assert.False(t, errors.As(err, &e), "falha de infra nunca vira sentinela: %v", err)
			default:
				assert.Nil(t, pag)
				assert.ErrorIs(t, err, c.want)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// errSec27Infra marca, na tabela, que o esperado é um erro de infra (500).
var errSec27Infra = errors.New("sec27: erro de infraestrutura esperado")

// Admin (irrestrito): UPDATE normal, sem posse e sem condição; quitado
// continua editável (200). O teto de taxa vale também para o admin.
func TestSEC27_Casos_Admin(t *testing.T) {
	ctx := context.Background()
	t.Run("quitado editável", func(t *testing.T) {
		pool, mock := novoMock(t)
		mock.ExpectExec(sec27UpdateRe+`$`).WithArgs("PIX", uint8(2), 100.0, 1.5, 98.5, sqlmock.AnyArg(), sqlmock.AnyArg(), "Pago com atraso", int64(5)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(sec27LinhaPagamento(5, 1, "Pago com atraso"))
		d := pagamentoValido()
		d.StatusPagamento = "Pago com atraso"
		pag, err := novosCasos(pool, escopoIrrestrito(t)).AtualizarPagamento(ctx, admin(t), 5, ler(d))
		require.NoError(t, err)
		assert.Equal(t, "Pago com atraso", pag.StatusPagamento)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("inexistente → 404 sem releitura", func(t *testing.T) {
		pool, mock := novoMock(t)
		mock.ExpectExec(sec27UpdateRe + `$`).WillReturnResult(sqlmock.NewResult(0, 0))
		_, err := novosCasos(pool, escopoIrrestrito(t)).AtualizarPagamento(ctx, admin(t), 5, ler(pagamentoValido()))
		assert.ErrorIs(t, err, vendas.ErrPagamentoNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("taxa acima de 30 → 400 sem SQL", func(t *testing.T) {
		pool, mock := novoMock(t)
		d := pagamentoValido()
		d.TaxaPct = 30.01
		_, err := novosCasos(pool, escopoIrrestrito(t)).AtualizarPagamento(ctx, admin(t), 5, ler(d))
		assert.ErrorIs(t, err, vendas.ErrTaxaPctInvalida)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("criação com taxa acima de 30 → 400 sem INSERT", func(t *testing.T) {
		pool, mock := novoMock(t)
		sec26Lock(mock, 1, "Pendente", 2) // SEC-35: o pedido é travado antes da validação dos campos
		mock.ExpectRollback()
		d := vendas.DadosNovoPagamento{PedidoID: 1, DadosPagamento: pagamentoValido()}
		d.TaxaPct = 100
		_, err := novosCasos(pool, escopoIrrestrito(t)).CriarPagamento(ctx, admin(t), ler(d))
		assert.ErrorIs(t, err, vendas.ErrTaxaPctInvalida)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
