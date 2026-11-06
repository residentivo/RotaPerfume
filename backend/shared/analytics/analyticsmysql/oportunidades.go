package analyticsmysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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
// cache, R4). O escopo de vendedor filtra pelo dono da oportunidade. O
// relógio dá o "hoje" (fuso tz.Local) do início da janela (BUG-30).
type Oportunidades struct {
	q       db.Querier
	relogio func() time.Time
}

// NovoOportunidades liga o adapter ao pool e ao relógio. nil: panic.
func NovoOportunidades(q db.Querier, relogio func() time.Time) *Oportunidades {
	if q == nil || relogio == nil {
		panic("analyticsmysql: NovoOportunidades exige Querier e relógio")
	}
	return &Oportunidades{q: q, relogio: relogio}
}

// Funil: oportunidades e valor estimado por etapa na janela (lista vazia,
// nunca nil). SemAcesso/zero value → o erro do Filtro, sem SQL. Erro do
// banco é erro (500): não há degradação para zero.
func (r *Oportunidades) Funil(ctx context.Context, e kernel.Escopo, j analytics.Janela) ([]analytics.ContagemEtapa, error) {
	filtro, filtroArgs, err := filtroVendedor(e, filtroOportunidadeDoVendedor)
	if err != nil {
		return nil, err
	}
	args := append(argsJanela(j, hojeEm(r.relogio)), filtroArgs...)
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
