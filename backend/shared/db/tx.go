package db

import (
	"context"
	"database/sql"
	"fmt"
)

// Querier é o que um repositório precisa para consultar: satisfeito tanto
// pelo pool (*sql.DB) quanto por uma transação (*sql.Tx). O repositório
// guarda um Querier no struct e não recebe mais *sql.DB por parâmetro.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

var (
	_ Querier = (*sql.DB)(nil)
	_ Querier = (*sql.Tx)(nil)
)

// Transactor é o Unit of Work (docs/arquitetura-ddd.md §5.2): abre uma tx,
// entrega à fn os repositórios R já ligados a ela, faz commit se fn devolver
// nil e rollback caso contrário.
//
// Regras do WithTx (O8-ii, R-d):
//  1. dentro da fn use SÓ os repositórios de R (nada do pool nem capturado de
//     fora da closure);
//  2. isolamento padrão do MariaDB/InnoDB (REPEATABLE READ), o mesmo de hoje;
//  3. a tx é cancelada pelo ctx da requisição (rollback pelo database/sql);
//  4. nenhum efeito externo (e-mail, evento, cookie) dentro da fn.
//
// O Transactor não deduz nada de RowsAffected (F-07): as condições ficam no
// WHERE dos repositórios.
type Transactor[R any] struct {
	pool *sql.DB
	bind func(Querier) R
}

// NewTransactor monta o UoW. pool e bind nil são erro de programação: panic
// na montagem (composition root), nunca em runtime.
func NewTransactor[R any](pool *sql.DB, bind func(Querier) R) *Transactor[R] {
	if pool == nil || bind == nil {
		panic("db: NewTransactor exige pool e bind não nulos")
	}
	return &Transactor[R]{pool: pool, bind: bind}
}

// WithTx executa fn numa transação. O erro de fn volta sem embrulho (o mapper
// HTTP decide pelo sentinela); falhas de begin/commit voltam com o nome da
// operação.
func (t *Transactor[R]) WithTx(ctx context.Context, fn func(R) error) error {
	tx, err := t.pool.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op após Commit
	if err := fn(t.bind(tx)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}
