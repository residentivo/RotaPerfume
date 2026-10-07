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
	"github.com/rotaperfumes/shared/vlog"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

func main() {
	vlog.Printf("main.go", "main", "chamando flag.Bool e declarando executar")
	executar := flag.Bool("executar", false, "grava as novas senhas e envia os e-mails (sem esta flag, só simula)")
	vlog.Printf("main.go", "main", "chamando flag.Parse")
	flag.Parse()

	vlog.Printf("main.go", "main", "chamando cmdutil.LoadEnvFromCwd")
	cmdutil.LoadEnvFromCwd()
	vlog.Printf("main.go", "main", "chamando config.Load e declarando cfg, err")
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("resetsenhas: config: %v", err)
	}
	vlog.SetEnabled(cfg.Verbose)
	vlog.Printf("main.go", "main", "chamando sharedsvc.NewSMTPEmailService e declarando emailSvc, err")
	emailSvc, err := sharedsvc.NewSMTPEmailService(cfg)
	if err != nil {
		log.Fatalf("resetsenhas: SMTP obrigatório (a senha só chega ao usuário por e-mail): %v", err)
	}

	vlog.Printf("main.go", "main", "chamando db.Open e declarando conn, err")
	conn, err := db.Open(cfg.DSN())
	if err != nil {
		log.Fatalf("resetsenhas: db: %v", err)
	}
	vlog.Printf("main.go", "main", "agendando defer: conn.Close")
	defer conn.Close()

	vlog.Printf("main.go", "main", "chamando context.WithTimeout e declarando ctx, cancel")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	vlog.Printf("main.go", "main", "agendando defer: cancel")
	defer cancel()
	vlog.Printf("main.go", "main", "chamando conn.PingContext e declarando err e verificando condição err != nil")
	if err := conn.PingContext(ctx); err != nil {
		log.Fatalf("resetsenhas: db ping: %v", err)
	}

	vlog.Printf("main.go", "main", "chamando services.NewResetSenhasAtivosService e declarando svc")
	svc := services.NewResetSenhasAtivosService(services.NewUsuarioService(conn, cfg, emailSvc))
	fmt.Printf("Banco: %s@%s:%s | SMTP: %s (from %s) | modo: %s\n",
		cfg.DBName, cfg.DBHost, cfg.DBPort, cfg.SMTPHost, cfg.SMTPFrom, modo(*executar))

	vlog.Printf("main.go", "main", "chamando svc.Executar e declarando itens, err")
	itens, err := svc.Executar(ctx, conn, !*executar)
	vlog.Printf("main.go", "main", "chamando imprimir")
	imprimir(itens)
	vlog.Printf("main.go", "main", "verificando condição err != nil")
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
	vlog.Printf("main.go", "modo", "verificando condição executar")
	if executar {
		return "EXECUTAR"
	}
	return "simulação"
}

func imprimir(itens []services.ItemResetMassa) {
	fmt.Printf("%-5s %-45s %-7s %s\n", "ID", "EMAIL", "ROLE", "STATUS")
	vlog.Printf("main.go", "imprimir", "montando literal map[string]int e declarando cont")
	cont := map[string]int{}
	vlog.Printf("main.go", "imprimir", "iniciando loop range sobre itens")
	for _, it := range itens {
		fmt.Printf("%-5d %-45s %-7s %s\n", it.ID, it.Email, it.Role, it.Status)
		cont[it.Status]++
	}
	vlog.Printf("main.go", "imprimir", "loop range concluído sobre itens: %d itens", len(itens))
	fmt.Printf("Total: %d | resetados: %d | simulados: %d | falha de e-mail: %d\n",
		len(itens), cont["resetado"], cont["simulado"], cont["falha_email"])
}
