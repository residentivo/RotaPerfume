package arquitetura_test

// CHORE-04 (Lote 31): os 3 testes SEC12 de integração ficaram em LF e
// gofmt (já normalizados em lote anterior; o .gitattributes agora fixa
// *.go em LF). Guarda contra a volta do CRLF e da formatação fora do gofmt.

import (
	"bytes"
	"go/format"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCHORE04_SEC12_GofmtELF(t *testing.T) {
	for _, p := range []string{
		"../repositories/sec12_chore02_refresh_integration_test.go",
		"../../../rotaperfumes-api/tests/handlers/sec12_http_integration_test.go",
		"../../../rotaperfumes-api/tests/services/sec12_chore02_integration_test.go",
	} {
		src, err := os.ReadFile(p)
		require.NoError(t, err, p)
		assert.False(t, bytes.Contains(src, []byte("\r\n")), "CRLF em %s", p)
		formatado, err := format.Source(src)
		require.NoError(t, err, p)
		assert.Equal(t, string(formatado), string(src), "fora do gofmt: %s", p)
	}
}
