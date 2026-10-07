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
	"github.com/rotaperfumes/shared/vlog"
)

func main() {
	vlog.Printf("main.go", "main", "registrando flag -csv")
	csvPathFlag := flag.String("csv", "", "caminho do CSV de estoque (default: dados/erp/estoque.csv na raiz do projeto)")
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
		log.Fatalf("%s: falha ao carregar config: %v", estoque.Tag, err)
	}
	vlog.SetEnabled(cfg.Verbose)

	vlog.Printf("main.go", "main", "montando opções do comando estoque")
	opts := estoque.Options{CSVFlag: *csvPathFlag, DryRun: *dryRun}
	vlog.Printf("main.go", "main", "executando estoque.Run com conexão MySQL (DSN não logado) e verificando erro")
	if err := estoque.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", estoque.Tag, err)
	}
}
