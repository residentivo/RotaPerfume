package handlers_test

// ARQ-02: contrato HTTP do Catálogo depois da migração para catalogohttp.
//   - JSON byte a byte igual ao dos models legados (structs congeladas aqui);
//   - PUT ignora sku/data_snapshot no body, como os DTOs legados de update;
//   - id de path com o parse legado (negativo/zero → 404, não 400);
//   - handler de escrita sem Principal falha fechado (500) sem tocar o banco;
//   - construtores exigem os casos de uso.

import (
	"bytes"
	"encoding/json"
	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/catalogohttp"
)

// produtoLegado é a cópia congelada de models.Produto (removido no ARQ-02).
type produtoLegado struct {
	ID             int64      `json:"id"`
	SKU            string     `json:"sku"`
	Descricao      string     `json:"descricao"`
	Categoria      string     `json:"categoria"`
	Marca          string     `json:"marca"`
	NotaOlfativa   string     `json:"nota_olfativa"`
	PrecoTabela    float64    `json:"preco_tabela"`
	CustoUnitario  float64    `json:"custo_unitario"`
	Unidade        string     `json:"unidade"`
	DataLancamento *time.Time `json:"data_lancamento"`
	Ativo          bool       `json:"ativo"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// estoqueLegado é a cópia congelada de models.Estoque (removido no ARQ-02).
type estoqueLegado struct {
	ID               int64     `json:"id"`
	DataSnapshot     time.Time `json:"data_snapshot"`
	SKU              string    `json:"sku"`
	ProdutoDescricao string    `json:"produto_descricao"`
	Saldo            int       `json:"saldo"`
	Ruptura          bool      `json:"ruptura"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

var (
	arq02Lanc       = time.Date(2024, 1, 15, 0, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	arq02Criado     = time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC)
	arq02Atualizado = time.Date(2024, 2, 2, 11, 30, 0, 0, time.UTC)
)

// envelopeLegado reproduz o writeJSON/writeJSONWithPagination legados (map
// + Encoder: chaves em ordem alfabética).
func envelopeLegado(data any, paginacao map[string]any) string {
	m := map[string]any{"success": true, "data": data}
	if paginacao != nil {
		m["pagination"] = paginacao
	}
	var b bytes.Buffer
	_ = json.NewEncoder(&b).Encode(m)
	return b.String()
}

func arq02Get(t *testing.T, server *httptest.Server, path, token string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, server.URL+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, string(readBody(t, resp))
}

func TestARQ02_ContratoJSON_ByteAByte(t *testing.T) {
	admin := generateToken(t, testCfg(), 1, "admin")
	produto := produtoLegado{1, "SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico",
		99.9, 45, "UN", &arq02Lanc, true, arq02Criado, arq02Atualizado}
	linhaProduto := func() *sqlmock.Rows {
		return sqlmock.NewRows(produtoColunasHeaderForHandler()).AddRow(int64(1), "SKU-001", "Perfume Teste",
			"Perfumaria", "Marca X", "Cítrico", 99.9, 45.0, "UN", arq02Lanc, true, arq02Criado, arq02Atualizado)
	}
	estoque := estoqueLegado{3, arq02Lanc, "SKU-001", "Perfume Teste", 0, true, arq02Criado, arq02Atualizado}
	linhaEstoque := func() *sqlmock.Rows {
		return sqlmock.NewRows(estoqueColunasHeaderForHandler()).
			AddRow(int64(3), arq02Lanc, "SKU-001", "Perfume Teste", 0, true, arq02Criado, arq02Atualizado)
	}
	semData := produto
	semData.DataLancamento = nil

	casos := []struct {
		nome  string
		path  string
		mock  func(sqlmock.Sqlmock)
		corpo string
	}{
		{"GET produto", "/api/produtos/1", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(`FROM produtos WHERE id = \?`).WillReturnRows(linhaProduto())
		}, envelopeLegado(produto, nil)},
		{"GET produto sem data_lancamento (null)", "/api/produtos/1", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(`FROM produtos WHERE id = \?`).WillReturnRows(sqlmock.NewRows(produtoColunasHeaderForHandler()).
				AddRow(int64(1), "SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico", 99.9, 45.0, "UN", nil, true, arq02Criado, arq02Atualizado))
		}, envelopeLegado(semData, nil)},
		{"lista de produtos", "/api/produtos?page=2&limit=1", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(`SELECT COUNT`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(3))
			m.ExpectQuery(`FROM produtos ORDER BY id ASC`).WithArgs(1, 1).WillReturnRows(linhaProduto())
		}, envelopeLegado([]produtoLegado{produto}, map[string]any{"page": 2, "limit": 1, "total": 3, "pages": 3})},
		{"lista vazia (data null)", "/api/produtos", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(`SELECT COUNT`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(0))
			m.ExpectQuery(`FROM produtos`).WillReturnRows(emptyProdutoRowsForHandler())
		}, envelopeLegado(nil, map[string]any{"page": 1, "limit": 20, "total": 0, "pages": 0})},
		{"GET estoque", "/api/estoque/3", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(`FROM estoque.+WHERE e\.id = \?`).WillReturnRows(linhaEstoque())
		}, envelopeLegado(estoque, nil)},
		{"lista de estoque", "/api/estoque?historico=true&ruptura=true&order_by=saldo&order_dir=asc", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(`SELECT COUNT\(\*\) FROM estoque.+WHERE e\.ruptura = \?`).WithArgs(true).
				WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
			m.ExpectQuery(`WHERE e\.ruptura = \? ORDER BY e\.saldo ASC LIMIT`).WithArgs(true, 20, 0).WillReturnRows(linhaEstoque())
		}, envelopeLegado([]estoqueLegado{estoque}, map[string]any{"page": 1, "limit": 20, "total": 1, "pages": 1})},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			c.mock(mock)

			status, corpo := arq02Get(t, server, c.path, admin)
			assert.Equal(t, http.StatusOK, status)
			assert.Equal(t, c.corpo, corpo, "JSON byte a byte igual ao legado")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestARQ02_PUT_IgnoraCamposNaoEditaveis: os DTOs legados de update não
