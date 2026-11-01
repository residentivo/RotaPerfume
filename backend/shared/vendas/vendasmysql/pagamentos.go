package vendasmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vlog"
)

const arqPagamentos = "pagamentos.go"

// fkPagamentosPedido é a FK pagamentos.pedido_id → pedidos (sql/12_ddl_pagamentos.sql).
const fkPagamentosPedido = "fk_pagamentos_pedido"

// pagamentoColunas usa a PK literal pagamento_id (não há coluna id
// desacoplada nesta tabela — ver sql/12_ddl_pagamentos.sql).
const pagamentoColunas = `pagamento_id, pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento, created_at, updated_at`

// Pagamentos é o repositório de pagamentos (porta vendasapp.Pagamentos).
type Pagamentos struct{ q db.Querier }

// NovoPagamentos liga o repositório ao pool (ou a uma tx). q nil: panic.
func NovoPagamentos(q db.Querier) *Pagamentos {
	if q == nil {
		panic("vendasmysql: NovoPagamentos exige Querier")
	}
	return &Pagamentos{q: q}
}

// Listar pagina os pagamentos do escopo que casam com o filtro. O escopo de
// vendedor filtra pelos pedidos do vendedor (V-02); não há filtro de
// vendedor de negócio (no irrestrito, sem condição).
func (r *Pagamentos) Listar(ctx context.Context, e kernel.Escopo, f vendas.FiltroPagamento, p kernel.Pagina) (kernel.Resultado[vendas.Pagamento], error) {
	vendedorID, err := vendedorDaListagem(e, 0)
	if err != nil {
		return kernel.Resultado[vendas.Pagamento]{}, err
	}
	w, args := wherePagamento(f, vendedorID)
	countQ := "SELECT COUNT(*) FROM pagamentos" + w
	q := "SELECT " + pagamentoColunas + " FROM pagamentos" + w + ordemPagamento(f.Ordenacao) + " LIMIT ? OFFSET ?"
	res, err := paginar(ctx, r.q, "pagamentos", countQ, q, args, p, scanPagamento)
	if err != nil {
		return res, err
	}
	vlog.Printf(arqPagamentos, "Pagamentos.Listar", "SELECT pagamentos linhas=%d total=%d", len(res.Itens), res.Total)
	return res, nil
}

func wherePagamento(f vendas.FiltroPagamento, vendedorID int64) (string, []any) {
	var conds []string
	var args []any
	if f.StatusPagamento != "" {
		conds, args = append(conds, "status_pagamento = ?"), append(args, f.StatusPagamento)
	}
	if f.FormaPagamento != "" {
		conds, args = append(conds, "forma_pagamento = ?"), append(args, f.FormaPagamento)
	}
	if f.PedidoID > 0 {
		conds, args = append(conds, "pedido_id = ?"), append(args, f.PedidoID)
	}
	if f.VencimentoDe != "" {
		conds, args = append(conds, "data_vencimento >= ?"), append(args, f.VencimentoDe)
	}
	if f.VencimentoAte != "" {
		conds, args = append(conds, "data_vencimento <= ?"), append(args, f.VencimentoAte)
	}
	if vendedorID > 0 {
		conds = append(conds, "pedido_id IN (SELECT pedido_id_origem FROM pedidos WHERE vendedor_id = ?)")
		args = append(args, vendedorID)
	}
	return db.Where(conds), args
}

// ordemPagamento monta " ORDER BY <coluna> <dir>" (padrão "pagamento_id ASC").
func ordemPagamento(o kernel.Ordenacao[vendas.CampoPagamento]) string {
	return " ORDER BY " + colunaPagamento(o.Campo()) + " " + db.DirecaoSQL(o.Direcao(), "ASC")
}

