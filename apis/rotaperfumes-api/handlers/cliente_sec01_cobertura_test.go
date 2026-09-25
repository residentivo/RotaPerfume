package handlers_test

// Cobertura complementar do SEC-01 (escopo de carteira na escrita de
// clientes) com sqlmock ESTRITO: qualquer query/exec não declarada falha o
// teste, então "banco inalterado" = nenhum UPDATE/INSERT esperado.
// Não repete os cenários de cliente_escopo_escrita_test.go.

import (
	"bytes"
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	reSelectClienteH         = `SELECT ` + clienteColunasRegex + ` FROM clientes WHERE cliente_id_origem = \? LIMIT 1`
	reUpdateClienteH         = `UPDATE clientes\s+SET cnpj = \?, razao_social = \?, segmento = \?, cidade = \?, uf = \?, bairro = \?, data_cadastro = \?\s+WHERE cliente_id_origem = \?`
	msgClienteNaoEncontradoH = "cliente não encontrado"
)

// doRawReq envia um body cru (permite JSON inválido) autenticado como o
// usuário normal id 2.
func doRawReq(t *testing.T, serverURL, method, path, raw string) (int, map[string]any) {
	t.Helper()
	token := generateToken(t, testCfg(), 2, "normal")
	req, err := http.NewRequest(method, serverURL+path, bytes.NewBufferString(raw))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, decodeResponse(t, readBody(t, resp))
}

// Body inválido para cliente fora da carteira / sem vendedor deve responder
// 404 (autorização antes do decode), nunca 400 — o 400 vazaria que o
// cliente existe/que a rota chegou a processar o payload.
func TestSEC01_BodyInvalido_SemAcesso_404AntesDoDecode(t *testing.T) {
	escopos := []struct {
		nome   string
		expect func(m sqlmock.Sqlmock)
	}{
		{"fora da carteira", func(m sqlmock.Sqlmock) {
			expectEscopoUsuarioH(m, 2, 10)
			expectCarteiraInexistenteH(m, 10, 7)
		}},
		{"sem vendedor", func(m sqlmock.Sqlmock) { expectEscopoSemVendedorH(m, 2) }},
	}
	rotas := []struct{ method, path string }{
		{"PUT", "/api/clientes/7"},
		{"PATCH", "/api/clientes/7/inativar"},
	}
	bodies := map[string]string{
		"json quebrado": `{"razao_social":`,
		"tipo errado":   `{"ativo":"sim","cnpj":123}`,
		"vazio":         ``,
	}
	for _, e := range escopos {
		for _, r := range rotas {
			for nomeBody, raw := range bodies {
				t.Run(e.nome+"/"+r.method+"/"+nomeBody, func(t *testing.T) {
					server, db, mock := setupTestServer(t)
					defer server.Close()
					defer db.Close()
					e.expect(mock)

					status, body := doRawReq(t, server.URL, r.method, r.path, raw)
					assert.Equal(t, http.StatusNotFound, status)
					assert.Equal(t, msgClienteNaoEncontradoH, body["error"])
					assert.NoError(t, mock.ExpectationsWereMet())
				})
			}
		}
	}
}

