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
)

func main() {
	pedidosCSVFlag := flag.String("pedidos-csv", "", "caminho do CSV de pedidos (default: dados/erp/pedidos.csv na raiz do projeto)")
	itensCSVFlag := flag.String("itens-csv", "", "caminho do CSV de itens de pedido (default: dados/erp/itens_pedido.csv na raiz do projeto)")
	dryRun := flag.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	flag.Parse()

	cmdutil.LoadEnvFromCwd()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", pedidos.Tag, err)
	}

	opts := pedidos.Options{PedidosCSVFlag: *pedidosCSVFlag, ItensCSVFlag: *itensCSVFlag, DryRun: *dryRun}
	if err := pedidos.Run(opts, cmdutil.OpenMySQL(cfg.DSN())); err != nil {
		log.Fatalf("%s: %v", pedidos.Tag, err)
	}
}
