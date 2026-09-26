// Command importclientes lê dados/crm/clientes.csv e importa (upsert) os
// registros na tabela `clientes`. A lógica fica em importers/clientes.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importclientes
//	cd apis/shared && go run ./cmd/importclientes -csv=/caminho/alternativo/clientes.csv
//	cd apis/shared && go run ./cmd/importclientes -dry-run
package main

import (
	"flag"
	"log"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/importers/clientes"
)

func main() {
	csvPathFlag := flag.String("csv", "", "caminho do CSV de clientes (default: dados/crm/clientes.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	cmdutil.LoadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", clientes.Tag, err)
	}

	opts := clientes.Options{CSVFlag: *csvPathFlag, DryRun: *dryRun}
	if err := clientes.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", clientes.Tag, err)
	}
}
