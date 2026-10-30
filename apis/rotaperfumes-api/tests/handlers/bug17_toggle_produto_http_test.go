package handlers_test

// BUG-17 (Lote 25) pelo HTTP: PATCH /api/produtos/{id}/inativar. Sem
// valor de ativo (sem body, body vazio, {} ou {"ativo":null}), um único
// UPDATE … SET ativo = NOT ativo; com valor, SET ativo = ?. Nos dois casos,
// a escrita vem ANTES de qualquer leitura (sqlmock em ordem estrita) e a
// resposta é a releitura. 0 linhas → 404 "produto não encontrado" sem
// releitura.

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	bug17ToggleReH  = `^UPDATE produtos SET ativo = NOT ativo WHERE id = \?$`
	bug17DefinirReH = `^UPDATE produtos SET ativo = \? WHERE id = \?$`
)

type bug17CasoH struct {
	nome   string
	body   *string // nil = sem body
	toggle bool
	valor  bool
	relido bool
}

func bug17Str(s string) *string { return &s }

func bug17CasosH() []bug17CasoH {
	return []bug17CasoH{
		{"sem body", nil, true, false, false},
		{"body vazio", bug17Str(""), true, false, true},
		{"objeto vazio", bug17Str(`{}`), true, false, false},
		{"ativo null", bug17Str(`{"ativo":null}`), true, false, true},
		{"ativo true", bug17Str(`{"ativo":true}`), false, true, true},
		{"ativo false", bug17Str(`{"ativo":false}`), false, false, false},
	}
}

func bug17PatchH(t *testing.T, url string, body *string) (int, map[string]any) {
	t.Helper()
	var req *http.Request
	if body == nil {
		req, _ = http.NewRequest(http.MethodPatch, url, nil)
	} else {
		req, _ = http.NewRequest(http.MethodPatch, url, bytes.NewBufferString(*body))
	}
	req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, decodeResponse(t, readBody(t, resp))
}

func bug17ExpectEscritaH(mock sqlmock.Sqlmock, c bug17CasoH, id int64) *sqlmock.ExpectedExec {
	if c.toggle {
		return mock.ExpectExec(bug17ToggleReH).WithArgs(id)
	}
	return mock.ExpectExec(bug17DefinirReH).WithArgs(c.valor, id)
}

func TestBUG17_HTTP_Toggle_EscritaAtomicaEReleitura(t *testing.T) {
	for _, c := range bug17CasosH() {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			bug17ExpectEscritaH(mock, c, 1).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(`SELECT ` + produtoColunasRegex + ` FROM produtos WHERE id = \? LIMIT 1`).
				WithArgs(int64(1)).WillReturnRows(produtoRowsAtivoForHandler(c.relido))

			status, body := bug17PatchH(t, server.URL+"/api/produtos/1/inativar", c.body)
			assert.Equal(t, http.StatusOK, status, "body: %v", body)
			data, _ := body["data"].(map[string]any)
			assert.Equal(t, c.relido, data["ativo"], "a resposta é o valor relido do banco")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestBUG17_HTTP_Toggle_ZeroLinhas_404SemReleitura(t *testing.T) {
	for _, c := range bug17CasosH() {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			bug17ExpectEscritaH(mock, c, 999).WillReturnResult(sqlmock.NewResult(0, 0))

			status, body := bug17PatchH(t, server.URL+"/api/produtos/999/inativar", c.body)
			assert.Equal(t, http.StatusNotFound, status)
			assert.Equal(t, "produto não encontrado", body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Erro de banco no UPDATE: 500 "erro interno" e nenhuma releitura.
func TestBUG17_HTTP_Toggle_ErroNoUpdate_500(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	mock.ExpectExec(bug17ToggleReH).WithArgs(int64(1)).WillReturnError(sqlmock.ErrCancelled)
	status, body := bug17PatchH(t, server.URL+"/api/produtos/1/inativar", nil)
	assert.Equal(t, http.StatusInternalServerError, status)
	assert.Equal(t, "erro interno", body["error"])
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Body inválido: 400 sem UPDATE (o contrato não muda).
func TestBUG17_HTTP_Toggle_BodyInvalido_400SemSQL(t *testing.T) {
	for _, b := range []string{`{"ativo":"sim"}`, `{`, `{"ativo":true} lixo`, `[]`} {
		t.Run(b, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			status, _ := bug17PatchH(t, server.URL+"/api/produtos/1/inativar", bug17Str(b))
			assert.Equal(t, http.StatusBadRequest, status)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
