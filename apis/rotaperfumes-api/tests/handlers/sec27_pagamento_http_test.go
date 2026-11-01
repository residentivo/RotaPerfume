package handlers_test

// SEC-27 (Lote 25) pelo HTTP (router real + sqlmock):
//   - taxa_pct fora de 0..30 → 400 "taxa_pct deve estar entre 0 e 30" no
//     POST e no PUT, para admin e normal, sem INSERT/UPDATE; 30 é aceito.
//   - PUT de pagamento quitado: normal → UPDATE condicional com 0 linhas,
//     releitura quitado → 409 "pagamento já quitado não pode ser editado";
//     admin → UPDATE normal, 200.
//   - "Em aberto" → "Pago" pelo normal: 200.
//   - 0 linhas e releitura inexistente → 404; corrida (releitura editável)
//     → 409.

import (
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

const (
	msgSEC27Taxa    = "taxa_pct deve estar entre 0 e 30"
	msgSEC27Quitado = "pagamento já quitado não pode ser editado"
	sec27UpdCondRe  = updatePagamentoRegH + `\s+AND status_pagamento NOT IN \(\?, \?\)`
)

func sec27PayloadH(status string, taxa float64) map[string]any {
	p := validPagamentoPayload()
	delete(p, "pedido_id")
	p["status_pagamento"] = status
	p["taxa_pct"] = taxa
	return p
}

// sec27PosseNormalH: usuário 2 → vendedor 2, dono do pedido 1 do pagamento 1.
func sec27PosseNormalH(mock sqlmock.Sqlmock, statusAtual string) {
	expectEscopoUsuarioH(mock, 2, 2)
	mock.ExpectQuery(pagamentoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pagamentoRowsComStatusForHandler(1, 1, 100.0, statusAtual))
	mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pedidoRowsVendedorH(1, 2))
}

func TestSEC27_HTTP_TaxaForaDoLimite_400SemGravar(t *testing.T) {
	for _, taxa := range []float64{30.01, 31, 100, 999.99, -0.01} {
		for _, papel := range []string{"admin", "normal"} {
			t.Run(papel+"/PUT", func(t *testing.T) {
				server, db, mock := setupTestServer(t)
				defer server.Close()
				defer db.Close()
				uid := int64(1)
				if papel == "normal" {
					uid = 2
					sec27PosseNormalH(mock, "Em aberto")
				}
				status, body := doReq(t, http.MethodPut, server.URL+"/api/pagamentos/1", generateToken(t, testCfg(), uid, papel), sec27PayloadH("Em aberto", taxa))
				assert.Equal(t, http.StatusBadRequest, status, "taxa %v", taxa)
				assert.Equal(t, msgSEC27Taxa, body["error"])
				assert.NoError(t, mock.ExpectationsWereMet(), "sem UPDATE")
			})
			t.Run(papel+"/POST", func(t *testing.T) {
				server, db, mock := setupTestServer(t)
				defer server.Close()
				defer db.Close()
				uid := int64(1)
				if papel == "normal" {
					uid = 2
					expectEscopoUsuarioH(mock, 2, 2)
				}
				expectLockPedidoH(mock, 1, 2) // SEC-35: o pedido é travado antes da validação dos campos
				mock.ExpectRollback()
				p := validPagamentoPayload()
				p["taxa_pct"] = taxa
				status, body := doReq(t, http.MethodPost, server.URL+"/api/pagamentos", generateToken(t, testCfg(), uid, papel), p)
				assert.Equal(t, http.StatusBadRequest, status, "taxa %v", taxa)
				assert.Equal(t, msgSEC27Taxa, body["error"])
				assert.NoError(t, mock.ExpectationsWereMet(), "sem INSERT")
			})
		}
	}
}

