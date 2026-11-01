package pedidos

import "github.com/rotaperfumes/shared/cmdutil"

// CLI é o comando cmd/importpedidos (flags -pedidos-csv, -itens-csv e
// -dry-run); o main só chama cmdutil.Executar.
func CLI(args []string, amb cmdutil.Ambiente) error {
	fs := cmdutil.NovasFlags(Tag, amb)
	pedidosCSV := fs.String("pedidos-csv", "", "caminho do CSV de pedidos (default: dados/erp/pedidos.csv na raiz do projeto)")
	itensCSV := fs.String("itens-csv", "", "caminho do CSV de itens de pedido (default: dados/erp/itens_pedido.csv na raiz do projeto)")
	dryRun := fs.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	if err := cmdutil.LerFlags(fs, args); err != nil {
		return err
	}
	cfg, err := cmdutil.Configurar(amb, false)
	if err != nil {
		return err
	}
	return Run(Options{PedidosCSVFlag: *pedidosCSV, ItensCSVFlag: *itensCSV, DryRun: *dryRun}, amb.Abrir(cfg.DSN()))
}
