// Command importpagamentos lê dados/erp/pagamentos.csv e importa (upsert)
// os registros na tabela `pagamentos`. A lógica fica em importers/pagamentos.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importpagamentos
//	cd apis/shared && go run ./cmd/importpagamentos -pagamentos-csv=/caminho/pagamentos.csv
//	cd apis/shared && go run ./cmd/importpagamentos -dry-run
package main

import (
	"flag"
	"log"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/importers/pagamentos"
)

func main() {
	pagamentosCSVFlag := flag.String("pagamentos-csv", "", "caminho do CSV de pagamentos (default: dados/erp/pagamentos.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	cmdutil.LoadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", pagamentos.Tag, err)
	}

	opts := pagamentos.Options{PagamentosCSVFlag: *pagamentosCSVFlag, DryRun: *dryRun}
	if err := pagamentos.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", pagamentos.Tag, err)
	}
}
