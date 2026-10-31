package handlers_test

// SEC-24 (nível HTTP): order_by malicioso nas 9 listagens com ordenação
// devolve 200 e o SQL enviado ao banco usa o ORDER BY padrão da entidade
// (o parse do httpx.OrdenacaoDaQuery descarta o campo fora do enum). A tabela
// por enum, com todos os payloads, fica em
// apis/shared/tests/repositories/sec24_ordenacao_injecao_test.go.
//
// Variantes de order_by:
//   - "id%3BDROP": o ";" codificado chega ao parse como "id;DROP".
//   - "id;DROP" cru: o net/url descarta o par com ";" (Go 1.17+), também
//     tem de dar 200 com o padrão.
//   - "id,(SELECT SLEEP(5))" codificado (time-based blind).

import (
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSEC24_ListagensOrderByInjetado_200ComOrdemPadrao(t *testing.T) {
	rotas := []struct {
		rota       string
		ordemRegex string // ORDER BY padrão, imediatamente antes do LIMIT
	}{
		{"/api/usuarios", `ORDER BY u\.id ASC\s+LIMIT \? OFFSET \?`},
		{"/api/senha-historico", `ORDER BY sh\.id DESC\s+LIMIT \? OFFSET \?`},
		{"/api/clientes", `ORDER BY cliente_id_origem ASC LIMIT \? OFFSET \?`},
		{"/api/produtos", `ORDER BY id ASC\s+LIMIT \? OFFSET \?`},
		{"/api/estoque", `ORDER BY data_snapshot DESC\s+LIMIT \? OFFSET \?`},
		{"/api/pedidos", `ORDER BY p\.pedido_id_origem DESC LIMIT \? OFFSET \?`},
		{"/api/pagamentos", `ORDER BY pagamento_id ASC LIMIT \? OFFSET \?`},
		{"/api/oportunidades", `ORDER BY oportunidade_id ASC LIMIT \? OFFSET \?`},
		{"/api/visitas", `ORDER BY visita_id ASC LIMIT \? OFFSET \?`},
	}
	payloads := []struct{ nome, query string }{
		{"ponto e vírgula codificado", "order_by=id%3BDROP"},
		{"ponto e vírgula cru", "order_by=id;DROP"},
		{"SLEEP codificado", "order_by=id%2C(SELECT%20SLEEP(5))&order_dir=asc%3B%20DROP"},
	}
	for _, r := range rotas {
		for _, p := range payloads {
			t.Run(r.rota+"/"+p.nome, func(t *testing.T) {
				server, db, mock := setupTestServer(t)
				defer server.Close()
				defer db.Close()

				mock.ExpectQuery(`SELECT COUNT\(\*\)`).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				mock.ExpectQuery(r.ordemRegex).
					WillReturnRows(sqlmock.NewRows([]string{"id"}))

				req, _ := http.NewRequest("GET", server.URL+r.rota+"?"+p.query, nil)
				req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
				resp, err := (&http.Client{}).Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				assert.Equal(t, http.StatusOK, resp.StatusCode, string(readBody(t, resp)))
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}
