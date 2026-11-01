package handlers_test

// ARQ-07 / N4 (🟣 SecBrain, ARQ-04): o log operacional do bloqueio de
// vendedor desligado voltou a levar a rota (rota=<path>), como no legado. O
// httpx.Autenticador grava o path no contexto (kernel.ComRota) e o
// AutorizadorEscopo o lê (kernel.RotaDe), sem a aplicação conhecer net/http.

import (
	"log"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARQ07_N4_BloqueioDesligadoLogaARota(t *testing.T) {
	logs := &logSeguro{}
	anterior := log.Writer()
	log.SetOutput(logs)
	t.Cleanup(func() { log.SetOutput(anterior) })

	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	expectEscopoDesligado(mock, 10)

	req, _ := http.NewRequest("GET", server.URL+"/api/clientes/1?busca=x", nil)
	req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 2, "normal"))
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Contains(t, logs.String(), "[escopo] bloqueado vendedor_desligado user_id=2 vendedor_id=10 rota=/api/clientes/1")
	assert.NotContains(t, logs.String(), "busca=x", "a query string não vai para o log (F-09)")
	assert.NoError(t, mock.ExpectationsWereMet())
}
