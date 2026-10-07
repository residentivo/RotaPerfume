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
	"github.com/rotaperfumes/shared/vlog"
)

func main() {
	vlog.Printf("main.go", "main", "registrando flag -pagamentos-csv")
	pagamentosCSVFlag := flag.String("pagamentos-csv", "", "caminho do CSV de pagamentos (default: dados/erp/pagamentos.csv na raiz do projeto)")
	vlog.Printf("main.go", "main", "registrando flag -dry-run")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	vlog.Printf("main.go", "main", "interpretando flags da linha de comando")
	flag.Parse()

	vlog.Printf("main.go", "main", "carregando .env a partir do diretório atual")
	cmdutil.LoadEnvFromCwd()

	vlog.Printf("main.go", "main", "carregando configuração (segredos não logados)")
	cfg, err := config.Load()
	vlog.Printf("main.go", "main", "verificando se err != nil após config.Load")
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", pagamentos.Tag, err)
	}
	vlog.SetEnabled(cfg.Verbose)

	vlog.Printf("main.go", "main", "montando opções do comando pagamentos")
	opts := pagamentos.Options{PagamentosCSVFlag: *pagamentosCSVFlag, DryRun: *dryRun}
	vlog.Printf("main.go", "main", "executando pagamentos.Run com conexão MySQL (DSN não logado) e verificando erro")
	if err := pagamentos.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", pagamentos.Tag, err)
	}
}
