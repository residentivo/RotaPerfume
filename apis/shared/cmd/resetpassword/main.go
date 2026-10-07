// Command resetpassword cria ou atualiza usuários no banco, com hash Argon2id
// válido (pepper PASSWORD_PEPPER do .env). A lógica fica em tools/resetpassword.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/resetpassword -list
//	cd apis/shared && go run ./cmd/resetpassword -email=admin@rotaperfumes.com.br -password-prompt -role=admin
//	printf '%s\n' "$NOVA_SENHA" | go run ./cmd/resetpassword -all-users -password-stdin
//	cd apis/shared && go run ./cmd/resetpassword -create-admin   # SEED_ADMIN_PASSWORD do .env ou aleatória
//
// Exige DB_USUARIO e DB_SENHA no .env (e PASSWORD_PEPPER, salvo em -list). -password=... ainda funciona, mas é
// depreciada (fica no histórico do shell) e gera aviso no stderr. No Git Bash
// (mintty), -password-prompt precisa de `winpty go run ...`.
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/term"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tools/resetpassword"
	// Este binário não usa config.DSN(); importa tz diretamente para fixar
	// time.Local em -03:00 antes do sql.Open (RISCO-01).
	_ "github.com/rotaperfumes/shared/tz"
	"github.com/rotaperfumes/shared/vlog"
)

func main() {
	vlog.Printf("main.go", "main", "declarando opções do resetpassword")
	var opts resetpassword.Options
	vlog.Printf("main.go", "main", "registrando flag -email")
	flag.StringVar(&opts.Email, "email", "", "email do usuário a criar/atualizar")
	vlog.Printf("main.go", "main", "registrando flag -password")
	flag.StringVar(&opts.Password, "password", "", "DEPRECIADA (fica no histórico do shell): senha em texto puro; prefira -password-prompt ou -password-stdin")
	vlog.Printf("main.go", "main", "registrando flag -password-stdin")
	flag.BoolVar(&opts.PasswordStdin, "password-stdin", false, "lê a senha da 1ª linha do stdin")
	vlog.Printf("main.go", "main", "registrando flag -password-prompt")
	flag.BoolVar(&opts.PasswordPrompt, "password-prompt", false, "pede a senha no terminal, sem eco e com confirmação (Git Bash: use winpty)")
	vlog.Printf("main.go", "main", "registrando flag -role")
	flag.StringVar(&opts.Role, "role", "normal", "papel (admin|normal) — usado ao criar novo usuário")
	vlog.Printf("main.go", "main", "registrando flag -nome")
	flag.StringVar(&opts.Nome, "nome", "", "nome completo — usado ao criar novo usuário (padrão: derivado do email)")
	vlog.Printf("main.go", "main", "registrando flag -id-vendedor")
	flag.Int64Var(&opts.IDVendedor, "id-vendedor", 0, "id do vendedor vinculado (opcional, usado ao criar)")
	vlog.Printf("main.go", "main", "registrando flag -all-users")
	flag.BoolVar(&opts.AllUsers, "all-users", false, "atualiza todos os PLACEHOLDER com a mesma senha")
	vlog.Printf("main.go", "main", "registrando flag -create-admin")
	flag.BoolVar(&opts.CreateAdmin, "create-admin", false, "garante que o admin principal existe (cria se faltar)")
	vlog.Printf("main.go", "main", "registrando flag -list")
	flag.BoolVar(&opts.List, "list", false, "lista os usuários atuais")
	vlog.Printf("main.go", "main", "interpretando flags da linha de comando")
	flag.Parse()

	// Sem fonte de senha, devolve "" e o Run usa env SEED_* / aleatória.
	vlog.Printf("main.go", "main", "obtendo descritor do stdin")
	stdinFd := int(os.Stdin.Fd())
	vlog.Printf("main.go", "main", "resolvendo a fonte da senha (senha não logada)")
	senha, err := resetpassword.ResolverSenha(opts, os.Stdin,
		func() bool { return term.IsTerminal(stdinFd) },
		func() ([]byte, error) { return term.ReadPassword(stdinFd) },
		os.Stderr,
	)
	vlog.Printf("main.go", "main", "verificando se err != nil após ResolverSenha")
	if err != nil {
		log.Fatal(err)
	}
	vlog.Printf("main.go", "main", "atribuindo a senha resolvida às opções (valor não logado)")
	opts.Password = senha

	vlog.Printf("main.go", "main", "carregando .env a partir do diretório atual")
	cmdutil.LoadEnvFromCwd()
	// Sem *Config: replica a regra de cfg.Verbose (config.go) — VERBOSE=true ou LOG_LEVEL=debug.
	vlog.SetEnabled(os.Getenv("VERBOSE") == "true" || os.Getenv("LOG_LEVEL") == "debug")

	// -list não grava senha: dispensa PASSWORD_PEPPER.
	vlog.Printf("main.go", "main", "verificando se é modo -list (list=%t)", opts.List)
	if !opts.List {
		vlog.Printf("main.go", "main", "carregando parâmetros de hash de senha (pepper não logado)")
		hs, err := config.LoadHashSenha()
		vlog.Printf("main.go", "main", "verificando se err != nil após LoadHashSenha")
		if err != nil {
			log.Fatalf("resetpassword: %v", err)
		}
		vlog.Printf("main.go", "main", "atribuindo parâmetros de hash às opções")
		opts.HashSenha = hs
	}

	vlog.Printf("main.go", "main", "montando DSN do banco (DSN não logado)")
	dsn, err := resetpassword.DSN()
	vlog.Printf("main.go", "main", "verificando se err != nil após resetpassword.DSN")
	if err != nil {
		log.Fatalf("resetpassword: %v", err)
	}
	vlog.Printf("main.go", "main", "abrindo handle MySQL via sql.Open")
	db, err := sql.Open("mysql", dsn)
	vlog.Printf("main.go", "main", "verificando se err != nil após sql.Open")
	if err != nil {
		log.Fatalf("resetpassword: sql.Open: %v", err)
	}
	vlog.Printf("main.go", "main", "agendando fechamento do handle MySQL")
	defer db.Close()

	vlog.Printf("main.go", "main", "criando contexto com timeout de 30s")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	vlog.Printf("main.go", "main", "agendando cancel do contexto")
	defer cancel()
	vlog.Printf("main.go", "main", "executando ping no banco e verificando erro")
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("resetpassword: ping: %v", err)
	}

	vlog.Printf("main.go", "main", "executando resetpassword.Run e verificando erro")
	if err := resetpassword.Run(ctx, db, opts, os.Stdout, resetpassword.GenerateRandomPassword); err != nil {
		log.Fatal(err)
	}
}
