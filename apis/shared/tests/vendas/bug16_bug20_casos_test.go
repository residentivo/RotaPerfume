package vendas_test

// BUG-16 / BUG-20 (Lote 25): casos de uso de Vendas com os adapters reais
// sobre sqlmock. A máquina de status roda dentro da tx do PUT (sob o SELECT
// status … FOR UPDATE): recusa → rollback sem UPDATE. Criação já Faturado
// baixa o estoque na MESMA tx do INSERT (falha na baixa desfaz o INSERT);
// criação Entregue é recusada antes de qualquer SQL. O ciclo completo
// (Em separação → Faturado → Faturado → Entregue) baixa o estoque uma vez só.

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vlog"
)

const (
	bug16StatusLockRe = `SELECT status, vendedor_id FROM pedidos WHERE pedido_id_origem = \? FOR UPDATE`
	bug16ItensAtuaRe  = `SELECT produto_id, quantidade, preco_praticado, desconto_pct FROM itens_pedido WHERE pedido_id = \?`
)

// bug16ItensAtuais são os itens de pedidoValido() como o banco os devolve.
func bug16ItensAtuais() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"produto_id", "quantidade", "preco_praticado", "desconto_pct"}).
		AddRow(int64(10), 2, 100.0, 10.0).AddRow(int64(11), 1, 50.0, 0.0)
}

func bug16LinhaPedido(id int64, status string) *sqlmock.Rows {
	agora := time.Now()
	return sqlmock.NewRows([]string{"pedido_id_origem", "cliente_id", "vendedor_id", "data_pedido", "canal", "status",
		"valor_total", "created_at", "updated_at", "c", "v"}).
		AddRow(id, int64(1), int64(2), agora, "App", status, 230.0, agora, agora, "C", "V")
}

func bug16Releitura(m sqlmock.Sqlmock, id int64, status string) {
	m.ExpectQuery(arq05PedidoPorIDRe).WithArgs(id).WillReturnRows(bug16LinhaPedido(id, status))
	m.ExpectQuery(arq05ItensRe).WithArgs(id).WillReturnRows(sqlmock.NewRows(nil))
}

func bug16Baixa(m sqlmock.Sqlmock, produtoID int64, sku string, delta int) {
	m.ExpectQuery(`SELECT sku FROM produtos WHERE id = \?`).WithArgs(produtoID).WillReturnRows(sqlmock.NewRows([]string{"sku"}).AddRow(sku))
	m.ExpectQuery(`SELECT id FROM estoque WHERE data_snapshot = \? AND sku = \? FOR UPDATE`).WithArgs(sqlmock.AnyArg(), sku).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectExec(`INSERT INTO estoque`).WithArgs(sqlmock.AnyArg(), sku, -delta, true).WillReturnResult(sqlmock.NewResult(0, 1))
}

func bug16Dados(status string) vendas.DadosPedido {
	d := pedidoValido()
	d.Status = status
	return d
}

func bug16CapturarVlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	out := &bytes.Buffer{}
	prevOut, prevVlog := log.Writer(), vlog.Enabled()
	log.SetOutput(out)
	vlog.SetEnabled(true)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		vlog.SetEnabled(prevVlog)
	})
	return out
}

