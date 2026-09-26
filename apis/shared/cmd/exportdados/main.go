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
)

func main() {
	outFlag := flag.String("out", "", "diretório de destino da exportação (default: export na raiz do projeto)")
	flag.Parse()

	cmdutil.LoadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", exportdados.Tag, err)
	}

	opts := exportdados.Options{OutFlag: *outFlag}
	if err := exportdados.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", exportdados.Tag, err)
	}
}
