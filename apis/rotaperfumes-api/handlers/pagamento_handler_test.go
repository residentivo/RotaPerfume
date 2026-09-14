package handlers_test

import (
	"bytes"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regexes/colunas que espelham as constantes de
// apis/shared/repositories/pagamento_repository.go.
const pagamentoColunasRegexH = `pagamento_id, pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento, created_at, updated_at`
const pagamentoFromRegexH = ` FROM pagamentos`

func pagamentoColunasHeaderForHandler() []string {
	return []string{
		"pagamento_id", "pedido_id", "forma_pagamento", "parcelas", "valor", "taxa_pct",
		"valor_liquido", "data_vencimento", "data_pagamento", "status_pagamento", "created_at", "updated_at",
	}
}

func pagamentoRowsForHandler(id, pedidoID int64, valor float64) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(pagamentoColunasHeaderForHandler()).
		AddRow(id, pedidoID, "PIX", uint8(1), valor, 0.0, valor, now, nil, "Em aberto", now, now)
}

func emptyPagamentoRowsForHandler() *sqlmock.Rows {
	return sqlmock.NewRows(pagamentoColunasHeaderForHandler())
}

func validPagamentoPayload() map[string]any {
	return map[string]any{
		"pedido_id":        1,
		"forma_pagamento":  "PIX",
		"parcelas":         1,
		"valor":            100.0,
		"taxa_pct":         0.0,
		"valor_liquido":    100.0,
		"data_vencimento":  "2024-01-15",
		"status_pagamento": "Em aberto",
	}
}

// ---------------------------------------------------------------------------
// ListPagamentos GET /api/pagamentos
// ---------------------------------------------------------------------------

func TestListPagamentos_Success_UsuarioComum(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT COUNT\(\*\)` + pagamentoFromRegexH).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT ` + pagamentoColunasRegexH + pagamentoFromRegexH + ` ORDER BY pagamento_id ASC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(pagamentoRowsForHandler(1, 1, 100.0))

	req, _ := http.NewRequest("GET", server.URL+"/api/pagamentos", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].([]any)
	assert.Len(t, data, 1)
	pagination := body["pagination"].(map[string]any)
	assert.Equal(t, float64(1), pagination["total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListPagamentos_ComFiltrosEPaginacao(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	whereRegex := ` WHERE status_pagamento = \? AND forma_pagamento = \? AND pedido_id = \? AND data_vencimento >= \? AND data_vencimento <= \?`
	mock.ExpectQuery(`SELECT COUNT\(\*\)` + pagamentoFromRegexH + whereRegex).
		WithArgs("Pago", "PIX", int64(5), "2024-01-01", "2024-01-31").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(25))
	mock.ExpectQuery(`SELECT ` + pagamentoColunasRegexH + pagamentoFromRegexH + whereRegex + ` ORDER BY pagamento_id ASC LIMIT \? OFFSET \?`).
		WithArgs("Pago", "PIX", int64(5), "2024-01-01", "2024-01-31", 10, 10).
		WillReturnRows(pagamentoRowsForHandler(1, 5, 200.0))

	req, _ := http.NewRequest("GET", server.URL+"/api/pagamentos?page=2&limit=10&status_pagamento=Pago&forma_pagamento=PIX&pedido_id=5&vencimento_de=2024-01-01&vencimento_ate=2024-01-31", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	pagination := body["pagination"].(map[string]any)
	assert.Equal(t, float64(2), pagination["page"])
	assert.Equal(t, float64(10), pagination["limit"])
	assert.Equal(t, float64(25), pagination["total"])
	assert.Equal(t, float64(3), pagination["pages"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListPagamentos_PedidoIDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/pagamentos?pedido_id=abc", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "pedido_id inválido", body["error"])
}

func TestListPagamentos_NaoAutenticado(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	req, _ := http.NewRequest("GET", server.URL+"/api/pagamentos", nil)
	// Sem Authorization header.

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestListPagamentos_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT COUNT\(\*\)` + pagamentoFromRegexH).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/pagamentos", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetPagamento GET /api/pagamentos/{id}
// ---------------------------------------------------------------------------

