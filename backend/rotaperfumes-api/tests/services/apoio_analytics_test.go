package services_test

// ARQ-07 (pendência m): os testes do Dashboard chamam analyticsapp.Consultas
// (montado pelo bootstrap sobre os adapters reais e o sqlmock) e asserem
// sobre os read models de analytics (o repassador com maps legados saiu).
// A porta de escopo é um dublê SEM SQL (§7.2-5): 0 → Irrestrito de admin;
// > 0 → carteira do vendedor; SemAcesso → resposta zerada (D-01).

import (
	"context"
	"database/sql"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsapp"
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

// consultasAnalytics: vendedorID 0 = admin (Irrestrito); > 0 = carteira.
func consultasAnalytics(db *sql.DB, vendedorID int64) *analyticsapp.Consultas {
	e := func() kernel.Escopo {
		if vendedorID > 0 {
			e, _ := kernel.EscopoDoVendedor(vendedorID)
			return e
		}
		admin, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
		e, _ := kernel.EscopoIrrestrito(admin)
		return e
	}()
	return bootstrap.NovoAnalytics(db, escopoFixo{e: e})
}

// consultasSemAcesso: o escopo SemAcesso (desligado opcional) devolve a
// resposta zerada antes de qualquer SQL.
func consultasSemAcesso(db *sql.DB, desligado bool) *analyticsapp.Consultas {
	return bootstrap.NovoAnalytics(db, escopoFixo{e: kernel.EscopoSemAcesso(), desligado: desligado})
}

// periodoDe é o parse do HTTP (today|week|month; inválido cai em month).
func periodoDe(s string) analytics.Periodo {
	p, _ := analytics.ParsePeriodo(s)
	return p
}
