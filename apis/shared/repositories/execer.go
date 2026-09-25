package repositories

import (
	"context"
	"database/sql"
)

// Execer é satisfeita tanto por *sql.DB quanto por *sql.Tx. Métodos de
// escrita que precisam participar de uma transação orquestrada pelo service
// (ex.: inativar vendedor + usuários vinculados) aceitam Execer em vez de
// *sql.DB, sem quebrar os chamadores que passam o pool diretamente.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
