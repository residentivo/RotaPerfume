package kernel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
)

func principal(t *testing.T, id int64, role kernel.Role) kernel.Principal {
	t.Helper()
	p, err := kernel.NovoPrincipal(id, role)
	require.NoError(t, err)
	return p
}

func TestEscopoIrrestrito_SoDeAdmin(t *testing.T) {
	casos := []struct {
		nome string
		p    kernel.Principal
		ok   bool
	}{
		{"admin", principal(t, 1, kernel.RoleAdmin), true},
		{"normal", principal(t, 2, kernel.RoleNormal), false},
		{"role desconhecido", principal(t, 3, kernel.RoleDoBanco("gerente")), false},
		{"zero value", kernel.Principal{}, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			e, err := kernel.EscopoIrrestrito(c.p)
			if !c.ok {
				require.ErrorIs(t, err, kernel.ErrEscopoInvalido)
				assert.ErrorIs(t, e.Validar(), kernel.ErrEscopoInvalido, "o escopo devolvido no erro é o zero value")
				return
			}
			require.NoError(t, err)
			assert.True(t, e.Irrestrito())
		})
	}
}

func TestEscopoDoVendedor(t *testing.T) {
	for _, id := range []int64{0, -1, -100} {
		_, err := kernel.EscopoDoVendedor(id)
		assert.ErrorIs(t, err, kernel.ErrEscopoInvalido, "id=%d", id)
	}
	e, err := kernel.EscopoDoVendedor(7)
	require.NoError(t, err)
	assert.Equal(t, int64(7), e.VendedorID())
}

// O1: Filtro é switch exaustivo; SemAcesso e zero value nunca viram "sem filtro".
func TestEscopo_Filtro(t *testing.T) {
	irrestrito, err := kernel.EscopoIrrestrito(principal(t, 1, kernel.RoleAdmin))
	require.NoError(t, err)
	vendedor, err := kernel.EscopoDoVendedor(7)
	require.NoError(t, err)

	casos := []struct {
		nome       string
		e          kernel.Escopo
		vendedorID int64
		filtra     bool
		err        error
		validar    error
		semAcesso  bool
		irrestrito bool
	}{
		{"irrestrito", irrestrito, 0, false, nil, nil, false, true},
		{"vendedor", vendedor, 7, true, nil, nil, false, false},
		{"sem acesso", kernel.EscopoSemAcesso(), 0, false, kernel.ErrEscopoSemAcesso, nil, true, false},
		{"zero value", kernel.Escopo{}, 0, false, kernel.ErrEscopoInvalido, kernel.ErrEscopoInvalido, false, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			id, filtra, err := c.e.Filtro()
			assert.Equal(t, c.vendedorID, id)
			assert.Equal(t, c.filtra, filtra)
			if c.err == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, c.err)
			}
			if c.validar == nil {
				assert.NoError(t, c.e.Validar())
			} else {
				assert.ErrorIs(t, c.e.Validar(), c.validar)
			}
			assert.Equal(t, c.semAcesso, c.e.SemAcesso())
			assert.Equal(t, c.irrestrito, c.e.Irrestrito())
			assert.Equal(t, c.vendedorID, c.e.VendedorID())
		})
	}
}

// E-04: Permite só para o próprio vendedor; SemAcesso e zero value negam tudo.
func TestEscopo_Permite(t *testing.T) {
	irrestrito, err := kernel.EscopoIrrestrito(principal(t, 1, kernel.RoleAdmin))
	require.NoError(t, err)
	vendedor, err := kernel.EscopoDoVendedor(7)
	require.NoError(t, err)

	casos := []struct {
		nome string
		e    kernel.Escopo
		id   int64
		want bool
	}{
		{"irrestrito vê qualquer", irrestrito, 99, true},
		{"irrestrito vê sem vendedor", irrestrito, 0, true},
		{"vendedor vê o próprio", vendedor, 7, true},
		{"vendedor não vê outro", vendedor, 8, false},
		{"vendedor não vê sem vendedor", vendedor, 0, false},
		{"sem acesso", kernel.EscopoSemAcesso(), 7, false},
		{"zero value", kernel.Escopo{}, 0, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			assert.Equal(t, c.want, c.e.Permite(c.id))
		})
	}
}

func TestEscopo_ErrosSaoInternos(t *testing.T) {
	for _, e := range []*kernel.Erro{kernel.ErrEscopoInvalido, kernel.ErrEscopoSemAcesso} {
		assert.Equal(t, kernel.KindInterno, e.Kind())
		assert.Equal(t, "erro interno", e.Error())
	}
	assert.NotErrorIs(t, kernel.ErrEscopoInvalido, kernel.ErrEscopoSemAcesso, "sentinelas distintos")
}
