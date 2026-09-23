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

// visitaColunasRegexH reflete a constante visitaColunas do repositório.
const visitaColunasRegexH = `visita_id, cliente_id, vendedor_id, data_visita, resultado, duracao_min, created_at, updated_at`

func visitaRowsForHandler() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{
		"visita_id", "cliente_id", "vendedor_id", "data_visita", "resultado", "duracao_min",
		"created_at", "updated_at",
	}).AddRow(int64(1), int64(100), int64(1), now, "Positiva", 30, now, now)
}

func emptyVisitaRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"visita_id", "cliente_id", "vendedor_id", "data_visita", "resultado", "duracao_min",
		"created_at", "updated_at",
	})
}

func validVisitaPayload() map[string]any {
	return map[string]any{
		"cliente_id":  100,
		"vendedor_id": 1,
		"data_visita": "2024-01-15",
		"resultado":   "Positiva",
		"duracao_min": 30,
	}
}

// ---------------------------------------------------------------------------
// ListVisitas GET /api/visitas
// ---------------------------------------------------------------------------

func TestListVisitas_Success_Admin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM visitas`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT `+visitaColunasRegexH+` FROM visitas ORDER BY visita_id ASC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(visitaRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas", nil)
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

// TestListVisitas_PermitidoParaNaoAdmin_ForcaCarteira cobre a mudança de
// rota: a listagem virou acesso comum, mas o vendedor_id da query é ignorado
// e forçado à carteira do vendedor vinculado ao usuário autenticado (evita
// bypass via URL, ex.: ?vendedor_id=<de outro vendedor>).
func TestListVisitas_PermitidoParaNaoAdmin_ForcaCarteira(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	vendedorWhere := ` WHERE vendedor_id = \?`
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(2)))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM visitas` + vendedorWhere).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas` + vendedorWhere + ` ORDER BY visita_id ASC LIMIT \? OFFSET \?`).
		WithArgs(int64(2), 20, 0).
		WillReturnRows(visitaRowsForHandler())

	// vendedor_id=99 na query é ignorado — o filtro real usado é o vendedor
	// (2) vinculado ao usuário autenticado.
	req, _ := http.NewRequest("GET", server.URL+"/api/visitas?vendedor_id=99", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestListVisitas_SemVendedorVinculado_ListaVazia cobre o caso de usuário
// role=normal sem id_vendedor vinculado: nunca deve enxergar dados de
// terceiros, retorna lista vazia (200) sem sequer consultar a tabela de
// visitas.
func TestListVisitas_SemVendedorVinculado_ListaVazia(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas", nil)
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

func TestListVisitas_ComFiltros(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM visitas WHERE cliente_id = \? AND vendedor_id = \? AND resultado = \?`).
		WithArgs(int64(100), int64(1), "Positiva").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT `+visitaColunasRegexH+` FROM visitas WHERE cliente_id = \? AND vendedor_id = \? AND resultado = \? ORDER BY visita_id ASC LIMIT \? OFFSET \?`).
		WithArgs(int64(100), int64(1), "Positiva", 20, 0).
		WillReturnRows(visitaRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas?cliente_id=100&vendedor_id=1&resultado=Positiva", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListVisitas_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM visitas`).
		WillReturnError(sqlmock.ErrCancelled)

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestListVisitas_ListaVazia(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM visitas`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery(`SELECT `+visitaColunasRegexH+` FROM visitas ORDER BY visita_id ASC LIMIT \? OFFSET \?`).
		WithArgs(20, 0).
		WillReturnRows(emptyVisitaRows())

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas", nil)
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
// GetVisita GET /api/visitas/{id}
// ---------------------------------------------------------------------------

func TestGetVisita_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Positiva", data["resultado"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVisita_NaoEncontrada(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas/999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "visita não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVisita_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

// TestGetVisita_PermitidoParaNaoAdmin cobre a mudança de rota: o detalhe
// virou acesso comum para o vendedor dono do registro.
func TestGetVisita_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// visitaRowsForHandler() tem vendedor_id=1 — usuário vinculado ao mesmo
	// vendedor.
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestGetVisita_NegadoParaNaoAdminForaDaCarteira cobre o cenário de IDOR:
// vendedor não-admin tentando ler visita de outro vendedor por enumeração de
// ID deve receber 404 (não 403).
func TestGetVisita_NegadoParaNaoAdminForaDaCarteira(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// visitaRowsForHandler() tem vendedor_id=1, mas o usuário está vinculado
	// ao vendedor 99 — não deve conseguir ver a visita.
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(99)))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "visita não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestGetVisita_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnError(sql.ErrConnDone)

	req, _ := http.NewRequest("GET", server.URL+"/api/visitas/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CreateVisita POST /api/visitas
// ---------------------------------------------------------------------------

func TestCreateVisita_Success(t *testing.T) {
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
	mock.ExpectExec(`INSERT INTO visitas`).
		WithArgs(int64(100), int64(1), sqlmock.AnyArg(), "Positiva", 30).
		WillReturnResult(sqlmock.NewResult(9, 1))

	req, _ := http.NewRequest("POST", server.URL+"/api/visitas", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, float64(9), data["visita_id"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateVisita_PermitidoParaNaoAdmin_ForcaVendedorID cobre a mudança de
// rota: vendedor não-admin pode criar visita, mas o vendedor_id do payload é
// sempre ignorado e forçado ao vendedor vinculado ao usuário autenticado
// (evita forjar registro para outro vendedor).
func TestCreateVisita_PermitidoParaNaoAdmin_ForcaVendedorID(t *testing.T) {
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
	mock.ExpectExec(`INSERT INTO visitas`).
		WithArgs(int64(100), int64(5), sqlmock.AnyArg(), "Positiva", 30).
		WillReturnResult(sqlmock.NewResult(9, 1))

	// payload tenta forjar vendedor_id=99 (outro vendedor) — deve ser
	// ignorado e substituído pelo vendedor vinculado (5).
	payload := validVisitaPayload()
	payload["vendedor_id"] = 99

	req, _ := http.NewRequest("POST", server.URL+"/api/visitas", makeJSON(payload))
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

// TestCreateVisita_SemVendedorVinculado_Forbidden cobre o caso de usuário
// role=normal sem id_vendedor vinculado: 403, sem sequer chegar a
// decodificar o payload de negócio.
func TestCreateVisita_SemVendedorVinculado_Forbidden(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))

	req, _ := http.NewRequest("POST", server.URL+"/api/visitas", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "usuário sem vendedor vinculado", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCreateVisita_ClienteForaDaCarteira cobre a validação de SecBrain:
// vendedor não-admin tentando criar visita para um cliente fora da própria
// carteira recebe 400, sem chegar a inserir o registro.
func TestCreateVisita_ClienteForaDaCarteira(t *testing.T) {
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

	req, _ := http.NewRequest("POST", server.URL+"/api/visitas", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "cliente não pertence à carteira deste vendedor", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateVisita_JSONInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("POST", server.URL+"/api/visitas", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])
}

func TestCreateVisita_ValidacaoNegocio(t *testing.T) {
	testCases := []struct {
		nome    string
		payload func() map[string]any
		mock    func(mock sqlmock.Sqlmock)
		wantMsg string
	}{
		{
			nome: "cliente_id ausente",
			payload: func() map[string]any {
				p := validVisitaPayload()
				p["cliente_id"] = 0
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "cliente_id é obrigatório e deve existir",
		},
		{
			nome: "vendedor_id ausente",
			payload: func() map[string]any {
				p := validVisitaPayload()
				p["vendedor_id"] = 0
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "vendedor_id é obrigatório e deve existir",
		},
		{
			nome: "resultado vazio",
			payload: func() map[string]any {
				p := validVisitaPayload()
				p["resultado"] = ""
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "resultado é obrigatório",
		},
		{
			nome: "duracao_min negativa",
			payload: func() map[string]any {
				p := validVisitaPayload()
				p["duracao_min"] = -1
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "duracao_min deve ser maior ou igual a zero",
		},
		{
			nome: "data_visita ausente",
			payload: func() map[string]any {
				p := validVisitaPayload()
				p["data_visita"] = ""
				return p
			},
			mock:    func(mock sqlmock.Sqlmock) {},
			wantMsg: "data_visita inválida (use o formato AAAA-MM-DD)",
		},
		{
			nome: "cliente_id inexistente",
			payload: func() map[string]any {
				return validVisitaPayload()
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
				return validVisitaPayload()
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
		{
			nome: "data_visita em formato inválido",
			payload: func() map[string]any {
				p := validVisitaPayload()
				p["data_visita"] = "15/01/2024"
				return p
			},
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
					WithArgs(int64(100)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
			},
			wantMsg: "data_visita inválida (use o formato AAAA-MM-DD)",
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

			req, _ := http.NewRequest("POST", server.URL+"/api/visitas", makeJSON(tc.payload()))
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

func TestCreateVisita_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnError(sql.ErrConnDone)

	req, _ := http.NewRequest("POST", server.URL+"/api/visitas", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// UpdateVisita PUT /api/visitas/{id}
// ---------------------------------------------------------------------------

func TestUpdateVisita_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// UpdateVisita busca o registro atual primeiro (owner check — sempre
	// executado, mesmo para admin) antes de decodificar/validar o payload.
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())
	mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(`UPDATE visitas`).
		WithArgs(int64(100), int64(1), sqlmock.AnyArg(), "Positiva", 30, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/1", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))
	data := body["data"].(map[string]any)
	assert.Equal(t, "Positiva", data["resultado"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateVisita_NegadoParaNaoAdminForaDaCarteira cobre o cenário de IDOR:
// vendedor não-admin tentando editar visita de outro vendedor recebe 404.
func TestUpdateVisita_NegadoParaNaoAdminForaDaCarteira(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// visitaRowsForHandler() tem vendedor_id=1, mas o usuário está vinculado
	// ao vendedor 99.
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(99)))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/1", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "visita não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateVisita_PermitidoParaNaoAdmin_ForcaVendedorID cobre a mudança de
// rota: vendedor não-admin pode editar visita da própria carteira, mas o
// vendedor_id do payload é sempre ignorado e forçado ao vendedor vinculado
// (evita reatribuir o registro a outro vendedor).
func TestUpdateVisita_PermitidoParaNaoAdmin_ForcaVendedorID(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// visitaRowsForHandler() tem vendedor_id=1 — usuário vinculado ao mesmo
	// vendedor (dono do registro).
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())
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
	mock.ExpectExec(`UPDATE visitas`).
		WithArgs(int64(100), int64(1), sqlmock.AnyArg(), "Positiva", 30, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())

	// payload tenta reatribuir vendedor_id=99 — deve ser ignorado e mantido
	// o vendedor vinculado (1).
	payload := validVisitaPayload()
	payload["vendedor_id"] = 99

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.True(t, body["success"].(bool))

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateVisita_ClienteForaDaCarteira cobre a validação de SecBrain:
// vendedor não-admin tentando reatribuir a visita a um cliente fora da
// própria carteira recebe 400.
func TestUpdateVisita_ClienteForaDaCarteira(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())
	mock.ExpectQuery(`SELECT carteira_id_origem, cliente_id, vendedor_id, data_inicio, data_fim, created_at, updated_at\s+FROM carteiras\s+WHERE vendedor_id = \? AND cliente_id = \? AND data_fim IS NULL\s+LIMIT 1`).
		WithArgs(int64(1), int64(100)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/1", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "cliente não pertence à carteira deste vendedor", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateVisita_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/abc", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}

func TestUpdateVisita_JSONInvalido(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// UpdateVisita busca o registro atual antes de decodificar o body.
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/1", bytes.NewBufferString("{invalido"))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "body JSON inválido", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateVisita_ValidacaoNegocio(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	// UpdateVisita busca o registro atual antes de validar o payload.
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())

	payload := validVisitaPayload()
	payload["duracao_min"] = -5

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/1", makeJSON(payload))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "duracao_min deve ser maior ou igual a zero", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateVisita_NaoEncontrada cobre o caso de ID inexistente: a busca do
// registro atual (owner check) já retorna "não encontrada" antes de
// qualquer tentativa de UPDATE.
func TestUpdateVisita_NaoEncontrada(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/999", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "visita não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateVisita_ErroInterno(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())
	mock.ExpectQuery(`SELECT 1 FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).
		WithArgs(int64(100)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
	mock.ExpectExec(`UPDATE visitas`).
		WillReturnError(sql.ErrConnDone)

	req, _ := http.NewRequest("PUT", server.URL+"/api/visitas/1", makeJSON(validVisitaPayload()))
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// DeleteVisita DELETE /api/visitas/{id}
// ---------------------------------------------------------------------------

const deleteVisitaRegexH = `DELETE FROM visitas WHERE visita_id = \?`

func TestDeleteVisita_Success(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())
	mock.ExpectExec(deleteVisitaRegexH).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req, _ := http.NewRequest("DELETE", server.URL+"/api/visitas/1", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteVisita_PermitidoParaNaoAdmin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// visitaRowsForHandler() tem vendedor_id=1 — usuário vinculado ao mesmo
	// vendedor (dono do registro).
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())
	mock.ExpectExec(deleteVisitaRegexH).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	req, _ := http.NewRequest("DELETE", server.URL+"/api/visitas/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteVisita_NaoEncontrada(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(999)).
		WillReturnError(sql.ErrNoRows)

	req, _ := http.NewRequest("DELETE", server.URL+"/api/visitas/999", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "visita não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestDeleteVisita_NegadoParaNaoAdminForaDaCarteira cobre o cenário de IDOR:
// vendedor não-admin tentando excluir visita de outro vendedor recebe 404.
func TestDeleteVisita_NegadoParaNaoAdminForaDaCarteira(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	userToken := generateToken(t, cfg, 2, "normal")

	// visitaRowsForHandler() tem vendedor_id=1, mas o usuário está vinculado
	// ao vendedor 99.
	mock.ExpectQuery(`SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(int64(99)))
	mock.ExpectQuery(`SELECT ` + visitaColunasRegexH + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(int64(1)).
		WillReturnRows(visitaRowsForHandler())

	req, _ := http.NewRequest("DELETE", server.URL+"/api/visitas/1", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "visita não encontrada", body["error"])

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDeleteVisita_IDInvalido(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	adminToken := generateToken(t, cfg, 1, "admin")

	req, _ := http.NewRequest("DELETE", server.URL+"/api/visitas/abc", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	body := decodeResponse(t, readBody(t, resp))
	assert.Equal(t, "id inválido", body["error"])
}
