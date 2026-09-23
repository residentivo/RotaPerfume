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

// oportunidadeColunasRegexH reflete a constante oportunidadeColunas do repositório.
const oportunidadeColunasRegexH = `oportunidade_id, cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado, data_fechamento, ciclo_dias, motivo_perda, created_at, updated_at`

func oportunidadeRowsForHandler() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"oportunidade_id", "cliente_id", "vendedor_id", "origem", "data_abertura", "etapa",
		"probabilidade_pct", "valor_estimado", "data_fechamento", "ciclo_dias", "motivo_perda",
		"created_at", "updated_at",
	}).AddRow(int64(1), int64(100), int64(1), "Site", now, "Prospeccao", 10.0, 1000.0, nil, nil, nil, now, now)
}

func emptyOportunidadeRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"oportunidade_id", "cliente_id", "vendedor_id", "origem", "data_abertura", "etapa",
		"probabilidade_pct", "valor_estimado", "data_fechamento", "ciclo_dias", "motivo_perda",
		"created_at", "updated_at",
	})
}

func validOportunidadePayload() map[string]any {
	return map[string]any{
		"cliente_id":        100,
		"vendedor_id":       1,
		"origem":            "Site",
		"data_abertura":     "2024-01-15",
		"etapa":             "Prospeccao",
		"probabilidade_pct": 10.0,
		"valor_estimado":    1000.0,
	}
}

// ---------------------------------------------------------------------------
// ListOportunidades GET /api/oportunidades
// ---------------------------------------------------------------------------

func TestListOportunidades_Success_Admin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oportunidades`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT `+oportunidadeColunasRegexH+` FROM oportunidades ORDER BY oportunidade_id ASC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(oportunidadeRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

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

// TestListOportunidades_PermitidoParaNaoAdmin_ForcaCarteira cobre a mudança
// de rota: a listagem virou acesso comum, mas o vendedor_id da query é
// ignorado e forçado à carteira do vendedor vinculado ao usuário autenticado
// (evita bypass via URL, ex.: ?vendedor_id=<de outro vendedor>).
func TestListOportunidades_PermitidoParaNaoAdmin_ForcaCarteira(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	vendedorWhere := ` WHERE vendedor_id = \?`
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(2)))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oportunidades` + vendedorWhere).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades` + vendedorWhere + ` ORDER BY oportunidade_id ASC LIMIT \? OFFSET \?`).
		WithArgs(int64(2), 20, 0).
		WillReturnRows(oportunidadeRowsForHandler())

	// vendedor_id=99 na query é ignorado — o filtro real usado é o vendedor
	// (2) vinculado ao usuário autenticado.
	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades?vendedor_id=99", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListOportunidades_SemVendedorVinculado_ListaVazia cobre o caso de
// usuário role=normal sem id_vendedor vinculado: nunca deve enxergar dados de
// terceiros, retorna lista vazia (200) sem sequer consultar a tabela de
// oportunidades.
func TestListOportunidades_SemVendedorVinculado_ListaVazia(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].([]any)
	assert.Len(t, data, 0)
	pagination := body["pagination"].(map[string]any)
	assert.Equal(t, float64(0), pagination["total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListOportunidades_ComFiltros(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oportunidades WHERE cliente_id = \? AND vendedor_id = \? AND etapa = \?`).
		WithArgs(int64(100), int64(1), "Prospeccao").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT `+oportunidadeColunasRegexH+` FROM oportunidades WHERE cliente_id = \? AND vendedor_id = \? AND etapa = \? ORDER BY oportunidade_id ASC LIMIT \? OFFSET \?`).
		WithArgs(int64(100), int64(1), "Prospeccao", 20, 0).
		WillReturnRows(oportunidadeRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades?cliente_id=100&vendedor_id=1&etapa=Prospeccao", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListOportunidades_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oportunidades`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetOportunidade GET /api/oportunidades/{id}
// ---------------------------------------------------------------------------

func TestGetOportunidade_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Prospeccao", data["etapa"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOportunidade_NaoEncontrada(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades/999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "oportunidade não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOportunidade_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

// TestGetOportunidade_PermitidoParaNaoAdmin cobre a mudança de rota: o
// detalhe virou acesso comum para o vendedor dono do registro.
func TestGetOportunidade_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// oportunidadeRowsForHandler() tem vendedor_id=1 — usuário vinculado ao
	// mesmo vendedor.
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetOportunidade_NegadoParaNaoAdminForaDaCarteira cobre o cenário de
// IDOR: vendedor não-admin tentando ler oportunidade de outro vendedor por
// enumeração de ID deve receber 404 (não 403, para não confirmar a
// existência do registro).
func TestGetOportunidade_NegadoParaNaoAdminForaDaCarteira(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// oportunidadeRowsForHandler() tem vendedor_id=1, mas o usuário está
	// vinculado ao vendedor 99 — não deve conseguir ver a oportunidade.
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(99)))
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "oportunidade não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CreateOportunidade POST /api/oportunidades
// ---------------------------------------------------------------------------

