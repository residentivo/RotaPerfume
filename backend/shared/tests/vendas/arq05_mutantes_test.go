package vendas_test

// ARQ-05 (TestBrain): testes que matam mutantes sobreviventes da validação
// do ARQ-05.
//
//   - g3: restrito() tratando o escopo zero value como irrestrito
//     (fail-open). Get/Delete/Update/Create de pagamento e Update de pedido
//     têm de checar a posse e responder 404, sem gravar nada e sem ler o body
//     antes da posse.
//   - i4: o upsert da baixa por faturamento sobrescrevendo o saldo
//     (VALUES(saldo)) em vez de somar a contribuição ao saldo atual, ou com a
//     ruptura calculada sobre outra expressão.
//   - j2: 1062 no UPDATE de produto (Produtos.exec) com o valor do driver
//     no erro (NB-3).

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
)

const (
	arq05PagamentoPorIDRe = `SELECT pagamento_id, pedido_id, .+ FROM pagamentos WHERE pagamento_id = \? LIMIT 1`
	arq05PedidoPorIDRe    = `FROM pedidos p .+ WHERE p\.pedido_id_origem = \? LIMIT 1`
	arq05ItensRe          = `FROM itens_pedido i .+ WHERE i\.pedido_id = \?`
)

func arq05LinhaPagamento(id, pedidoID int64) *sqlmock.Rows {
	agora := time.Now()
	return sqlmock.NewRows([]string{"pagamento_id", "pedido_id", "forma_pagamento", "parcelas", "valor", "taxa_pct",
		"valor_liquido", "data_vencimento", "data_pagamento", "status_pagamento", "created_at", "updated_at"}).
		AddRow(id, pedidoID, "PIX", uint8(1), 100.0, 0.0, 100.0, agora, nil, "Em aberto", agora, agora)
}

func arq05LinhaPedido(id, vendedorID int64) *sqlmock.Rows {
	agora := time.Now()
	return sqlmock.NewRows([]string{"pedido_id_origem", "cliente_id", "vendedor_id", "data_pedido", "canal", "status",
		"valor_total", "created_at", "updated_at", "c", "v"}).
		AddRow(id, int64(1), vendedorID, agora, "App", "Em separação", 1.0, agora, agora, "C", "V")
}

// leitorVigiado devolve v e registra se o body foi lido.
func leitorVigiado[T any](v T, lido *bool) func() (T, error) {
	return func() (T, error) { *lido = true; return v, nil }
}

// Escopo zero value vindo da porta (autorizador defeituoso): a posse do
// registro continua checada (restrito() é fail-closed). Resposta = o 404 do
// registro, sem nenhuma escrita e sem ler o body antes da posse.
func TestARQ05_EscopoZeroValue_PosseFalhaFechado(t *testing.T) {
	ctx := context.Background()
	casos := []struct {
		nome    string
		prepara func(m sqlmock.Sqlmock)
		chama   func(t *testing.T, pool *sql.DB, lido *bool) (any, error)
		want    error
	}{
		{"ObterPagamento", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
			m.ExpectQuery(arq05PedidoPorIDRe).WithArgs(int64(1)).WillReturnRows(arq05LinhaPedido(1, 2))
		}, func(t *testing.T, pool *sql.DB, _ *bool) (any, error) {
			return novosCasos(pool, kernel.Escopo{}).ObterPagamento(ctx, admin(t), 5)
		}, vendas.ErrPagamentoNaoEncontrado},
		{"ExcluirPagamento", func(m sqlmock.Sqlmock) {
			// SEC-35: a posse é conferida sob o lock do pedido, na tx.
			m.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
			sec26Lock(m, 1, "Pendente", 2)
			m.ExpectRollback()
		}, func(t *testing.T, pool *sql.DB, _ *bool) (any, error) {
			return nil, novosCasos(pool, kernel.Escopo{}).ExcluirPagamento(ctx, admin(t), 5)
		}, vendas.ErrPagamentoNaoEncontrado},
		{"AtualizarPagamento", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(arq05PagamentoPorIDRe).WithArgs(int64(5)).WillReturnRows(arq05LinhaPagamento(5, 1))
			m.ExpectQuery(arq05PedidoPorIDRe).WithArgs(int64(1)).WillReturnRows(arq05LinhaPedido(1, 2))
		}, func(t *testing.T, pool *sql.DB, lido *bool) (any, error) {
			return novosCasos(pool, kernel.Escopo{}).AtualizarPagamento(ctx, admin(t), 5, leitorVigiado(pagamentoValido(), lido))
		}, vendas.ErrPagamentoNaoEncontrado},
		{"CriarPagamento", func(m sqlmock.Sqlmock) {
			// SEC-35: a posse é conferida sob o lock do pedido, na tx.
			sec26Lock(m, 1, "Pendente", 2)
			m.ExpectRollback()
		}, func(t *testing.T, pool *sql.DB, _ *bool) (any, error) {
			d := vendas.DadosNovoPagamento{PedidoID: 1, DadosPagamento: pagamentoValido()}
			return novosCasos(pool, kernel.Escopo{}).CriarPagamento(ctx, admin(t), ler(d))
		}, vendas.ErrPedidoNaoEncontrado},
		{"AtualizarPedido", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(arq05PedidoPorIDRe).WithArgs(int64(1)).WillReturnRows(arq05LinhaPedido(1, 2))
			m.ExpectQuery(arq05ItensRe).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows(nil))
		}, func(t *testing.T, pool *sql.DB, lido *bool) (any, error) {
			return novosCasos(pool, kernel.Escopo{}).AtualizarPedido(ctx, admin(t), 1, leitorVigiado(pedidoValido(), lido))
		}, vendas.ErrPedidoNaoEncontrado},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			pool, mock := novoMock(t)
			c.prepara(mock)
			lido := false
			r, err := c.chama(t, pool, &lido)
			require.ErrorIs(t, err, c.want, "zero value nunca vira irrestrito")
			assert.Nil(t, nilSeVazio(r), "nada devolvido fora da carteira")
			assert.False(t, lido, "a posse é checada antes do decode")
			assert.NoError(t, mock.ExpectationsWereMet(), "nenhuma escrita e nenhuma leitura extra")
		})
	}
}

