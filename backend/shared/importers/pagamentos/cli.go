package pagamentos

import "github.com/rotaperfumes/shared/cmdutil"

// CLI é o comando cmd/importpagamentos (flags -pagamentos-csv e -dry-run);
// o main só chama cmdutil.Executar.
func CLI(args []string, amb cmdutil.Ambiente) error {
	fs := cmdutil.NovasFlags(Tag, amb)
	csv := fs.String("pagamentos-csv", "", "caminho do CSV de pagamentos (default: dados/erp/pagamentos.csv na raiz do projeto)")
	dryRun := fs.Bool("dry-run", false, "faz parsing e validação sem gravar no banco")
	if err := cmdutil.LerFlags(fs, args); err != nil {
		return err
	}
	cfg, err := cmdutil.Configurar(amb, false)
	if err != nil {
		return err
	}
	return Run(Options{PagamentosCSVFlag: *csv, DryRun: *dryRun}, amb.Abrir(cfg.DSN()))
}
