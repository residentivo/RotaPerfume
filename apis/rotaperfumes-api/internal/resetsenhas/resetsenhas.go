// Package resetsenhas é o comando cmd/resetsenhas (OPS-01) fora do main
// (ARQ-07): redefine a senha de TODOS os usuários ativos com o mesmo fluxo
// do reset administrativo, pelo caso de uso identidadeapp.ResetarSenhasAtivos
// (sem Principal: caso de uso de CLI, O5), montado pela composition root.
// Simula por padrão; só grava e envia com -executar (M-03). Recusa rodar sem
// SMTP: a senha nunca é impressa nem logada (M-02), então sem e-mail ela se
// perderia. A saída mascara os e-mails (NB-7).
package resetsenhas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/adapters/turnstile"
	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/notificacao/notificacaosmtp"
	"github.com/rotaperfumes/shared/vlog"
)

// Tag prefixa as mensagens de erro do comando.
const Tag = "resetsenhas"

// prazoComando cobre o ping e o lote inteiro.
const prazoComando = 30 * time.Minute

// Deps são as dependências do comando que não vêm do cmdutil.Ambiente.
type Deps struct {
	// AbrirBanco abre o pool (db.Open no comando: ping de 5 s e DSN mascarado).
	AbrirBanco func(dsn string) (*sql.DB, error)
	// Email é o envio SMTP obrigatório (notificacaosmtp.Novo no comando).
	Email func(cfg *config.Config) (identidadeapp.EnviadorSenhaInicial, error)
}

// DepsPadrao liga o MariaDB e o SMTP reais.
func DepsPadrao() Deps {
	return Deps{
		AbrirBanco: db.Open,
		Email: func(cfg *config.Config) (identidadeapp.EnviadorSenhaInicial, error) {
			return notificacaosmtp.Novo(cfg)
		},
	}
}

// CLI devolve o comando; o main só chama cmdutil.Executar.
func CLI(deps Deps) cmdutil.Comando {
	return func(args []string, amb cmdutil.Ambiente) error {
		fs := cmdutil.NovasFlags(Tag, amb)
		executar := fs.Bool("executar", false, "grava as novas senhas e envia os e-mails (sem esta flag, só simula)")
		if err := cmdutil.LerFlags(fs, args); err != nil {
			return err
		}
		amb.CarregarEnv()
		cfg, err := amb.Config()
		if err != nil {
			return fmt.Errorf("config: %w", err)
		}
		vlog.SetEnabled(cfg.Verbose)
		return rodar(cfg, *executar, amb, deps)
	}
}

func rodar(cfg *config.Config, executar bool, amb cmdutil.Ambiente, deps Deps) error {
	email, err := deps.Email(cfg)
	if err != nil {
		return fmt.Errorf("SMTP obrigatório (a senha só chega ao usuário por e-mail): %w", err)
	}
	pool, err := deps.AbrirBanco(cfg.DSN())
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), prazoComando)
	defer cancel()
	if err := pool.PingContext(ctx); err != nil {
		return fmt.Errorf("db ping: %w", err)
	}

	identidade := bootstrap.NovaIdentidade(pool, cfg, bootstrap.ExtIdentidade{
		Captcha: turnstile.Novo(cfg.TurnstileSecretKey, cfg.TurnstileHostnames), Publicador: identidadeapp.SemPublicador{}, Email: email,
	})
	fmt.Fprintf(amb.Saida, "Banco: %s@%s:%s | SMTP: %s (from %s) | modo: %s\n",
		cfg.DBName, cfg.DBHost, cfg.DBPort, cfg.SMTPHost, vlog.MaskEmail(cfg.SMTPFrom), modo(executar))

	itens, err := identidade.ResetarSenhasAtivos(ctx, !executar)
	Imprimir(amb.Saida, itens)
	if err != nil {
		if errors.Is(err, identidadeapp.ErrEmailResetFalhou) {
			fmt.Fprintln(amb.Erro, "PARADO: o e-mail do último usuário falhou. A senha dele já foi trocada;")
			fmt.Fprintln(amb.Erro, "corrija o SMTP e redefina essa senha pelo admin (ou rode de novo).")
		}
		return err
	}
	if !executar {
		fmt.Fprintln(amb.Saida, "Simulação: nada foi gravado nem enviado. Rode com -executar para aplicar.")
	}
	return nil
}

func modo(executar bool) string {
	if executar {
		return "EXECUTAR"
	}
	return "simulação"
}

// Imprimir escreve a tabela do lote com os e-mails mascarados (NB-7) e o
// total por status.
func Imprimir(out io.Writer, itens []identidadeapp.ItemResetMassa) {
	fmt.Fprintf(out, "%-5s %-45s %-7s %s\n", "ID", "EMAIL", "ROLE", "STATUS")
	cont := map[string]int{}
	for _, it := range itens {
		fmt.Fprintf(out, "%-5d %-45s %-7s %s\n", it.ID, vlog.MaskEmail(it.Email), it.Role, it.Status)
		cont[it.Status]++
	}
	fmt.Fprintf(out, "Total: %d | resetados: %d | simulados: %d | falha de e-mail: %d\n",
		len(itens), cont[identidadeapp.StatusResetado], cont[identidadeapp.StatusSimulado], cont[identidadeapp.StatusFalhaEmail])
}
