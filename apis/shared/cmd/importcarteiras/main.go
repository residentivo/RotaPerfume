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
	"github.com/rotaperfumes/shared/vlog"
)

func main() {
	vlog.Printf("main.go", "main", "registrando flag -csv")
	csvPathFlag := flag.String("csv", "", "caminho do CSV de carteiras (default: dados/crm/carteira.csv na raiz do projeto)")
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
		log.Fatalf("%s: falha ao carregar config: %v", carteiras.Tag, err)
	}
	vlog.SetEnabled(cfg.Verbose)

	vlog.Printf("main.go", "main", "montando opções do comando carteiras")
	opts := carteiras.Options{CSVFlag: *csvPathFlag, DryRun: *dryRun}
	vlog.Printf("main.go", "main", "executando carteiras.Run com conexão MySQL (DSN não logado) e verificando erro")
	if err := carteiras.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", carteiras.Tag, err)
	}
}