func TestGetPagamento_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT ` + pagamentoColunasRegexH + pagamentoFromRegexH + ` WHERE pagamento_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pagamentoRowsForHandler(1, 1, 100.0))

	req, _ := http.NewRequest("GET", server.URL+"/api/pagamentos/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(1), data["pagamento_id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPagamento_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT ` + pagamentoColunasRegexH + pagamentoFromRegexH + ` WHERE pagamento_id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnRows(emptyPagamentoRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/pagamentos/999", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "pagamento não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetPagamento_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("GET", server.URL+"/api/pagamentos/abc", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

// ---------------------------------------------------------------------------
// CreatePagamento POST /api/pagamentos
// ---------------------------------------------------------------------------

func TestCreatePagamento_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT 1 FROM pedidos WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO pagamentos \(pedido_id, forma_pagamento, parcelas, valor, taxa_pct, valor_liquido, data_vencimento, data_pagamento, status_pagamento\)`).
		WithArgs(int64(1), "PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto").
		WillReturnResult(sqlmock.NewResult(1, 1))

	req, _ := http.NewRequest("POST", server.URL+"/api/pagamentos", makeJSON(validPagamentoPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(1), data["pagamento_id"])
	assert.Equal(t, float64(1), data["pedido_id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreatePagamento_NaoAutenticado(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	req, _ := http.NewRequest("POST", server.URL+"/api/pagamentos", makeJSON(validPagamentoPayload()))

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCreatePagamento_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("POST", server.URL+"/api/pagamentos", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestCreatePagamento_PedidoNaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT 1 FROM pedidos WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("POST", server.URL+"/api/pagamentos", makeJSON(validPagamentoPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "pedido não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreatePagamento_ValidacaoNegocio(t *testing.T) {
	testCases := []struct {
		nome    string
		payload func() map[string]any
		wantMsg string
	}{
		{
			nome: "pedido_id ausente",
			payload: func() map[string]any {
				p := validPagamentoPayload()
				p["pedido_id"] = 0
				return p
			},
			wantMsg: "pedido_id é obrigatório",
		},
		{
			nome: "forma_pagamento inválida",
			payload: func() map[string]any {
				p := validPagamentoPayload()
				p["forma_pagamento"] = "Bitcoin"
				return p
			},
			wantMsg: "forma_pagamento inválida",
		},
		{
			nome: "status_pagamento inválido",
			payload: func() map[string]any {
				p := validPagamentoPayload()
				p["status_pagamento"] = "Cancelado"
				return p
			},
			wantMsg: "status_pagamento inválido",
		},
		{
			nome: "parcelas menor que 1",
			payload: func() map[string]any {
				p := validPagamentoPayload()
				p["parcelas"] = 0
				return p
			},
			wantMsg: "parcelas deve ser maior ou igual a 1",
		},
		{
			nome: "valor negativo",
			payload: func() map[string]any {
				p := validPagamentoPayload()
				p["valor"] = -1.0
				return p
			},
			wantMsg: "valor deve ser maior ou igual a zero",
		},
		{
			nome: "taxa_pct negativa",
			payload: func() map[string]any {
				p := validPagamentoPayload()
				p["taxa_pct"] = -1.0
				return p
			},
			wantMsg: "taxa_pct deve ser maior ou igual a zero",
		},
		{
			nome: "valor_liquido negativo",
			payload: func() map[string]any {
				p := validPagamentoPayload()
				p["valor_liquido"] = -1.0
				return p
			},
			wantMsg: "valor_liquido deve ser maior ou igual a zero",
		},
		{
			nome: "data_vencimento ausente",
			payload: func() map[string]any {
				p := validPagamentoPayload()
				p["data_vencimento"] = ""
				return p
			},
			wantMsg: "data_vencimento é obrigatória (use o formato AAAA-MM-DD)",
		},
		{
			nome: "data_vencimento em formato inválido",
			payload: func() map[string]any {
				p := validPagamentoPayload()
				p["data_vencimento"] = "15/01/2024"
				return p
			},
			wantMsg: "data_vencimento inválida (use o formato AAAA-MM-DD)",
		},
		{
			nome: "data_pagamento em formato inválido",
			payload: func() map[string]any {
				p := validPagamentoPayload()
				p["data_pagamento"] = "15/01/2024"
				return p
			},
			wantMsg: "data_pagamento inválida (use o formato AAAA-MM-DD)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			userToken := generateToken(t, cfg, 2, "normal")

			payload := tc.payload()
			// pedido_id=0 falha antes de tocar o repo; caso contrário, o
			// serviço checa a existência do pedido primeiro.
			if pid, ok := payload["pedido_id"].(int); !ok || pid != 0 {
				mock.ExpectQuery(`SELECT 1 FROM pedidos WHERE id = \? LIMIT 1`).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			}

			req, _ := http.NewRequest("POST", server.URL+"/api/pagamentos", makeJSON(payload))
			req.Header.Set("Authorization", "Bearer "+userToken)

			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, tc.wantMsg, body["error"])
		})
	}
}

// ---------------------------------------------------------------------------
// UpdatePagamento PUT /api/pagamentos/{id}
// ---------------------------------------------------------------------------

func TestUpdatePagamento_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectExec(`UPDATE pagamentos\s+SET forma_pagamento = \?, parcelas = \?, valor = \?, taxa_pct = \?, valor_liquido = \?, data_vencimento = \?, data_pagamento = \?, status_pagamento = \?\s+WHERE pagamento_id = \?`).
		WithArgs("PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + pagamentoColunasRegexH + pagamentoFromRegexH + ` WHERE pagamento_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pagamentoRowsForHandler(1, 1, 100.0))

	payload := validPagamentoPayload()
	delete(payload, "pedido_id") // não editável pelo UPDATE

	req, _ := http.NewRequest("PUT", server.URL+"/api/pagamentos/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(1), data["pagamento_id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePagamento_IgnoraPedidoIDNoPayload(t *testing.T) {
	// pedido_id enviado no corpo do PUT é ignorado — UpdatePagamentoRequest
	// não possui o campo pedido_id.
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectExec(`UPDATE pagamentos`).
		WithArgs("PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + pagamentoColunasRegexH + pagamentoFromRegexH + ` WHERE pagamento_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(pagamentoRowsForHandler(1, 1, 100.0))

	payload := validPagamentoPayload()
	payload["pedido_id"] = 999 // será ignorado pelo handler/DTO

	req, _ := http.NewRequest("PUT", server.URL+"/api/pagamentos/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(1), data["pedido_id"]) // permanece o do banco

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePagamento_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("PUT", server.URL+"/api/pagamentos/abc", makeJSON(validPagamentoPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestUpdatePagamento_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	req, _ := http.NewRequest("PUT", server.URL+"/api/pagamentos/1", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestUpdatePagamento_ValidacaoNegocio(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	payload := validPagamentoPayload()
	payload["forma_pagamento"] = "Bitcoin"

	req, _ := http.NewRequest("PUT", server.URL+"/api/pagamentos/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "forma_pagamento inválida", body["error"])
}

func TestUpdatePagamento_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectExec(`UPDATE pagamentos`).
		WithArgs("PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto", int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req, _ := http.NewRequest("PUT", server.URL+"/api/pagamentos/999", makeJSON(validPagamentoPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "pagamento não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdatePagamento_NaoAutenticado(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	req, _ := http.NewRequest("PUT", server.URL+"/api/pagamentos/1", makeJSON(validPagamentoPayload()))

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
