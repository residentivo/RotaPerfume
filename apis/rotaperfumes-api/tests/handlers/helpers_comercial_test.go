package handlers_test

import (
	"database/sql"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/rotaperfumes-api/internal/comercialhttp"
	"github.com/rotaperfumes/shared/config"
)

// ARQ-04 (§7.2-4): os handlers do Comercial são os do comercialhttp,
// montados pelo bootstrap sobre o db do sqlmock (adapters reais, mesmo SQL
// do legado). A assinatura é a dos construtores legados, para o corpo dos
// testes não mudar. O Dashboard legado recebe o mesmo AutorizadorEscopo do
// bootstrap (ponte handlers/scope.go até o ARQ-06).

func novoVendedorHandler(db *sql.DB, _ *config.Config) *comercialhttp.VendedorHandler {
	return bootstrap.NovoComercialHTTP(bootstrap.NovoComercial(db)).Vendedor
}

func novoDashboardHandler(db *sql.DB, cfg *config.Config) *handlers.DashboardHandler {
	return handlers.NewDashboardHandler(db, cfg, bootstrap.NovoComercial(db).Escopo())
}
