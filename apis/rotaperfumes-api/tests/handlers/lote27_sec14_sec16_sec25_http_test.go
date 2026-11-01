package handlers_test

// Lote 27, pelo router real (bootstrap.HandlerHTTP):
//   - SEC-14: 413 de corpo acima de 256 KiB (Content-Length e chunked),
//     401 antes do 413 em rota admin e teto de 200 itens no POST/PUT de
//     pedido;
//   - SEC-16: page acima de 10.000 é limitado (OFFSET 999.900, sem 500);
//   - SEC-25: ?q= com mais de 100 caracteres → 400 sem SQL; '%' literal.

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lote27Msg413 = `{"success":false,"error":"corpo da requisição excede o limite de 256 KB"}`

// lote27SemTamanho esconde o tamanho do reader (corpo chunked).
type lote27SemTamanho struct{ io.Reader }

// lote27Serve chama o handler do servidor de teste direto, com recorder:
// sem TCP, o 413 que não lê o corpo não depende do comportamento do cliente.
func lote27Serve(server *httptest.Server, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(w, req)
	return w
}

func lote27Req(t *testing.T, metodo, alvo string, body io.Reader, tok string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(metodo, alvo, body)
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return req
}

// ---------------------------------------------------------------------------
// SEC-14 — corpo
// ---------------------------------------------------------------------------

func TestSEC14_HTTP_ToggleProduto_CorpoGrande_413(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	admin := generateToken(t, testCfg(), 1, "admin")
	corpo := `{"ativo":true}` + strings.Repeat(" ", 256<<10)

	t.Run("chunked", func(t *testing.T) {
		req := lote27Req(t, http.MethodPatch, "/api/produtos/1/inativar", lote27SemTamanho{strings.NewReader(corpo)}, admin)
		req.ContentLength = -1
		w := lote27Serve(server, req)
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		assert.JSONEq(t, lote27Msg413, w.Body.String())
	})
	t.Run("Content-Length", func(t *testing.T) {
		req := lote27Req(t, http.MethodPatch, "/api/produtos/1/inativar", strings.NewReader(corpo), admin)
		w := lote27Serve(server, req)
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		assert.JSONEq(t, lote27Msg413, w.Body.String())
	})
	assert.NoError(t, mock.ExpectationsWereMet(), "nenhum SQL")
}

func TestSEC14_HTTP_RotaAdminSemAutenticacao_CorpoGrande_401(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	grande := bytes.Repeat([]byte("a"), (256<<10)+1)

	w := lote27Serve(server, lote27Req(t, http.MethodPost, "/api/produtos", bytes.NewReader(grande), ""))
	assert.Equal(t, http.StatusUnauthorized, w.Code, "401 antes do 413")

	vendedor := generateToken(t, testCfg(), 2, "normal")
	w = lote27Serve(server, lote27Req(t, http.MethodPost, "/api/produtos", bytes.NewReader(grande), vendedor))
	assert.Equal(t, http.StatusForbidden, w.Code, "403 antes do 413")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func lote27PedidoComItens(n int) map[string]any {
	p := validPedidoPayload()
	itens := make([]map[string]any, n)
	for i := range itens {
		itens[i] = map[string]any{"produto_id": i + 1, "quantidade": 1, "preco_praticado": 1.0, "desconto_pct": 0.0}
	}
	p["itens"] = itens
	return p
}

func TestSEC14_HTTP_Pedido201Itens_400(t *testing.T) {
	for _, c := range []struct{ metodo, alvo string }{
		{http.MethodPost, "/api/pedidos"},
		{http.MethodPut, "/api/pedidos/1"},
	} {
		t.Run(c.metodo, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			admin := generateToken(t, testCfg(), 1, "admin")

			corpo := makeJSON(lote27PedidoComItens(201))
			require.Less(t, corpo.Len(), 256<<10, "o corpo cabe no teto: o 400 é do limite de itens")
			w := lote27Serve(server, lote27Req(t, c.metodo, c.alvo, corpo, admin))
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.JSONEq(t, `{"success":false,"error":"o pedido deve ter no máximo 200 itens"}`, w.Body.String())
			assert.NoError(t, mock.ExpectationsWereMet(), "nenhum SQL")
		})
	}
}

// 200 itens passam da validação: o pedido chega ao banco (aqui, o BEGIN
// falha de propósito e a resposta é 500, nunca o 400 do limite).
func TestSEC14_HTTP_Pedido200Itens_PassaDaValidacao(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	admin := generateToken(t, testCfg(), 1, "admin")
	mock.ExpectBegin().WillReturnError(assert.AnError)

	w := lote27Serve(server, lote27Req(t, http.MethodPost, "/api/pedidos", makeJSON(lote27PedidoComItens(200)), admin))
	assert.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "200 itens")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// SEC-16 — teto de page
