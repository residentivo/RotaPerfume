// Command importpagamentos importa o CSV de pagamentos (upsert). A lógica (flags, config,
// conexão e o pipeline do importkit) fica em importers/pagamentos; este main é só a
// composition root.
//
// Uso:
//
//	cd backend/shared && go run ./cmd/importpagamentos
//	cd backend/shared && go run ./cmd/importpagamentos -dry-run
//	cd backend/shared && go run ./cmd/importpagamentos -h
package main

import (
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/pagamentos"
)

func main() {
	os.Exit(cmdutil.Executar(pagamentos.Tag, pagamentos.CLI, os.Args[1:], cmdutil.AmbientePadrao()))
}
