package importkit

import "github.com/rotaperfumes/shared/cmdutil"

// AjudaDryRun é o texto da flag -dry-run de todos os importadores.
const AjudaDryRun = "faz parsing e validação sem gravar no banco"

// CLI monta o comando de um importador de um CSV só (flags -csv e -dry-run):
// lê as flags, carrega o .env e a config (com as credenciais do banco, como
// sempre foi) e chama run com a conexão MariaDB. O `main` só faz
// os.Exit(cmdutil.Executar(...)).
func CLI(tag, ajudaCSV string, run func(csv string, dryRun bool, open cmdutil.Opener) error) cmdutil.Comando {
	return func(args []string, amb cmdutil.Ambiente) error {
		fs := cmdutil.NovasFlags(tag, amb)
		csv := fs.String("csv", "", ajudaCSV)
		dryRun := fs.Bool("dry-run", false, AjudaDryRun)
		if err := cmdutil.LerFlags(fs, args); err != nil {
			return err
		}
		cfg, err := cmdutil.Configurar(amb, false)
		if err != nil {
			return err
		}
		return run(*csv, *dryRun, amb.Abrir(cfg.DSN()))
	}
}
