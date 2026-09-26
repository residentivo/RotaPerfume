// Command importcarteiras lê dados/crm/carteira.csv e importa (upsert) os
// registros na tabela `carteiras`. A lógica fica em importers/carteiras.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importcarteiras
//	cd apis/shared && go run ./cmd/importcarteiras -csv=/caminho/alternativo/carteira.csv
//	cd apis/shared && go run ./cmd/importcarteiras -dry-run
package main

import (
	"flag"
	"log"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/importers/carteiras"
)

func main() {
	csvPathFlag := flag.String("csv", "", "caminho do CSV de carteiras (default: dados/crm/carteira.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	cmdutil.LoadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", carteiras.Tag, err)
	}

	opts := carteiras.Options{CSVFlag: *csvPathFlag, DryRun: *dryRun}
	if err := carteiras.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", carteiras.Tag, err)
	}
}
