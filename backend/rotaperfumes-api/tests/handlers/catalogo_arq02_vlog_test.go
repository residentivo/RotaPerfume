package handlers_test

// ARQ-02 / P1 (§6.2-§6.3): com o vlog ligado, uma listagem do Catálogo emite
// os 4 níveis (rota, caso de uso, decisão quando há, query) e nenhum valor
// digitado (busca, categoria, marca, SKU) aparece no log.

import (
	"bytes"
	"log"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/vlog"
)

type logSeguro struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logSeguro) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logSeguro) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func capturarVlogLigado(t *testing.T) *logSeguro {
	t.Helper()
	out := &logSeguro{}
	prevOut, prevVlog := log.Writer(), vlog.Enabled()
	log.SetOutput(out)
	vlog.SetEnabled(true)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		vlog.SetEnabled(prevVlog)
	})
	return out
}

func TestARQ02_Vlog_QuatroNiveisSemValores(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	admin := generateToken(t, testCfg(), 1, "admin")
	out := capturarVlogLigado(t)

	mock.ExpectQuery(`SELECT COUNT`).WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(1))
	mock.ExpectQuery(`FROM produtos`).WillReturnRows(produtoRowsForHandler())
	status, _ := arq02Get(t, server, "/api/produtos?q=BuscaSecreta&categoria=CatSecreta&marca=MarcaSecreta&order_by=sku", admin)
	require.Equal(t, http.StatusOK, status)

	// Estoque com SKU inexistente: nível 3 (decisão) e 400.
	mock.ExpectQuery(`SELECT 1 FROM produtos WHERE sku = \?`).WillReturnRows(sqlmock.NewRows([]string{"1"}))
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/estoque",
		makeJSON(map[string]any{"sku": "SKU-SECRETO", "data_snapshot": "2024-06-01", "saldo": 3}))
	req.Header.Set("Authorization", "Bearer "+admin)
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.NoError(t, mock.ExpectationsWereMet())

	saida := out.String()
	for _, nivel := range []string{
		"[produto_handler.go] [ProdutoHandler.ListProdutos] início GET",             // 1. rota
		"[produtos.go] [Catalogo.ListarProdutos] início page=1 limit=20 busca=true", // 2. caso de uso
		"[produtos.go] [Catalogo.ListarProdutos] fim ok",
		"[produtos.go] [Produtos.Listar] SELECT produtos linhas=1 total=1",           // 4. query
		"[estoque.go] [Catalogo.AjustarEstoque] decisão sku inexistente em produtos", // 3. decisão
		"[estoque.go] [Catalogo.AjustarEstoque] fim erro=invalido",                   // erro só pelo Kind
		"[produtos.go] [Produtos.ExisteSKU] SELECT produtos existe=false",
	} {
		assert.Contains(t, saida, nivel)
	}
	for _, valor := range []string{"BuscaSecreta", "CatSecreta", "MarcaSecreta", "SKU-SECRETO", "SELECT COUNT", "LIKE"} {
		assert.NotContains(t, saida, valor, "o vlog não leva valores digitados nem texto de SQL")
	}
	assert.False(t, strings.Contains(saida, "sku não corresponde"), "o vlog não repete a mensagem do erro")
}
