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
	"github.com/rotaperfumes/shared/vlog"
)

func main() {
	vlog.Printf("main.go", "main", "registrando flag -csv")
	csvPathFlag := flag.String("csv", "", "caminho do CSV de produtos (default: dados/erp/produtos.csv na raiz do projeto)")
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
		log.Fatalf("%s: falha ao carregar config: %v", produtos.Tag, err)
	}
	vlog.SetEnabled(cfg.Verbose)

	vlog.Printf("main.go", "main", "montando opções do comando produtos")
	opts := produtos.Options{CSVFlag: *csvPathFlag, DryRun: *dryRun}
	vlog.Printf("main.go", "main", "executando produtos.Run com conexão MySQL (DSN não logado) e verificando erro")
	if err := produtos.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", produtos.Tag, err)
	}
}
