// Command resetpassword cria ou atualiza usuários no banco, com hash bcrypt
// válido. A lógica fica em tools/resetpassword.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/resetpassword -list
//	cd apis/shared && go run ./cmd/resetpassword -email=admin@rotaperfumes.com.br -password-prompt -role=admin
//	printf '%s\n' "$NOVA_SENHA" | go run ./cmd/resetpassword -all-users -password-stdin
//	cd apis/shared && go run ./cmd/resetpassword -create-admin   # SEED_ADMIN_PASSWORD do .env ou aleatória
//
// Exige DB_USUARIO e DB_SENHA no .env. -password=... ainda funciona, mas é
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
	"github.com/rotaperfumes/shared/tools/resetpassword"
	// Este binário não usa config.DSN(); importa tz diretamente para fixar
	// time.Local em -03:00 antes do sql.Open (RISCO-01).
	_ "github.com/rotaperfumes/shared/tz"
)

func main() {
	var opts resetpassword.Options
	flag.StringVar(&opts.Email, "email", "", "email do usuário a criar/atualizar")
	flag.StringVar(&opts.Password, "password", "", "DEPRECIADA (fica no histórico do shell): senha em texto puro; prefira -password-prompt ou -password-stdin")
	flag.BoolVar(&opts.PasswordStdin, "password-stdin", false, "lê a senha da 1ª linha do stdin")
	flag.BoolVar(&opts.PasswordPrompt, "password-prompt", false, "pede a senha no terminal, sem eco e com confirmação (Git Bash: use winpty)")
	flag.StringVar(&opts.Role, "role", "normal", "papel (admin|normal) — usado ao criar novo usuário")
	flag.StringVar(&opts.Nome, "nome", "", "nome completo — usado ao criar novo usuário (padrão: derivado do email)")
	flag.Int64Var(&opts.IDVendedor, "id-vendedor", 0, "id do vendedor vinculado (opcional, usado ao criar)")
	flag.BoolVar(&opts.AllUsers, "all-users", false, "atualiza todos os PLACEHOLDER com a mesma senha")
	flag.BoolVar(&opts.CreateAdmin, "create-admin", false, "garante que o admin principal existe (cria se faltar)")
	flag.BoolVar(&opts.List, "list", false, "lista os usuários atuais")
	flag.Parse()

	// Sem fonte de senha, devolve "" e o Run usa env SEED_* / aleatória.
	stdinFd := int(os.Stdin.Fd())
	senha, err := resetpassword.ResolverSenha(opts, os.Stdin,
		func() bool { return term.IsTerminal(stdinFd) },
		func() ([]byte, error) { return term.ReadPassword(stdinFd) },
		os.Stderr,
	)
	if err != nil {
		log.Fatal(err)
	}
	opts.Password = senha

	cmdutil.LoadEnvFromCwd()

	dsn, err := resetpassword.DSN()
	if err != nil {
		log.Fatalf("resetpassword: %v", err)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("resetpassword: sql.Open: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("resetpassword: ping: %v", err)
	}

	if err := resetpassword.Run(ctx, db, opts, os.Stdout, resetpassword.GenerateRandomPassword); err != nil {
		log.Fatal(err)
	}
}
