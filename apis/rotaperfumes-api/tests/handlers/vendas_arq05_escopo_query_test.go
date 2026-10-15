package handlers_test

// ARQ-05 (TestBrain, E-03): na listagem restrita, ?vendedor_id=<outro> é
// ignorado; o SQL usa o vendedor do escopo (args do sqlmock), nunca o da
// query. Vale para pedidos (filtro de vendedor) e pagamentos (subconsulta
// por pedidos.vendedor_id, V-02).

import (
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARQ05_ListagemRestrita_IgnoraVendedorDaQuery(t *testing.T) {
	casos := []struct {
		rota, reCount, reLista string
	}{
		{"/api/pedidos?vendedor_id=99", `SELECT COUNT\(\*\) FROM pedidos p .+ WHERE p\.vendedor_id = \?$`,
			`WHERE p\.vendedor_id = \? ORDER BY p\.pedido_id_origem DESC LIMIT \? OFFSET \?`},
		{"/api/pagamentos?vendedor_id=99", `SELECT COUNT\(\*\) FROM pagamentos WHERE pedido_id IN \(SELECT pedido_id_origem FROM pedidos WHERE vendedor_id = \?\)$`,
			`WHERE pedido_id IN \(SELECT pedido_id_origem FROM pedidos WHERE vendedor_id = \?\) ORDER BY pagamento_id ASC LIMIT \? OFFSET \?`},
	}
	for _, c := range casos {
		t.Run(c.rota, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(10)))
			expectVendedorDesligado(mock, int64(10), false)
			mock.ExpectQuery(c.reCount).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
			mock.ExpectQuery(c.reLista).WithArgs(int64(10), 20, 0).WillReturnRows(sqlmock.NewRows(nil))

			status, _ := getComToken(t, server.URL+c.rota, 2, "normal")

			require.Equal(t, http.StatusOK, status)
			assert.NoError(t, mock.ExpectationsWereMet(), "o vendedor do escopo (10) substitui o da query (99)")
		})
	}
}