// PUT recusado pela máquina de status: rollback depois do lock e da leitura
// dos itens, sem UPDATE de cabeçalho, sem DELETE de itens e sem baixa.
func TestBUG16_Casos_TransicaoProibida_RollbackSemUpdate(t *testing.T) {
	casos := []struct {
		atual, destino string
		want           error
	}{
		{"Faturado", "Cancelado", vendas.ErrPedidoFaturadoSoPodeIrParaEntregue},
		{"Faturado", "Em separação", vendas.ErrPedidoFaturadoSoPodeIrParaEntregue},
		{"Entregue", "Faturado", vendas.ErrPedidoEntregueNaoPodeMudarStatus},
		{"Entregue", "Cancelado", vendas.ErrPedidoEntregueNaoPodeMudarStatus},
		{"Entregue", "Em separação", vendas.ErrPedidoEntregueNaoPodeMudarStatus},
		{"Em separação", "Entregue", vendas.ErrPedidoEntregueSemFaturamento},
		{"Cancelado", "Entregue", vendas.ErrPedidoEntregueSemFaturamento},
	}
	for _, c := range casos {
		t.Run(c.atual+"→"+c.destino, func(t *testing.T) {
			out := bug16CapturarVlog(t)
			pool, mock := novoMock(t)
			mock.ExpectBegin()
			mock.ExpectQuery(bug16StatusLockRe).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"status", "vendedor_id"}).AddRow(c.atual, int64(1)))
			mock.ExpectQuery(bug16ItensAtuaRe).WithArgs(int64(5)).WillReturnRows(bug16ItensAtuais())
			mock.ExpectRollback()

			p, err := novosCasos(pool, escopoIrrestrito(t)).AtualizarPedido(context.Background(), admin(t), 5, ler(bug16Dados(c.destino)))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, c.want)
			assert.NoError(t, mock.ExpectationsWereMet())
			assert.Contains(t, out.String(), "[uow.go] [TxPedido.Atualizar] decisão bloqueado", "vlog da recusa (só o id)")
			assert.Contains(t, out.String(), "id=5")
		})
	}
}

// Itens diferentes em pedido Faturado/Entregue (transição permitida): 409
// com o texto antigo; o vlog rotula como itens_faturado.
func TestBUG16_Casos_ItensDiferentesPosFaturamento_409(t *testing.T) {
	for _, c := range []struct{ atual, destino string }{{"Faturado", "Entregue"}, {"Faturado", "Faturado"}, {"Entregue", "Entregue"}} {
		t.Run(c.atual+"→"+c.destino, func(t *testing.T) {
			out := bug16CapturarVlog(t)
			pool, mock := novoMock(t)
			mock.ExpectBegin()
			mock.ExpectQuery(bug16StatusLockRe).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"status", "vendedor_id"}).AddRow(c.atual, int64(1)))
			mock.ExpectQuery(bug16ItensAtuaRe).WithArgs(int64(5)).WillReturnRows(
				sqlmock.NewRows([]string{"produto_id", "quantidade", "preco_praticado", "desconto_pct"}).AddRow(int64(10), 3, 100.0, 10.0))
			mock.ExpectRollback()

			_, err := novosCasos(pool, escopoIrrestrito(t)).AtualizarPedido(context.Background(), admin(t), 5, ler(bug16Dados(c.destino)))
			assert.ErrorIs(t, err, vendas.ErrPedidoJaFaturadoNaoPodeAlterarItens)
			assert.NoError(t, mock.ExpectationsWereMet())
			assert.Contains(t, out.String(), "decisão bloqueado itens_faturado id=5")
		})
	}
}

