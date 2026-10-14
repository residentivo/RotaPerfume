package handlers_test

import (
	"database/sql"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/rotaperfumes-api/internal/catalogohttp"
	"github.com/rotaperfumes/shared/config"
)

// ARQ-02 (§7.2-4): os handlers do Catálogo são os do catalogohttp, montados
// pelo bootstrap sobre o db do sqlmock (adapters reais, mesmo SQL do legado).
// A assinatura é a dos construtores legados, para o corpo dos testes não mudar.

func novoProdutoHandler(db *sql.DB, _ *config.Config) *catalogohttp.ProdutoHandler {
	p, _ := bootstrap.NovoCatalogoHTTP(db)
	return p
}

func novoEstoqueHandler(db *sql.DB, _ *config.Config) *catalogohttp.EstoqueHandler {
	_, e := bootstrap.NovoCatalogoHTTP(db)
	return e
}
