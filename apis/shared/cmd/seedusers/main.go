// Command seedusers gera hashes bcrypt para os placeholders nos SQLs de seed
// e executa os arquivos contra o MySQL. A lógica fica em tools/seedusers.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/seedusers
package main

import (
	"flag"
	"log"
	"os"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tools/seedusers"
)

func main() {
	var opts seedusers.Options
	flag.BoolVar(&opts.NoExec, "no-exec", false, "apenas substitui placeholders; não executa SQL no MySQL")
	flag.BoolVar(&opts.DryRun, "dry-run", false, "imprime hashes gerados sem modificar arquivos")
	flag.BoolVar(&opts.ShowPassword, "show-password", false, "exibe as senhas de seed geradas/usadas no console (cuidado: evite em ambientes compartilhados)")
	flag.Parse()

	// Carrega .env da raiz do projeto (sobe diretórios a partir de cwd).
	cmdutil.LoadEnvFromCwd()

	// Carrega config (lê .env via os.Getenv; o Makefile exporta antes de chamar)
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", seedusers.Tag, err)
	}

	deps := seedusers.Deps{Out: os.Stdout, ProjectRoot: seedusers.FindProjectRoot, RunSQL: seedusers.RunMySQL}
	if err := seedusers.Run(cfg, opts, deps); err != nil {
		log.Fatalf("%s: %v", seedusers.Tag, err)
	}
}