// Ciclo completo: a baixa acontece UMA vez, na entrada em Faturado. O
// reenvio de Faturado e a ida para Entregue só gravam o cabeçalho (sem
// DELETE/INSERT de itens e sem baixa); depois de Entregue, nada muda.
func TestBUG16_Casos_CicloCompleto_SemSegundaBaixa(t *testing.T) {
	ctx := context.Background()
	pool, mock := novoMock(t)
	c := novosCasos(pool, escopoIrrestrito(t))

	// 1. Criado Em separação: sem baixa.
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO pedidos`).WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Em separação", 230.0).
		WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido`).WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()
	bug16Releitura(mock, 5, "Em separação")
	_, err := c.CriarPedido(ctx, admin(t), ler(bug16Dados("Em separação")))
	require.NoError(t, err)

	// 2. Em separação → Faturado: reescreve os itens e baixa os dois.
	mock.ExpectBegin()
	mock.ExpectQuery(bug16StatusLockRe).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"status", "vendedor_id"}).AddRow("Em separação", int64(1)))
	mock.ExpectQuery(bug16ItensAtuaRe).WithArgs(int64(5)).WillReturnRows(bug16ItensAtuais())
	mock.ExpectExec(`UPDATE pedidos`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM itens_pedido`).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`INSERT INTO itens_pedido`).WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido`).WillReturnResult(sqlmock.NewResult(4, 1))
	bug16Baixa(mock, 10, "SKU-10", 2)
	bug16Baixa(mock, 11, "SKU-11", 1)
	mock.ExpectCommit()
	bug16Releitura(mock, 5, "Faturado")
	_, err = c.AtualizarPedido(ctx, admin(t), 5, ler(bug16Dados("Faturado")))
	require.NoError(t, err)

	// 3. Faturado → Faturado (reenvio, itens iguais): idempotente.
	// 4. Faturado → Entregue (itens iguais): só o cabeçalho.
	for _, passo := range []struct{ atual, destino string }{{"Faturado", "Faturado"}, {"Faturado", "Entregue"}, {"Entregue", "Entregue"}} {
		mock.ExpectBegin()
		mock.ExpectQuery(bug16StatusLockRe).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"status", "vendedor_id"}).AddRow(passo.atual, int64(1)))
		mock.ExpectQuery(bug16ItensAtuaRe).WithArgs(int64(5)).WillReturnRows(bug16ItensAtuais())
		mock.ExpectExec(`UPDATE pedidos`).WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", passo.destino, 230.0, int64(5)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		bug16Releitura(mock, 5, passo.destino)
		_, err = c.AtualizarPedido(ctx, admin(t), 5, ler(bug16Dados(passo.destino)))
		require.NoError(t, err, passo.atual+"→"+passo.destino)
	}

	// 5. Entregue → Cancelado: terminal.
	mock.ExpectBegin()
	mock.ExpectQuery(bug16StatusLockRe).WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"status", "vendedor_id"}).AddRow("Entregue", int64(1)))
	mock.ExpectQuery(bug16ItensAtuaRe).WithArgs(int64(5)).WillReturnRows(bug16ItensAtuais())
	mock.ExpectRollback()
	_, err = c.AtualizarPedido(ctx, admin(t), 5, ler(bug16Dados("Cancelado")))
	assert.ErrorIs(t, err, vendas.ErrPedidoEntregueNaoPodeMudarStatus)

	assert.NoError(t, mock.ExpectationsWereMet(), "uma única baixa por item em todo o ciclo")
}

// BUG-20: criação já Faturado baixa o estoque na mesma tx do INSERT.
func TestBUG20_Casos_CriarFaturado_BaixaNaMesmaTx(t *testing.T) {
	pool, mock := novoMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO pedidos`).WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", "Faturado", 230.0).
		WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido`).WithArgs(int64(9), int64(10), 2, 100.0, 10.0, 180.0).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`INSERT INTO itens_pedido`).WithArgs(int64(9), int64(11), 1, 50.0, 0.0, 50.0).WillReturnResult(sqlmock.NewResult(2, 1))
	bug16Baixa(mock, 10, "SKU-10", 2)
	bug16Baixa(mock, 11, "SKU-11", 1)
	mock.ExpectCommit()
	bug16Releitura(mock, 9, "Faturado")

	p, err := novosCasos(pool, escopoIrrestrito(t)).CriarPedido(context.Background(), admin(t), ler(bug16Dados("Faturado")))
	require.NoError(t, err)
	assert.Equal(t, int64(9), p.PedidoIDOrigem)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// BUG-20: qualquer falha na baixa desfaz o INSERT (rollback), com o erro
