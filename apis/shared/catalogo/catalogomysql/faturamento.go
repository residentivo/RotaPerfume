package catalogomysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

// ARQ-05: as escritas de estoque que eram do repositories.EstoqueRepository
// legado. A baixa por faturamento é consumida por Vendas pela porta
// vendasapp.BaixaEstoque, com o Estoques ligado ao MESMO *sql.Tx do pedido
// (bootstrap): faturar o pedido e baixar o estoque são atômicos (§5.2). O
// SQL é o do legado, salvo a ordem das atribuições do upsert (BUG-21).

const arqFaturamento = "faturamento.go"

// errProdutoDaBaixa: o item faturado aponta para um produto inexistente
// (inconsistência; o FK garante o produto). É erro de infraestrutura (500),
// nunca o 404 de catalogo.ErrProdutoNaoEncontrado, como no legado.
var errProdutoDaBaixa = errors.New("catalogomysql: produto do item não encontrado para baixa de estoque")

// SKUDoProduto devolve o sku do produto (a tabela estoque referencia
// produtos por sku, não por id).
func (r *Estoques) SKUDoProduto(ctx context.Context, produtoID int64) (string, error) {
	var sku string
	err := r.q.QueryRowContext(ctx, `SELECT sku FROM produtos WHERE id = ? LIMIT 1`, produtoID).Scan(&sku)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errProdutoDaBaixa
	}
	if err != nil {
		return "", fmt.Errorf("catalogomysql: sku por produto_id: %w", db.SemValorDuplicado(err))
	}
	vlog.Printf(arqFaturamento, "Estoques.SKUDoProduto", "SELECT produtos linhas=1")
	return sku, nil
}

// AjustarSaldoPorFaturamento aplica um DELTA ao saldo do sku no dia, para
// uso EXCLUSIVO do faturamento de pedidos: `saldo = saldo - delta` no
// próprio SQL, seguro sob concorrência (ao contrário do upsert absoluto do
// ERP). Sem registro do dia, o INSERT parte de saldo 0 (saldo final =
// -delta); o carry-over do dia anterior não é feito aqui. ruptura é
// recalculada (saldo resultante <= 0).
//
// O SELECT … FOR UPDATE serializa dois faturamentos simultâneos do mesmo
// (data_snapshot, sku) antes do upsert. Tem de rodar na tx do pedido.
func (r *Estoques) AjustarSaldoPorFaturamento(ctx context.Context, sku string, dia time.Time, delta int) error {
	data := dia.Format(kernel.LayoutData)
	var existente int
	err := r.q.QueryRowContext(ctx, `SELECT id FROM estoque WHERE data_snapshot = ? AND sku = ? FOR UPDATE`, data, sku).Scan(&existente)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("catalogomysql: lock estoque para ajuste de faturamento: %w", db.SemValorDuplicado(err))
	}
	// VALUES(saldo) carrega a CONTRIBUIÇÃO (-delta), não o saldo final: no
	// INSERT ela já é o saldo (0 - delta); no UPDATE, soma ao saldo atual.
	//
	// BUG-21: o MariaDB avalia as atribuições do ON DUPLICATE KEY UPDATE da
	// esquerda para a direita (sem SIMULTANEOUS_ASSIGNMENT no sql_mode), e
	// uma coluna já atribuída é lida com o valor NOVO. Com saldo antes de
	// ruptura, "(saldo + VALUES(saldo)) <= 0" aplicava a baixa duas vezes.
	// ruptura vem PRIMEIRO, calculada sobre o saldo antigo + a contribuição
	// (= o saldo resultante): o resultado é o mesmo com avaliação da esquerda
	// para a direita ou simultânea, então não depende do sql_mode.
	const q = `
		INSERT INTO estoque (data_snapshot, sku, saldo, ruptura)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			ruptura = (saldo + VALUES(saldo)) <= 0,
			saldo = saldo + VALUES(saldo)`
	contribuicao := -delta
	if _, err = r.q.ExecContext(ctx, q, data, sku, contribuicao, contribuicao <= 0); err != nil {
		return fmt.Errorf("catalogomysql: ajustar saldo estoque por faturamento: %w", db.SemValorDuplicado(err))
	}
	vlog.Printf(arqFaturamento, "Estoques.AjustarSaldoPorFaturamento", "UPSERT estoque delta=%d", delta)
	return nil
}
