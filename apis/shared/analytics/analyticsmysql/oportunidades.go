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

const arqOportunidades = "oportunidades.go"

// SQL do funil (DASH-01): a janela (constante ou vazia) e o filtro de
// vendedor (constante ou vazio, decidido pelo Filtro()) entram entre o
// início e o fim. Sem fmt.Sprintf: só concatenação de constantes.
const (
	sqlFunilInicio = `
		SELECT etapa, COUNT(*), COALESCE(SUM(valor_estimado), 0)
		FROM oportunidades
		WHERE 1=1`
	sqlFunilFim = `
		GROUP BY etapa`
)

// Oportunidades é a contagem do funil de vendas (porta
// analyticsapp.IndicadoresFunil). Cada chamada faz a própria consulta (sem
// cache, R4). O escopo de vendedor filtra pelo dono da oportunidade.
type Oportunidades struct{ q db.Querier }

// NovoOportunidades liga o adapter ao pool. q nil: panic.
func NovoOportunidades(q db.Querier) *Oportunidades {
	if q == nil {
		panic("analyticsmysql: NovoOportunidades exige Querier")
	}
	return &Oportunidades{q: q}
}

// Funil: oportunidades e valor estimado por etapa na janela (lista vazia,
// nunca nil). SemAcesso/zero value → o erro do Filtro, sem SQL. Erro do
// banco é erro (500): não há degradação para zero.
func (r *Oportunidades) Funil(ctx context.Context, e kernel.Escopo, j analytics.Janela) ([]analytics.ContagemEtapa, error) {
	filtro, args, err := filtroVendedor(e, filtroOportunidadeDoVendedor)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.QueryContext(ctx, sqlFunilInicio+clausulaJanelaAbertura(j)+filtro+sqlFunilFim, args...)
	if err != nil {
		return nil, fmt.Errorf("analyticsmysql: funil: %w", err)
	}
	defer rows.Close()

	contagens := make([]analytics.ContagemEtapa, 0, 6)
	for rows.Next() {
		var c analytics.ContagemEtapa
		var valor sql.NullFloat64
		if err := rows.Scan(&c.Etapa, &c.Quantidade, &valor); err != nil {
			return nil, fmt.Errorf("analyticsmysql: funil scan: %w", err)
		}
		c.Valor = valor.Float64
		contagens = append(contagens, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analyticsmysql: funil rows: %w", err)
	}
	vlog.Printf(arqOportunidades, "Oportunidades.Funil", "SELECT oportunidades por etapa linhas=%d", len(contagens))
	return contagens, nil
}