// de infraestrutura (500), nunca um sentinela.
func TestBUG20_Casos_CriarFaturado_FalhaNaBaixa_Rollback(t *testing.T) {
	errBanco := errors.New("db down")
	casos := map[string]func(m sqlmock.Sqlmock){
		"sku do produto": func(m sqlmock.Sqlmock) {
			m.ExpectQuery(`SELECT sku FROM produtos WHERE id = \?`).WithArgs(int64(10)).WillReturnError(errBanco)
		},
		"lock do estoque": func(m sqlmock.Sqlmock) {
			m.ExpectQuery(`SELECT sku FROM produtos WHERE id = \?`).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"sku"}).AddRow("SKU-10"))
			m.ExpectQuery(`SELECT id FROM estoque`).WillReturnError(errBanco)
		},
		"upsert do segundo item": func(m sqlmock.Sqlmock) {
			bug16Baixa(m, 10, "SKU-10", 2)
			m.ExpectQuery(`SELECT sku FROM produtos WHERE id = \?`).WithArgs(int64(11)).WillReturnRows(sqlmock.NewRows([]string{"sku"}).AddRow("SKU-11"))
			m.ExpectQuery(`SELECT id FROM estoque`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectExec(`INSERT INTO estoque`).WillReturnError(errBanco)
		},
	}
	for nome, falhar := range casos {
		t.Run(nome, func(t *testing.T) {
			pool, mock := novoMock(t)
			mock.ExpectBegin()
			mock.ExpectExec(`INSERT INTO pedidos`).WillReturnResult(sqlmock.NewResult(9, 1))
			mock.ExpectExec(`INSERT INTO itens_pedido`).WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectExec(`INSERT INTO itens_pedido`).WillReturnResult(sqlmock.NewResult(2, 1))
			falhar(mock)
			mock.ExpectRollback()

			p, err := novosCasos(pool, escopoIrrestrito(t)).CriarPedido(context.Background(), admin(t), ler(bug16Dados("Faturado")))
			assert.Nil(t, p)
			assert.ErrorIs(t, err, errBanco)
			assert.NoError(t, mock.ExpectationsWereMet(), "sem commit e sem releitura")
		})
	}
}

// BUG-20: falha no próprio INSERT do pedido também não chega à baixa.
func TestBUG20_Casos_CriarFaturado_FalhaNoInsert_SemBaixa(t *testing.T) {
	pool, mock := novoMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO pedidos`).WillReturnError(errors.New("db down"))
	mock.ExpectRollback()
	_, err := novosCasos(pool, escopoIrrestrito(t)).CriarPedido(context.Background(), admin(t), ler(bug16Dados("Faturado")))
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// BUG-20: criar já Entregue pularia a baixa → 400 antes de qualquer SQL
// (nenhuma expectativa no mock: um BEGIN faria o teste falhar).
func TestBUG20_Casos_CriarEntregue_400SemSQL(t *testing.T) {
	out := bug16CapturarVlog(t)
	pool, mock := novoMock(t)
	p, err := novosCasos(pool, escopoIrrestrito(t)).CriarPedido(context.Background(), admin(t), ler(bug16Dados(" Entregue ")))
	assert.Nil(t, p)
	assert.ErrorIs(t, err, vendas.ErrPedidoEntregueSemFaturamento)
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.Contains(t, out.String(), "decisão bloqueado status_entregue_sem_faturamento")
}

// Criação nos status que não baixam: só INSERT de cabeçalho e itens.
func TestBUG20_Casos_CriarSemBaixa(t *testing.T) {
	for _, status := range []string{"Em separação", "Cancelado"} {
		t.Run(status, func(t *testing.T) {
			pool, mock := novoMock(t)
			mock.ExpectBegin()
			mock.ExpectExec(`INSERT INTO pedidos`).WithArgs(int64(1), int64(2), sqlmock.AnyArg(), "App", status, 230.0).
				WillReturnResult(sqlmock.NewResult(9, 1))
			mock.ExpectExec(`INSERT INTO itens_pedido`).WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectExec(`INSERT INTO itens_pedido`).WillReturnResult(sqlmock.NewResult(2, 1))
			mock.ExpectCommit()
			bug16Releitura(mock, 9, status)
			_, err := novosCasos(pool, escopoIrrestrito(t)).CriarPedido(context.Background(), admin(t), ler(bug16Dados(status)))
			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
