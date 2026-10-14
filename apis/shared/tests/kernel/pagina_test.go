package kernel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/kernel"
)

// F-10: default 20, máximo 100, page ≥ 1.
func TestNovaPagina(t *testing.T) {
	casos := []struct {
		nome                      string
		numero, limite            int
		wantNum, wantLim, wantOff int
	}{
		{"defaults", 0, 0, 1, 20, 0},
		{"negativos", -3, -5, 1, 20, 0},
		{"limite 1", 1, 1, 1, 1, 0},
		{"limite 100", 2, 100, 2, 100, 100},
		{"limite 101 vira 100", 1, 101, 1, 100, 0},
		{"limite enorme vira 100", 3, 1 << 30, 3, 100, 200},
		{"página 3 de 20", 3, 20, 3, 20, 40},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p := kernel.NovaPagina(c.numero, c.limite)
			assert.Equal(t, c.wantNum, p.Numero())
			assert.Equal(t, c.wantLim, p.Limite())
			assert.Equal(t, c.wantOff, p.Offset())
		})
	}
}

func TestPagina_ZeroValueEhDefault(t *testing.T) {
	var p kernel.Pagina
	assert.Equal(t, 1, p.Numero())
	assert.Equal(t, kernel.LimitePadrao, p.Limite())
	assert.Equal(t, 0, p.Offset())
}

func TestResultado_Paginas(t *testing.T) {
	casos := []struct {
		total, limite, want int
	}{
		{0, 20, 0},
		{1, 20, 1},
		{20, 20, 1},
		{21, 20, 2},
		{37, 10, 4},
		{100, 100, 1},
	}
	for _, c := range casos {
		r := kernel.NovoResultado([]int{}, kernel.NovaPagina(1, c.limite), c.total)
		assert.Equal(t, c.want, r.Paginas(), "total=%d limite=%d", c.total, c.limite)
		assert.Equal(t, c.total, r.Total)
	}
}

func TestNovoResultado_GuardaItens(t *testing.T) {
	itens := []string{"a", "b"}
	r := kernel.NovoResultado(itens, kernel.NovaPagina(2, 2), 5)
	assert.Equal(t, itens, r.Itens)
	assert.Equal(t, 2, r.Pagina.Numero())
	assert.Equal(t, 3, r.Paginas())
}
