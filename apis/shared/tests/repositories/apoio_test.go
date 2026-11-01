package repositories_test

// ARQ-07 (pendência i): os testes de pedidos, pagamentos e da baixa de
// estoque chamam os adapters de vendasmysql/catalogomysql diretamente (os
// repassadores com a assinatura legada saíram). Fica aqui só o Unit of Work
// do pedido com o MESMO bind do bootstrap (pedidos + estoque na tx, §5.2) e o
// escopo construído como o AutorizadorEscopo o devolveria.

import (
	"context"
	"database/sql"
	"github.com/rotaperfumes/shared/analytics"
	"time"

	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasapp"
	"github.com/rotaperfumes/shared/vendas/vendasmysql"
)

// txPedido é o UoW do pedido montado como no bootstrap.
func txPedido(pool *sql.DB) *db.Transactor[vendasapp.TxPedido] {
	return db.NewTransactor(pool, func(q db.Querier) vendasapp.TxPedido {
		return vendasapp.TxPedido{Pedidos: vendasmysql.NovoPedidos(q), Pagamentos: vendasmysql.NovoPagamentos(q), Estoque: catalogomysql.NovoEstoques(q)}
	})
}

// inserirPedido grava cabeçalho e itens na tx do UoW (os ids voltam nos itens).
func inserirPedido(ctx context.Context, pool *sql.DB, p *vendas.Pedido, itens []vendas.ItemPedido) error {
	p.Itens = itens
	return txPedido(pool).WithTx(ctx, func(r vendasapp.TxPedido) error { return r.Pedidos.Inserir(ctx, p) })
}

// atualizarPedido roda o roteiro do PUT de produção (vendasapp.TxPedido.Atualizar) na tx.
func atualizarPedido(ctx context.Context, pool *sql.DB, id int64, p *vendas.Pedido, itens []vendas.ItemPedido) error {
	p.Itens = itens
	return txPedido(pool).WithTx(ctx, func(r vendasapp.TxPedido) error { return r.Atualizar(ctx, escopoAdmin(), id, p, time.Now()) })
}

// escopoAdmin é o escopo Irrestrito (admin, sem filtro de carteira).
func escopoAdmin() kernel.Escopo {
	admin, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	e, _ := kernel.EscopoIrrestrito(admin)
	return e
}

// escopoVendedor é o escopo restrito à carteira do vendedor.
func escopoVendedor(id int64) kernel.Escopo {
	e, _ := kernel.EscopoDoVendedor(id)
	return e
}

// escopoDe: 0 = admin (Irrestrito); > 0 = carteira do vendedor.
func escopoDe(vendedorID int64) kernel.Escopo {
	if vendedorID > 0 {
		return escopoVendedor(vendedorID)
	}
	return escopoAdmin()
}

// periodoDe é o parse do HTTP (today|week|month; fora do enum, month).
func periodoDe(s string) analytics.Periodo {
	p, _ := analytics.ParsePeriodo(s)
	return p
}

// valorEQtd desembrulha os totais de vendas (valor, quantidade, erro).
func valorEQtd(t analytics.TotaisVendas, err error) (float64, int, error) {
	return t.Valor, t.Quantidade, err
}

// tempoDe converte o NullTime das tabelas de casos no ponteiro do domínio
// (nil ou inválido = NULL).
func tempoDe(d *sql.NullTime) *time.Time {
	if d == nil || !d.Valid {
		return nil
	}
	return &d.Time
}