// Na própria carteira, body inválido volta a ser 400 (a autorização passou).
func TestSEC01_Update_PropriaCarteira_BodyInvalido_400(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	expectEscopoUsuarioH(mock, 2, 10)
	expectCarteiraAtivaH(mock, 10, 7)

	status, body := doRawReq(t, server.URL, "PUT", "/api/clientes/7", `{"razao_social":`)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "body JSON inválido", body["error"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Campos extras vendedor_id/carteira_id no body são ignorados: só o UPDATE
// de clientes é executado (nenhum exec em carteiras — mock estrito).
func TestSEC01_Update_CamposExtrasDeCarteiraIgnorados(t *testing.T) {
	extras := []map[string]any{
		{"vendedor_id": 99},
		{"carteira_id": 5},
		{"vendedor_id": 99, "carteira_id": 5, "cliente_id_origem": 1234, "ativo": false},
	}
	for i, extra := range extras {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			expectEscopoUsuarioH(mock, 2, 10)
			expectCarteiraAtivaH(mock, 10, 1)
			mock.ExpectExec(reUpdateClienteH).
				WithArgs("12345678000199", "Empresa Teste LTDA", "varejo", "São Paulo", "SP", "Centro", sqlmock.AnyArg(), int64(1)).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(reSelectClienteH).WithArgs(int64(1)).WillReturnRows(clienteRowsForHandler())

			payload := validClientePayload()
			for k, v := range extra {
				payload[k] = v
			}
			// WithArgs(..., int64(1)) no UPDATE garante que o id do path
			// prevalece sobre qualquer id do body.
			status, body := doClienteReq(t, server.URL, "PUT", "/api/clientes/1", payload)
			assert.Equal(t, http.StatusOK, status)
			assert.True(t, body["success"].(bool))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Toggle na própria carteira: explícito e toggle puro → 200.
func TestSEC01_Toggle_PropriaCarteira_200(t *testing.T) {
	casos := []struct {
		nome      string
		payload   map[string]any
		novoAtivo bool
	}{
		{"explicito false", map[string]any{"ativo": false}, false},
		{"explicito true (mesmo valor)", map[string]any{"ativo": true}, true},
		{"sem body (toggle)", nil, false},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			expectEscopoUsuarioH(mock, 2, 10)
			expectCarteiraAtivaH(mock, 10, 1)
			mock.ExpectQuery(reSelectClienteH).WithArgs(int64(1)).WillReturnRows(clienteRowsForHandler())
			mock.ExpectExec(`UPDATE clientes SET ativo = \? WHERE cliente_id_origem = \?`).
				WithArgs(tc.novoAtivo, int64(1)).
				WillReturnResult(sqlmock.NewResult(0, 1))

			status, body := doClienteReq(t, server.URL, "PATCH", "/api/clientes/1/inativar", tc.payload)
			assert.Equal(t, http.StatusOK, status)
			assert.Equal(t, tc.novoAtivo, body["data"].(map[string]any)["ativo"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Autorização passou, mas o cliente sumiu entre a checagem de carteira e a
// escrita (corrida) → 404 com a mesma mensagem.
func TestSEC01_UpdateToggle_ClienteRemovidoAposChecagem_404(t *testing.T) {
	casos := []struct {
		nome, method, path string
		payload            map[string]any
		expect             func(m sqlmock.Sqlmock)
	}{
		{"update", "PUT", "/api/clientes/1", validClientePayload(), func(m sqlmock.Sqlmock) {
			m.ExpectExec(reUpdateClienteH).WillReturnResult(sqlmock.NewResult(0, 0))
		}},
		{"toggle", "PATCH", "/api/clientes/1/inativar", map[string]any{"ativo": false}, func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reSelectClienteH).WithArgs(int64(1)).WillReturnRows(emptyClienteRows())
		}},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			expectEscopoUsuarioH(mock, 2, 10)
			expectCarteiraAtivaH(mock, 10, 1)
			tc.expect(mock)

			status, body := doClienteReq(t, server.URL, tc.method, tc.path, tc.payload)
			assert.Equal(t, http.StatusNotFound, status)
			assert.Equal(t, msgClienteNaoEncontradoH, body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Vínculo órfão (usuarios.id_vendedor aponta para vendedor inexistente) é
// tratado como "sem vendedor": Create 403, Update/Toggle 404 — sem escrita.
func TestSEC01_VinculoOrfao(t *testing.T) {
	casos := []struct {
		nome, method, path string
		status             int
		msg                string
	}{
		{"create", "POST", "/api/clientes", http.StatusForbidden, msgSemVendedorH},
		{"update", "PUT", "/api/clientes/1", http.StatusNotFound, msgClienteNaoEncontradoH},
		{"toggle", "PATCH", "/api/clientes/1/inativar", http.StatusNotFound, msgClienteNaoEncontradoH},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			mock.ExpectQuery(escopoUsuarioRegexH).WithArgs(int64(2)).
				WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(10)))
			mock.ExpectQuery(reVendedorDesligado).WithArgs(int64(10)).
				WillReturnRows(sqlmock.NewRows([]string{"desligado"})) // vendedor inexistente

			status, body := doClienteReq(t, server.URL, tc.method, tc.path, validClientePayload())
			assert.Equal(t, tc.status, status)
			assert.Equal(t, tc.msg, body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Create por usuário normal: body inválido (após autorização) → 400 sem
// abrir transação.
func TestSEC01_Create_Normal_BodyInvalido_400SemTransacao(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	expectEscopoUsuarioH(mock, 2, 10)

	status, body := doRawReq(t, server.URL, "POST", "/api/clientes", `{"cnpj":`)
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "body JSON inválido", body["error"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Create por usuário sem vendedor: 403 mesmo com body inválido (autorização
// antes do decode), sem nenhum INSERT.
func TestSEC01_Create_SemVendedor_BodyInvalido_403(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	expectEscopoSemVendedorH(mock, 2)

	status, body := doRawReq(t, server.URL, "POST", "/api/clientes", `{"cnpj":`)
	assert.Equal(t, http.StatusForbidden, status)
	assert.Equal(t, msgSemVendedorH, body["error"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Falhas em cada etapa da transação de CreateClienteNaCarteira → 500 com
// rollback (nenhum commit), "erro interno".
func TestSEC01_Create_Normal_FalhasNaTransacao_500(t *testing.T) {
	errDB := errors.New("falha simulada")
	casos := []struct {
		nome   string
		expect func(m sqlmock.Sqlmock)
	}{
		{"begin", func(m sqlmock.Sqlmock) {
			m.ExpectBegin().WillReturnError(errDB)
		}},
		{"insert cliente", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(reInsertCliente).WillReturnError(errDB)
			m.ExpectRollback()
		}},
		{"commit", func(m sqlmock.Sqlmock) {
			m.ExpectBegin()
			m.ExpectExec(reInsertCliente).WillReturnResult(sqlmock.NewResult(101, 1))
			m.ExpectExec(reInsertCarteira).WithArgs(int64(101), int64(10), sqlmock.AnyArg(), nil).
				WillReturnResult(sqlmock.NewResult(900, 1))
			m.ExpectCommit().WillReturnError(errDB)
		}},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			expectEscopoUsuarioH(mock, 2, 10)
			tc.expect(mock)

			status, body := doClienteReq(t, server.URL, "POST", "/api/clientes", validClientePayload())
			assert.Equal(t, http.StatusInternalServerError, status)
			assert.Equal(t, "erro interno", body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Erro ao resolver escopo (falha no SELECT id_vendedor) → 500 em
// Create/Update/Toggle, sem escrita.
func TestSEC01_ErroAoResolverEscopo_500(t *testing.T) {
	for _, r := range []struct{ method, path string }{
		{"POST", "/api/clientes"},
		{"PUT", "/api/clientes/1"},
		{"PATCH", "/api/clientes/1/inativar"},
	} {
		t.Run(r.method, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			mock.ExpectQuery(escopoUsuarioRegexH).WithArgs(int64(2)).WillReturnError(errors.New("db down"))

			status, body := doClienteReq(t, server.URL, r.method, r.path, validClientePayload())
			assert.Equal(t, http.StatusInternalServerError, status)
			assert.Equal(t, "erro interno", body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
