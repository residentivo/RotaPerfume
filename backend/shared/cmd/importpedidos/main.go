// Command importpedidos importa o CSV de pedidos (upsert). A lógica (flags, config,
// conexão e o pipeline do importkit) fica em importers/pedidos; este main é só a
// composition root.
//
// Uso:
//
//	cd backend/shared && go run ./cmd/importpedidos
//	cd backend/shared && go run ./cmd/importpedidos -dry-run
//	cd backend/shared && go run ./cmd/importpedidos -h
package main

import (
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/pedidos"
)

func main() {
	os.Exit(cmdutil.Executar(pedidos.Tag, pedidos.CLI, os.Args[1:], cmdutil.AmbientePadrao()))
}
