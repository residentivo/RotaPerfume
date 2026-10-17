// Command exportdados exporta as tabelas do banco de volta para arquivos CSV
// no mesmo formato/estrutura de dados/ (crm/ e erp/). A lógica fica em
// tools/exportdados; este main é só a composition root.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/exportdados
//	cd apis/shared && go run ./cmd/exportdados -out=/caminho/alternativo/export
package main

import (
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/tools/exportdados"
)

func main() {
	os.Exit(cmdutil.Executar(exportdados.Tag, exportdados.CLI, os.Args[1:], cmdutil.AmbientePadrao()))
}
