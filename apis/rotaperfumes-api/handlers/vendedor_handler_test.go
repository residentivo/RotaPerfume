// Package handlers_test contém testes de integração dos handlers HTTP.
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
// apis/shared/repositories/vendedor_repository.go e carteira_repository.go.
const vendedorGetColunasRegexH = `id, nome, regiao, uf, data_admissao, data_desligamento, meta_mensal, created_at, updated_at FROM vendedores WHERE id = \? LIMIT 1`
const clienteResumoColunasRegexH = `c\.cliente_id_origem, c\.cnpj, c\.razao_social, c\.segmento, c\.cidade, c\.uf, ca\.carteira_id_origem, ca\.data_inicio, ca\.data_fim`
const clienteResumoFromRegexH = ` FROM carteiras ca JOIN clientes c ON c\.cliente_id_origem = ca\.cliente_id`

func vendedorGetRowsH(id int64, nome string, dataDesligamento any) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_admissao", "data_desligamento", "meta_mensal", "created_at", "updated_at"}).
		AddRow(id, nome, "Sudeste", "SP", now, dataDesligamento, 5000.0, now, now)
}

func clienteResumoRowsH() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{"id", "cnpj", "razao_social", "segmento", "cidade", "uf", "carteira_id", "data_inicio", "data_fim"}).
		AddRow(int64(1), "11.111.111/0001-11", "Cliente A", "Varejo", "São Paulo", "SP", int64(10), now, nil).
		AddRow(int64(2), "22.222.222/0001-22", "Cliente B", "Atacado", "Campinas", "SP", int64(11), now, nil)
}

func emptyClienteResumoRowsH() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "cnpj", "razao_social", "segmento", "cidade", "uf", "carteira_id", "data_inicio", "data_fim"})
}

func validVendedorPayload() map[string]any {
	return map[string]any{
		"nome":          "Novo Vendedor",
		"regiao":        "Sudeste",
		"uf":            "SP",
		"data_admissao": "2024-01-15",
		"meta_mensal":   5000.0,
	}
}

// ---------------------------------------------------------------------------
// ListVendedores GET /api/vendedores
// ---------------------------------------------------------------------------

