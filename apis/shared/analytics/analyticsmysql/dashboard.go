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

const arqDashboard = "dashboard.go"

// Dashboard são as consultas agregadoras de pedidos e vendedores (porta
// analyticsapp.IndicadoresVendas). Cada chamada faz as próprias consultas
// (sem cache, R4). O relógio dá o "hoje" das séries zeradas (o mesmo das
// Deps do caso de uso, ligado no bootstrap).
type Dashboard struct {
	q       db.Querier
	relogio func() time.Time
}

// NovoDashboard liga o adapter ao pool e ao relógio. nil: panic.
func NovoDashboard(q db.Querier, relogio func() time.Time) *Dashboard {
	if q == nil || relogio == nil {
		panic("analyticsmysql: NovoDashboard exige Querier e relógio")
	}
	return &Dashboard{q: q, relogio: relogio}
}

// VendasTotais: valor e quantidade de pedidos do período (sem cancelados e
// devolvidos). Sem a tabela pedidos, zero.
func (r *Dashboard) VendasTotais(ctx context.Context, e kernel.Escopo, p analytics.Periodo) (analytics.TotaisVendas, error) {
	filtro, args, err := filtroVendedor(e, filtroPedidoDoVendedor)
	if err != nil {
		return analytics.TotaisVendas{}, err
	}
	var valor sql.NullFloat64
	var qtd sql.NullInt64
	err = r.q.QueryRowContext(ctx, fmt.Sprintf(fmtVendasTotais, clausulaPeriodoPedido(p))+filtro, args...).Scan(&valor, &qtd)
	if err != nil {
		return analytics.TotaisVendas{}, semTabela(err, "vendas totais")
	}
	vlog.Printf(arqDashboard, "Dashboard.VendasTotais", "SELECT pedidos totais linhas=1")
	return analytics.TotaisVendas{Valor: valor.Float64, Quantidade: int(qtd.Int64)}, nil
}

// TotalPedidos: quantidade de pedidos do período. Sem a tabela, zero.
func (r *Dashboard) TotalPedidos(ctx context.Context, e kernel.Escopo, p analytics.Periodo) (int, error) {
	filtro, args, err := filtroVendedor(e, filtroPedidoDoVendedor)
	if err != nil {
		return 0, err
	}
	var total sql.NullInt64
	err = r.q.QueryRowContext(ctx, fmt.Sprintf(fmtTotalPedidos, clausulaPeriodoPedido(p))+filtro, args...).Scan(&total)
	if err != nil {
		return 0, semTabela(err, "total pedidos")
	}
	vlog.Printf(arqDashboard, "Dashboard.TotalPedidos", "SELECT pedidos count linhas=1")
	return int(total.Int64), nil
}

// MetaMensalTotal: soma das metas dos vendedores ativos do escopo. Sem a
// tabela vendedores, zero.
func (r *Dashboard) MetaMensalTotal(ctx context.Context, e kernel.Escopo) (float64, error) {
	filtro, args, err := filtroVendedor(e, filtroVendedorPorID)
	if err != nil {
		return 0, err
	}
	var meta sql.NullFloat64
	if err = r.q.QueryRowContext(ctx, sqlMetaMensalTotal+filtro, args...).Scan(&meta); err != nil {
		return 0, semTabela(err, "meta mensal total")
	}
	vlog.Printf(arqDashboard, "Dashboard.MetaMensalTotal", "SELECT vendedores meta linhas=1")
	return meta.Float64, nil
}

// semTabela: "tabela não existe" vira nil (o chamador devolve zero, como o
// legado); qualquer outro erro é embrulhado só com o nome da operação.
func semTabela(err error, op string) error {
	if tabelaInexistente(err) {
		return nil
	}
	return fmt.Errorf("analyticsmysql: %s: %w", op, err)
}

