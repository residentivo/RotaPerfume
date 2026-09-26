package handlers_test

// Escopo de carteira (SEC-01/SEC-03) exercitado pela API pública: o
// VendedorHandler real atrás do JWTMiddleware real. Substitui o antigo teste
// white-box de resolverVendedorScope/responderErroEscopo/vendedorScope:
//   - GET /api/vendedores (ListVendedores) mostra o escopo resolvido:
//     admin → lista completa; normal → só o próprio vendedor; sem vínculo →
//     200 []; desligado → 403; erro de banco → 500 sem vazar detalhe;
//   - GET /api/vendedores/{id}/clientes (ListClientesDoVendedor) mostra
//     PermiteVendedor: normal só enxerga a própria carteira (404 nas demais).

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/middleware"
)

const (
	scUserID           = int64(7)
	scReListaTodos     = `SELECT id, nome, regiao, uf, data_desligamento\s+FROM vendedores\s+ORDER BY`
	scReListaProprio   = `SELECT id, nome, regiao, uf, data_desligamento\s+FROM vendedores\s+WHERE id = \?`
	scReExisteVendedor = `SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`
)

func scVendedorResumoRows(ids ...int64) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_desligamento"})
	for _, id := range ids {
		rows.AddRow(id, "Vendedor", "Sul", "PR", nil)
	}
	return rows
}

// scRequisitar executa fn (um handler público) atrás do JWTMiddleware real,
// autenticado como scUserID com a role informada.
func scRequisitar(t *testing.T, role, path string, fn http.HandlerFunc) (int, map[string]any) {
	t.Helper()
	cfg := testCfg()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+generateToken(t, cfg, scUserID, role))
	rec := httptest.NewRecorder()

	mux := http.NewServeMux()
	mux.Handle("GET /api/vendedores", fn)
	mux.Handle("GET /api/vendedores/{id}/clientes", fn)
	middleware.JWTMiddleware(cfg, true, false)(mux).ServeHTTP(rec, req)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "body: %s", rec.Body.String())
	return rec.Code, body
}

func scNewDB(t *testing.T) (*handlers.VendedorHandler, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return handlers.NewVendedorHandler(db, testCfg()), mock
}

func scExpectIDVendedor(m sqlmock.Sqlmock, v any) {
	m.ExpectQuery(reUsuarioVendedor).WithArgs(scUserID).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(v))
}

func scExpectDesligado(m sqlmock.Sqlmock, vendedorID int64, desligado int) {
	m.ExpectQuery(reVendedorDesligado).WithArgs(vendedorID).
		WillReturnRows(sqlmock.NewRows([]string{"d"}).AddRow(desligado))
}

func TestEscopo_ListVendedores_Matriz(t *testing.T) {
	dbErr := errors.New("db down: detalhe interno")

	casos := []struct {
		nome       string
		role       string
		setup      func(m sqlmock.Sqlmock)
		wantStatus int
		wantErro   string    // "" = sucesso
		wantIDs    []float64 // ids devolvidos em data (sucesso)
	}{
		{
			nome: "admin: sem consulta de escopo, lista todos",
			role: "admin",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(scReListaTodos).WillReturnRows(scVendedorResumoRows(10, 11))
			},
			wantStatus: http.StatusOK,
			wantIDs:    []float64{10, 11},
		},
		{
			nome: "normal ativo: lista só o próprio vendedor",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				scExpectIDVendedor(m, 10)
				scExpectDesligado(m, 10, 0)
				m.ExpectQuery(scReListaProprio).WithArgs(int64(10)).WillReturnRows(scVendedorResumoRows(10))
			},
			wantStatus: http.StatusOK,
			wantIDs:    []float64{10},
		},
		{
			nome: "normal desligado: 403 com a mensagem do contrato",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				scExpectIDVendedor(m, 10)
				scExpectDesligado(m, 10, 1)
			},
			wantStatus: http.StatusForbidden,
			wantErro:   msgVendedorDesligadoH,
		},
		{
			nome: "vinculo orfao (vendedor inexistente): sem vendedor, 200 vazio",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				scExpectIDVendedor(m, 10)
				m.ExpectQuery(reVendedorDesligado).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"d"}))
			},
			wantStatus: http.StatusOK,
		},
		{
			nome:       "id_vendedor NULL: sem vendedor, sem consultar desligado",
			role:       "normal",
			setup:      func(m sqlmock.Sqlmock) { scExpectIDVendedor(m, nil) },
			wantStatus: http.StatusOK,
		},
		{
			nome:       "id_vendedor <= 0: sem vendedor, sem consultar desligado",
			role:       "normal",
			setup:      func(m sqlmock.Sqlmock) { scExpectIDVendedor(m, 0) },
			wantStatus: http.StatusOK,
		},
		{
			nome: "usuario inexistente: sem vendedor",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reUsuarioVendedor).WithArgs(scUserID).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}))
			},
			wantStatus: http.StatusOK,
		},
		{
			nome: "erro ao ler id_vendedor: 500 sem vazar detalhe",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reUsuarioVendedor).WithArgs(scUserID).WillReturnError(dbErr)
			},
			wantStatus: http.StatusInternalServerError,
			wantErro:   "erro interno",
		},
		{
			nome: "erro ao checar desligado: 500 sem vazar detalhe",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				scExpectIDVendedor(m, 10)
				m.ExpectQuery(reVendedorDesligado).WithArgs(int64(10)).WillReturnError(dbErr)
			},
			wantStatus: http.StatusInternalServerError,
			wantErro:   "erro interno",
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			h, mock := scNewDB(t)
			c.setup(mock)

			status, body := scRequisitar(t, c.role, "/api/vendedores", h.ListVendedores)

			assert.Equal(t, c.wantStatus, status)
			if c.wantErro != "" {
				assert.Equal(t, false, body["success"])
				assert.Equal(t, c.wantErro, body["error"])
				assert.NotContains(t, body["error"], "db down", "detalhe interno não pode vazar")
			} else {
				assert.Equal(t, true, body["success"])
				data, _ := body["data"].([]any)
				var ids []float64
				for _, it := range data {
					ids = append(ids, it.(map[string]any)["id"].(float64))
				}
				assert.Equal(t, c.wantIDs, ids)
				if c.wantIDs == nil {
					assert.Equal(t, []any{}, body["data"], "sem vendedor: lista vazia, nunca dados de terceiros")
				}
			}
			assert.NoError(t, mock.ExpectationsWereMet(), "consultas executadas devem ser exatamente as esperadas")
		})
	}
}

