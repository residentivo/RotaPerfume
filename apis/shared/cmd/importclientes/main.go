// Command importclientes importa o CSV de clientes (upsert). A lógica (flags, config,
// conexão e o pipeline do importkit) fica em importers/clientes; este main é só a
// composition root.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importclientes
//	cd apis/shared && go run ./cmd/importclientes -dry-run
//	cd apis/shared && go run ./cmd/importclientes -h
package main

import (
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/clientes"
)

func main() {
	os.Exit(cmdutil.Executar(clientes.Tag, clientes.CLI, os.Args[1:], cmdutil.AmbientePadrao()))
}
