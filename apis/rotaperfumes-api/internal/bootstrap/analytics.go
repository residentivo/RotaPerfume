package bootstrap

import (
	"database/sql"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/analyticshttp"
	"github.com/rotaperfumes/shared/analytics/analyticsapp"
	"github.com/rotaperfumes/shared/analytics/analyticsmysql"
)

// NovoAnalytics liga o contexto Analytics (ARQ-06) sobre o pool: os
// adapters MySQL do Dashboard (e o do funil de oportunidades, DASH-01) e o
// escopo de carteira de Comercial (o AutorizadorEscopo.ResolverParaAnalytics,
// a mesma regra de todo o sistema, D-01).
func NovoAnalytics(pool *sql.DB, escopo analyticsapp.EscopoAnalytics) *analyticsapp.Consultas {
	relogio := time.Now // o mesmo "hoje" para as séries zeradas do caso de uso e do adapter
	return analyticsapp.Novo(analyticsapp.Deps{
		Vendas:   analyticsmysql.NovoDashboard(pool, relogio),
		Clientes: analyticsmysql.NovoClientes(pool),
		Funil:    analyticsmysql.NovoOportunidades(pool),
		Escopo:   escopo,
		Relogio:  relogio,
	})
}

// NovoAnalyticsHTTP monta o handler do Dashboard sobre as consultas.
func NovoAnalyticsHTTP(c *analyticsapp.Consultas) *analyticshttp.DashboardHandler {
	return analyticshttp.NovoDashboardHandler(c)
}
