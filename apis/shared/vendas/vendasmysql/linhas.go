// Package vendasmysql implementa as portas de vendasapp sobre MySQL
// (ARQ-05). O SQL é o dos repositórios legados de pedido e pagamento,
// copiado sem alteração; a ordenação sai de switch sobre os enums do
// domínio (C-02); todo valor vai por placeholder (C-03); e o escopo de
// carteira só vira SQL por kernel.Escopo.Filtro (O1): SemAcesso e zero
// value voltam como erro ANTES de qualquer consulta.
//
// Nenhum método abre transação: a tx do pedido (cabeçalho + itens + baixa
// de estoque) vem do UoW da aplicação (db.Transactor, §5.2, C9), que liga
// estes repositórios ao *sql.Tx.
package vendasmysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

// wrap embrulha o erro de infra só com o nome da operação (O6-iv); um 1062
// perde o valor duplicado antes (N2: db.SemValorDuplicado).
func wrap(op string, err error) error {
	return fmt.Errorf("vendasmysql: %s: %w", op, db.SemValorDuplicado(err))
}

// execPorID roda um UPDATE/DELETE por id: nenhuma linha casada vira
// naoEncontrado (DSN com clientFoundRows=true, BUG-04).
func execPorID(ctx context.Context, q db.Querier, arq, rotulo, op string, naoEncontrado error, sqlTexto string, args ...any) error {
	res, err := q.ExecContext(ctx, sqlTexto, args...)
	if err != nil {
		return wrap(op, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("vendasmysql: %s rowsAffected: %w", op, err)
	}
	vlog.Printf(arq, rotulo, "afetadas=%d", n)
	if n == 0 {
		return naoEncontrado
	}
	return nil
}

// existe roda um "SELECT 1 … LIMIT 1".
func existe(ctx context.Context, q db.Querier, arq, rotulo, op, sqlTexto string, id int64) (bool, error) {
	var um int
	err := q.QueryRowContext(ctx, sqlTexto, id).Scan(&um)
	if errors.Is(err, sql.ErrNoRows) {
		vlog.Printf(arq, rotulo, "existe=false")
		return false, nil
	}
	if err != nil {
		return false, wrap(op, err)
	}
	vlog.Printf(arq, rotulo, "existe=true")
	return true, nil
}

// inserir roda um INSERT e devolve o id gerado (AUTO_INCREMENT).
func inserir(ctx context.Context, q db.Querier, op, sqlTexto string, args ...any) (int64, error) {
	res, err := q.ExecContext(ctx, sqlTexto, args...)
	if err != nil {
		return 0, wrap(op, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("vendasmysql: %s lastInsertId: %w", op, err)
	}
	return id, nil
}

// paginar roda o COUNT e o SELECT paginado de uma listagem (os textos de
// SQL vêm prontos do chamador, como no legado).
func paginar[T any](ctx context.Context, q db.Querier, op, countQ, listQ string, args []any, p kernel.Pagina, scan func(db.Linha) (*T, error)) (kernel.Resultado[T], error) {
	var total int
	if err := q.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return kernel.Resultado[T]{}, fmt.Errorf("vendasmysql: count %s: %w", op, err)
	}
	itens, err := db.Consultar(ctx, q, "vendasmysql: list "+op, listQ, db.Paginada(args, p), scan)
	if err != nil {
		return kernel.Resultado[T]{}, err
	}
	return kernel.NovoResultado(itens, p, total), nil
}

// lerUma lê uma linha; sql.ErrNoRows vira naoEncontrado.
func lerUma[T any](s db.Linha, naoEncontrado error, op string, dest func(*T) []any) (*T, error) {
	var v T
	err := s.Scan(dest(&v)...)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, naoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("vendasmysql: scan %s: %w", op, err)
	}
	return &v, nil
}

// vendedorDaListagem aplica o escopo a uma listagem: no escopo de vendedor,
// o filtro de negócio da query é SOBRESCRITO pelo vendedor do escopo
// (E-03); no irrestrito vale o da query (0 = sem filtro). SemAcesso e zero
// value: erro, sem SQL (O1). Nunca se deduz "global" de um vendedor
// ausente: "sem filtro" só existe com err == nil && !filtra (Irrestrito).
func vendedorDaListagem(e kernel.Escopo, daQuery int64) (int64, error) {
	vendedorID, filtra, err := e.Filtro()
	if err != nil {
		return 0, err
	}
	if filtra {
		return vendedorID, nil
	}
	return daQuery, nil
}