// colunaPagamento é a whitelist de pagamentos: enum → coluna (literal; fora
// do enum cai em "pagamento_id").
func colunaPagamento(c vendas.CampoPagamento) string {
	switch c {
	case vendas.OrdemPagamentoPedidoID:
		return "pedido_id"
	case vendas.OrdemPagamentoForma:
		return "forma_pagamento"
	case vendas.OrdemPagamentoParcelas:
		return "parcelas"
	case vendas.OrdemPagamentoValor:
		return "valor"
	case vendas.OrdemPagamentoTaxaPct:
		return "taxa_pct"
	case vendas.OrdemPagamentoValorLiquido:
		return "valor_liquido"
	case vendas.OrdemPagamentoDataVencimento:
		return "data_vencimento"
	case vendas.OrdemPagamentoDataPagamento:
		return "data_pagamento"
	case vendas.OrdemPagamentoStatus:
		return "status_pagamento"
	case vendas.OrdemPagamentoCriadoEm:
		return "created_at"
	case vendas.OrdemPagamentoAtualizadoEm:
		return "updated_at"
	default:
		return "pagamento_id"
	}
}

// PorID busca o pagamento; vendas.ErrPagamentoNaoEncontrado sem linha.
func (r *Pagamentos) PorID(ctx context.Context, id int64) (*vendas.Pagamento, error) {
	q := "SELECT " + pagamentoColunas + " FROM pagamentos WHERE pagamento_id = ? LIMIT 1"
	p, err := scanPagamento(r.q.QueryRowContext(ctx, q, id))
	if err != nil {
		return nil, err
	}
	vlog.Printf(arqPagamentos, "Pagamentos.PorID", "SELECT pagamentos linhas=1")
	return p, nil
}

func scanPagamento(s db.Linha) (*vendas.Pagamento, error) {
	var dataPagamento sql.NullTime
	p, err := db.LerUma(s, vendas.ErrPagamentoNaoEncontrado, "vendasmysql: scan pagamento", func(p *vendas.Pagamento) []any {
		return []any{&p.PagamentoID, &p.PedidoID, &p.FormaPagamento, &p.Parcelas, &p.Valor, &p.TaxaPct,
			&p.ValorLiquido, &p.DataVencimento, &dataPagamento, &p.StatusPagamento, &p.CreatedAt, &p.UpdatedAt}
	})
	if err != nil {
		return nil, err
	}
	if dataPagamento.Valid {
		p.DataPagamento = &dataPagamento.Time
	}
	return p, nil
}

