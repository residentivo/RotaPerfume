package httpx_test

// SEC-25 (Lote 27): httpx.BuscaDaQuery aceita até 100 caracteres (runes)
// depois do trim; acima disso é ErrBuscaMuitoLonga (400), sem truncar.

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/kernel"
)

func TestSEC25_BuscaDaQuery(t *testing.T) {
	casos := []struct {
		nome, q, want string
		err           bool
	}{
		{"ausente", "", "", false},
		{"trim", "  perfume  ", "perfume", false},
		{"curingas passam (são escapados no adapter)", "%_!", "%_!", false},
		{"100 ASCII", strings.Repeat("a", 100), strings.Repeat("a", 100), false},
		{"101 ASCII", strings.Repeat("a", 101), "", true},
		{"100 ç (200 bytes)", strings.Repeat("ç", 100), strings.Repeat("ç", 100), false},
		{"101 ç", strings.Repeat("ç", 101), "", true},
		{"100 + espaços nas pontas", "   " + strings.Repeat("b", 100) + "   ", strings.Repeat("b", 100), false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/x?q="+url.QueryEscape(c.q), nil)
			got, err := httpx.BuscaDaQuery(req)
			if c.err {
				require.ErrorIs(t, err, httpx.ErrBuscaMuitoLonga)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
	assert.Equal(t, kernel.KindInvalido, httpx.ErrBuscaMuitoLonga.Kind(), "400")
	assert.Equal(t, "o parâmetro q deve ter no máximo 100 caracteres", httpx.ErrBuscaMuitoLonga.Error())
}
