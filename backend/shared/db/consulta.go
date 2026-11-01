package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/rotaperfumes/shared/kernel"
)

// Linha é o que um scan precisa: *sql.Row e *sql.Rows.
type Linha interface {
	Scan(dest ...any) error
}

// Consultar executa uma listagem e lê todas as linhas com scan. op é o nome
// da operação com o prefixo do pacote ("identidademysql: listar usuários"),
// usado no wrap do erro: nunca valores (O6-iv). Sem linhas devolve nil (o
// JSON legado de várias listagens é "data": null).
func Consultar[T any](ctx context.Context, q Querier, op, sqlTexto string, args []any, scan func(Linha) (*T, error)) ([]T, error) {
	rows, err := q.QueryContext(ctx, sqlTexto, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s iteração: %w", op, err)
	}
	return out, nil
}

// Paginada acrescenta LIMIT/OFFSET aos args do WHERE sem alterar o slice
// original (que também vai para o COUNT).
func Paginada(args []any, p kernel.Pagina) []any {
	return append(append([]any{}, args...), p.Limite(), p.Offset())
}

// Where junta as condições com AND ("" sem condições).
func Where(conds []string) string {
	if len(conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conds, " AND ")
}

// DirecaoSQL traduz a direção; a indefinida (zero value) cai no padrão.
func DirecaoSQL(d kernel.Direcao, padrao string) string {
	switch d {
	case kernel.Asc:
		return "ASC"
	case kernel.Desc:
		return "DESC"
	default:
		return padrao
	}
}
