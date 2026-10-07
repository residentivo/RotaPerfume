// Command seedusers gera hashes Argon2id (com pepper) para os placeholders dos SQLs de seed,
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
	"github.com/rotaperfumes/shared/vlog"
)

func main() {
	vlog.Printf("main.go", "main", "declarando opções do seedusers")
	var opts seedusers.Options
	vlog.Printf("main.go", "main", "registrando flag -no-exec")
	flag.BoolVar(&opts.NoExec, "no-exec", false, "apenas grava os SQLs com hash em tmp/seed (mantidos; apague depois); não executa no MySQL")
	vlog.Printf("main.go", "main", "registrando flag -dry-run")
	flag.BoolVar(&opts.DryRun, "dry-run", false, "imprime hashes gerados sem gravar arquivos")
	vlog.Printf("main.go", "main", "registrando flag -show-password")
	flag.BoolVar(&opts.ShowPassword, "show-password", false, "exibe as senhas de seed geradas/usadas no console (cuidado: evite em ambientes compartilhados)")
	vlog.Printf("main.go", "main", "interpretando flags da linha de comando")
	flag.Parse()

	// Carrega .env da raiz do projeto (sobe diretórios a partir de cwd).
	vlog.Printf("main.go", "main", "carregando .env a partir do diretório atual")
	cmdutil.LoadEnvFromCwd()

	// Carrega config (lê .env via os.Getenv; o Makefile exporta antes de chamar)
	vlog.Printf("main.go", "main", "carregando configuração conforme o modo (segredos não logados)")
	cfg, err := carregarConfig(opts)
	vlog.Printf("main.go", "main", "verificando se err != nil após carregarConfig")
	if err != nil {
		log.Fatalf("%s: falha ao carregar config: %v", seedusers.Tag, err)
	}
	vlog.SetEnabled(cfg.Verbose)

	vlog.Printf("main.go", "main", "montando dependências do seedusers")
	deps := seedusers.Deps{Out: os.Stdout, ProjectRoot: cmdutil.FindProjectRoot, RunSQL: seedusers.RunMySQL}
	vlog.Printf("main.go", "main", "executando seedusers.Run (no_exec=%t, dry_run=%t) e verificando erro", opts.NoExec, opts.DryRun)
	if err := seedusers.Run(cfg, opts, deps); err != nil {
		log.Fatalf("%s: %v", seedusers.Tag, err)
	}
}

// carregarConfig não exige DB_USUARIO/DB_SENHA em -dry-run/-no-exec, que não
// tocam o banco (SEC-10); a execução real usa o Load completo (SEC-11).
func carregarConfig(opts seedusers.Options) (*config.Config, error) {
	vlog.Printf("main.go", "carregarConfig", "verificando se o modo dispensa credenciais do banco (dry_run=%t, no_exec=%t)", opts.DryRun, opts.NoExec)
	if opts.DryRun || opts.NoExec {
		return config.LoadSemCredenciaisDB()
	}
	return config.Load()
}