// Inserir grava o pagamento e preenche PagamentoID. Pedido inexistente (FK
// fk_pagamentos_pedido, 1452) vira vendas.ErrPedidoNaoEncontrado SEM o erro
// do driver (SEC-35).
func (r *Pagamentos) Inserir(ctx context.Context, p *vendas.Pagamento) error {
	const q = `
		INSERT INTO pagamentos (pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	id, err := db.Inserir(ctx, r.q, "vendasmysql: create pagamento", q, p.PedidoID, p.FormaPagamento, p.Parcelas,
		p.Valor, p.TaxaPct, p.ValorLiquido, p.DataVencimento, nullTime(p.DataPagamento), p.StatusPagamento)
	if db.ViolacaoFK(err, fkPagamentosPedido) {
		vlog.Printf(arqPagamentos, "Pagamentos.Inserir", "decisão pedido_inexistente_fk")
		return vendas.ErrPedidoNaoEncontrado
	}
	if err != nil {
		return err
	}
	p.PagamentoID = id
	vlog.Printf(arqPagamentos, "Pagamentos.Inserir", "INSERT pagamentos id=%d", id)
	return nil
}

// Atualizar grava os campos editáveis. pagamento_id e pedido_id NÃO estão no
// SET (V-02: o vínculo com o pedido é definitivo).
func (r *Pagamentos) Atualizar(ctx context.Context, id int64, p *vendas.Pagamento) error {
	const q = `
		UPDATE pagamentos
		SET forma_pagamento = ?, parcelas = ?, valor = ?, taxa_pct = ?, valor_liquido = ?, data_vencimento = ?, data_pagamento = ?, status_pagamento = ?
		WHERE pagamento_id = ?`
	return db.ExecPorID(ctx, r.q, arqPagamentos, "Pagamentos.Atualizar", "vendasmysql: update pagamento", vendas.ErrPagamentoNaoEncontrado,
		q, p.FormaPagamento, p.Parcelas, p.Valor, p.TaxaPct, p.ValorLiquido, p.DataVencimento,
		nullTime(p.DataPagamento), p.StatusPagamento, id)
}

// AtualizarSeNaoQuitado é o Atualizar do perfil restrito (SEC-27): o
// UPDATE só casa se o pagamento NÃO estiver quitado, no mesmo statement
// (sem janela entre ler e gravar). Nenhuma linha casada (inexistente ou
// quitado) vira ErrPagamentoNaoEncontrado; o caso de uso relê para
// distinguir o 404 do 409.
func (r *Pagamentos) AtualizarSeNaoQuitado(ctx context.Context, id int64, p *vendas.Pagamento) error {
	const q = `
		UPDATE pagamentos
		SET forma_pagamento = ?, parcelas = ?, valor = ?, taxa_pct = ?, valor_liquido = ?, data_vencimento = ?, data_pagamento = ?, status_pagamento = ?
		WHERE pagamento_id = ? AND status_pagamento NOT IN (?, ?)`
	return db.ExecPorID(ctx, r.q, arqPagamentos, "Pagamentos.AtualizarSeNaoQuitado", "vendasmysql: update pagamento nao quitado", vendas.ErrPagamentoNaoEncontrado,
		q, p.FormaPagamento, p.Parcelas, p.Valor, p.TaxaPct, p.ValorLiquido, p.DataVencimento,
		nullTime(p.DataPagamento), p.StatusPagamento, id, string(vendas.StatusPago), string(vendas.StatusPagoComAtraso))
}

// ExistePorPedido informa se há pagamento do pedido (trava V-01 da exclusão).
func (r *Pagamentos) ExistePorPedido(ctx context.Context, pedidoID int64) (bool, error) {
	return db.Existe(ctx, r.q, arqPagamentos, "Pagamentos.ExistePorPedido", "vendasmysql: exists pagamento por pedido_id",
		`SELECT 1 FROM pagamentos WHERE pedido_id = ? LIMIT 1`, pedidoID)
}

// TravarParaExcluir lê pedido e status com SELECT … FOR UPDATE (SEC-35):
// fixa o pagamento até o fim da tx da exclusão. Preenche PagamentoID,
// PedidoID e StatusPagamento; vendas.ErrPagamentoNaoEncontrado sem linha.
func (r *Pagamentos) TravarParaExcluir(ctx context.Context, id int64) (*vendas.Pagamento, error) {
	p := &vendas.Pagamento{PagamentoID: id}
	err := r.q.QueryRowContext(ctx, `SELECT pedido_id, status_pagamento FROM pagamentos WHERE pagamento_id = ? FOR UPDATE`, id).
		Scan(&p.PedidoID, &p.StatusPagamento)
	if errors.Is(err, sql.ErrNoRows) {
		vlog.Printf(arqPagamentos, "Pagamentos.TravarParaExcluir", "SELECT pagamentos FOR UPDATE linhas=0")
		return nil, vendas.ErrPagamentoNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("vendasmysql: lock pagamento para exclusao: %w", err)
	}
	vlog.Printf(arqPagamentos, "Pagamentos.TravarParaExcluir", "SELECT pagamentos FOR UPDATE linhas=1")
	return p, nil
}

// ExcluirSeNaoQuitado remove o pagamento (hard delete: não há deleted_at)
// só se ele NÃO estiver quitado, no mesmo statement (SEC-35, defesa junto
// com o lock). Nenhuma linha casada vira vendas.ErrPagamentoNaoEncontrado.
func (r *Pagamentos) ExcluirSeNaoQuitado(ctx context.Context, id int64) error {
	return db.ExecPorID(ctx, r.q, arqPagamentos, "Pagamentos.ExcluirSeNaoQuitado", "vendasmysql: delete pagamento nao quitado", vendas.ErrPagamentoNaoEncontrado,
		`DELETE FROM pagamentos WHERE pagamento_id = ? AND status_pagamento NOT IN (?, ?)`,
		id, string(vendas.StatusPago), string(vendas.StatusPagoComAtraso))
}

// nullTime grava nil como NULL (data_pagamento é opcional).
func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}
