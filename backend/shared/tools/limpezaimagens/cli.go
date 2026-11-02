package limpezaimagens

import (
	"errors"

	"github.com/rotaperfumes/shared/cmdutil"
)

// errFalhasAoApagar: a execução terminou, mas algum arquivo não pôde ser
// apagado (o total está no log).
var errFalhasAoApagar = errors.New("houve falhas ao apagar arquivos (ver falhas_ao_apagar no log)")

// CLI é o comando cmd/limpezaimagens (flag -dry-run); o main só chama
// cmdutil.Executar. O diretório vem de PRODUTO_IMAGENS_DIR (config.Load).
func CLI(args []string, amb cmdutil.Ambiente) error {
	fs := cmdutil.NovasFlags(Tag, amb)
	dryRun := fs.Bool("dry-run", false, "só relata os totais, sem apagar nada")
	if err := cmdutil.LerFlags(fs, args); err != nil {
		return err
	}
	cfg, err := cmdutil.Configurar(amb, false)
	if err != nil {
		return err
	}
	r, err := Run(Options{Dir: cfg.ProdutoImagensDir, DryRun: *dryRun}, amb.Abrir(cfg.DSN()))
	if err != nil {
		return err
	}
	if r.FalhasAoApagar > 0 {
		return errFalhasAoApagar
	}
	return nil
}
