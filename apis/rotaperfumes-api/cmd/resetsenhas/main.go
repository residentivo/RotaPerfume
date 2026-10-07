// Command resetsenhas redefine a senha de TODOS os usuários ativos (OPS-01),
// com o mesmo fluxo do POST /api/admin/reset-password: senha aleatória, hash
// Argon2id com o PASSWORD_PEPPER, troca obrigatória no próximo login, e-mail
// com a nova senha, registro em senha_historico e revogação das sessões.
//
// Uso (a partir de apis/rotaperfumes-api):
//
//	go run ./cmd/resetsenhas             # simulação: só lista quem seria atingido
//	go run ./cmd/resetsenhas -executar   # grava e envia os e-mails
//
// Recusa rodar sem SMTP configurado: a senha nunca é impressa nem logada, então
// sem e-mail ela se perderia. Para no primeiro e-mail que falhar.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/db"
	sharedsvc "github.com/rotaperfumes/shared/services"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

func main() {
	executar := flag.Bool("executar", false, "grava as novas senhas e envia os e-mails (sem esta flag, só simula)")
	flag.Parse()

	cmdutil.LoadEnvFromCwd()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("resetsenhas: config: %v", err)
	}
	emailSvc, err := sharedsvc.NewSMTPEmailService(cfg)
	if err != nil {
		log.Fatalf("resetsenhas: SMTP obrigatório (a senha só chega ao usuário por e-mail): %v", err)
	}

	conn, err := db.Open(cfg.DSN())
	if err != nil {
		log.Fatalf("resetsenhas: db: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		log.Fatalf("resetsenhas: db ping: %v", err)
	}

	svc := services.NewResetSenhasAtivosService(services.NewUsuarioService(conn, cfg, emailSvc))
	fmt.Printf("Banco: %s@%s:%s | SMTP: %s (from %s) | modo: %s\n",
		cfg.DBName, cfg.DBHost, cfg.DBPort, cfg.SMTPHost, cfg.SMTPFrom, modo(*executar))

	itens, err := svc.Executar(ctx, conn, !*executar)
	imprimir(itens)
	if err != nil {
		if errors.Is(err, services.ErrEmailResetFalhou) {
			fmt.Fprintln(os.Stderr, "PARADO: o e-mail do último usuário falhou. A senha dele já foi trocada;")
			fmt.Fprintln(os.Stderr, "corrija o SMTP e redefina essa senha pelo admin (ou rode de novo).")
		}
		log.Fatalf("resetsenhas: %v", err)
	}
	if !*executar {
		fmt.Println("Simulação: nada foi gravado nem enviado. Rode com -executar para aplicar.")
	}
}

func modo(executar bool) string {
	if executar {
		return "EXECUTAR"
	}
	return "simulação"
}

func imprimir(itens []services.ItemResetMassa) {
	fmt.Printf("%-5s %-45s %-7s %s\n", "ID", "EMAIL", "ROLE", "STATUS")
	cont := map[string]int{}
	for _, it := range itens {
		fmt.Printf("%-5d %-45s %-7s %s\n", it.ID, it.Email, it.Role, it.Status)
		cont[it.Status]++
	}
	fmt.Printf("Total: %d | resetados: %d | simulados: %d | falha de e-mail: %d\n",
		len(itens), cont["resetado"], cont["simulado"], cont["falha_email"])
}
