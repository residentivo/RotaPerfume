package analyticsmysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqClientes = "clientes.go"

// SQL das métricas de cliente do cliente_repository.go legado (sem
// alteração): o WHERE (carteira do escopo e condições constantes) entra
// entre o início e o fim.
const (
	sqlContarClientes        = "SELECT COUNT(*) FROM clientes"
	condAtivo                = "ativo = ?"
	sqlClientesPorSegmentoIn = `
		SELECT segmento, COUNT(*) AS total
		FROM clientes`
	sqlClientesPorSegmentoFim = `
		GROUP BY segmento
		ORDER BY total DESC`
	sqlClientesPorUFIn = `
		SELECT uf, COUNT(*) AS total
		FROM clientes`
	sqlClientesPorUFFim = `
		GROUP BY uf
		ORDER BY total DESC`
)

// Clientes são as contagens da base de clientes (porta
// analyticsapp.IndicadoresClientes). O escopo de vendedor restringe à
// carteira ATIVA do vendedor.
type Clientes struct{ q db.Querier }

// NovoClientes liga o adapter ao pool. q nil: panic.
func NovoClientes(q db.Querier) *Clientes {
	if q == nil {
		panic("analyticsmysql: NovoClientes exige Querier")
	}
	return &Clientes{q: q}
}

// Total: clientes cadastrados no escopo.
func (r *Clientes) Total(ctx context.Context, e kernel.Escopo) (int, error) {
	where, args, err := whereCarteira(e)
	if err != nil {
		return 0, err
	}
	return r.contar(ctx, "Clientes.Total", "count total clientes", where, args)
}

// PorAtivo: clientes do escopo com o status ativo informado.
func (r *Clientes) PorAtivo(ctx context.Context, e kernel.Escopo, ativo bool) (int, error) {
	where, carteira, err := whereCarteira(e, condAtivo)
	if err != nil {
		return 0, err
	}
	return r.contar(ctx, "Clientes.PorAtivo", "count clientes por ativo", where, append([]any{ativo}, carteira...))
}

// NovosNoPeriodo: clientes do escopo com data_cadastro no período.
func (r *Clientes) NovosNoPeriodo(ctx context.Context, e kernel.Escopo, p analytics.Periodo) (int, error) {
	where, args, err := whereCarteira(e, clausulaPeriodoCadastro(p))
	if err != nil {
		return 0, err
	}
	return r.contar(ctx, "Clientes.NovosNoPeriodo", "count novos clientes", where, args)
}

// contar executa um SELECT COUNT(*) FROM clientes com o WHERE informado.
func (r *Clientes) contar(ctx context.Context, rotulo, op, where string, args []any) (int, error) {
	var total int
	if err := r.q.QueryRowContext(ctx, sqlContarClientes+where, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("analyticsmysql: %s: %w", op, err)
	}
	vlog.Printf(arqClientes, rotulo, "SELECT clientes count linhas=1")
	return total, nil
}

// PorSegmento: a distribuição do escopo por segmento, do maior para o menor
// total (lista vazia, nunca nil).
func (r *Clientes) PorSegmento(ctx context.Context, e kernel.Escopo) ([]analytics.ContagemSegmento, error) {
	where, args, err := whereCarteira(e)
	if err != nil {
		return nil, err
	}
	out, err := distribuicao(ctx, r.q, sqlClientesPorSegmentoIn+where+sqlClientesPorSegmentoFim, args, "segmento",
		func(rows *sql.Rows) (analytics.ContagemSegmento, error) {
			var c analytics.ContagemSegmento
			return c, rows.Scan(&c.Segmento, &c.Total)
		})
	if err == nil {
		vlog.Printf(arqClientes, "Clientes.PorSegmento", "SELECT clientes por segmento linhas=%d", len(out))
	}
	return out, err
}

// PorUF: a distribuição do escopo por UF, do maior para o menor total
// (lista vazia, nunca nil).
func (r *Clientes) PorUF(ctx context.Context, e kernel.Escopo) ([]analytics.ContagemUF, error) {
	where, args, err := whereCarteira(e)
	if err != nil {
		return nil, err
	}
	out, err := distribuicao(ctx, r.q, sqlClientesPorUFIn+where+sqlClientesPorUFFim, args, "uf",
		func(rows *sql.Rows) (analytics.ContagemUF, error) {
			var c analytics.ContagemUF
			return c, rows.Scan(&c.UF, &c.Total)
		})
	if err == nil {
		vlog.Printf(arqClientes, "Clientes.PorUF", "SELECT clientes por uf linhas=%d", len(out))
	}
	return out, err
}

// distribuicao lê as linhas (agrupamento, total); erro de query, de scan ou
// de iteração volta embrulhado com o nome do agrupamento e lista nil.
func distribuicao[T any](ctx context.Context, q db.Querier, sqlTexto string, args []any, grupo string, scan func(*sql.Rows) (T, error)) ([]T, error) {
	rows, err := q.QueryContext(ctx, sqlTexto, args...)
	if err != nil {
		return nil, fmt.Errorf("analyticsmysql: count por %s: %w", grupo, err)
	}
	defer rows.Close()

	out := make([]T, 0)
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("analyticsmysql: scan %s: %w", grupo, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analyticsmysql: count por %s iteração: %w", grupo, err)
	}
	return out, nil
}
