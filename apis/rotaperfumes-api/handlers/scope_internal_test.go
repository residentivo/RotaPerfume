package handlers

// ARQ-04 N1: teste interno da ponte (vendedorScope não é exportado). O zero
// value é restrito e sem vendedor (SemAcesso) e a conversão do Escopo novo
// sai só do Filtro(). TODO(ARQ-05/06): sai com a ponte.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/kernel"
)

func TestARQ04_N1_PonteEscopo_ZeroValueEhSemAcesso(t *testing.T) {
	var zero vendedorScope
	assert.True(t, zero.Restrito(), "zero value nunca é irrestrito")
	assert.True(t, zero.SemAcesso())
	assert.False(t, zero.PermiteVendedor(1))

	admin, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	irrestrito, _ := kernel.EscopoIrrestrito(admin)
	doVendedor, _ := kernel.EscopoDoVendedor(10)
	casos := []struct {
		nome       string
		e          kernel.Escopo
		restrito   bool
		vendedorID int64
		semAcesso  bool
	}{
		{"irrestrito", irrestrito, false, 0, false},
		{"vendedor", doVendedor, true, 10, false},
		{"sem acesso", kernel.EscopoSemAcesso(), true, 0, true},
		{"zero value do Escopo", kernel.Escopo{}, true, 0, true},
	}
	for _, c := range casos {
		s := escopoLegado(c.e)
		assert.Equal(t, c.restrito, s.Restrito(), c.nome)
		assert.Equal(t, c.vendedorID, s.VendedorID, c.nome)
		assert.Equal(t, c.semAcesso, s.SemAcesso(), c.nome)
	}
}
