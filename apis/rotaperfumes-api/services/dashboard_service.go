// Package services (da API) orquestra regras de negocio dos endpoints de dashboard.
package services

import (
	"context"
	"database/sql"
	"log"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/repositories"
)

// DashboardService agrega metricas de dashboard.
type DashboardService struct {
	repo        *repositories.DashboardRepository
	clienteRepo *repositories.ClienteRepository
	Cfg         *config.Config
}

// NewDashboardService cria um DashboardService com pool de conexao injetado.
func NewDashboardService(db *sql.DB, cfg *config.Config) *DashboardService {
	return &DashboardService{
		repo:        repositories.NewDashboardRepository(),
		clienteRepo: repositories.NewClienteRepository(),
		Cfg:         cfg,
	}
}

// GetMetrics retorna metricas agregadas de vendas para o periodo informado.
// periodo: "today" (dia atual), "week" (ultimos 7 dias) ou "month" (mes atual).
// vendedorID > 0 (usuario normal) restringe todos os numeros ao proprio
// vendedor (pedidos.vendedor_id); 0 (admin) considera todos.
func (s *DashboardService) GetMetrics(ctx context.Context, db *sql.DB, periodo string, vendedorID int64) (map[string]any, error) {
	if s.Cfg.Verbose {
		log.Printf("[dashboard] GetMetrics periodo=%s vendedor_id=%d", periodo, vendedorID)
	}

	// Vendas do dia/mes (sera 0 se tabela pedidos nao existir).
	totalVendasValor, totalVendasQuantidade, err := s.repo.GetVendasTotais(ctx, db, periodo, vendedorID)
	if err != nil {
		return nil, err
	}

	// Total de pedidos.
	totalPedidos, err := s.repo.GetTotalPedidos(ctx, db, periodo, vendedorID)
	if err != nil {
		return nil, err
	}

	// Ranking top 10 vendedores (por valor de vendas).
	topVendedores, err := s.repo.GetTopVendedores(ctx, db, 10, vendedorID)
	if err != nil {
		return nil, err
	}

	// Metas: comparativo real vs meta dos vendedores ativos.
	metas, err := s.repo.GetMetasVendedores(ctx, db, vendedorID)
	if err != nil {
		return nil, err
	}

	// Meta mensal total: soma de meta_mensal dos vendedores ativos (no escopo).
	metaMensalTotal, err := s.repo.GetMetaMensalTotal(ctx, db, vendedorID)
	if err != nil {
		return nil, err
	}

	// Ticket medio (derivado dos totais ja filtrados pelo escopo).
	var ticketMedio float64
	if totalVendasQuantidade > 0 {
		ticketMedio = totalVendasValor / float64(totalVendasQuantidade)
	}

	return metricsResponse(periodo, totalVendasValor, totalVendasQuantidade, totalPedidos,
		ticketMedio, topVendedores, metas, metaMensalTotal, false), nil
}

// EmptyMetrics retorna as metricas zeradas (mesmo formato de GetMetrics),
// usadas quando o usuario normal nao tem vendedor vinculado ou quando o
// vendedor vinculado esta desligado. vendedorDesligado preenche o flag
// vendedor_desligado da resposta (aviso exibido pelo frontend).
func (s *DashboardService) EmptyMetrics(periodo string, vendedorDesligado bool) map[string]any {
	if s.Cfg.Verbose {
		log.Printf("[dashboard] EmptyMetrics periodo=%s vendedor_desligado=%t (sem acesso)", periodo, vendedorDesligado)
	}
	return metricsResponse(periodo, 0, 0, 0, 0,
		[]repositories.VendedorRanking{}, []repositories.MetaVendedor{}, 0, vendedorDesligado)
}

// metricsResponse monta o payload de /api/dashboard/metrics.
func metricsResponse(
	periodo string,
	totalVendas float64,
	totalVendasQtd, totalPedidos int,
	ticketMedio float64,
	topVendedores []repositories.VendedorRanking,
	metas []repositories.MetaVendedor,
	metaMes float64,
	vendedorDesligado bool,
) map[string]any {
	return map[string]any{
		"periodo":            periodo,
		"total_vendas":       totalVendas,
		"total_vendas_qtd":   totalVendasQtd,
		"total_pedidos":      totalPedidos,
		"ticket_medio":       ticketMedio,
		"top_vendedores":     topVendedores,
		"metas_vendedores":   metas,
		"meta_mes":           metaMes,
		"vendedor_desligado": vendedorDesligado,
	}
}

