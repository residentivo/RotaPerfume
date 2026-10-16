package resetpassword

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql" // driver do AbrirMySQL
	"golang.org/x/term"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/vlog"
)

// prazoComando é o prazo do ping e da ação no banco.
const prazoComando = 30 * time.Second

// Conexao é a conexão aberta pelo comando (satisfeita por *sql.DB).
type Conexao interface {
	DB
	PingContext(ctx context.Context) error
	Close() error
}

// Terminal lê a senha sem eco (-password-prompt).
type Terminal struct {
	EhTerminal func() bool
	LerSemEco  func() ([]byte, error)
}

// DepsCLI são as dependências do comando que não vêm do cmdutil.Ambiente.
type DepsCLI struct {
	Terminal Terminal
	// AbrirBanco abre a conexão (AbrirMySQL no comando; o Ping é do CLI).
	AbrirBanco func(dsn string) (Conexao, error)
	// LoadHashSenha é o config.LoadHashSenha (pepper e parâmetros Argon2id).
	LoadHashSenha func() (config.HashSenha, error)
	Gerador       PasswordGenerator
}

// DepsPadrao liga o terminal real (stdin), o MySQL e o gerador aleatório.
func DepsPadrao() DepsCLI {
	fd := int(os.Stdin.Fd())
	return DepsCLI{
		Terminal: Terminal{
			EhTerminal: func() bool { return term.IsTerminal(fd) },
			LerSemEco:  func() ([]byte, error) { return term.ReadPassword(fd) },
		},
		AbrirBanco:    AbrirMySQL,
		LoadHashSenha: config.LoadHashSenha,
		Gerador:       GenerateRandomPassword,
	}
}

// AbrirMySQL faz o sql.Open("mysql", dsn); o Ping fica a cargo do CLI.
func AbrirMySQL(dsn string) (Conexao, error) {
	return sql.Open("mysql", dsn)
}

// CLI é o comando cmd/resetpassword; o main só chama cmdutil.Executar com a
// tag vazia (as mensagens de erro já trazem o prefixo "resetpassword:").
func CLI(deps DepsCLI) cmdutil.Comando {
	return func(args []string, amb cmdutil.Ambiente) error {
		opts, err := lerOpcoes(args, amb)
		if err != nil {
			return err
		}
		// Sem fonte de senha, devolve "" e o Run usa env SEED_* / aleatória.
		if opts.Password, err = ResolverSenha(opts, amb.Entrada, deps.Terminal.EhTerminal, deps.Terminal.LerSemEco, amb.Erro); err != nil {
			return err
		}
		amb.CarregarEnv()
		// Sem *Config: a mesma regra de cfg.Verbose (VERBOSE=true ou LOG_LEVEL=debug).
		vlog.SetEnabled(os.Getenv("VERBOSE") == "true" || os.Getenv("LOG_LEVEL") == "debug")
		if !opts.List { // -list não grava senha: dispensa PASSWORD_PEPPER
			if opts.HashSenha, err = deps.LoadHashSenha(); err != nil {
				return fmt.Errorf("resetpassword: %w", err)
			}
		}
		return executar(opts, amb, deps)
	}
}

// executar abre o banco, confere a conexão e roda a ação.
func executar(opts Options, amb cmdutil.Ambiente, deps DepsCLI) error {
	dsn, err := DSN()
	if err != nil {
		return fmt.Errorf("resetpassword: %w", err)
	}
	db, err := deps.AbrirBanco(dsn)
	if err != nil {
		return fmt.Errorf("resetpassword: sql.Open: %w", err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), prazoComando)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("resetpassword: ping: %w", err)
	}
	return Run(ctx, db, opts, amb.Saida, deps.Gerador)
}

// lerOpcoes registra e interpreta as flags.
func lerOpcoes(args []string, amb cmdutil.Ambiente) (Options, error) {
	var opts Options
	fs := cmdutil.NovasFlags("resetpassword", amb)
	fs.StringVar(&opts.Email, "email", "", "email do usuário a criar/atualizar")
	fs.StringVar(&opts.Password, "password", "", "DEPRECIADA (fica no histórico do shell): senha em texto puro; prefira -password-prompt ou -password-stdin")
	fs.BoolVar(&opts.PasswordStdin, "password-stdin", false, "lê a senha da 1ª linha do stdin")
	fs.BoolVar(&opts.PasswordPrompt, "password-prompt", false, "pede a senha no terminal, sem eco e com confirmação (Git Bash: use winpty)")
	fs.StringVar(&opts.Role, "role", "normal", "papel (admin|normal) — usado ao criar novo usuário")
	fs.StringVar(&opts.Nome, "nome", "", "nome completo — usado ao criar novo usuário (padrão: derivado do email)")
	fs.Int64Var(&opts.IDVendedor, "id-vendedor", 0, "id do vendedor vinculado (opcional, usado ao criar)")
	fs.BoolVar(&opts.AllUsers, "all-users", false, "atualiza todos os PLACEHOLDER com a mesma senha")
	fs.BoolVar(&opts.CreateAdmin, "create-admin", false, "garante que o admin principal existe (cria se faltar)")
	fs.BoolVar(&opts.List, "list", false, "lista os usuários atuais")
	return opts, cmdutil.LerFlags(fs, args)
}
