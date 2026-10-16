// Command importestoque importa o CSV de estoque (upsert). A lógica (flags, config,
// conexão e o pipeline do importkit) fica em importers/estoque; este main é só a
// composition root.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importestoque
//	cd apis/shared && go run ./cmd/importestoque -dry-run
//	cd apis/shared && go run ./cmd/importestoque -h
package main

import (
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/estoque"
)

func main() {
	os.Exit(cmdutil.Executar(estoque.Tag, estoque.CLI, os.Args[1:], cmdutil.AmbientePadrao()))
}
