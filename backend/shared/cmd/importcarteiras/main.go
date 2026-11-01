// Command importcarteiras importa o CSV de carteiras (upsert). A lógica (flags, config,
// conexão e o pipeline do importkit) fica em importers/carteiras; este main é só a
// composition root.
//
// Uso:
//
//	cd backend/shared && go run ./cmd/importcarteiras
//	cd backend/shared && go run ./cmd/importcarteiras -dry-run
//	cd backend/shared && go run ./cmd/importcarteiras -h
package main

import (
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/carteiras"
)

func main() {
	os.Exit(cmdutil.Executar(carteiras.Tag, carteiras.CLI, os.Args[1:], cmdutil.AmbientePadrao()))
}
