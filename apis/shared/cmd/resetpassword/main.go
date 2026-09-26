// Command resetpassword cria ou atualiza usuários no banco, com hash bcrypt
// válido. A lógica fica em tools/resetpassword.
//
// Uso:
//
//	cd apis/shared && go run ./cmd/resetpassword -list
//	cd apis/shared && go run ./cmd/resetpassword -email=admin@rotaperfumes.com.br -password=Senha123 -role=admin
//	cd apis/shared && go run ./cmd/resetpassword -all-users -password=SenhaPadrao123
//	cd apis/shared && go run ./cmd/resetpassword -create-admin -password=Admin@123
package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/tools/resetpassword"
	// Este binário não usa config.DSN(); importa tz diretamente para fixar
	// time.Local em -03:00 antes do sql.Open (RISCO-01).
	_ "github.com/rotaperfumes/shared/tz"
)

func main() {
	var opts resetpassword.Options
	flag.StringVar(&opts.Email, "email", "", "email do usuário a criar/atualizar")
	flag.StringVar(&opts.Password, "password", "", "senha em texto puro (vazio = gera aleatória de 16 chars)")
	flag.StringVar(&opts.Role, "role", "normal", "papel (admin|normal) — usado ao criar novo usuário")
	flag.StringVar(&opts.Nome, "nome", "", "nome completo — usado ao criar novo usuário (padrão: derivado do email)")
	flag.Int64Var(&opts.IDVendedor, "id-vendedor", 0, "id do vendedor vinculado (opcional, usado ao criar)")
	flag.BoolVar(&opts.AllUsers, "all-users", false, "atualiza todos os PLACEHOLDER com a mesma senha")
	flag.BoolVar(&opts.CreateAdmin, "create-admin", false, "garante que o admin principal existe (cria se faltar)")
	flag.BoolVar(&opts.List, "list", false, "lista os usuários atuais")
	flag.Parse()

	cmdutil.LoadEnvFromCwd()

	db, err := sql.Open("mysql", resetpassword.DSN())
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