func TestListVendedores_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_desligamento\s+FROM vendedores\s+ORDER BY nome ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_desligamento"}).
			AddRow(int64(1), "Vendedor Um", "Sudeste", "SP", nil).
			AddRow(int64(2), "Vendedor Dois", "Sul", "PR", nil))

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].([]any)
	require.Len(t, data, 2)
	first := data[0].(map[string]any)
	assert.Equal(t, "Vendedor Um", first["nome"])
	assert.Nil(t, first["data_desligamento"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListVendedores_AtivosEInativos cobre o comportamento central da
// correção do bug: vendedores ativos e inativos devem vir juntos na listagem,
// com data_desligamento populado para os inativos, permitindo ao frontend
// exibir um marcador "[inativo]" em vez de o vendedor simplesmente sumir.
func TestListVendedores_AtivosEInativos(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	desligadoEm := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_desligamento\s+FROM vendedores\s+ORDER BY nome ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_desligamento"}).
			AddRow(int64(1), "Vendedor Ativo", "Sudeste", "SP", nil).
			AddRow(int64(2), "Vendedor Inativo", "Sul", "RS", desligadoEm))

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	data := body["data"].([]any)
	require.Len(t, data, 2)

	ativo := data[0].(map[string]any)
	assert.Equal(t, "Vendedor Ativo", ativo["nome"])
	assert.Nil(t, ativo["data_desligamento"])

	inativo := data[1].(map[string]any)
	assert.Equal(t, "Vendedor Inativo", inativo["nome"])
	assert.NotNil(t, inativo["data_desligamento"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListVendedores_ListaVazia(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_desligamento\s+FROM vendedores\s+ORDER BY nome ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_desligamento"}))

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	// data pode ser null (slice nula) quando não há resultados.
	assert.True(t, body["data"] == nil || len(body["data"].([]any)) == 0)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListVendedores_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_desligamento\s+FROM vendedores\s+ORDER BY nome ASC`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListVendedores_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_desligamento\s+FROM vendedores\s+ORDER BY nome ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_desligamento"}).
			AddRow(int64(1), "Vendedor Um", "Sudeste", "SP", nil).
			AddRow(int64(2), "Vendedor Dois", "Sul", "PR", nil))

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListVendedores_Normal_SemMetaMensal é regressão de segurança: GET
// /api/vendedores (acesso comum) devolve apenas VendedorResumo — a meta
// mensal dos colegas nunca deve aparecer para usuário normal.
func TestListVendedores_Normal_SemMetaMensal(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	userToken := generateToken(t, testCfg(), 2, "normal")

	mock.ExpectQuery(`SELECT id, nome, regiao, uf, data_desligamento\s+FROM vendedores\s+ORDER BY nome ASC`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "regiao", "uf", "data_desligamento"}).
			AddRow(int64(1), "Vendedor Um", "Sudeste", "SP", nil).
			AddRow(int64(2), "Vendedor Dois", "Sul", "PR", time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)))

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw := readBody(t, resp)
	assert.NotContains(t, string(raw), "meta_mensal")

	body := decodeResponse(t, raw)
	lista := body["data"].([]any)
	require.Len(t, lista, 2)
	for _, item := range lista {
		v := item.(map[string]any)
		assert.NotContains(t, v, "meta_mensal")
		assert.ElementsMatch(t, []string{"id", "nome", "regiao", "uf", "data_desligamento"}, mapKeys(v))
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestListVendedores_NaoAutenticado(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores", nil)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ---------------------------------------------------------------------------
// GetVendedor GET /api/vendedores/{id}
// ---------------------------------------------------------------------------

func TestGetVendedor_Success_ComClientes(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(vendedorGetColunasRegexH).
		WithArgs(int64(1)).
		WillReturnRows(vendedorGetRowsH(1, "João Vendedor", nil))
	mock.ExpectQuery(clienteResumoColunasRegexH + clienteResumoFromRegexH + ` WHERE ca\.vendedor_id = \? AND ca\.data_fim IS NULL ORDER BY c\.razao_social ASC`).
		WithArgs(int64(1)).
		WillReturnRows(clienteResumoRowsH())

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "João Vendedor", data["nome"])
	clientes := data["clientes"].([]any)
	require.Len(t, clientes, 2)
	primeiroCliente := clientes[0].(map[string]any)
	assert.Equal(t, "Cliente A", primeiroCliente["razao_social"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVendedor_Success_SemClientes(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(vendedorGetColunasRegexH).
		WithArgs(int64(2)).
		WillReturnRows(vendedorGetRowsH(2, "Maria Vendedora", nil))
	mock.ExpectQuery(clienteResumoColunasRegexH + clienteResumoFromRegexH + ` WHERE ca\.vendedor_id = \? AND ca\.data_fim IS NULL ORDER BY c\.razao_social ASC`).
		WithArgs(int64(2)).
		WillReturnRows(emptyClienteResumoRowsH())

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores/2", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Maria Vendedora", data["nome"])
	assert.True(t, data["clientes"] == nil || len(data["clientes"].([]any)) == 0)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVendedor_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(vendedorGetColunasRegexH).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores/999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "vendedor não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVendedor_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

// ---------------------------------------------------------------------------
// ListClientesDoVendedor GET /api/vendedores/{id}/clientes
// ---------------------------------------------------------------------------

func TestListClientesDoVendedor_Success_ComClientes(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(vendedorExistsByIDRegexH).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(clienteResumoColunasRegexH + clienteResumoFromRegexH + ` WHERE ca\.vendedor_id = \? AND ca\.data_fim IS NULL ORDER BY c\.razao_social ASC`).
		WithArgs(int64(1)).
		WillReturnRows(clienteResumoRowsH())

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores/1/clientes", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].([]any)
	require.Len(t, data, 2)
	primeiro := data[0].(map[string]any)
	assert.Equal(t, "Cliente A", primeiro["razao_social"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListClientesDoVendedor_Success_SemClientes(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(vendedorExistsByIDRegexH).WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(clienteResumoColunasRegexH + clienteResumoFromRegexH + ` WHERE ca\.vendedor_id = \? AND ca\.data_fim IS NULL ORDER BY c\.razao_social ASC`).
		WithArgs(int64(2)).
		WillReturnRows(emptyClienteResumoRowsH())

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores/2/clientes", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data, ok := body["data"].([]any)
	assert.True(t, !ok || len(data) == 0)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListClientesDoVendedor_VendedorNaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(vendedorExistsByIDRegexH).WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores/999/clientes", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "vendedor não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListClientesDoVendedor_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores/abc/clientes", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

// TestListClientesDoVendedor_PermitidoParaNaoAdmin garante que a rota é
// "acesso comum": usuários não-admin também conseguem consultar os clientes
// vinculados a um vendedor (usado pelo dropdown em cascata do frontend em
// Oportunidades) — desde que seja a própria carteira (ver
// TestListClientesDoVendedor_NegadoParaNaoAdminDeOutroVendedor).
func TestListClientesDoVendedor_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(1)))
	mock.ExpectQuery(vendedorExistsByIDRegexH).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(clienteResumoColunasRegexH + clienteResumoFromRegexH + ` WHERE ca\.vendedor_id = \? AND ca\.data_fim IS NULL ORDER BY c\.razao_social ASC`).
		WithArgs(int64(1)).
		WillReturnRows(clienteResumoRowsH())

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores/1/clientes", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListClientesDoVendedor_NegadoParaNaoAdminDeOutroVendedor garante que um
// usuário role=normal não consegue listar a carteira de outro vendedor
// (apenas a sua própria) — corrige o item 12 do relatório de segurança.
func TestListClientesDoVendedor_NegadoParaNaoAdminDeOutroVendedor(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(99)))

	req, _ := http.NewRequest("GET", server.URL+"/api/vendedores/1/clientes", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "vendedor não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CreateVendedor POST /api/vendedores
// ---------------------------------------------------------------------------

func TestCreateVendedor_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`INSERT INTO vendedores \(nome, regiao, uf, data_admissao, data_desligamento, meta_mensal\)`).
		WithArgs("Novo Vendedor", "Sudeste", "SP", sqlmock.AnyArg(), nil, 5000.0).
		WillReturnResult(sqlmock.NewResult(5, 1))

	req, _ := http.NewRequest("POST", server.URL+"/api/vendedores", makeJSON(validVendedorPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(5), data["id"])
	assert.Equal(t, "Novo Vendedor", data["nome"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateVendedor_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/vendedores", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestCreateVendedor_ValidacaoNegocio(t *testing.T) {
	testCases := []struct {
		nome    string
		payload func() map[string]any
		wantMsg string
	}{
		{
			nome: "nome ausente",
			payload: func() map[string]any {
				p := validVendedorPayload()
				p["nome"] = ""
				return p
			},
			wantMsg: "nome é obrigatório",
		},
		{
			nome: "regiao ausente",
			payload: func() map[string]any {
				p := validVendedorPayload()
				p["regiao"] = ""
				return p
			},
			wantMsg: "regiao é obrigatória",
		},
		{
			nome: "uf inválida",
			payload: func() map[string]any {
				p := validVendedorPayload()
				p["uf"] = "SAO"
				return p
			},
			wantMsg: "uf deve ter 2 letras",
		},
		{
			nome: "data_admissao inválida",
			payload: func() map[string]any {
				p := validVendedorPayload()
				p["data_admissao"] = "15/01/2024"
				return p
			},
			wantMsg: "data_admissao inválida (use o formato AAAA-MM-DD)",
		},
		{
			nome: "meta_mensal negativa",
			payload: func() map[string]any {
				p := validVendedorPayload()
				p["meta_mensal"] = -1.0
				return p
			},
			wantMsg: "meta_mensal deve ser maior ou igual a zero",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, _ := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			req, _ := http.NewRequest("POST", server.URL+"/api/vendedores", makeJSON(tc.payload()))
			req.Header.Set("Authorization", "Bearer "+adminToken)

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
// UpdateVendedor PUT /api/vendedores/{id}
// ---------------------------------------------------------------------------

func TestUpdateVendedor_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE vendedores\s+SET nome = \?, regiao = \?, uf = \?, data_admissao = \?, meta_mensal = \?\s+WHERE id = \?`).
		WithArgs("Novo Vendedor", "Sudeste", "SP", sqlmock.AnyArg(), 5000.0, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(vendedorGetColunasRegexH).
		WithArgs(int64(1)).
		WillReturnRows(vendedorGetRowsH(1, "Novo Vendedor", nil))

	req, _ := http.NewRequest("PUT", server.URL+"/api/vendedores/1", makeJSON(validVendedorPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Novo Vendedor", data["nome"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateVendedor_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/vendedores/abc", makeJSON(validVendedorPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestUpdateVendedor_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/vendedores/1", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestUpdateVendedor_ValidacaoNegocio(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	payload := validVendedorPayload()
	payload["uf"] = "S"

	req, _ := http.NewRequest("PUT", server.URL+"/api/vendedores/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "uf deve ter 2 letras", body["error"])
}

func TestUpdateVendedor_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE vendedores\s+SET nome = \?, regiao = \?, uf = \?, data_admissao = \?, meta_mensal = \?\s+WHERE id = \?`).
		WithArgs("Novo Vendedor", "Sudeste", "SP", sqlmock.AnyArg(), 5000.0, int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req, _ := http.NewRequest("PUT", server.URL+"/api/vendedores/999", makeJSON(validVendedorPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "vendedor não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// DeleteVendedor DELETE /api/vendedores/{id}
// ---------------------------------------------------------------------------

func TestDeleteVendedor_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(vendedorGetColunasRegexH).
		WithArgs(int64(1)).
		WillReturnRows(vendedorGetRowsH(1, "João Vendedor", time.Now()))

	req, _ := http.NewRequest("DELETE", server.URL+"/api/vendedores/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(1), data["id"])
	assert.NotNil(t, data["data_desligamento"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteVendedor_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req, _ := http.NewRequest("DELETE", server.URL+"/api/vendedores/999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "vendedor não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// ReativarVendedor POST /api/vendedores/{id}/reativar
// ---------------------------------------------------------------------------

func TestReativarVendedor_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(vendedorGetColunasRegexH).
		WithArgs(int64(1)).
		WillReturnRows(vendedorGetRowsH(1, "João Vendedor", nil))

	req, _ := http.NewRequest("POST", server.URL+"/api/vendedores/1/reativar", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(1), data["id"])
	assert.Nil(t, data["data_desligamento"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReativarVendedor_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/vendedores/abc/reativar", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestReativarVendedor_NaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(999)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	req, _ := http.NewRequest("POST", server.URL+"/api/vendedores/999/reativar", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "vendedor não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReativarVendedor_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectExec(`UPDATE vendedores SET data_desligamento = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnError(sql.ErrConnDone)

	req, _ := http.NewRequest("POST", server.URL+"/api/vendedores/1/reativar", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "erro interno", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// VincularCliente POST /api/vendedores/{id}/clientes
// ---------------------------------------------------------------------------

const vendedorExistsByIDRegexH = `SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`
const clienteGetByIDRegexH = `SELECT .+ FROM clientes WHERE cliente_id_origem = \? LIMIT 1`
const carteiraGetVinculoAtivoByClienteIDRegexH = `SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`
const carteiraGetVinculoAtivoRegexH = `SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`
const carteiraEncerrarVinculoRegexH = `UPDATE carteiras SET data_fim = \? WHERE carteira_id_origem = \?`
const carteiraCreateRegexH = `INSERT INTO carteiras \(cliente_id, vendedor_id, data_inicio, data_fim\)\s+VALUES \(\?, \?, \?, \?\)`
const carteiraGetVinculoByClienteVendedorDataRegexH = `SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE cliente_id = \? AND vendedor_id = \? AND data_inicio = \?\s+LIMIT 1`

func clienteGetByIDRowsH(idOrigem int64, razaoSocial string) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"cliente_id_origem", "cnpj", "razao_social", "segmento", "cidade", "uf", "bairro",
		"data_cadastro", "ativo", "created_at", "updated_at",
	}).AddRow(idOrigem, "11.111.111/0001-11", razaoSocial, "Varejo", "São Paulo", "SP", "Centro", now, true, now, now)
}

func carteiraVinculoRowH(carteiraIDOrigem, clienteID, vendedorID int64) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"carteira_id_origem", "cliente_id", "vendedor_id", "data_inicio", "data_fim", "created_at", "updated_at",
	}).AddRow(carteiraIDOrigem, clienteID, vendedorID, now, nil, now, now)
}

func TestVincularCliente_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(vendedorExistsByIDRegexH).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(clienteGetByIDRegexH).WithArgs(int64(10)).
		WillReturnRows(clienteGetByIDRowsH(10, "Cliente A"))
	mock.ExpectQuery(carteiraGetVinculoAtivoByClienteIDRegexH).WithArgs(int64(10)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(carteiraGetVinculoByClienteVendedorDataRegexH).WithArgs(int64(10), int64(1), sqlmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(carteiraCreateRegexH).
		WillReturnResult(sqlmock.NewResult(7, 1))

	req, _ := http.NewRequest("POST", server.URL+"/api/vendedores/1/clientes", makeJSON(map[string]any{"cliente_id": 10}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Cliente A", data["razao_social"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVincularCliente_BodyInvalido(t *testing.T) {
	testCases := []struct {
		nome    string
		body    func() *bytes.Buffer
		wantMsg string
	}{
		{
			nome:    "JSON malformado",
			body:    func() *bytes.Buffer { return bytes.NewBufferString("{invalido") },
			wantMsg: "body JSON inválido",
		},
		{
			nome:    "cliente_id ausente",
			body:    func() *bytes.Buffer { return makeJSON(map[string]any{}) },
			wantMsg: "cliente_id é obrigatório",
		},
		{
			nome:    "cliente_id zero",
			body:    func() *bytes.Buffer { return makeJSON(map[string]any{"cliente_id": 0}) },
			wantMsg: "cliente_id é obrigatório",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, _ := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			req, _ := http.NewRequest("POST", server.URL+"/api/vendedores/1/clientes", tc.body())
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, tc.wantMsg, body["error"])
		})
	}
}

func TestVincularCliente_VendedorNaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(vendedorExistsByIDRegexH).WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("POST", server.URL+"/api/vendedores/999/clientes", makeJSON(map[string]any{"cliente_id": 10}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "vendedor não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVincularCliente_ClienteNaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(vendedorExistsByIDRegexH).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(clienteGetByIDRegexH).WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("POST", server.URL+"/api/vendedores/1/clientes", makeJSON(map[string]any{"cliente_id": 999}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "cliente não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestVincularCliente_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/vendedores/abc/clientes", makeJSON(map[string]any{"cliente_id": 10}))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

// ---------------------------------------------------------------------------
// DesvincularCliente DELETE /api/vendedores/{id}/clientes/{clienteId}
// ---------------------------------------------------------------------------

func TestDesvincularCliente_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(carteiraGetVinculoAtivoRegexH).WithArgs(int64(1), int64(10)).
		WillReturnRows(carteiraVinculoRowH(300, 10, 1))
	mock.ExpectExec(carteiraEncerrarVinculoRegexH).
		WithArgs(sqlmock.AnyArg(), int64(300)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req, _ := http.NewRequest("DELETE", server.URL+"/api/vendedores/1/clientes/10", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDesvincularCliente_VinculoNaoEncontrado(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(carteiraGetVinculoAtivoRegexH).WithArgs(int64(1), int64(10)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("DELETE", server.URL+"/api/vendedores/1/clientes/10", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "vínculo entre cliente e vendedor não encontrado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDesvincularCliente_IDInvalido(t *testing.T) {
	testCases := []struct {
		nome    string
		url     string
		wantMsg string
	}{
		{"vendedor id inválido", "/api/vendedores/abc/clientes/10", "id inválido"},
		{"cliente id inválido", "/api/vendedores/1/clientes/abc", "cliente id inválido"},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, _ := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			req, _ := http.NewRequest("DELETE", server.URL+tc.url, nil)
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, tc.wantMsg, body["error"])
		})
	}
}

func TestDeleteVendedor_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("DELETE", server.URL+"/api/vendedores/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}
