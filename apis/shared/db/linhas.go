package db

// Passos comuns dos adapters *mysql (ARQ-07, pendência h: eram cópias em
// comercialmysql e vendasmysql). op é o nome da operação JÁ com o prefixo do
// pacote ("comercialmysql: atualizar cliente"): vai no wrap do erro, nunca
// valores (O6-iv). arq/rotulo vão só para o vlog de query (nível 4).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rotaperfumes/shared/vlog"
)

// ExecPorID roda um UPDATE/DELETE por id: nenhuma linha casada vira
// naoEncontrado (DSN com clientFoundRows=true, BUG-04).
func ExecPorID(ctx context.Context, q Querier, arq, rotulo, op string, naoEncontrado error, sqlTexto string, args ...any) error {
	res, err := q.ExecContext(ctx, sqlTexto, args...)
	if err != nil {
		return embrulhar(op, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s rowsAffected: %w", op, err)
	}
	vlog.Printf(arq, rotulo, "afetadas=%d", n)
	if n == 0 {
		return naoEncontrado
	}
	return nil
}

// Existe roda um "SELECT 1 … LIMIT 1" por id.
func Existe(ctx context.Context, q Querier, arq, rotulo, op, sqlTexto string, id int64) (bool, error) {
	var um int
	err := q.QueryRowContext(ctx, sqlTexto, id).Scan(&um)
	if errors.Is(err, sql.ErrNoRows) {
		vlog.Printf(arq, rotulo, "existe=false")
		return false, nil
	}
	if err != nil {
		return false, embrulhar(op, err)
	}
	vlog.Printf(arq, rotulo, "existe=true")
	return true, nil
}

// Inserir roda um INSERT e devolve o id gerado (AUTO_INCREMENT).
func Inserir(ctx context.Context, q Querier, op, sqlTexto string, args ...any) (int64, error) {
	res, err := q.ExecContext(ctx, sqlTexto, args...)
	if err != nil {
		return 0, embrulhar(op, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s lastInsertId: %w", op, err)
	}
	return id, nil
}

// LerUma lê uma linha; sql.ErrNoRows vira naoEncontrado.
func LerUma[T any](s Linha, naoEncontrado error, op string, dest func(*T) []any) (*T, error) {
	var v T
	err := s.Scan(dest(&v)...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, naoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return &v, nil
}

// embrulhar: só o nome da operação; um 1062 perde o valor duplicado antes (N2).
func embrulhar(op string, err error) error {
	return fmt.Errorf("%s: %w", op, SemValorDuplicado(err))
}
