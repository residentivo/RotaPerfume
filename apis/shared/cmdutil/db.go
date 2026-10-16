package cmdutil

// Contratos de banco dos comandos (importers/<nome> e tools/exportdados).
//
// Cada comando recebe a conexão por injeção (Opener/DB) em vez de abrir o
// banco por dentro: em produção o `main.go` passa OpenMySQL(cfg.DSN()); em
// testes, um *sql.DB do sqlmock (que satisfaz DB e Conn) ou um fake.

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql" // driver MySQL usado por OpenMySQL
)

// DB é o subconjunto de *sql.DB usado pelas funções de leitura (lookups) e
// gravação (upsert) dos importadores. É satisfeita por *sql.DB e *sql.Tx.
type DB interface {
	Query(query string, args ...any) (*sql.Rows, error)
	Prepare(query string) (*sql.Stmt, error)
}

// Conn é a conexão aberta pelo Run de cada importador: DB + Ping + Close.
// É satisfeita por *sql.DB (inclusive o do sqlmock).
type Conn interface {
	DB
	Ping() error
	Close() error
}

// Opener abre a conexão com o banco. O Run só a chama depois de ler o CSV e
// quando não é --dry-run (mesma ordem dos comandos originais).
type Opener func() (Conn, error)

// OpenMySQL devolve um Opener que faz sql.Open("mysql", dsn). O Ping fica a
// cargo do Run do importador.
func OpenMySQL(dsn string) Opener {
	return func() (Conn, error) {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, fmt.Errorf("sql.Open: %w", err)
		}
		return db, nil
	}
}
