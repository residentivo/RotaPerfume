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
// periodo: "today" (dia atual) ou "month" (mes atual).
func (s *DashboardService) GetMetrics(ctx context.Context, db *sql.DB, periodo string) (map[string]any, error) {
	if s.Cfg.Verbose {
		log.Printf("[dashboard] GetMetrics periodo=%s", periodo)
	}

	// Vendas do dia/mes (sera 0 se tabela pedidos nao existir).
	totalVendasValor, totalVendasQuantidade, err := s.repo.GetVendasTotais(ctx, db, periodo)
	if err != nil {
		return nil, err
	}

	// Total de pedidos.
	totalPedidos, err := s.repo.GetTotalPedidos(ctx, db, periodo)
	if err != nil {
		return nil, err
	}

	// Ranking top 10 vendedores (por valor de vendas).
	topVendedores, err := s.repo.GetTopVendedores(ctx, db, 10)
	if err != nil {
		return nil, err
	}

	// Metas: comparativo real vs meta dos vendedores ativos.
	metas, err := s.repo.GetMetasVendedores(ctx, db)
	if err != nil {
		return nil, err
	}

	// Ticket medio.
	var ticketMedio float64
	if totalVendasQuantidade > 0 {
		ticketMedio = totalVendasValor / float64(totalVendasQuantidade)
	}

	return map[string]any{
		"periodo":           periodo,
		"total_vendas_valor":  totalVendasValor,
		"total_vendas_qtd":    totalVendasQuantidade,
		"total_pedidos":       totalPedidos,
		"ticket_medio":        ticketMedio,
		"top_vendedores":      topVendedores,
		"metas_vendedores":    metas,
	}, nil
}

// GetVendasSeries retorna serie temporal de vendas dos ultimos N dias.
func (s *DashboardService) GetVendasSeries(ctx context.Context, db *sql.DB, dias int) ([]map[string]any, error) {
	if s.Cfg.Verbose {
		log.Printf("[dashboard] GetVendasSeries dias=%d", dias)
	}
	return s.repo.GetVendasSeries(ctx, db, dias)
}

// GetVendedoresRanking retorna ranking paginado de vendedores com vendas e meta.
func (s *DashboardService) GetVendedoresRanking(ctx context.Context, db *sql.DB, page, limit int) ([]map[string]any, int, error) {
	if s.Cfg.Verbose {
		log.Printf("[dashboard] GetVendedoresRanking page=%d limit=%d", page, limit)
	}
	return s.repo.GetVendedoresRanking(ctx, db, page, limit)
}

// GetClienteMetrics retorna metricas agregadas da base de clientes: totais
// (geral, ativos, inativos), novos cadastros no periodo informado e a
// distribuicao por segmento e por UF.
// periodo: "today" (dia atual) ou "month" (mes atual).
func (s *DashboardService) GetClienteMetrics(ctx context.Context, db *sql.DB, periodo string) (map[string]any, error) {
	if s.Cfg.Verbose {
		log.Printf("[dashboard] GetClienteMetrics periodo=%s", periodo)
	}

	total, err := s.clienteRepo.CountTotal(ctx, db)
	if err != nil {
		return nil, err
	}
	totalAtivos, err := s.clienteRepo.CountPorAtivo(ctx, db, true)
	if err != nil {
		return nil, err
	}
	totalInativos, err := s.clienteRepo.CountPorAtivo(ctx, db, false)
	if err != nil {
		return nil, err
	}
	novosNoPeriodo, err := s.clienteRepo.CountNovosNoPeriodo(ctx, db, periodo)
	if err != nil {
		return nil, err
	}
	porSegmento, err := s.clienteRepo.CountPorSegmento(ctx, db)
	if err != nil {
		return nil, err
	}
	porUF, err := s.clienteRepo.CountPorUF(ctx, db)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"periodo":          periodo,
		"total_clientes":   total,
		"total_ativos":     totalAtivos,
		"total_inativos":   totalInativos,
		"novos_no_periodo": novosNoPeriodo,
		"por_segmento":     porSegmento,
		"por_uf":           porUF,
	}, nil
}
