// Command seedusers gera hashes bcrypt para os placeholders dos SQLs de seed,
// grava cópias renderizadas em <raiz>/tmp/seed (sql/ nunca é alterado) e as
// executa contra o MySQL. A lógica fica em tools/seedusers.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/seedusers            # exige DB_USUARIO/DB_SENHA no .env; apaga tmp/seed ao final
//	cd apis/shared && go run ./cmd/seedusers -no-exec   # mantém os SQLs em tmp/seed (apague depois)
//	cd apis/shared && go run ./cmd/seedusers -dry-run   # só imprime os hashes
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
	flag.BoolVar(&opts.NoExec, "no-exec", false, "apenas grava os SQLs com hash em tmp/seed (mantidos; apague depois); não executa no MySQL")
	flag.BoolVar(&opts.DryRun, "dry-run", false, "imprime hashes gerados sem gravar arquivos")
	flag.BoolVar(&opts.ShowPassword, "show-password", false, "exibe as senhas de seed geradas/usadas no console (cuidado: evite em ambientes compartilhados)")
	flag.Parse()

	// Carrega .env da raiz do projeto (sobe diretórios a partir de cwd).
	cmdutil.LoadEnvFromCwd()

	// Carrega config (lê .env via os.Getenv; o Makefile exporta antes de chamar)
	cfg, err := carregarConfig(opts)
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", seedusers.Tag, err)
	}

	deps := seedusers.Deps{Out: os.Stdout, ProjectRoot: cmdutil.FindProjectRoot, RunSQL: seedusers.RunMySQL}
	if err := seedusers.Run(cfg, opts, deps); err != nil {
		log.Fatalf("%s: %v", seedusers.Tag, err)
	}
}

// carregarConfig não exige DB_USUARIO/DB_SENHA em -dry-run/-no-exec, que não
// tocam o banco (SEC-10); a execução real usa o Load completo (SEC-11).
func carregarConfig(opts seedusers.Options) (*config.Config, error) {
	if opts.DryRun || opts.NoExec {
		return config.LoadSemCredenciaisDB()
	}
	return config.Load()
}