// ---------------------------------------------------------------------------

func TestSEC16_HTTP_ListProdutos_PageMaxInt_ClampEm10000(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), 2, "normal")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(`SELECT `+produtoColunasRegex+` FROM produtos ORDER BY id ASC LIMIT \? OFFSET \?`).
		WithArgs(100, 999_900).
		WillReturnRows(emptyProdutoRowsForHandler())

	w := lote27Serve(server, lote27Req(t, http.MethodGet, "/api/produtos?page=9223372036854775807&limit=100", nil, tok))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeResponse(t, w.Body.Bytes())
	pag := body["pagination"].(map[string]any)
	assert.Equal(t, float64(10_000), pag["page"])
	assert.Equal(t, float64(100), pag["limit"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC16_HTTP_ListProdutos_PageForaDaFaixaDoInt_Pagina1(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), 2, "normal")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT `+produtoColunasRegex+` FROM produtos ORDER BY id ASC LIMIT \? OFFSET \?`).
		WithArgs(100, 0).
		WillReturnRows(emptyProdutoRowsForHandler())

	w := lote27Serve(server, lote27Req(t, http.MethodGet, "/api/produtos?page=99999999999999999999&limit=100", nil, tok))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	pag := decodeResponse(t, w.Body.Bytes())["pagination"].(map[string]any)
	assert.Equal(t, float64(1), pag["page"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// SEC-25 — ?q=
// ---------------------------------------------------------------------------

func TestSEC25_HTTP_BuscaMuitoLonga_400SemSQL(t *testing.T) {
	for _, rota := range []string{"/api/produtos", "/api/clientes", "/api/visitas", "/api/oportunidades", "/api/pedidos"} {
		for _, q := range []string{strings.Repeat("a", 101), strings.Repeat("ç", 101)} {
			t.Run(rota+"/"+string([]rune(q)[:1]), func(t *testing.T) {
				server, db, mock := setupTestServer(t)
				defer server.Close()
				defer db.Close()
				admin := generateToken(t, testCfg(), 1, "admin")

				w := lote27Serve(server, lote27Req(t, http.MethodGet, rota+"?q="+url.QueryEscape(q), nil, admin))
				assert.Equal(t, http.StatusBadRequest, w.Code)
				assert.JSONEq(t, `{"success":false,"error":"o parâmetro q deve ter no máximo 100 caracteres"}`, w.Body.String())
				assert.NoError(t, mock.ExpectationsWereMet(), "nenhum SQL")
			})
		}
	}
}

func TestSEC25_HTTP_Busca100Caracteres_OK(t *testing.T) {
	for _, c := range []struct{ nome, q string }{
		{"100 ASCII", strings.Repeat("a", 100)},
		{"100 ç", strings.Repeat("ç", 100)},
	} {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			tok := generateToken(t, testCfg(), 2, "normal")
			like := "%" + c.q + "%"

			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos WHERE \(descricao LIKE \? ESCAPE '!' OR sku LIKE \? ESCAPE '!'\)`).
				WithArgs(like, like).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectQuery(`SELECT `+produtoColunasRegex+` FROM produtos WHERE \(descricao LIKE \? ESCAPE '!' OR sku LIKE \? ESCAPE '!'\) ORDER BY id ASC LIMIT \? OFFSET \?`).
				WithArgs(like, like, 20, 0).
				WillReturnRows(emptyProdutoRowsForHandler())

			w := lote27Serve(server, lote27Req(t, http.MethodGet, "/api/produtos?q="+url.QueryEscape(c.q), nil, tok))
			assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSEC25_HTTP_CuringaLiteral(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	tok := generateToken(t, testCfg(), 2, "normal")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM produtos WHERE \(descricao LIKE \? ESCAPE '!' OR sku LIKE \? ESCAPE '!'\)`).
		WithArgs("%!%%", "%!%%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT `+produtoColunasRegex+` FROM produtos WHERE .+ LIMIT \? OFFSET \?`).
		WithArgs("%!%%", "%!%%", 20, 0).
		WillReturnRows(emptyProdutoRowsForHandler())

	w := lote27Serve(server, lote27Req(t, http.MethodGet, "/api/produtos?q=%25", nil, tok))
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.NoError(t, mock.ExpectationsWereMet())
}