// GetVendasSeries retorna serie temporal de vendas dos ultimos N dias, no
// formato {dias, pontos} esperado pelo frontend (VendasSeries).
// vendedorID > 0 restringe aos pedidos do vendedor; 0 (admin) = todos.
func (s *DashboardService) GetVendasSeries(ctx context.Context, db *sql.DB, dias int, vendedorID int64) (map[string]any, error) {
	if s.Cfg.Verbose {
		log.Printf("[dashboard] GetVendasSeries dias=%d vendedor_id=%d", dias, vendedorID)
	}
	pontos, err := s.repo.GetVendasSeries(ctx, db, dias, vendedorID)
	if err != nil {
		return nil, err
	}
	return vendasSeriesResponse(dias, pontos), nil
}

// EmptyVendasSeries retorna a serie zerada (todos os N dias com zero), no
// mesmo formato de GetVendasSeries, sem consultar o banco.
func (s *DashboardService) EmptyVendasSeries(dias int) map[string]any {
	if s.Cfg.Verbose {
		log.Printf("[dashboard] EmptyVendasSeries dias=%d (sem vendedor vinculado)", dias)
	}
	return vendasSeriesResponse(dias, s.repo.EmptySeries(dias))
}

// vendasSeriesResponse monta o payload de /api/dashboard/vendas.
func vendasSeriesResponse(dias int, pontos []map[string]any) map[string]any {
	return map[string]any{
		"dias":   dias,
		"pontos": pontos,
	}
}

// GetVendedoresRanking retorna ranking paginado de vendedores com vendas e meta.
// vendedorID > 0 restringe à linha do proprio vendedor; 0 (admin) = todos.
func (s *DashboardService) GetVendedoresRanking(ctx context.Context, db *sql.DB, page, limit int, vendedorID int64) ([]map[string]any, int, error) {
	if s.Cfg.Verbose {
		log.Printf("[dashboard] GetVendedoresRanking page=%d limit=%d vendedor_id=%d", page, limit, vendedorID)
	}
	return s.repo.GetVendedoresRanking(ctx, db, page, limit, vendedorID)
}

// GetClienteMetrics retorna metricas agregadas da base de clientes: totais
// (geral, ativos, inativos), novos cadastros no periodo informado e a
// distribuicao por segmento e por UF.
// periodo: "today" (dia atual), "week" (ultimos 7 dias) ou "month" (mes atual).
// vendedorID > 0 (usuario normal) restringe aos clientes da carteira ativa
// do vendedor; 0 (admin) considera todos.
func (s *DashboardService) GetClienteMetrics(ctx context.Context, db *sql.DB, periodo string, vendedorID int64) (map[string]any, error) {
	if s.Cfg.Verbose {
		log.Printf("[dashboard] GetClienteMetrics periodo=%s vendedor_id=%d", periodo, vendedorID)
	}

	total, err := s.clienteRepo.CountTotal(ctx, db, vendedorID)
	if err != nil {
		return nil, err
	}
	totalAtivos, err := s.clienteRepo.CountPorAtivo(ctx, db, true, vendedorID)
	if err != nil {
		return nil, err
	}
	totalInativos, err := s.clienteRepo.CountPorAtivo(ctx, db, false, vendedorID)
	if err != nil {
		return nil, err
	}
	novosNoPeriodo, err := s.clienteRepo.CountNovosNoPeriodo(ctx, db, periodo, vendedorID)
	if err != nil {
		return nil, err
	}
	porSegmento, err := s.clienteRepo.CountPorSegmento(ctx, db, vendedorID)
	if err != nil {
		return nil, err
	}
	porUF, err := s.clienteRepo.CountPorUF(ctx, db, vendedorID)
	if err != nil {
		return nil, err
	}

	return clienteMetricsResponse(periodo, total, totalAtivos, totalInativos, novosNoPeriodo, porSegmento, porUF), nil
}

// EmptyClienteMetrics retorna as metricas de clientes zeradas (mesmo formato
// de GetClienteMetrics, listas como []), sem consultar o banco. Usada quando
// o escopo do usuario nao permite ver nenhum cliente.
func (s *DashboardService) EmptyClienteMetrics(periodo string) map[string]any {
	if s.Cfg.Verbose {
		log.Printf("[dashboard] EmptyClienteMetrics periodo=%s (sem acesso)", periodo)
	}
	return clienteMetricsResponse(periodo, 0, 0, 0, 0,
		[]repositories.SegmentoContagem{}, []repositories.UFContagem{})
}

// clienteMetricsResponse monta o payload de /api/dashboard/clientes.
func clienteMetricsResponse(
	periodo string,
	total, totalAtivos, totalInativos, novosNoPeriodo int,
	porSegmento []repositories.SegmentoContagem,
	porUF []repositories.UFContagem,
) map[string]any {
	return map[string]any{
		"periodo":          periodo,
		"total_clientes":   total,
		"total_ativos":     totalAtivos,
		"total_inativos":   totalInativos,
		"novos_no_periodo": novosNoPeriodo,
		"por_segmento":     porSegmento,
		"por_uf":           porUF,
	}
}
