package cmdutil

// Kit dos comandos de linha de comando (ARQ-07): o `main` de cada comando é
// só a composition root (`os.Exit(cmdutil.Executar(tag, pacote.CLI,
// os.Args[1:], cmdutil.AmbientePadrao()))`); a lógica — flags, config,
// conexão e execução — fica num pacote testável, que recebe o Ambiente por
// injeção.

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/vlog"
)

// Códigos de saída de Executar.
const (
	SaidaOK   = 0
	SaidaErro = 1
	SaidaUso  = 2 // flag inválida (como o flag.ExitOnError)
)

// Ambiente são as dependências de processo de um comando, substituíveis nos
// testes.
type Ambiente struct {
	Entrada io.Reader // stdin
	Saida   io.Writer // stdout (relatórios do comando)
	Erro    io.Writer // stderr (uso das flags e avisos)
	// CarregarEnv carrega o .env da raiz do projeto (LoadEnvFromCwd).
	CarregarEnv func()
	// Config é o config.Load (exige DB_USUARIO/DB_SENHA, SEC-11).
	Config func() (*config.Config, error)
	// ConfigSemDB é o config.LoadSemCredenciaisDB: só para os modos que não
	// tocam o banco (dry-run), M-03.
	ConfigSemDB func() (*config.Config, error)
	// Abrir é o OpenMySQL.
	Abrir func(dsn string) Opener
}

// AmbientePadrao é o ambiente real do processo.
func AmbientePadrao() Ambiente {
	return Ambiente{
		Entrada:     os.Stdin,
		Saida:       os.Stdout,
		Erro:        os.Stderr,
		CarregarEnv: LoadEnvFromCwd,
		Config:      config.Load,
		ConfigSemDB: config.LoadSemCredenciaisDB,
		Abrir:       OpenMySQL,
	}
}

// Comando é a lógica de um CLI.
type Comando func(args []string, amb Ambiente) error

// ErrJaRelatado: o comando já escreveu o próprio resumo da falha; Executar
// só devolve o código 1, sem logar de novo.
var ErrJaRelatado = errors.New("falha já relatada pelo comando")

// ErroDeUso embrulha um erro de flags (código de saída 2).
type ErroDeUso struct{ Err error }

func (e *ErroDeUso) Error() string { return e.Err.Error() }
func (e *ErroDeUso) Unwrap() error { return e.Err }

// Executar roda o comando e devolve o código de saída: 0 ok (ou -h), 2 flag
// inválida (o uso já foi escrito em amb.Erro) e 1 erro, logado como
// "<tag>: <erro>" (o mesmo texto do log.Fatalf dos mains antigos); com tag
// vazia, o erro sai como veio (já traz o prefixo do comando).
func Executar(tag string, cmd Comando, args []string, amb Ambiente) int {
	err := cmd(args, amb)
	var uso *ErroDeUso
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return SaidaOK
	case errors.As(err, &uso):
		return SaidaUso
	case errors.Is(err, ErrJaRelatado):
		return SaidaErro
	case tag == "":
		log.Print(err)
		return SaidaErro
	default:
		log.Printf("%s: %v", tag, err)
		return SaidaErro
	}
}

// NovasFlags cria o FlagSet do comando: sem os.Exit (ContinueOnError) e com
// o uso escrito em amb.Erro.
func NovasFlags(nome string, amb Ambiente) *flag.FlagSet {
	fs := flag.NewFlagSet(nome, flag.ContinueOnError)
	fs.SetOutput(amb.Erro)
	return fs
}

// LerFlags interpreta args; flag inválida vira ErroDeUso (-h passa como
// flag.ErrHelp).
func LerFlags(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return err
		}
		return &ErroDeUso{Err: err}
	}
	return nil
}

// Configurar carrega o .env, a config (sem as credenciais do banco só quando
// semDB, M-03) e liga o vlog conforme VERBOSE.
func Configurar(amb Ambiente, semDB bool) (*config.Config, error) {
	amb.CarregarEnv()
	carregar := amb.Config
	if semDB {
		carregar = amb.ConfigSemDB
	}
	cfg, err := carregar()
	if err != nil {
		return nil, fmt.Errorf("falha ao carregar config: %w", err)
	}
	vlog.SetEnabled(cfg.Verbose)
	return cfg, nil
}
