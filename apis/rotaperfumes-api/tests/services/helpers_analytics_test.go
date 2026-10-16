package services_test

// ARQ-06: o DashboardService saiu de rotaperfumes-api/services. Os testes
// (mesmos nomes e asserções) usam os nomes antigos, declarados AQUI como
// repassadores finos para analyticsapp.Consultas, montado pelo bootstrap
// sobre os adapters reais (analyticsmysql) e o db do sqlmock. Nenhuma regra
// mora aqui.
//
// O legado recebia o vendedorID de escopo já resolvido (0 = admin); a
// consulta nova recebe o Principal e resolve o escopo pela porta. Os
// repassadores ligam a porta a um escopo fixo, SEM SQL (§7.2-5: 0 →
// Irrestrito de admin; > 0 → EscopoDoVendedor). Os Empty* do legado são a
// mesma consulta com o escopo SemAcesso (resposta zerada sem SQL, D-01). O
// read model volta no map do legado (mesmas chaves e tipos Go). O cfg
// (Verbose) não tem mais efeito: o log é o vlog global. Sai no ARQ-07.

import (
	"context"
	"database/sql"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsapp"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/kernel"
)

// escopoFixo satisfaz analyticsapp.EscopoAnalytics sem consultar o banco.
type escopoFixo struct {
	e         kernel.Escopo
	desligado bool
}

func (f escopoFixo) ResolverParaAnalytics(context.Context, kernel.Principal) (kernel.Escopo, bool, error) {
	return f.e, f.desligado, nil
}

// escopoDoVendedorLegado traduz o vendedorID de escopo do legado.
func escopoDoVendedorLegado(vendedorID int64) kernel.Escopo {
	if vendedorID > 0 {
		e, _ := kernel.EscopoDoVendedor(vendedorID)
		return e
	}
	admin, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	e, _ := kernel.EscopoIrrestrito(admin)
	return e
}

type dashboardService struct{ db *sql.DB }

// NewDashboardService substitui services.NewDashboardService.
func NewDashboardService(db *sql.DB, _ *config.Config) dashboardService {
	return dashboardService{db: db}
}

func (s dashboardService) consultas(e kernel.Escopo, desligado bool) *analyticsapp.Consultas {
	return bootstrap.NovoAnalytics(s.db, escopoFixo{e: e, desligado: desligado})
}

func periodoLegado(s string) analytics.Periodo {
	p, _ := analytics.ParsePeriodo(s)
	return p
}

func (s dashboardService) GetMetrics(ctx context.Context, _ *sql.DB, periodo string, vendedorID int64) (map[string]any, error) {
	m, err := s.consultas(escopoDoVendedorLegado(vendedorID), false).Metricas(ctx, analyticsapp.ConsultaMetricas{Periodo: periodoLegado(periodo)})
	if err != nil {
		return nil, err
	}
	return metricasLegado(m), nil
}

func (s dashboardService) EmptyMetrics(periodo string, vendedorDesligado bool) map[string]any {
	m, _ := s.consultas(kernel.EscopoSemAcesso(), vendedorDesligado).Metricas(context.Background(), analyticsapp.ConsultaMetricas{Periodo: periodoLegado(periodo)})
	return metricasLegado(m)
}

func metricasLegado(m analytics.Metricas) map[string]any {
	return map[string]any{
		"periodo": m.Periodo.String(), "total_vendas": m.TotalVendas, "total_vendas_qtd": m.TotalVendasQtd,
		"total_pedidos": m.TotalPedidos, "ticket_medio": m.TicketMedio, "top_vendedores": m.TopVendedores,
		"metas_vendedores": m.MetasVendedores, "meta_mes": m.MetaMes, "vendedor_desligado": m.VendedorDesligado,
	}
}

func (s dashboardService) GetVendasSeries(ctx context.Context, _ *sql.DB, dias int, vendedorID int64) (map[string]any, error) {
	serie, err := s.consultas(escopoDoVendedorLegado(vendedorID), false).SerieVendas(ctx, analyticsapp.ConsultaSerieVendas{Dias: analytics.DiasDe(dias)})
	if err != nil {
		return nil, err
	}
	return serieLegado(serie), nil
}

func (s dashboardService) EmptyVendasSeries(dias int) map[string]any {
	serie, _ := s.consultas(kernel.EscopoSemAcesso(), false).SerieVendas(context.Background(), analyticsapp.ConsultaSerieVendas{Dias: analytics.DiasDe(dias)})
	return serieLegado(serie)
}

func serieLegado(s analytics.SerieVendas) map[string]any {
	pontos := make([]map[string]any, 0, len(s.Pontos))
	for _, p := range s.Pontos {
		pontos = append(pontos, map[string]any{"dia": p.Dia, "total_vendas": p.TotalVendas, "total_pedidos": p.TotalPedidos})
	}
	return map[string]any{"dias": s.Dias.Int(), "pontos": pontos}
}

func (s dashboardService) GetVendedoresRanking(ctx context.Context, _ *sql.DB, page, limit int, vendedorID int64) ([]map[string]any, int, error) {
	res, err := s.consultas(escopoDoVendedorLegado(vendedorID), false).Ranking(ctx, analyticsapp.ConsultaRanking{Pagina: kernel.NovaPagina(page, limit)})
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

func (s dashboardService) GetClienteMetrics(ctx context.Context, _ *sql.DB, periodo string, vendedorID int64) (map[string]any, error) {
	m, err := s.consultas(escopoDoVendedorLegado(vendedorID), false).MetricasClientes(ctx, analyticsapp.ConsultaMetricasClientes{Periodo: periodoLegado(periodo)})
	if err != nil {
		return nil, err
	}
	return clientesLegado(m), nil
}

func (s dashboardService) EmptyClienteMetrics(periodo string) map[string]any {
	m, _ := s.consultas(kernel.EscopoSemAcesso(), false).MetricasClientes(context.Background(), analyticsapp.ConsultaMetricasClientes{Periodo: periodoLegado(periodo)})
	return clientesLegado(m)
}

func clientesLegado(m analytics.MetricasClientes) map[string]any {
	return map[string]any{
		"periodo": m.Periodo.String(), "total_clientes": m.TotalClientes, "total_ativos": m.TotalAtivos,
		"total_inativos": m.TotalInativos, "novos_no_periodo": m.NovosNoPeriodo,
		"por_segmento": m.PorSegmento, "por_uf": m.PorUF,
	}
}

// ParsePagination substitui services.ParsePagination (removido no ARQ-06
// com o último handler legado): a regra única é kernel.NovaPagina, lida da
// query string pelo httpx (F-10).
func ParsePagination(page, limit string) (int, int) {
	p := httpx.PaginaDaQueryStrings(page, limit)
	return p.Numero(), p.Limite()
}