// Taxa no limite (30) é aceita, inclusive pelo normal.
func TestSEC27_HTTP_Taxa30_Aceita(t *testing.T) {
	for _, papel := range []string{"admin", "normal"} {
		t.Run(papel, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			uid := int64(1)
			if papel == "normal" {
				uid = 2
				sec27PosseNormalH(mock, "Em aberto")
				mock.ExpectExec(sec27UpdCondRe).
					WithArgs("PIX", uint8(1), 100.0, 30.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto", int64(1), "Pago", "Pago com atraso").
					WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				mock.ExpectExec(updatePagamentoRegH+`\s*$`).
					WithArgs("PIX", uint8(1), 100.0, 30.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto", int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectQuery(pagamentoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pagamentoRowsForHandler(1, 1, 100.0))

			status, body := doReq(t, http.MethodPut, server.URL+"/api/pagamentos/1", generateToken(t, testCfg(), uid, papel), sec27PayloadH("Em aberto", 30))
			assert.Equal(t, http.StatusOK, status, "body: %v", body)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// PUT de quitado pelo normal: 409 (UPDATE condicional com 0 linhas e
// releitura quitado), para Pago e Pago com atraso, qualquer destino.
func TestSEC27_HTTP_PUT_QuitadoNormal_409(t *testing.T) {
	for _, atual := range []string{"Pago", "Pago com atraso"} {
		for _, destino := range []string{"Em aberto", "Pago", "Inadimplente"} {
			t.Run(atual+"→"+destino, func(t *testing.T) {
				server, db, mock := setupTestServer(t)
				defer server.Close()
				defer db.Close()
				sec27PosseNormalH(mock, atual)
				mock.ExpectExec(sec27UpdCondRe).
					WithArgs("PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), nil, destino, int64(1), "Pago", "Pago com atraso").
					WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectQuery(pagamentoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pagamentoRowsComStatusForHandler(1, 1, 100.0, atual))

				status, body := doReq(t, http.MethodPut, server.URL+"/api/pagamentos/1", generateToken(t, testCfg(), 2, "normal"), sec27PayloadH(destino, 0))
				assert.Equal(t, http.StatusConflict, status)
				assert.Equal(t, msgSEC27Quitado, body["error"])
				assert.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

// PUT de quitado pelo admin: UPDATE sem condição, 200.
func TestSEC27_HTTP_PUT_QuitadoAdmin_200(t *testing.T) {
	for _, atual := range []string{"Pago", "Pago com atraso"} {
		t.Run(atual, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			mock.ExpectExec(updatePagamentoRegH+`\s*$`).
				WithArgs("PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), nil, "Em aberto", int64(1)).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(pagamentoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pagamentoRowsForHandler(1, 1, 100.0))

			status, body := doReq(t, http.MethodPut, server.URL+"/api/pagamentos/1", generateToken(t, testCfg(), 1, "admin"), sec27PayloadH("Em aberto", 0))
			assert.Equal(t, http.StatusOK, status)
			assert.Equal(t, true, body["success"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// "Em aberto" → "Pago" pelo normal: o UPDATE condicional casa, 200.
func TestSEC27_HTTP_PUT_EmAbertoParaPago_Normal_200(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	sec27PosseNormalH(mock, "Em aberto")
	mock.ExpectExec(sec27UpdCondRe).
		WithArgs("PIX", uint8(1), 100.0, 0.0, 100.0, sqlmock.AnyArg(), sqlmock.AnyArg(), "Pago", int64(1), "Pago", "Pago com atraso").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(pagamentoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pagamentoRowsComStatusForHandler(1, 1, 100.0, "Pago"))

	p := sec27PayloadH("Pago", 0)
	p["data_pagamento"] = "2024-01-16"
	status, body := doReq(t, http.MethodPut, server.URL+"/api/pagamentos/1", generateToken(t, testCfg(), 2, "normal"), p)
	assert.Equal(t, http.StatusOK, status)
	data, _ := body["data"].(map[string]any)
	assert.Equal(t, "Pago", data["status_pagamento"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 0 linhas no condicional: a releitura separa o 404 do 409 (corrida).
func TestSEC27_HTTP_PUT_ZeroLinhas_Releitura(t *testing.T) {
	casos := []struct {
		nome   string
		relida *sqlmock.Rows
		want   int
		msg    string
	}{
		{"pagamento excluído entre a posse e o UPDATE → 404", emptyPagamentoRowsForHandler(), http.StatusNotFound, "pagamento não encontrado"},
		{"corrida: releitura editável → 409", pagamentoRowsComStatusForHandler(1, 1, 100.0, "Em aberto"), http.StatusConflict, msgSEC27Quitado},
		{"quitado por outra requisição → 409", pagamentoRowsComStatusForHandler(1, 1, 100.0, "Pago"), http.StatusConflict, msgSEC27Quitado},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			sec27PosseNormalH(mock, "Em aberto")
			mock.ExpectExec(sec27UpdCondRe).WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectQuery(pagamentoByIDRegexH).WithArgs(int64(1)).WillReturnRows(c.relida)

			status, body := doReq(t, http.MethodPut, server.URL+"/api/pagamentos/1", generateToken(t, testCfg(), 2, "normal"), sec27PayloadH("Em aberto", 0))
			assert.Equal(t, c.want, status)
			assert.Equal(t, c.msg, body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Ordem: posse (404) → decode → validação (400) → UPDATE (409). Um quitado
// fora da carteira dá 404; um quitado com payload inválido dá 400.
func TestSEC27_HTTP_PUT_OrdemDasChecagens(t *testing.T) {
	t.Run("quitado fora da carteira → 404 sem UPDATE", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		expectEscopoUsuarioH(mock, 2, 99)
		mock.ExpectQuery(pagamentoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pagamentoRowsComStatusForHandler(1, 1, 100.0, "Pago"))
		mock.ExpectQuery(pedidoByIDRegexH).WithArgs(int64(1)).WillReturnRows(pedidoRowsVendedorH(1, 2))
		status, body := doReq(t, http.MethodPut, server.URL+"/api/pagamentos/1", generateToken(t, testCfg(), 2, "normal"), sec27PayloadH("Em aberto", 0))
		assert.Equal(t, http.StatusNotFound, status)
		assert.Equal(t, "pagamento não encontrado", body["error"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("quitado com forma inválida → 400 sem UPDATE", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		sec27PosseNormalH(mock, "Pago")
		p := sec27PayloadH("Pago", 0)
		p["forma_pagamento"] = "Bitcoin"
		status, body := doReq(t, http.MethodPut, server.URL+"/api/pagamentos/1", generateToken(t, testCfg(), 2, "normal"), p)
		assert.Equal(t, http.StatusBadRequest, status)
		assert.Equal(t, "forma_pagamento inválida", body["error"])
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
