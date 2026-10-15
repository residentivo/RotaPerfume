// Package handlers contém handlers HTTP.
package handlers

import (
	"context"

	"github.com/rotaperfumes/shared/kernel"
)

// ARQ-04: o escopo de carteira é resolvido pelo comercialapp.AutorizadorEscopo
// (a regra única, E-01). Este arquivo é só a PONTE para o handler legado do
// Dashboard (Analytics): converte o kernel.Escopo no vendedorScope que ele
// usa. ARQ-05: pedidos e pagamentos migraram para internal/vendashttp e não
// passam mais por aqui. TODO(ARQ-06): sai com o Dashboard.

// EscopoCarteira é a porta do escopo novo usada pelo Dashboard
// (comercialapp.AutorizadorEscopo a satisfaz; ligada no bootstrap).
type EscopoCarteira interface {
	ResolverParaAnalytics(ctx context.Context, p kernel.Principal) (kernel.Escopo, bool, error)
}

// vendedorScope é a forma legada do escopo. O ZERO VALUE é restrito e sem
// vendedor, isto é, SemAcesso (N1): só o escopo Irrestrito (admin) liga
// Irrestrito. Restrito com VendedorID 0 é SemAcesso (o Dashboard responde
// zerado antes de consultar), então nenhum caminho chega ao repositório
// legado com 0 = "global" a partir de um usuário restrito.
type vendedorScope struct {
	Irrestrito bool
	VendedorID int64
	escopo     kernel.Escopo
}

// escopoLegado converte o Escopo novo SÓ pelo Filtro() (O1): erro
// (SemAcesso ou zero value) → restrito sem vendedor; filtra → o vendedor;
// sem filtro (só Irrestrito) → irrestrito.
func escopoLegado(e kernel.Escopo) vendedorScope {
	vendedorID, filtra, err := e.Filtro()
	switch {
	case err != nil:
		return vendedorScope{escopo: e}
	case filtra:
		return vendedorScope{VendedorID: vendedorID, escopo: e}
	default:
		return vendedorScope{Irrestrito: true, escopo: e}
	}
}

// Restrito informa se o escopo limita a carteira (tudo que não é admin).
func (s vendedorScope) Restrito() bool { return !s.Irrestrito }

// SemAcesso reporta o escopo de quem não tem carteira.
func (s vendedorScope) SemAcesso() bool {
	return s.Restrito() && s.VendedorID <= 0
}

// PermiteVendedor reporta se o escopo enxerga registros do vendedorID.
func (s vendedorScope) PermiteVendedor(vendedorID int64) bool {
	return s.escopo.Permite(vendedorID)
}
