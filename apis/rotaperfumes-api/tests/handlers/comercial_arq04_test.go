package handlers_test

// ARQ-04: contrato HTTP do Comercial byte a byte contra os models legados
// apagados (structs CONGELADAS abaixo, mesma ordem de campos e tags),
// serializados pelo envelope legado (map + Encoder); nil × vazio das listas
// (lição 2 do ARQ-03); montagem dos handlers e os ramos sem Principal.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/analyticshttp"
	"github.com/rotaperfumes/rotaperfumes-api/internal/comercialhttp"
	"github.com/rotaperfumes/rotaperfumes-api/internal/vendashttp"
)

// Cópias congeladas de shared/models (vendedor.go, cliente.go, visita.go,
// oportunidade.go) e de repositories.ClienteResumo/VendedorResumo.
type (
	legadoVendedor struct {
		ID               int64      `json:"id"`
		Nome             string     `json:"nome"`
		Regiao           string     `json:"regiao"`
		UF               string     `json:"uf"`
		DataAdmissao     time.Time  `json:"data_admissao"`
		DataDesligamento *time.Time `json:"data_desligamento"`
		MetaMensal       float64    `json:"meta_mensal"`
		CreatedAt        time.Time  `json:"created_at"`
		UpdatedAt        time.Time  `json:"updated_at"`
	}
	legadoClienteResumo struct {
		ID          int64      `json:"id"`
		CNPJ        string     `json:"cnpj"`
		RazaoSocial string     `json:"razao_social"`
		Segmento    string     `json:"segmento"`
		Cidade      string     `json:"cidade"`
		UF          string     `json:"uf"`
		CarteiraID  int64      `json:"carteira_id"`
		DataInicio  time.Time  `json:"data_inicio"`
		DataFim     *time.Time `json:"data_fim"`
	}
	legadoVendedorDetalhe struct {
		legadoVendedor
		Clientes []legadoClienteResumo `json:"clientes"`
	}
	legadoCliente struct {
		ClienteIDOrigem int64     `json:"cliente_id_origem"`
		CNPJ            string    `json:"cnpj"`
		RazaoSocial     string    `json:"razao_social"`
		Segmento        string    `json:"segmento"`
		Cidade          string    `json:"cidade"`
		UF              string    `json:"uf"`
		Bairro          string    `json:"bairro"`
		DataCadastro    time.Time `json:"data_cadastro"`
		Ativo           bool      `json:"ativo"`
		CreatedAt       time.Time `json:"created_at"`
		UpdatedAt       time.Time `json:"updated_at"`
	}
	legadoOportunidade struct {
		OportunidadeID   int64      `json:"oportunidade_id"`
		ClienteID        int64      `json:"cliente_id"`
		VendedorID       int64      `json:"vendedor_id"`
		Origem           string     `json:"origem"`
		DataAbertura     time.Time  `json:"data_abertura"`
		Etapa            string     `json:"etapa"`
		ProbabilidadePct float64    `json:"probabilidade_pct"`
		ValorEstimado    float64    `json:"valor_estimado"`
		DataFechamento   *time.Time `json:"data_fechamento"`
		CicloDias        *int       `json:"ciclo_dias"`
		MotivoPerda      *string    `json:"motivo_perda"`
		CreatedAt        time.Time  `json:"created_at"`
		UpdatedAt        time.Time  `json:"updated_at"`
	}
)

// envelopeLegado serializa como o writeJSON/writeJSONWithPagination legado.
func envelopeComercialLegado(t *testing.T, corpo map[string]any) string {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, json.NewEncoder(&b).Encode(corpo))
	return b.String()
}

func getComToken(t *testing.T, url string, uid int64, role string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), uid, role))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, string(readBody(t, resp))
}

