// Command importvisitas importa o CSV de visitas (upsert). A lógica (flags, config,
// conexão e o pipeline do importkit) fica em importers/visitas; este main é só a
// composition root.
//
// Uso:
//
//	cd backend/shared && go run ./cmd/importvisitas
//	cd backend/shared && go run ./cmd/importvisitas -dry-run
//	cd backend/shared && go run ./cmd/importvisitas -h
package main

import (
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/visitas"
)

func main() {
	os.Exit(cmdutil.Executar(visitas.Tag, visitas.CLI, os.Args[1:], cmdutil.AmbientePadrao()))
}
