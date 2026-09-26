// Command importoportunidades lê dados/crm/oportunidades.csv e importa
// (upsert) os registros na tabela `oportunidades`. A lógica fica em
// importers/oportunidades.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importoportunidades
//	cd apis/shared && go run ./cmd/importoportunidades -csv=/caminho/alternativo/oportunidades.csv
//	cd apis/shared && go run ./cmd/importoportunidades -dry-run
package main

import (
	"flag"
	"log"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/importers/oportunidades"
)

func main() {
	csvPathFlag := flag.String("csv", "", "caminho do CSV de oportunidades (default: dados/crm/oportunidades.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	cmdutil.LoadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", oportunidades.Tag, err)
	}

	opts := oportunidades.Options{CSVFlag: *csvPathFlag, DryRun: *dryRun}
	if err := oportunidades.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", oportunidades.Tag, err)
	}
}
