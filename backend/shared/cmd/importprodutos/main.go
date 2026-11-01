// Command importprodutos importa o CSV de produtos (upsert). A lógica (flags, config,
// conexão e o pipeline do importkit) fica em importers/produtos; este main é só a
// composition root.
//
// Uso:
//
//	cd backend/shared && go run ./cmd/importprodutos
//	cd backend/shared && go run ./cmd/importprodutos -dry-run
//	cd backend/shared && go run ./cmd/importprodutos -h
package main

import (
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/produtos"
)

func main() {
	os.Exit(cmdutil.Executar(produtos.Tag, produtos.CLI, os.Args[1:], cmdutil.AmbientePadrao()))
}
