package repositories_test

// ARQ-06: o DashboardRepository e as métricas de cliente do
// ClienteRepository migraram para analytics/analyticsmysql (SQL idêntico).
// Os testes mantêm os nomes e exercitam os adapters novos por estes
// repassadores finos, que só traduzem a assinatura antiga (db por
// parâmetro, periodo em string, dias/page/limit em int, vendedorID int64 de
// escopo) para a nova, e devolvem os read models tipados nos formatos que
// as asserções legadas leem (os pontos da série e as linhas do ranking como
// map[string]any, com os mesmos tipos Go do legado). O vendedorID de escopo
// segue o §7.2-5 (escopoLegado: 0 ou negativo → Irrestrito de admin; > 0 →
// EscopoDoVendedor). O periodo fora do enum cai no mês, o zero value de
// analytics.Periodo (o default do legado). Nenhuma regra mora aqui. Sai no
// ARQ-07.

import (
	"context"
	"database/sql"
	"time"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsmysql"
	"github.com/rotaperfumes/shared/kernel"
)

// periodoLegado: ParsePeriodo devolve PeriodoMes (zero value) para o texto
// fora do enum, como o default do switch legado.
func periodoLegado(s string) analytics.Periodo {
	p, _ := analytics.ParsePeriodo(s)
	return p
}

type dashboardRepo struct{}

// NewDashboardRepository substitui repositories.NewDashboardRepository.
func NewDashboardRepository() dashboardRepo { return dashboardRepo{} }

func (dashboardRepo) GetVendasTotais(ctx context.Context, db *sql.DB, periodo string, vendedorID int64) (float64, int, error) {
	t, err := analyticsmysql.NovoDashboard(db, time.Now).VendasTotais(ctx, escopoLegado(vendedorID), periodoLegado(periodo))
	return t.Valor, t.Quantidade, err
}

func (dashboardRepo) GetTotalPedidos(ctx context.Context, db *sql.DB, periodo string, vendedorID int64) (int, error) {
	return analyticsmysql.NovoDashboard(db, time.Now).TotalPedidos(ctx, escopoLegado(vendedorID), periodoLegado(periodo))
}

func (dashboardRepo) GetMetaMensalTotal(ctx context.Context, db *sql.DB, vendedorID int64) (float64, error) {
	return analyticsmysql.NovoDashboard(db, time.Now).MetaMensalTotal(ctx, escopoLegado(vendedorID))
}

func (dashboardRepo) GetTopVendedores(ctx context.Context, db *sql.DB, limit int, vendedorID int64) ([]analytics.VendedorRanking, error) {
	return analyticsmysql.NovoDashboard(db, time.Now).TopVendedores(ctx, escopoLegado(vendedorID), limit)
}

func (dashboardRepo) GetMetasVendedores(ctx context.Context, db *sql.DB, vendedorID int64) ([]analytics.MetaVendedor, error) {
	return analyticsmysql.NovoDashboard(db, time.Now).MetasVendedores(ctx, escopoLegado(vendedorID))
}

func (dashboardRepo) GetVendasSeries(ctx context.Context, db *sql.DB, dias int, vendedorID int64) ([]map[string]any, error) {
	pontos, err := analyticsmysql.NovoDashboard(db, time.Now).SerieVendas(ctx, escopoLegado(vendedorID), analytics.DiasDe(dias))
	if err != nil {
		return nil, err
	}
	return pontosLegado(pontos), nil
}

func (dashboardRepo) EmptySeries(dias int) []map[string]any {
	return pontosLegado(analytics.SerieZerada(dias, time.Now()))
}

func (dashboardRepo) GetVendedoresRanking(ctx context.Context, db *sql.DB, page, limit int, vendedorID int64) ([]map[string]any, int, error) {
	res, err := analyticsmysql.NovoDashboard(db, time.Now).Ranking(ctx, escopoLegado(vendedorID), kernel.NovaPagina(page, limit))
	if err != nil {
		return nil, 0, err
	}
	linhas := make([]map[string]any, 0, len(res.Itens))
	for _, l := range res.Itens {
		linhas = append(linhas, map[string]any{
			"vendedor_id": l.VendedorID, "vendedor_nome": l.VendedorNome, "regiao": l.Regiao, "uf": l.UF,
			"total_vendas": l.TotalVendas, "total_pedidos": l.TotalPedidos, "ticket_medio": l.TicketMedio,
			"meta": l.Meta, "atingimento_meta": l.AtingimentoMeta,
		})
	}
	return linhas, res.Total, nil
}

// pontosLegado devolve os pontos no formato do legado ({dia, total_vendas,
// total_pedidos}, float64 e int).
func pontosLegado(pontos []analytics.PontoVenda) []map[string]any {
	out := make([]map[string]any, 0, len(pontos))
	for _, p := range pontos {
		out = append(out, map[string]any{"dia": p.Dia, "total_vendas": p.TotalVendas, "total_pedidos": p.TotalPedidos})
	}
	return out
}

// Métricas de cliente (ClienteRepository legado; clienteRepo está em
// helpers_comercial_test.go).

func (clienteRepo) CountTotal(ctx context.Context, db *sql.DB, vendedorID int64) (int, error) {
	return analyticsmysql.NovoClientes(db).Total(ctx, escopoLegado(vendedorID))
}

func (clienteRepo) CountPorAtivo(ctx context.Context, db *sql.DB, ativo bool, vendedorID int64) (int, error) {
	return analyticsmysql.NovoClientes(db).PorAtivo(ctx, escopoLegado(vendedorID), ativo)
}

func (clienteRepo) CountNovosNoPeriodo(ctx context.Context, db *sql.DB, periodo string, vendedorID int64) (int, error) {
	return analyticsmysql.NovoClientes(db).NovosNoPeriodo(ctx, escopoLegado(vendedorID), periodoLegado(periodo))
}

func (clienteRepo) CountPorSegmento(ctx context.Context, db *sql.DB, vendedorID int64) ([]analytics.ContagemSegmento, error) {
	return analyticsmysql.NovoClientes(db).PorSegmento(ctx, escopoLegado(vendedorID))
}

func (clienteRepo) CountPorUF(ctx context.Context, db *sql.DB, vendedorID int64) ([]analytics.ContagemUF, error) {
	return analyticsmysql.NovoClientes(db).PorUF(ctx, escopoLegado(vendedorID))
}