func TestARQ04_ContratoJSON_ByteAByte(t *testing.T) {
	quando := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	ciclo, motivo := 12, "preço"

	t.Run("GET /api/clientes/{id}", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(`FROM clientes WHERE cliente_id_origem = \? LIMIT 1`).WithArgs(int64(5)).WillReturnRows(
			sqlmock.NewRows([]string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k"}).
				AddRow(int64(5), "11222333000181", "ACME", "varejo", "Curitiba", "PR", "", quando, true, quando, quando))
		status, body := getComToken(t, server.URL+"/api/clientes/5", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": legadoCliente{
			ClienteIDOrigem: 5, CNPJ: "11222333000181", RazaoSocial: "ACME", Segmento: "varejo", Cidade: "Curitiba", UF: "PR",
			DataCadastro: quando, Ativo: true, CreatedAt: quando, UpdatedAt: quando}}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/vendedores/{id} sem clientes: clientes []", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(`FROM vendedores WHERE id = \? LIMIT 1`).WithArgs(int64(3)).WillReturnRows(
			sqlmock.NewRows([]string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}).
				AddRow(int64(3), "Ana", "Sul", "PR", quando, nil, 10.5, quando, quando))
		mock.ExpectQuery(`FROM carteiras ca JOIN clientes c`).WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows([]string{"x"}))
		status, body := getComToken(t, server.URL+"/api/vendedores/3", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": legadoVendedorDetalhe{
			legadoVendedor: legadoVendedor{ID: 3, Nome: "Ana", Regiao: "Sul", UF: "PR", DataAdmissao: quando, MetaMensal: 10.5, CreatedAt: quando, UpdatedAt: quando},
			Clientes:       []legadoClienteResumo{}}}), body)
		assert.Contains(t, body, `"clientes":[]`)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/vendedores/{id}/clientes", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))
		mock.ExpectQuery(`FROM carteiras ca JOIN clientes c`).WithArgs(int64(3)).WillReturnRows(
			sqlmock.NewRows([]string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}).
				AddRow(int64(5), "11222333000181", "ACME", "varejo", "Curitiba", "PR", int64(9), quando, nil))
		status, body := getComToken(t, server.URL+"/api/vendedores/3/clientes", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": []legadoClienteResumo{{
			ID: 5, CNPJ: "11222333000181", RazaoSocial: "ACME", Segmento: "varejo", Cidade: "Curitiba", UF: "PR", CarteiraID: 9, DataInicio: quando}}}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/vendedores admin sem linhas: data null", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(`FROM vendedores\s+ORDER BY nome ASC`).WillReturnRows(sqlmock.NewRows([]string{"x"}))
		status, body := getComToken(t, server.URL+"/api/vendedores", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": []legadoVendedor(nil)}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/vendedores sem carteira: data []", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))
		status, body := getComToken(t, server.URL+"/api/vendedores", 7, "normal")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": []any{}}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/visitas sem carteira: data [] e paginação zerada, sem SQL de listagem", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(reUsuarioVendedor).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))
		status, body := getComToken(t, server.URL+"/api/visitas?page=2&limit=5", 7, "normal")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": []any{},
			"pagination": map[string]any{"page": 2, "limit": 5, "total": 0, "pages": 0}}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/oportunidades admin sem linhas: data null", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM oportunidades`).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(0))
		mock.ExpectQuery(`FROM oportunidades ORDER BY`).WillReturnRows(sqlmock.NewRows([]string{"x"}))
		status, body := getComToken(t, server.URL+"/api/oportunidades", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": nil,
			"pagination": map[string]any{"page": 1, "limit": 20, "total": 0, "pages": 0}}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("GET /api/oportunidades/{id}", func(t *testing.T) {
		server, db, mock := setupTestServer(t)
		defer server.Close()
		defer db.Close()
		mock.ExpectQuery(`FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).WithArgs(int64(8)).WillReturnRows(
			sqlmock.NewRows([]string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m"}).
				AddRow(int64(8), int64(5), int64(3), "indicação", quando, "Fechado perdido", 40.0, 99.9, quando, ciclo, motivo, quando, quando))
		status, body := getComToken(t, server.URL+"/api/oportunidades/8", 1, "admin")
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, envelopeComercialLegado(t, map[string]any{"success": true, "data": legadoOportunidade{
			OportunidadeID: 8, ClienteID: 5, VendedorID: 3, Origem: "indicação", DataAbertura: quando, Etapa: "Fechado perdido",
			ProbabilidadePct: 40, ValorEstimado: 99.9, DataFechamento: &quando, CicloDias: &ciclo, MotivoPerda: &motivo,
			CreatedAt: quando, UpdatedAt: quando}}), body)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestARQ04_ComercialHTTP_PanicSemCasos(t *testing.T) {
	assert.Panics(t, func() { comercialhttp.NovoVendedorHandler(nil) })
	assert.Panics(t, func() { comercialhttp.NovoClienteHandler(nil) })
	assert.Panics(t, func() { comercialhttp.NovoVisitaHandler(nil) })
	assert.Panics(t, func() { comercialhttp.NovoOportunidadeHandler(nil) })
}

// Gestão sem Principal (rota mal montada): 500 "erro interno" sem SQL.
// Rotas com escopo sem Principal: SemAcesso (TestEscopo_SemUsuarioNoContexto).
func TestARQ04_GestaoSemPrincipal_500(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	h := novoVendedorHandler(db, testCfg())

	for nome, chamar := range map[string]func(w http.ResponseWriter){
		"CreateVendedor": func(w http.ResponseWriter) {
			h.CreateVendedor(w, httptest.NewRequest(http.MethodPost, "/api/vendedores", nil))
		},
		"GetVendedor": func(w http.ResponseWriter) {
			r := httptest.NewRequest(http.MethodGet, "/api/vendedores/3", nil)
			r.SetPathValue("id", "3")
			h.GetVendedor(w, r)
		},
	} {
		rec := httptest.NewRecorder()
		chamar(rec)
		assert.Equal(t, http.StatusInternalServerError, rec.Code, nome)
		assert.Contains(t, rec.Body.String(), "erro interno", nome)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestARQ04_CreateVendedor_BodyInvalido_400(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/vendedores", strings.NewReader("{x"))
	req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, string(readBody(t, resp)), "body JSON inválido")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// N6: os handlers de Vendas (ARQ-05: vendashttp, exigem os casos de uso) e o
// de Analytics (ARQ-06: analyticshttp, exige as consultas, que exigem o
// escopo) dão panic na montagem.
func TestARQ04_N6_HandlersLegadosExigemEscopo(t *testing.T) {
	assert.Panics(t, func() { vendashttp.NovoPedidoHandler(nil) })
	assert.Panics(t, func() { vendashttp.NovoPagamentoHandler(nil) })
	assert.Panics(t, func() { analyticshttp.NovoDashboardHandler(nil) })
}
