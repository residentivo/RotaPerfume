// Command importoportunidades importa o CSV de oportunidades (upsert). A lógica (flags, config,
// conexão e o pipeline do importkit) fica em importers/oportunidades; este main é só a
// composition root.
//
// Uso:
//
//	cd backend/shared && go run ./cmd/importoportunidades
//	cd backend/shared && go run ./cmd/importoportunidades -dry-run
//	cd backend/shared && go run ./cmd/importoportunidades -h
package main

import (
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/importers/oportunidades"
)

func main() {
	os.Exit(cmdutil.Executar(oportunidades.Tag, oportunidades.CLI, os.Args[1:], cmdutil.AmbientePadrao()))
}
