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
	"github.com/rotaperfumes/shared/vlog"
)

func main() {
	vlog.Printf("main.go", "main", "registrando flag -csv")
	csvPathFlag := flag.String("csv", "", "caminho do CSV de clientes (default: dados/crm/clientes.csv na raiz do projeto)")
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
		log.Fatalf("%s: falha ao carregar config: %v", clientes.Tag, err)
	}
	vlog.SetEnabled(cfg.Verbose)

	vlog.Printf("main.go", "main", "montando opções do comando clientes")
	opts := clientes.Options{CSVFlag: *csvPathFlag, DryRun: *dryRun}
	vlog.Printf("main.go", "main", "executando clientes.Run com conexão MySQL (DSN não logado) e verificando erro")
	if err := clientes.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", clientes.Tag, err)
	}
}
