// Command importprodutos lê dados/erp/produtos.csv e importa (upsert) os
// registros na tabela `produtos`. A lógica fica em importers/produtos.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importprodutos
//	cd apis/shared && go run ./cmd/importprodutos -csv=/caminho/alternativo/produtos.csv
//	cd apis/shared && go run ./cmd/importprodutos -dry-run
package main

import (
	"flag"
	"log"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/importers/produtos"
)

func main() {
	csvPathFlag := flag.String("csv", "", "caminho do CSV de produtos (default: dados/erp/produtos.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	cmdutil.LoadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", produtos.Tag, err)
	}

	opts := produtos.Options{CSVFlag: *csvPathFlag, DryRun: *dryRun}
	if err := produtos.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", produtos.Tag, err)
	}
}
