package exportdados

import "github.com/rotaperfumes/shared/cmdutil"

// CLI é o comando cmd/exportdados (flag -out); o main só chama
// cmdutil.Executar.
func CLI(args []string, amb cmdutil.Ambiente) error {
	fs := cmdutil.NovasFlags(Tag, amb)
	out := fs.String("out", "", "diretório de destino da exportação (default: export na raiz do projeto)")
	if err := cmdutil.LerFlags(fs, args); err != nil {
		return err
	}
	cfg, err := cmdutil.Configurar(amb, false)
	if err != nil {
		return err
	}
	return Run(Options{OutFlag: *out}, amb.Abrir(cfg.DSN()))
}