// nilSeVazio normaliza ponteiros tipados nil (r é any).
func nilSeVazio(r any) any {
	switch v := r.(type) {
	case *vendas.Pagamento:
		if v == nil {
			return nil
		}
	case *vendas.Pedido:
		if v == nil {
			return nil
		}
	}
	return r
}

// A baixa por faturamento SOMA a contribuição (-delta) ao saldo existente
// (nunca sobrescreve o saldo do dia) e recalcula a ruptura sobre o saldo
// resultante; o SELECT … FOR UPDATE vem antes, na mesma conexão. BUG-21: a
// ruptura é atribuída ANTES do saldo (o MariaDB avalia da esquerda para a
// direita), para não aplicar a baixa duas vezes.
func TestARQ05_AjusteFaturamento_UpsertSomaAoSaldo(t *testing.T) {
	const upsertRe = `INSERT INTO estoque \(data_snapshot, sku, saldo, ruptura\)\s+VALUES \(\?, \?, \?, \?\)\s+` +
		`ON DUPLICATE KEY UPDATE\s+ruptura = \(saldo \+ VALUES\(saldo\)\) <= 0,\s+saldo = saldo \+ VALUES\(saldo\)\s*$`
	dia := time.Date(2024, 3, 9, 15, 4, 5, 0, time.Local)
	casos := []struct {
		nome         string
		delta        int
		contribuicao int
		ruptura      bool
	}{
		{"baixa de 3", 3, -3, true},
		{"baixa zero", 0, 0, true},
		{"delta negativo (entrada)", -2, 2, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			pool, mock := novoMock(t)
			mock.ExpectQuery(`SELECT id FROM estoque WHERE data_snapshot = \? AND sku = \? FOR UPDATE`).
				WithArgs("2024-03-09", "SKU-1").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
			mock.ExpectExec(upsertRe).WithArgs("2024-03-09", "SKU-1", c.contribuicao, c.ruptura).
				WillReturnResult(sqlmock.NewResult(0, 2))
			require.NoError(t, catalogomysql.NovoEstoques(pool).AjustarSaldoPorFaturamento(context.Background(), "SKU-1", dia, c.delta))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// NB-3 também no UPDATE de produto: um 1062 do driver não carrega o valor
// duplicado para o erro (log), mas continua sendo um 1062 (errors.As) e 500.
func TestARQ05_NB3_DuplicadaNoUpdateDoProdutoSemValor(t *testing.T) {
	ctx := context.Background()
	casos := map[string]struct {
		re     string
		gravar func(db *sql.DB) error
	}{
		"Atualizar": {`UPDATE produtos\s+SET descricao`, func(db *sql.DB) error {
			return catalogomysql.NovoProdutos(db).Atualizar(ctx, 5, catalogo.CamposProduto{Descricao: "D"})
		}},
		"DefinirAtivo": {`UPDATE produtos SET ativo = \? WHERE id = \?`, func(db *sql.DB) error {
			return catalogomysql.NovoProdutos(db).DefinirAtivo(ctx, 5, true)
		}},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			pool, mock := novoMock(t)
			mock.ExpectExec(c.re).WillReturnError(&mysql.MySQLError{Number: 1062,
				Message: "Duplicate entry 'VALOR-SECRETO' for key 'produtos.uk_produtos_sku'"})
			err := c.gravar(pool)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "VALOR-SECRETO")
			var me *mysql.MySQLError
			require.ErrorAs(t, err, &me)
			assert.Equal(t, uint16(1062), me.Number)
			var ke *kernel.Erro
			assert.False(t, errors.As(err, &ke), "status HTTP preservado: 500")
			assert.False(t, errors.Is(err, catalogo.ErrProdutoNaoEncontrado))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
