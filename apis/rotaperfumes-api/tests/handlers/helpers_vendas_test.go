package handlers_test

import (
	"database/sql"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/rotaperfumes-api/internal/vendashttp"
	"github.com/rotaperfumes/shared/config"
)

// ARQ-05 (§7.2-4): os handlers de Vendas são os do vendashttp, montados pelo
// bootstrap sobre o db do sqlmock (adapters reais, mesmo SQL do legado; o
// escopo é o AutorizadorEscopo do Comercial). A assinatura é a dos
// construtores legados sem o escopo, para o corpo dos testes não mudar.

func novoPedidoHandler(db *sql.DB, _ *config.Config) *vendashttp.PedidoHandler {
	return bootstrap.NovoVendasHTTP(bootstrap.NovoVendas(db, bootstrap.NovoComercial(db).Escopo())).Pedido
}

func novoPagamentoHandler(db *sql.DB, _ *config.Config) *vendashttp.PagamentoHandler {
	return bootstrap.NovoVendasHTTP(bootstrap.NovoVendas(db, bootstrap.NovoComercial(db).Escopo())).Pagamento
}
