package repositories

import "errors"

// ErrNotFound é retornado quando um registro não é encontrado.
var ErrNotFound = errors.New("repositories: registro não encontrado")

// rowScanner é a interface comum entre *sql.Row e *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}
