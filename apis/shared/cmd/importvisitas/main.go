// Command importvisitas lê dados/crm/visitas.csv e importa
// (upsert) os registros na tabela `visitas`. A lógica fica em
// importers/visitas.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importvisitas
//	cd apis/shared && go run ./cmd/importvisitas -csv=/caminho/alternativo/visitas.csv
//	cd apis/shared && go run ./cmd/importvisitas -dry-run
package main

import (
	"flag"
	"log"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/importers/visitas"
)

func main() {
	csvPathFlag := flag.String("csv", "", "caminho do CSV de visitas (default: dados/crm/visitas.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	cmdutil.LoadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", visitas.Tag, err)
	}

	opts := visitas.Options{CSVFlag: *csvPathFlag, DryRun: *dryRun}
	if err := visitas.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", visitas.Tag, err)
	}
}
