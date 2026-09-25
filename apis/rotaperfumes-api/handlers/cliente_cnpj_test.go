package handlers_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NEG-01: contrato HTTP de CNPJ em POST/PUT /api/clientes.
//   - vazio → 400 "cnpj é obrigatório";
//   - ≠ 14 dígitos, todos iguais ou DV inválido → 400 "cnpj inválido";
//   - duplicado (MySQL 1062 em uq_clientes_cnpj) → 409 "cnpj já cadastrado",
//     sem revelar id, vendedor ou razão social do cliente existente.

const reInsertClienteH = `INSERT INTO clientes \(cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo\)`

func erroDuplicadoCNPJH() error {
	return &mysql.MySQLError{Number: 1062, Message: "Duplicate entry '11222333000181' for key 'clientes.uq_clientes_cnpj'"}
}

func doAdminClienteReq(t *testing.T, url, method string, payload map[string]any) (int, map[string]any, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, makeJSON(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw := readBody(t, resp)
	return resp.StatusCode, decodeResponse(t, raw), string(raw)
}

func TestCreateCliente_CNPJ(t *testing.T) {
	cases := []struct {
		name       string
		cnpj       string
		setup      func(mock sqlmock.Sqlmock)
		wantStatus int
		wantErr    string
	}{
		{name: "vazio", cnpj: "", wantStatus: http.StatusBadRequest, wantErr: "cnpj é obrigatório"},
		{name: "13 dígitos", cnpj: "1122233300018", wantStatus: http.StatusBadRequest, wantErr: "cnpj inválido"},
		{name: "letras", cnpj: "ABCDEFGHIJKLMN", wantStatus: http.StatusBadRequest, wantErr: "cnpj inválido"},
		{name: "todos iguais", cnpj: "00.000.000/0000-00", wantStatus: http.StatusBadRequest, wantErr: "cnpj inválido"},
		{name: "DV inválido", cnpj: "11.222.333/0001-82", wantStatus: http.StatusBadRequest, wantErr: "cnpj inválido"},
		{
			name: "máscara aceita e gravada só com dígitos",
			cnpj: "11.222.333/0001-81",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(reInsertClienteH).
					WithArgs("11222333000181", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), true).
					WillReturnResult(sqlmock.NewResult(101, 1))
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "duplicado retorna 409 genérico",
			cnpj: "11222333000181",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(reInsertClienteH).WillReturnError(erroDuplicadoCNPJH())
			},
			wantStatus: http.StatusConflict,
			wantErr:    "cnpj já cadastrado",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			if tc.setup != nil {
				tc.setup(mock)
			}
			payload := validClientePayload()
			payload["cnpj"] = tc.cnpj

			status, body, raw := doAdminClienteReq(t, server.URL+"/api/clientes", "POST", payload)

			assert.Equal(t, tc.wantStatus, status)
			if tc.wantErr != "" {
				assert.Equal(t, false, body["success"])
				assert.Nil(t, body["data"])
				assert.Equal(t, tc.wantErr, body["error"])
				assert.False(t, strings.Contains(raw, "uq_clientes_cnpj"), "não pode vazar detalhes do banco")
			} else {
				assert.Equal(t, "11222333000181", body["data"].(map[string]any)["cnpj"])
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestCreateCliente_CNPJDuplicado_UsuarioNormal: no fluxo com carteira (tx),
// o 409 também é genérico e a transação é desfeita.
func TestCreateCliente_CNPJDuplicado_UsuarioNormal(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	expectEscopoUsuarioH(mock, 2, 10)
	mock.ExpectBegin()
	mock.ExpectExec(reInsertClienteH).WillReturnError(erroDuplicadoCNPJH())
	mock.ExpectRollback()

	status, body := doClienteReq(t, server.URL, "POST", "/api/clientes", validClientePayload())

	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, "cnpj já cadastrado", body["error"])
	assert.Nil(t, body["data"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// cnpjLegadoDVInvalidoH: 14 dígitos, não repetidos, mas com DV inválido —
// formato de cliente legado/importado antes do NEG-01.
const cnpjLegadoDVInvalidoH = "29401965569816"

// TestUpdateCliente_CNPJ: NEG-04 — o PUT exige DV válido em toda gravação,
// inclusive com o mesmo CNPJ já gravado. A recusa (400) acontece antes de
// qualquer query; o 404 vem do UPDATE com 0 linhas.
func TestUpdateCliente_CNPJ(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		cnpj       string
		setup      func(mock sqlmock.Sqlmock)
		wantStatus int
		wantErr    string
	}{
		{name: "CNPJ com DV inválido (sem query)", cnpj: "11222333000182", wantStatus: http.StatusBadRequest, wantErr: "cnpj inválido"},
		{name: "mesmo CNPJ legado com DV inválido, sem máscara (sem query)", cnpj: cnpjLegadoDVInvalidoH, wantStatus: http.StatusBadRequest, wantErr: "cnpj inválido"},
		{name: "mesmo CNPJ legado com DV inválido, com máscara (sem query)", cnpj: "29.401.965/5698-16", wantStatus: http.StatusBadRequest, wantErr: "cnpj inválido"},
		{name: "id inexistente com DV inválido dá 400 (precedência)", path: "/api/clientes/999999", cnpj: cnpjLegadoDVInvalidoH, wantStatus: http.StatusBadRequest, wantErr: "cnpj inválido"},
		{name: "formato inválido", cnpj: "123", wantStatus: http.StatusBadRequest, wantErr: "cnpj inválido"},
		{
			name: "duplicado retorna 409 genérico",
			cnpj: "11.444.777/0001-61",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(reUpdateClienteH).
					WithArgs("11444777000161", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int64(1)).
					WillReturnError(erroDuplicadoCNPJH())
			},
			wantStatus: http.StatusConflict,
			wantErr:    "cnpj já cadastrado",
		},
		{
			name: "cliente inexistente (UPDATE com 0 linhas)",
			cnpj: "11222333000181",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(reUpdateClienteH).
					WithArgs("11222333000181", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantStatus: http.StatusNotFound,
			wantErr:    "cliente não encontrado",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			if tc.setup != nil {
				tc.setup(mock)
			}
			payload := validClientePayload()
			payload["cnpj"] = tc.cnpj
			path := tc.path
			if path == "" {
				path = "/api/clientes/1"
			}

			status, body, raw := doAdminClienteReq(t, server.URL+path, "PUT", payload)

			assert.Equal(t, tc.wantStatus, status)
			assert.Equal(t, false, body["success"])
			assert.Nil(t, body["data"])
			assert.Equal(t, tc.wantErr, body["error"])
			assert.False(t, strings.Contains(raw, "Empresa Teste"), "não pode revelar dados do cliente existente")
			// Sem setup, qualquer query inesperada faria o sqlmock devolver
			// erro (500), não o 400 afirmado acima.
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestToggleAtivoCliente_CNPJLegadoDVInvalido: NEG-04 — o PATCH /inativar não
// grava CNPJ, então cliente legado com DV inválido continua podendo ser
// inativado/reativado (200).
func TestToggleAtivoCliente_CNPJLegadoDVInvalido(t *testing.T) {
	cases := []struct {
		name      string
		payload   map[string]any
		novoAtivo bool
	}{
		{name: "toggle sem body", payload: nil, novoAtivo: false},
		{name: "explícito false", payload: map[string]any{"ativo": false}, novoAtivo: false},
		{name: "explícito true", payload: map[string]any{"ativo": true}, novoAtivo: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			now := time.Now()
			rows := sqlmock.NewRows([]string{
				"cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro",
				"data_cadastro", "ativo", "created_at", "updated_at",
			}).AddRow(int64(1), cnpjLegadoDVInvalidoH, "Legado LTDA", "varejo", "São Paulo", "SP", "Centro", now, true, now, now)
			mock.ExpectQuery(reSelectClienteH).WithArgs(int64(1)).WillReturnRows(rows)
			mock.ExpectExec(`UPDATE clientes SET ativo = \? WHERE cliente_id_origem = \?`).
				WithArgs(tc.novoAtivo, int64(1)).
				WillReturnResult(sqlmock.NewResult(0, 1))

			status, body, _ := doAdminClienteReq(t, server.URL+"/api/clientes/1/inativar", "PATCH", tc.payload)

			require.Equal(t, http.StatusOK, status)
			data := body["data"].(map[string]any)
			assert.Equal(t, tc.novoAtivo, data["ativo"])
			assert.Equal(t, cnpjLegadoDVInvalidoH, data["cnpj"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