// TopVendedores: os `limite` vendedores ativos de maior meta, com as vendas
// do mês. Sem a tabela, lista vazia.
func (r *Dashboard) TopVendedores(ctx context.Context, e kernel.Escopo, limite int) ([]analytics.VendedorRanking, error) {
	filtro, args, err := filtroVendedor(e, filtroVendedorAlias)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.QueryContext(ctx, sqlTopVendedoresInicio+filtro+sqlTopVendedoresFim, append(args, limite)...)
	if err != nil {
		if tabelaInexistente(err) {
			return []analytics.VendedorRanking{}, nil
		}
		return nil, fmt.Errorf("analyticsmysql: top vendedores: %w", err)
	}
	defer rows.Close()

	top := make([]analytics.VendedorRanking, 0)
	for rows.Next() {
		var v analytics.VendedorRanking
		if err := rows.Scan(&v.ID, &v.Nome, &v.Meta); err != nil {
			return nil, fmt.Errorf("analyticsmysql: top vendedores scan: %w", err)
		}
		top = append(top, v)
	}
	if len(top) > 0 {
		vendas := r.vendasDoMes(ctx, idsDe(top, func(v analytics.VendedorRanking) int64 { return v.ID }))
		for i := range top {
			if vm, ok := vendas[top[i].ID]; ok {
				top[i].AplicarVendas(vm)
			}
		}
	}
	vlog.Printf(arqDashboard, "Dashboard.TopVendedores", "SELECT vendedores top linhas=%d", len(top))
	return top, nil
}

// MetasVendedores: as metas dos vendedores ativos com o realizado do mês.
func (r *Dashboard) MetasVendedores(ctx context.Context, e kernel.Escopo) ([]analytics.MetaVendedor, error) {
	filtro, args, err := filtroVendedor(e, filtroVendedorAlias)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.QueryContext(ctx, sqlMetasVendedoresInicio+filtro+sqlMetasVendedoresFim, args...)
	if err != nil {
		return nil, fmt.Errorf("analyticsmysql: metas vendedores: %w", err)
	}
	defer rows.Close()

	metas := make([]analytics.MetaVendedor, 0)
	for rows.Next() {
		var m analytics.MetaVendedor
		if err := rows.Scan(&m.ID, &m.Nome, &m.Regiao, &m.UF, &m.Meta); err != nil {
			return nil, fmt.Errorf("analyticsmysql: metas vendedores scan: %w", err)
		}
		metas = append(metas, m)
	}
	if len(metas) > 0 {
		vendas := r.vendasDoMes(ctx, idsDe(metas, func(m analytics.MetaVendedor) int64 { return m.ID }))
		for i := range metas {
			if vm, ok := vendas[metas[i].ID]; ok {
				metas[i].AplicarVendas(vm)
			}
		}
	}
	vlog.Printf(arqDashboard, "Dashboard.MetasVendedores", "SELECT vendedores metas linhas=%d", len(metas))
	return metas, nil
}

// vendasDoMes soma as vendas do mês dos vendedores (enriquecimento do
// legado). Falha é silenciosa: a lista já tem os dados de meta; linha que
// não escaneia é ignorada.
func (r *Dashboard) vendasDoMes(ctx context.Context, ids []any) map[int64]analytics.VendasDoMes {
	vendas := make(map[int64]analytics.VendasDoMes)
	rows, err := r.q.QueryContext(ctx, sqlVendasDoMesInicio+marcadores(len(ids))+sqlVendasDoMesFim, ids...)
	if err != nil {
		return vendas
	}
	defer rows.Close()
	for rows.Next() {
		var vendedorID int64
		var total sql.NullFloat64
		var qtd sql.NullInt64
		if err := rows.Scan(&vendedorID, &total, &qtd); err != nil {
			continue
		}
		vendas[vendedorID] = analytics.VendasDoMes{Total: total.Float64, Quantidade: int(qtd.Int64)}
	}
	return vendas
}

// idsDe devolve os ids como argumentos do IN (int64, como o legado).
func idsDe[T any](itens []T, id func(T) int64) []any {
	ids := make([]any, len(itens))
	for i, v := range itens {
		ids[i] = id(v)
	}
	return ids
}

// SerieVendas: um ponto por dia da janela. Sem a tabela pedidos ou sem
// vendas no período, a série zerada; dias sem venda entram zerados.
func (r *Dashboard) SerieVendas(ctx context.Context, e kernel.Escopo, d analytics.Dias) ([]analytics.PontoVenda, error) {
	filtro, filtroArgs, err := filtroVendedor(e, filtroPedidoDoVendedor)
	if err != nil {
		return nil, err
	}
	dias := d.Int()
	rows, err := r.q.QueryContext(ctx, sqlSerieVendasInicio+filtro+sqlSerieVendasFim, append([]any{dias}, filtroArgs...)...)
	if err != nil {
		if tabelaInexistente(err) {
			return analytics.SerieZerada(dias, r.relogio()), nil
		}
		return nil, fmt.Errorf("analyticsmysql: série de vendas: %w", err)
	}
	defer rows.Close()

	vendas := lerVendasPorDia(rows)
	if len(vendas) == 0 {
		return analytics.SerieZerada(dias, r.relogio()), nil
	}
	pontos := r.serieCompleta(ctx, dias, vendas)
	vlog.Printf(arqDashboard, "Dashboard.SerieVendas", "SELECT pedidos por dia linhas=%d pontos=%d", len(vendas), len(pontos))
	return pontos, nil
}

