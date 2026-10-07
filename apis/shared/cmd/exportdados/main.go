// Command exportdados exporta as tabelas do banco de volta para arquivos CSV
// no mesmo formato/estrutura de dados/ (crm/ e erp/). A lógica fica em
// tools/exportdados.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/exportdados
//	cd apis/shared && go run ./cmd/exportdados -out=/caminho/alternativo/export
package main

import (
	"flag"
	"log"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tools/exportdados"
	"github.com/rotaperfumes/shared/vlog"
)

func main() {
	vlog.Printf("main.go", "main", "registrando flag -out")
	outFlag := flag.String("out", "", "diretório de destino da exportação (default: export na raiz do projeto)")
	vlog.Printf("main.go", "main", "interpretando flags da linha de comando")
	flag.Parse()

	vlog.Printf("main.go", "main", "carregando .env a partir do diretório atual")
	cmdutil.LoadEnvFromCwd()

	vlog.Printf("main.go", "main", "carregando configuração (segredos não logados)")
	cfg, err := config.Load()
	vlog.Printf("main.go", "main", "verificando se err != nil após config.Load")
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", exportdados.Tag, err)
	}
	vlog.SetEnabled(cfg.Verbose)

	vlog.Printf("main.go", "main", "montando opções do comando exportdados")
	opts := exportdados.Options{OutFlag: *outFlag}
	vlog.Printf("main.go", "main", "executando exportdados.Run com conexão MySQL (DSN não logado) e verificando erro")
	if err := exportdados.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", exportdados.Tag, err)
	}
}
