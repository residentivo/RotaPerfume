package handlers_test

// ARQ-04 / P1 (§6.2-§6.3, O7, M-04): com o vlog ligado, o Comercial emite os
// níveis (rota, caso de uso, decisão de escopo, query) e nenhum valor
// digitado nem o CNPJ/razão social aparece no log.

import (
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARQ04_Vlog_NiveisSemValores(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	normal := generateToken(t, testCfg(), 7, "normal")
	semVinculo := generateToken(t, testCfg(), 8, "normal")
	out := capturarVlogLigado(t)

	// Restrito ativo lista a carteira com busca.
	mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(10))
	mock.ExpectQuery(reVendedorDesligado).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"d"}).AddRow(0))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes`).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
	mock.ExpectQuery(`FROM clientes WHERE`).WillReturnRows(sqlmock.NewRows([]string{"x"}))
	status, _ := getComToken2(t, server.URL+"/api/clientes?q=BuscaSecreta", normal)
	require.Equal(t, http.StatusOK, status)

	// Restrito cadastra cliente com CNPJ duplicado: 409 sem o CNPJ no log.
	mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(10))
	mock.ExpectQuery(reVendedorDesligado).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"d"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO clientes`).WillReturnError(erroDuplicadoCNPJH())
	mock.ExpectRollback()
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/clientes", makeJSON(map[string]any{
		"cnpj": "11.222.333/0001-81", "razao_social": "RazaoSecreta", "segmento": "varejo", "cidade": "Curitiba", "uf": "PR"}))
	req.Header.Set("Authorization", "Bearer "+normal)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusConflict, resp.StatusCode)

	// Sem vínculo: decisão SemAcesso, sem SQL de listagem.
	mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(8)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))
	status, _ = getComToken2(t, server.URL+"/api/visitas", semVinculo)
	require.Equal(t, http.StatusOK, status)
	require.NoError(t, mock.ExpectationsWereMet())

	saida := out.String()
	for _, nivel := range []string{
		"[cliente_handler.go] [ClienteHandler.ListClientes] início GET",              // 1. rota
		"[clientes.go] [Comercial.ListarClientes] início page=1 limit=20 busca=true", // 2. caso de uso
		"[clientes.go] [Comercial.ListarClientes] fim ok",
		"[usuarios.go] [Usuarios.VendedorDoUsuario] SELECT usuarios vinculado=true", // 4. query (Identidade)
		"[vendedores.go] [Vendedores.Desligado] SELECT vendedores desligado=false",
		"[clientes.go] [Clientes.Listar] SELECT clientes linhas=0 total=0 carteira=true",
		"[clientes.go] [Comercial.CriarCliente] fim erro=conflito",                                 // erro só pelo Kind
		"[escopo.go] [AutorizadorEscopo.resolver] decisão sem_acesso user_id=8 motivo=sem_vinculo", // 3. decisão
	} {
		assert.Contains(t, saida, nivel)
	}
	for _, valor := range []string{"BuscaSecreta", "RazaoSecreta", "11222333000181", "11.222.333", "Duplicate entry", "SELECT COUNT", "LIKE"} {
		assert.NotContains(t, saida, valor, "o log não leva valores digitados, CNPJ (M-04) nem texto de SQL")
	}
	assert.Contains(t, saida, "[clientes] cnpj duplicado recusado (uq_clientes_cnpj)", "log operacional sem o valor (E-07)")
}

func getComToken2(t *testing.T, url, token string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, string(readBody(t, resp))
}
