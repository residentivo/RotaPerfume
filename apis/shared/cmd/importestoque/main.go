// Command importestoque lê dados/erp/estoque.csv e importa (upsert) os
// registros na tabela `estoque`. A lógica fica em importers/estoque.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importestoque
//	cd apis/shared && go run ./cmd/importestoque -csv=/caminho/alternativo/estoque.csv
//	cd apis/shared && go run ./cmd/importestoque -dry-run
package main

import (
	"flag"
	"log"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/importers/estoque"
)

func main() {
	csvPathFlag := flag.String("csv", "", "caminho do CSV de estoque (default: dados/erp/estoque.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	cmdutil.LoadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", estoque.Tag, err)
	}

	opts := estoque.Options{CSVFlag: *csvPathFlag, DryRun: *dryRun}
	if err := estoque.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", estoque.Tag, err)
	}
}