// tinham sku (produto) nem sku/data_snapshot (estoque); esses campos no body,
// mesmo com tipo errado, são ignorados (não viram 400).
func TestARQ02_PUT_IgnoraCamposNaoEditaveis(t *testing.T) {
	admin := generateToken(t, testCfg(), 1, "admin")
	payloadProduto := validProdutoPayload()
	payloadProduto["sku"] = 5
	payloadProduto["ativo"] = "x"
	casos := []struct {
		nome string
		path string
		body map[string]any
		mock func(sqlmock.Sqlmock)
	}{
		{"produto com sku numérico", "/api/produtos/1", payloadProduto, func(m sqlmock.Sqlmock) {
			m.ExpectExec(`UPDATE produtos`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery(`FROM produtos WHERE id = \?`).WillReturnRows(produtoRowsForHandler())
		}},
		{"estoque com sku e data_snapshot numéricos", "/api/estoque/1",
			map[string]any{"saldo": 4, "sku": 9, "data_snapshot": 123, "ruptura": "x"},
			func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`FROM estoque.+WHERE e\.id = \?`).WillReturnRows(estoqueRowsForHandler(1, 10, false))
				m.ExpectExec(`UPDATE estoque`).WithArgs(4, false, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery(`FROM estoque.+WHERE e\.id = \?`).WillReturnRows(estoqueRowsForHandler(1, 4, false))
			}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			c.mock(mock)

			req, _ := http.NewRequest(http.MethodPut, server.URL+c.path, makeJSON(c.body))
			req.Header.Set("Authorization", "Bearer "+admin)
			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestARQ02_IDNegativoOuZero_Vira400 (CHORE-03): o parse estrito recusa
// negativo e zero com 400 "id inválido" antes de qualquer SQL (era 404).
func TestARQ02_IDNegativoOuZero_Vira400(t *testing.T) {
	admin := generateToken(t, testCfg(), 1, "admin")
	for _, path := range []string{"/api/produtos/-1", "/api/produtos/0", "/api/estoque/-3"} {
		t.Run(path, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			status, corpo := arq02Get(t, server, path, admin)
			assert.Equal(t, http.StatusBadRequest, status)
			assert.Equal(t, "id inválido", decodeResponse(t, []byte(corpo))["error"])
			assert.NoError(t, mock.ExpectationsWereMet(), "nenhum SQL")
		})
	}
}

// TestARQ02_EscritaSemPrincipal_FalhaFechado: chamado fora do Autenticador
// (sem Principal no contexto), todo handler de escrita responde 500 "erro
// interno" sem executar SQL (§9, O5).
func TestARQ02_EscritaSemPrincipal_FalhaFechado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	prod, est := bootstrap.NovoCatalogoHTTP(db)
	casos := []struct {
		nome string
		h    http.HandlerFunc
	}{
		{"CreateProduto", prod.CreateProduto},
		{"UpdateProduto", prod.UpdateProduto},
		{"ToggleAtivoProduto", prod.ToggleAtivoProduto},
		{"CreateEstoque", est.CreateEstoque},
		{"UpdateEstoque", est.UpdateEstoque},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"saldo":1}`))
			req.SetPathValue("id", "1")
			w := httptest.NewRecorder()
			c.h(w, req)
			assert.Equal(t, http.StatusInternalServerError, w.Code)
			assert.JSONEq(t, `{"success":false,"error":"erro interno"}`, w.Body.String())
		})
	}
	assert.NoError(t, mock.ExpectationsWereMet(), "nenhuma consulta sem Principal")
}

func TestARQ02_HandlersExigemCasosDeUso(t *testing.T) {
	assert.Panics(t, func() { catalogohttp.NovoProdutoHandler(nil) })
	assert.Panics(t, func() { catalogohttp.NovoEstoqueHandler(nil) })
}
