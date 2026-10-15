// Package handlers contém handlers HTTP.
package handlers

import (
	"context"
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/kernel"
)

// ARQ-04: o escopo de carteira é resolvido pelo comercialapp.AutorizadorEscopo
// (a regra única, E-01). Este arquivo é só a PONTE para os handlers legados de
// Vendas (pedido, pagamento) e Analytics (dashboard): converte o kernel.Escopo
// no vendedorScope que eles já usam. TODO(ARQ-05/06): sai com eles.

// EscopoCarteira é a porta do escopo novo (comercialapp.AutorizadorEscopo a
// satisfaz; ligada no bootstrap).
type EscopoCarteira interface {
	Resolver(ctx context.Context, p kernel.Principal) (kernel.Escopo, error)
	ResolverParaAnalytics(ctx context.Context, p kernel.Principal) (kernel.Escopo, bool, error)
	ClienteNaCarteira(ctx context.Context, e kernel.Escopo, clienteID int64) (bool, error)
}

// vendedorScope é a forma legada do escopo. O ZERO VALUE é restrito e sem
// vendedor, isto é, SemAcesso (N1): só o escopo Irrestrito (admin) liga
// Irrestrito. Restrito com VendedorID 0 é SemAcesso (os handlers respondem
// vazio/404 antes de consultar), então nenhum caminho chega aos
// repositórios legados com 0 = "global" a partir de um usuário restrito.
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

// resolverVendedorScope resolve o escopo da requisição pelo autorizador (a
// cada chamada, sem cache). Sem usuário → SemAcesso, como no legado.
// Desligado → comercial.ErrVendedorDesligado (403
// via responderErroEscopo); erro de banco → 500.
func resolverVendedorScope(r *http.Request, escopo EscopoCarteira) (vendedorScope, error) {
	p, _ := httpx.PrincipalFrom(r) // sem usuário: o autorizador devolve SemAcesso
	e, err := escopo.Resolver(r.Context(), p)
	if err != nil {
		return vendedorScope{}, err // zero value = SemAcesso (N1)
	}
	return escopoLegado(e), nil
}

// responderErroEscopo responde pelo mapper: desligado → 403 com a mensagem
// do contrato; qualquer outro erro → log com a tag + 500 "erro interno".
func responderErroEscopo(w http.ResponseWriter, tag string, err error) {
	httpx.ResponderErro(w, tag+" escopo", err)
}

// clienteNaCarteiraDoVendedor reporta se o cliente está na carteira ativa do
// escopo (consulta própria a cada chamada).
func clienteNaCarteiraDoVendedor(ctx context.Context, escopo EscopoCarteira, s vendedorScope, clienteID int64) (bool, error) {
	return escopo.ClienteNaCarteira(ctx, s.escopo, clienteID)
}
