package comercial

import (
	"errors"

	"github.com/rotaperfumes/shared/kernel"
)

// VendedorDaEscrita é a política de escrita sob escopo de carteira (E-05),
// função pura: decide o vendedor de um comando de criação/edição.
//   - escopo de vendedor: o vendedor_id do payload é IGNORADO e vale o do
//     escopo; restrito=true obriga o caso de uso a exigir o cliente na
//     carteira ativa desse vendedor;
//   - irrestrito (admin): vale o do payload, sem checagem de carteira;
//   - SemAcesso: ErrSemVendedor (403 "usuário sem vendedor vinculado");
//   - zero value: kernel.ErrEscopoInvalido (500, tag [escopo] invalido; N5),
//     nunca o 403 de quem só não tem carteira.
//
// A tradução passa por Escopo.Filtro (O1): nunca se deduz "irrestrito" de
// um vendedor ausente.
func VendedorDaEscrita(e kernel.Escopo, doPayload int64) (vendedorID int64, restrito bool, err error) {
	id, restrito, err := e.Filtro()
	switch {
	case errors.Is(err, kernel.ErrEscopoSemAcesso):
		return 0, false, ErrSemVendedor
	case err != nil:
		return 0, false, err
	case restrito:
		return id, true, nil
	}
	return doPayload, false, nil
}
