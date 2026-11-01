package bootstrap_test

// SEC-14 (Lote 27): bootstrap.NovoServidor fixa os timeouts F-04 e, contra
// slowloris e cabeçalhos gigantes, ReadHeaderTimeout de 5 s e
// MaxHeaderBytes de 64 KiB.

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
)

func TestSEC14_NovoServidor(t *testing.T) {
	h := http.NotFoundHandler()
	srv := bootstrap.NovoServidor(h)

	assert.Equal(t, ":8080", srv.Addr)
	assert.NotNil(t, srv.Handler)
	assert.Equal(t, 5*time.Second, srv.ReadHeaderTimeout)
	assert.Equal(t, 64<<10, srv.MaxHeaderBytes)
	assert.Equal(t, 15*time.Second, srv.ReadTimeout, "F-04 inalterado")
	assert.Equal(t, 15*time.Second, srv.WriteTimeout, "F-04 inalterado")
	assert.Equal(t, 60*time.Second, srv.IdleTimeout, "F-04 inalterado")
}
