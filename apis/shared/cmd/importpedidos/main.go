// Command importpedidos lê dados/erp/pedidos.csv e dados/erp/itens_pedido.csv
// e importa (upsert) os registros nas tabelas `pedidos` e `itens_pedido`,
// nessa ordem. A lógica fica em importers/pedidos.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/importpedidos
//	cd apis/shared && go run ./cmd/importpedidos -pedidos-csv=/caminho/pedidos.csv -itens-csv=/caminho/itens_pedido.csv
//	cd apis/shared && go run ./cmd/importpedidos -dry-run
package main

import (
	"flag"
	"log"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/importers/pedidos"
	"github.com/rotaperfumes/shared/vlog"
)

func main() {
	vlog.Printf("main.go", "main", "registrando flag -pedidos-csv")
	pedidosCSVFlag := flag.String("pedidos-csv", "", "caminho do CSV de pedidos (default: dados/erp/pedidos.csv na raiz do projeto)")
	vlog.Printf("main.go", "main", "registrando flag -itens-csv")
	itensCSVFlag := flag.String("itens-csv", "", "caminho do CSV de itens de pedido (default: dados/erp/itens_pedido.csv na raiz do projeto)")
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
		log.Fatalf("%s: falha ao carregar config: %v", pedidos.Tag, err)
	}
	vlog.SetEnabled(cfg.Verbose)

	vlog.Printf("main.go", "main", "montando opções do comando pedidos")
	opts := pedidos.Options{PedidosCSVFlag: *pedidosCSVFlag, ItensCSVFlag: *itensCSVFlag, DryRun: *dryRun}
	vlog.Printf("main.go", "main", "executando pedidos.Run com conexão MySQL (DSN não logado) e verificando erro")
	if err := pedidos.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", pedidos.Tag, err)
	}
}