// lerVendasPorDia lê as vendas por dia; linha que não escaneia é ignorada
// (como o legado).
func lerVendasPorDia(rows *sql.Rows) map[string]analytics.PontoVenda {
	vendas := make(map[string]analytics.PontoVenda)
	for rows.Next() {
		var dia string
		var valor sql.NullFloat64
		var qtd sql.NullInt64
		if err := rows.Scan(&dia, &valor, &qtd); err != nil {
			continue
		}
		vendas[dia] = analytics.PontoVenda{Dia: dia, TotalVendas: valor.Float64, TotalPedidos: int(qtd.Int64)}
	}
	return vendas
}

// serieCompleta pega no banco a sequência de dias da janela (para casar com
// o CURDATE() do servidor) e preenche cada dia. Falha na consulta dos dias
// devolve a série zerada (como o legado); dia que não escaneia é ignorado.
func (r *Dashboard) serieCompleta(ctx context.Context, dias int, vendas map[string]analytics.PontoVenda) []analytics.PontoVenda {
	rows, err := r.q.QueryContext(ctx, sqlDiasDaSerie, dias)
	if err != nil {
		return analytics.SerieZerada(dias, r.relogio())
	}
	defer rows.Close()
	var sequencia []string
	for rows.Next() {
		var dia string
		if err := rows.Scan(&dia); err != nil {
			continue
		}
		sequencia = append(sequencia, dia)
	}
	return analytics.SerieCompleta(sequencia, vendas)
}

// Ranking pagina o ranking de vendedores ativos do escopo (o restrito vê só
// a própria linha; total 0 ou 1). A ordenação é do SQL, antes do LIMIT.
func (r *Dashboard) Ranking(ctx context.Context, e kernel.Escopo, p kernel.Pagina) (kernel.Resultado[analytics.LinhaRanking], error) {
	// Uma única tradução do escopo (O1) para as duas consultas: o COUNT filtra
	// por vendedores.id e a lista por v.id, com o mesmo argumento.
	vendedorID, filtra, err := e.Filtro()
	if err != nil {
		return kernel.Resultado[analytics.LinhaRanking]{}, err
	}
	var filtroTotal, filtroLista string
	var args []any
	if filtra {
		filtroTotal, filtroLista, args = filtroVendedorPorID, filtroVendedorAlias, []any{vendedorID}
	}
	var total int
	if err := r.q.QueryRowContext(ctx, sqlRankingTotal+filtroTotal, args...).Scan(&total); err != nil {
		return kernel.Resultado[analytics.LinhaRanking]{}, fmt.Errorf("analyticsmysql: ranking count: %w", err)
	}
	linhas, err := r.linhasRanking(ctx, sqlRankingInicio+filtroLista+sqlRankingFim, db.Paginada(args, p), p.Limite())
	if err != nil {
		return kernel.Resultado[analytics.LinhaRanking]{}, err
	}
	vlog.Printf(arqDashboard, "Dashboard.Ranking", "SELECT vendedores ranking linhas=%d total=%d", len(linhas), total)
	return kernel.NovoResultado(linhas, p, total), nil
}

// linhasRanking lê as linhas do ranking (lista vazia, nunca nil).
func (r *Dashboard) linhasRanking(ctx context.Context, q string, args []any, capacidade int) ([]analytics.LinhaRanking, error) {
	rows, err := r.q.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("analyticsmysql: ranking: %w", err)
	}
	defer rows.Close()

	linhas := make([]analytics.LinhaRanking, 0, capacidade)
	for rows.Next() {
		var d analytics.DadosRanking
		if err := rows.Scan(&d.VendedorID, &d.VendedorNome, &d.Regiao, &d.UF, &d.Meta, &d.TotalVendas, &d.TotalPedidos, &d.Atingimento); err != nil {
			return nil, fmt.Errorf("analyticsmysql: ranking scan: %w", err)
		}
		linhas = append(linhas, analytics.NovaLinhaRanking(d))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("analyticsmysql: ranking rows: %w", err)
	}
	return linhas, nil
}
