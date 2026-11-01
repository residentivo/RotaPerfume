// Package vendasmysql implementa as portas de vendasapp sobre MariaDB
// (protocolo MySQL)
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
	"fmt"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
)

// wrap embrulha o erro de infra só com o nome da operação (O6-iv); um 1062
// perde o valor duplicado antes (N2: db.SemValorDuplicado).
func wrap(op string, err error) error {
	return fmt.Errorf("vendasmysql: %s: %w", op, db.SemValorDuplicado(err))
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
