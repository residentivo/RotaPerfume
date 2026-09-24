package repositories

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Teste interno (package repositories) porque vendedorFilter não é exportado.

func TestVendedorFilter(t *testing.T) {
	casos := []struct {
		nome       string
		coluna     string
		vendedorID int64
		wantSQL    string
		wantArgs   []any
	}{
		{"admin (0) sem filtro", "vendedor_id", 0, "", nil},
		{"negativo sem filtro", "vendedor_id", -5, "", nil},
		{"pedidos.vendedor_id", "vendedor_id", 7, " AND vendedor_id = ?", []any{int64(7)}},
		{"vendedores.id", "id", 3, " AND id = ?", []any{int64(3)}},
		{"alias v.id", "v.id", 42, " AND v.id = ?", []any{int64(42)}},
	}
	for _, tt := range casos {
		t.Run(tt.nome, func(t *testing.T) {
			sqlFrag, args := vendedorFilter(tt.coluna, tt.vendedorID)
			assert.Equal(t, tt.wantSQL, sqlFrag)
			assert.Equal(t, tt.wantArgs, args)
			if tt.wantArgs != nil {
				// O valor nunca é interpolado no SQL, só vai por placeholder.
				assert.NotContains(t, sqlFrag, "7")
				assert.NotContains(t, sqlFrag, "42")
			}
		})
	}
}