// TestEscopo_SemCache: requisições seguidas refletem o estado atual do
// vendedor (cada uma consulta o banco), alternando 403/200.
func TestEscopo_SemCache(t *testing.T) {
	h, mock := scNewDB(t)
	for i, desligado := range []bool{true, false, true, false} {
		d := 0
		if desligado {
			d = 1
		}
		scExpectIDVendedor(mock, 10)
		scExpectDesligado(mock, 10, d)
		if !desligado {
			mock.ExpectQuery(scReListaProprio).WithArgs(int64(10)).WillReturnRows(scVendedorResumoRows())
		}

		status, _ := scRequisitar(t, "normal", "/api/vendedores", h.ListVendedores)
		if desligado {
			assert.Equal(t, http.StatusForbidden, status, "chamada %d", i+1)
		} else {
			assert.Equal(t, http.StatusOK, status, "chamada %d", i+1)
		}
		require.NoError(t, mock.ExpectationsWereMet(), "chamada %d deve consultar o banco", i+1)
	}
}

// TestEscopo_SemUsuarioNoContexto: handler chamado sem JWT no contexto (não
// deveria ocorrer em rota protegida) aplica o escopo mais restritivo, sem
// nenhuma consulta.
func TestEscopo_SemUsuarioNoContexto(t *testing.T) {
	h, mock := scNewDB(t)
	rec := httptest.NewRecorder()
	h.ListVendedores(rec, httptest.NewRequest(http.MethodGet, "/api/vendedores", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, []any{}, body["data"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestEscopo_PermiteVendedor: ListClientesDoVendedor só prossegue quando o
// escopo permite o vendedor do path.
func TestEscopo_PermiteVendedor(t *testing.T) {
	casos := []struct {
		nome        string
		role        string
		idVendedor  any // vínculo do usuário normal
		pathID      string
		wantPermite bool
	}{
		{"admin permite qualquer vendedor", "admin", nil, "99", true},
		{"normal: mesmo vendedor", "normal", 10, "10", true},
		{"normal: outro vendedor", "normal", 10, "11", false},
		{"normal sem vínculo (NULL)", "normal", nil, "0", false},
		{"normal sem vínculo, id negativo", "normal", 0, "-1", false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			h, mock := scNewDB(t)
			if c.role == "normal" {
				scExpectIDVendedor(mock, c.idVendedor)
				if v, ok := c.idVendedor.(int); ok && v > 0 {
					scExpectDesligado(mock, int64(v), 0)
				}
			}
			if c.wantPermite {
				// Vendedor inexistente: a busca chega ao service (404 do service).
				mock.ExpectQuery(scReExisteVendedor).WillReturnRows(sqlmock.NewRows([]string{"1"}))
			}

			status, body := scRequisitar(t, c.role, "/api/vendedores/"+c.pathID+"/clientes", h.ListClientesDoVendedor)

			assert.Equal(t, http.StatusNotFound, status)
			assert.Equal(t, "vendedor não encontrado", body["error"], "resposta idêntica permitido/negado: não confirma existência")
			assert.NoError(t, mock.ExpectationsWereMet(), "negado não pode consultar a carteira")
		})
	}
}