func TestCreateOportunidade_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO oportunidades`).
		WithArgs(int64(100), int64(1), "Site", sqlmock.AnyArg(), "Prospeccao", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(9, 1))

	req, _ := http.NewRequest("POST", server.URL+"/api/oportunidades", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(9), data["oportunidade_id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateOportunidade_PermitidoParaNaoAdmin_ForcaVendedorID cobre a
// mudança de rota: vendedor não-admin pode criar oportunidade, mas o
// vendedor_id do payload é sempre ignorado e forçado ao vendedor vinculado
// ao usuário autenticado (evita forjar registro para outro vendedor).
func TestCreateOportunidade_PermitidoParaNaoAdmin_ForcaVendedorID(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(5)))
	mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`).
		WithArgs(int64(5), int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{
			"carteira_id_origem", "cliente_id", "vendedor_id", "data_inicio", "data_fim", "created_at", "updated_at",
		}).AddRow(int64(500), int64(100), int64(5), time.Now(), nil, time.Now(), time.Now()))
	mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(`INSERT INTO oportunidades`).
		WithArgs(int64(100), int64(5), "Site", sqlmock.AnyArg(), "Prospeccao", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(9, 1))

	// payload tenta forjar vendedor_id=99 (outro vendedor) — deve ser
	// ignorado e substituído pelo vendedor vinculado (5).
	payload := validOportunidadePayload()
	payload["vendedor_id"] = 99

	req, _ := http.NewRequest("POST", server.URL+"/api/oportunidades", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(5), data["vendedor_id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateOportunidade_SemVendedorVinculado_Forbidden cobre o caso de
// usuário role=normal sem id_vendedor vinculado: 403, sem sequer chegar a
// decodificar o payload de negócio.
func TestCreateOportunidade_SemVendedorVinculado_Forbidden(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))

	req, _ := http.NewRequest("POST", server.URL+"/api/oportunidades", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "usuário sem vendedor vinculado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateOportunidade_ClienteForaDaCarteira cobre a validação de
// SecBrain: vendedor não-admin tentando criar oportunidade para um cliente
// fora da própria carteira recebe 400, sem chegar a inserir o registro.
func TestCreateOportunidade_ClienteForaDaCarteira(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(5)))
	mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`).
		WithArgs(int64(5), int64(100)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("POST", server.URL+"/api/oportunidades", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "cliente não pertence à carteira deste vendedor", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateOportunidade_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/oportunidades", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestCreateOportunidade_ValidacaoNegocio(t *testing.T) {
	testCases := []struct {
		nome    string
		payload func() map[string]any
		mock    func(mock sqlmock.Sqlmock)
		wantMsg string
	}{
		{
			nome: "cliente_id ausente",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["cliente_id"] = 0
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "cliente_id é obrigatório e deve existir",
		},
		{
			nome: "vendedor_id ausente",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["vendedor_id"] = 0
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "vendedor_id é obrigatório e deve existir",
		},
		{
			nome: "origem vazia",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["origem"] = ""
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "origem é obrigatória",
		},
		{
			nome: "etapa vazia",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["etapa"] = ""
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "etapa é obrigatória",
		},
		{
			nome: "probabilidade fora do range",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["probabilidade_pct"] = 150.0
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "probabilidade_pct deve estar entre 0 e 100",
		},
		{
			nome: "valor_estimado negativo",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["valor_estimado"] = -1.0
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "valor_estimado deve ser maior ou igual a zero",
		},
		{
			nome: "etapa Fechado perdido sem motivo_perda",
			payload: func() map[string]any {
				p := validOportunidadePayload()
				p["etapa"] = "Fechado perdido"
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "motivo_perda é obrigatório quando etapa = \"Fechado perdido\"",
		},
		{
			nome: "cliente_id inexistente",
			payload: func() map[string]any {
				return validOportunidadePayload()
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnError(sql.ErrNoRows)
			},
			wantMsg: "cliente_id é obrigatório e deve existir",
		},
		{
			nome: "vendedor_id inexistente",
			payload: func() map[string]any {
				return validOportunidadePayload()
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnError(sql.ErrNoRows)
			},
			wantMsg: "vendedor_id é obrigatório e deve existir",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			cfg := testCfg()
			adminToken := generateToken(t, cfg, 1, "admin")

			tc.mock(mock)

			req, _ := http.NewRequest("POST", server.URL+"/api/oportunidades", makeJSON(tc.payload()))
			req.Header.Set("Authorization", "Bearer "+adminToken)

			resp, err := (&http.Client{}).Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			body := decodeResponse(t, readBody(t, resp))
			assert.Equal(t, tc.wantMsg, body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateOportunidade PUT /api/oportunidades/{id}
// ---------------------------------------------------------------------------

func TestUpdateOportunidade_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// UpdateOportunidade busca o registro atual primeiro (owner check —
	// sempre executado, mesmo para admin) antes de decodificar/validar o
	// payload.
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())
	mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(`UPDATE oportunidades`).
		WithArgs(int64(100), int64(1), "Site", sqlmock.AnyArg(), "Prospeccao", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/1", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Prospeccao", data["etapa"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateOportunidade_NegadoParaNaoAdminForaDaCarteira cobre o cenário de
// IDOR: vendedor não-admin tentando editar oportunidade de outro vendedor
// recebe 404.
func TestUpdateOportunidade_NegadoParaNaoAdminForaDaCarteira(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// oportunidadeRowsForHandler() tem vendedor_id=1, mas o usuário está
	// vinculado ao vendedor 99.
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(99)))
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/1", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "oportunidade não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateOportunidade_PermitidoParaNaoAdmin_ForcaVendedorID cobre a
// mudança de rota: vendedor não-admin pode editar oportunidade da própria
// carteira, mas o vendedor_id do payload é sempre ignorado e forçado ao
// vendedor vinculado (evita reatribuir o registro a outro vendedor).
func TestUpdateOportunidade_PermitidoParaNaoAdmin_ForcaVendedorID(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// oportunidadeRowsForHandler() tem vendedor_id=1 — usuário vinculado ao
	// mesmo vendedor (dono do registro).
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())
	mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`).
		WithArgs(int64(1), int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{
			"carteira_id_origem", "cliente_id", "vendedor_id", "data_inicio", "data_fim", "created_at", "updated_at",
		}).AddRow(int64(500), int64(100), int64(1), time.Now(), nil, time.Now(), time.Now()))
	mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(`UPDATE oportunidades`).
		WithArgs(int64(100), int64(1), "Site", sqlmock.AnyArg(), "Prospeccao", 10.0, 1000.0, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())

	// payload tenta reatribuir vendedor_id=99 — deve ser ignorado e mantido
	// o vendedor vinculado (1).
	payload := validOportunidadePayload()
	payload["vendedor_id"] = 99

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateOportunidade_ClienteForaDaCarteira cobre a validação de
// SecBrain: vendedor não-admin tentando reatribuir a oportunidade a um
// cliente fora da própria carteira recebe 400.
func TestUpdateOportunidade_ClienteForaDaCarteira(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())
	mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`).
		WithArgs(int64(1), int64(100)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/1", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "cliente não pertence à carteira deste vendedor", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateOportunidade_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/abc", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestUpdateOportunidade_JSONInvalido(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// UpdateOportunidade busca o registro atual antes de decodificar o body.
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/1", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateOportunidade_ValidacaoNegocio(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// UpdateOportunidade busca o registro atual antes de validar o payload.
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())

	payload := validOportunidadePayload()
	payload["probabilidade_pct"] = -5.0

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "probabilidade_pct deve estar entre 0 e 100", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateOportunidade_NaoEncontrada cobre o caso de ID inexistente: a
// busca do registro atual (owner check) já retorna "não encontrada" antes de
// qualquer tentativa de UPDATE.
func TestUpdateOportunidade_NaoEncontrada(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("PUT", server.URL+"/api/oportunidades/999", makeJSON(validOportunidadePayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "oportunidade não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListOportunidades_ListaVazia(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oportunidades`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT `+oportunidadeColunasRegexH+` FROM oportunidades ORDER BY oportunidade_id ASC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(emptyOportunidadeRows())

	req, _ := http.NewRequest("GET", server.URL+"/api/oportunidades", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	pagination := body["pagination"].(map[string]any)
	assert.Equal(t, float64(0), pagination["total"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// DeleteOportunidade DELETE /api/oportunidades/{id}
// ---------------------------------------------------------------------------

const deleteOportunidadeRegexH = `DELETE FROM oportunidades WHERE oportunidade_id = \?`

func TestDeleteOportunidade_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())
	mock.ExpectExec(deleteOportunidadeRegexH).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req, _ := http.NewRequest("DELETE", server.URL+"/api/oportunidades/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteOportunidade_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// oportunidadeRowsForHandler() tem vendedor_id=1 — usuário vinculado ao
	// mesmo vendedor (dono do registro).
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())
	mock.ExpectExec(deleteOportunidadeRegexH).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req, _ := http.NewRequest("DELETE", server.URL+"/api/oportunidades/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteOportunidade_NaoEncontrada(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("DELETE", server.URL+"/api/oportunidades/999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "oportunidade não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestDeleteOportunidade_NegadoParaNaoAdminForaDaCarteira cobre o cenário de
// IDOR: vendedor não-admin tentando excluir oportunidade de outro vendedor
// recebe 404.
func TestDeleteOportunidade_NegadoParaNaoAdminForaDaCarteira(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// oportunidadeRowsForHandler() tem vendedor_id=1, mas o usuário está
	// vinculado ao vendedor 99.
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(99)))
	mock.ExpectQuery(`SELECT ` + oportunidadeColunasRegexH + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(oportunidadeRowsForHandler())

	req, _ := http.NewRequest("DELETE", server.URL+"/api/oportunidades/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "oportunidade não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteOportunidade_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("DELETE", server.URL+"/api/oportunidades/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}
