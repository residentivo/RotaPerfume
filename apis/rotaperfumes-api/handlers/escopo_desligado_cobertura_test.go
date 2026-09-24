package handlers_test

// Cobertura complementar do bloqueio de vendedor desligado (contrato 🟣
// SecBrain), sem duplicar escopo_vendedor_desligado_test.go:
//
//   - 403 + mensagem exata nas rotas com escopo que ainda não tinham caso,
//     sem NENHUMA query de negócio (sqlmock é estrito: qualquer query não
//     esperada falha → 500, e ExpectationsWereMet confere as esperadas);
//   - admin nunca consulta IsDesligado (nem id_vendedor);
//   - usuário normal com vendedor ativo segue o fluxo normal;
//   - vínculo órfão (IsDesligado → ErrNotFound) ≡ usuário sem vendedor;
//   - desligamento/reativação refletidos na requisição seguinte (sem cache);
//   - /api/auth/me (todas as variações) e /api/auth/reset-password
//     acessíveis ao desligado.
//
// O Dashboard com desligado (200 zerado + vendedor_desligado=true) já é
// coberto por TestDashboardEscopo_VendedorDesligado_RespostaZerada.

import (
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/services"
)

const (
	desligUserID     = int64(2)
	desligVendedorID = int64(10)
)

// rotaEscopo descreve uma rota de negócio sujeita a resolverVendedorScope.
type rotaEscopo struct {
	metodo string
	path   string
	body   string
}

func (r rotaEscopo) nome() string { return r.metodo + " " + r.path }

// doReqEscopo executa a requisição autenticada e devolve status + body decodificado.
func doReqEscopo(t *testing.T, server *httptest.Server, token string, r rotaEscopo) (int, map[string]any) {
	t.Helper()
	var body io.Reader
	if r.body != "" {
		body = strings.NewReader(r.body)
	}
	req, err := http.NewRequest(r.metodo, server.URL+r.path, body)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)
	if r.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode, decodeResponse(t, readBody(t, resp))
}

// Bodies JSON válidos (formato) — garantem que o 403 não depende de falha
// de decode e que o body nem chega a ser processado.
const (
	bodyPedido       = `{"cliente_id":1,"vendedor_id":10,"data_pedido":"2024-03-10","canal":"App","status":"Faturado","itens":[{"produto_id":1,"quantidade":1,"preco_praticado":10,"desconto_pct":0}]}`
	bodyPagamento    = `{"pedido_id":1,"forma_pagamento":"Dinheiro","parcelas":1,"valor":10,"taxa_pct":0,"valor_liquido":10,"data_vencimento":"2024-03-10","status_pagamento":"Pago"}`
	bodyOportunidade = `{"cliente_id":1,"vendedor_id":10,"origem":"Indicação","data_abertura":"2024-03-10","etapa":"Prospecção","probabilidade_pct":10,"valor_estimado":100}`
	bodyVisita       = `{"cliente_id":1,"vendedor_id":10,"data_visita":"2024-03-10","resultado":"Pedido","duracao_min":30}`
)

// TestEscopoDesligado_403_DemaisRotas completa a matriz de
// TestEscopo_VendedorDesligado_Bloqueia403 com as rotas de pedidos,
// pagamentos, oportunidades e visitas que ainda não tinham caso.
func TestEscopoDesligado_403_DemaisRotas(t *testing.T) {
	rotas := []rotaEscopo{
		{"GET", "/api/pedidos/1", ""},
		{"POST", "/api/pedidos", bodyPedido},
		{"PUT", "/api/pedidos/1", bodyPedido},
		{"GET", "/api/pagamentos", ""},
		{"POST", "/api/pagamentos", bodyPagamento},
		{"PUT", "/api/pagamentos/1", bodyPagamento},
		{"DELETE", "/api/pagamentos/1", ""},
		{"GET", "/api/oportunidades", ""},
		{"GET", "/api/oportunidades/1", ""},
		{"POST", "/api/oportunidades", bodyOportunidade},
		{"DELETE", "/api/oportunidades/1", ""},
		{"GET", "/api/visitas", ""},
		{"GET", "/api/visitas/1", ""},
		{"PUT", "/api/visitas/1", bodyVisita},
		{"DELETE", "/api/visitas/1", ""},
		// Mesmo com body válido, POST/PUT de clientes bloqueiam antes do decode.
		{"POST", "/api/clientes", `{"cnpj":"1","razao_social":"X","segmento":"S","cidade":"C","uf":"PR"}`},
		{"PUT", "/api/clientes/1", `{"cnpj":"1","razao_social":"X","segmento":"S","cidade":"C","uf":"PR","data_cadastro":"2024-03-10"}`},
		{"PATCH", "/api/clientes/1/inativar", `{"ativo":false}`},
		// Filtros/paginação não alteram o bloqueio.
		{"GET", "/api/pedidos?vendedor_id=10&page=2&limit=5", ""},
		{"GET", "/api/clientes?q=abc", ""},
	}

	for _, r := range rotas {
		t.Run(r.nome(), func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			expectEscopoDesligado(mock, desligVendedorID)

			status, body := doReqEscopo(t, server, generateToken(t, testCfg(), desligUserID, "normal"), r)

			assert.Equal(t, http.StatusForbidden, status)
			assert.Equal(t, false, body["success"])
			assert.Equal(t, msgVendedorDesligadoH, body["error"])
			assert.Nil(t, body["data"])
			// Somente as 2 queries de escopo; nenhuma query de negócio.
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// rotasDetalhe são rotas GET de detalhe cuja primeira query de negócio é a
// busca do registro por id; com sql.ErrNoRows a resposta é 404.
var rotasDetalhe = []struct {
	path   string
	reNeg  string
	msg404 string
}{
	{"/api/pedidos/1", `FROM\s+pedidos`, "pedido não encontrado"},
	{"/api/pagamentos/1", `FROM\s+pagamentos`, "pagamento não encontrado"},
	{"/api/oportunidades/1", `FROM\s+oportunidades`, "oportunidade não encontrada"},
	{"/api/visitas/1", `FROM\s+visitas`, "visita não encontrada"},
	{"/api/clientes/1", `FROM\s+clientes`, "cliente não encontrado"},
}

// TestEscopoDesligado_Admin_SemConsultaDeEscopo garante que o admin vai
// direto para a query de negócio: nenhuma consulta a usuarios.id_vendedor
// nem a vendedores.data_desligamento (qualquer uma delas seria query
// inesperada → 500).
func TestEscopoDesligado_Admin_SemConsultaDeEscopo(t *testing.T) {
	for _, r := range rotasDetalhe {
		t.Run(r.path, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			mock.ExpectQuery(r.reNeg).WillReturnError(sql.ErrNoRows)

			status, body := doReqEscopo(t, server, generateToken(t, testCfg(), 1, "admin"), rotaEscopo{"GET", r.path, ""})

			assert.Equal(t, http.StatusNotFound, status)
			assert.Equal(t, r.msg404, body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestEscopoDesligado_NormalAtivo_Inalterado garante que o usuário normal
// com vendedor ATIVO passa pela checagem (id_vendedor + IsDesligado=false) e
// segue para a query de negócio — sem 403.
func TestEscopoDesligado_NormalAtivo_Inalterado(t *testing.T) {
	for _, r := range rotasDetalhe {
		t.Run(r.path, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			expectEscopoUsuarioH(mock, desligUserID, desligVendedorID)
			mock.ExpectQuery(r.reNeg).WillReturnError(sql.ErrNoRows)

			status, body := doReqEscopo(t, server, generateToken(t, testCfg(), desligUserID, "normal"), rotaEscopo{"GET", r.path, ""})

			assert.Equal(t, http.StatusNotFound, status)
			assert.Equal(t, r.msg404, body["error"])
			assert.NotEqual(t, msgVendedorDesligadoH, body["error"])
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestEscopoDesligado_VinculoOrfao_ComoSemVinculo garante que, quando
// id_vendedor aponta para vendedor inexistente (IsDesligado → ErrNotFound),
// cada rota responde EXATAMENTE como para id_vendedor NULL — e nunca 403 de
// desligado nem dados de terceiros. As duas variantes são executadas e
// comparadas entre si.
func TestEscopoDesligado_VinculoOrfao_ComoSemVinculo(t *testing.T) {
	casos := []struct {
		rota       rotaEscopo
		wantStatus int
		wantErr    string // "" = sucesso (lista vazia)
	}{
		{rotaEscopo{"GET", "/api/clientes", ""}, http.StatusOK, ""},
		{rotaEscopo{"GET", "/api/pedidos", ""}, http.StatusOK, ""},
		{rotaEscopo{"GET", "/api/pagamentos", ""}, http.StatusOK, ""},
		{rotaEscopo{"GET", "/api/oportunidades", ""}, http.StatusOK, ""},
		{rotaEscopo{"GET", "/api/visitas", ""}, http.StatusOK, ""},
		{rotaEscopo{"GET", "/api/clientes/1", ""}, http.StatusNotFound, "cliente não encontrado"},
		{rotaEscopo{"GET", "/api/pedidos/1", ""}, http.StatusNotFound, "pedido não encontrado"},
		{rotaEscopo{"GET", "/api/pagamentos/1", ""}, http.StatusNotFound, "pagamento não encontrado"},
		{rotaEscopo{"GET", "/api/oportunidades/1", ""}, http.StatusNotFound, "oportunidade não encontrada"},
		{rotaEscopo{"GET", "/api/visitas/1", ""}, http.StatusNotFound, "visita não encontrada"},
		{rotaEscopo{"DELETE", "/api/pedidos/1", ""}, http.StatusNotFound, "pedido não encontrado"},
		{rotaEscopo{"DELETE", "/api/visitas/1", ""}, http.StatusNotFound, "visita não encontrada"},
		{rotaEscopo{"PUT", "/api/oportunidades/1", bodyOportunidade}, http.StatusNotFound, "oportunidade não encontrada"},
		{rotaEscopo{"PUT", "/api/pagamentos/1", bodyPagamento}, http.StatusNotFound, "pagamento não encontrado"},
		{rotaEscopo{"POST", "/api/pedidos", bodyPedido}, http.StatusForbidden, "usuário sem vendedor vinculado"},
		{rotaEscopo{"POST", "/api/pagamentos", bodyPagamento}, http.StatusForbidden, "usuário sem vendedor vinculado"},
		{rotaEscopo{"POST", "/api/oportunidades", bodyOportunidade}, http.StatusForbidden, "usuário sem vendedor vinculado"},
		{rotaEscopo{"POST", "/api/visitas", bodyVisita}, http.StatusForbidden, "usuário sem vendedor vinculado"},
		{rotaEscopo{"GET", "/api/vendedores/10/clientes", ""}, http.StatusNotFound, "vendedor não encontrado"},
	}

	variantes := []struct {
		nome  string
		setup func(mock sqlmock.Sqlmock)
	}{
		{"orfao", func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery(reUsuarioVendedor).WithArgs(desligUserID).
				WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(desligVendedorID))
			mock.ExpectQuery(reVendedorDesligado).WithArgs(desligVendedorID).
				WillReturnRows(sqlmock.NewRows([]string{"desligado"})) // ErrNoRows → ErrNotFound
		}},
		{"id_vendedor_null", func(mock sqlmock.Sqlmock) {
			expectEscopoSemVendedorH(mock, desligUserID)
		}},
	}

	for _, c := range casos {
		t.Run(c.rota.nome(), func(t *testing.T) {
			var respostas []map[string]any
			for _, v := range variantes {
				t.Run(v.nome, func(t *testing.T) {
					server, db, mock := setupTestServer(t)
					defer server.Close()
					defer db.Close()
					v.setup(mock)

					status, body := doReqEscopo(t, server, generateToken(t, testCfg(), desligUserID, "normal"), c.rota)

					assert.Equal(t, c.wantStatus, status)
					if c.wantErr == "" {
						assert.Equal(t, true, body["success"])
						assert.Equal(t, []any{}, body["data"])
					} else {
						assert.Equal(t, c.wantErr, body["error"])
					}
					assert.NotEqual(t, msgVendedorDesligadoH, body["error"])
					assert.NoError(t, mock.ExpectationsWereMet(), "nenhuma query de negócio")
					respostas = append(respostas, body)
				})
			}
			if len(respostas) == 2 {
				assert.Equal(t, respostas[1], respostas[0], "órfão deve responder igual a id_vendedor NULL")
			}
		})
	}
}

// TestEscopoDesligado_SemCache_ReflecteNaProximaRequisicao usa o MESMO
// servidor (mesmos handlers) para duas requisições seguidas, mudando apenas
// o estado do vendedor no banco entre elas: a segunda requisição tem de
// refletir o novo estado (sem cache de escopo em memória/JWT).
func TestEscopoDesligado_SemCache_ReflecteNaProximaRequisicao(t *testing.T) {
	casos := []struct {
		nome    string
		estados []bool // desligado? por requisição
	}{
		{"reativacao: desligado -> ativo", []bool{true, false}},
		{"desligamento: ativo -> desligado", []bool{false, true}},
		{"alternancia: desligado -> ativo -> desligado", []bool{true, false, true}},
	}
	rota := rotaEscopo{"GET", "/api/pedidos/1", ""}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			token := generateToken(t, testCfg(), desligUserID, "normal")

			for i, desligado := range c.estados {
				mock.ExpectQuery(reUsuarioVendedor).WithArgs(desligUserID).
					WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(desligVendedorID))
				expectVendedorDesligado(mock, desligVendedorID, desligado)
				if !desligado {
					mock.ExpectQuery(`FROM\s+pedidos`).WillReturnError(sql.ErrNoRows)
				}

				status, body := doReqEscopo(t, server, token, rota)
				if desligado {
					assert.Equal(t, http.StatusForbidden, status, "req %d", i+1)
					assert.Equal(t, msgVendedorDesligadoH, body["error"], "req %d", i+1)
				} else {
					assert.Equal(t, http.StatusNotFound, status, "req %d", i+1)
					assert.Equal(t, "pedido não encontrado", body["error"], "req %d", i+1)
				}
				require.NoError(t, mock.ExpectationsWereMet(), "req %d deve consultar o banco de novo", i+1)
			}
		})
	}
}

// usuarioMeRow monta a linha de usuarios usada por GET /api/auth/me.
func usuarioMeRow(id int64, role string, idVendedor any) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
		AddRow(id, "Usuário", "u@test.com", "hash", role, idVendedor, true, false, now, now, &now, "Vendedor X")
}

const reUsuarioMe = `FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v`

// TestMe_VendedorDesligado_Variacoes complementa TestMe_VendedorDesligado:
// /me nunca é bloqueado e informa vendedor_desligado corretamente.
func TestMe_VendedorDesligado_Variacoes(t *testing.T) {
	casos := []struct {
		nome        string
		uid         int64
		role        string
		idVendedor  any
		setupDeslig func(mock sqlmock.Sqlmock)
		wantStatus  int
		wantDeslig  any
	}{
		{"normal ativo", 3, "normal", int64(10), func(m sqlmock.Sqlmock) { expectVendedorDesligado(m, 10, false) }, http.StatusOK, false},
		{"normal vinculo orfao", 3, "normal", int64(10), func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reVendedorDesligado).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"d"}))
		}, http.StatusOK, false},
		{"normal id_vendedor NULL (sem consulta de desligado)", 3, "normal", nil, nil, http.StatusOK, false},
		{"admin com id_vendedor (sem consulta de desligado)", 1, "admin", int64(10), nil, http.StatusOK, false},
		{"erro de banco na checagem → 500", 3, "normal", int64(10), func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reVendedorDesligado).WithArgs(int64(10)).WillReturnError(errors.New("db down"))
		}, http.StatusInternalServerError, nil},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			mock.ExpectQuery(reUsuarioMe).WithArgs(c.uid).WillReturnRows(usuarioMeRow(c.uid, c.role, c.idVendedor))
			if c.setupDeslig != nil {
				c.setupDeslig(mock)
			}

			status, body := doReqEscopo(t, server, generateToken(t, testCfg(), c.uid, c.role), rotaEscopo{"GET", "/api/auth/me", ""})

			assert.Equal(t, c.wantStatus, status)
			if c.wantStatus == http.StatusOK {
				data := body["data"].(map[string]any)
				assert.Equal(t, c.wantDeslig, data["vendedor_desligado"])
			} else {
				assert.Equal(t, "erro interno", body["error"])
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestResetPassword_VendedorDesligado_Acessivel garante que o usuário
// vinculado a vendedor desligado consegue trocar a própria senha: a rota não
// passa por resolverVendedorScope (nenhuma consulta a
// vendedores.data_desligamento — seria query inesperada).
func TestResetPassword_VendedorDesligado_Acessivel(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	cfg := testCfg()
	hash, _ := services.NewAuthService().HashPassword(cfg, "senha-atual")
	now := time.Now()
	mock.ExpectQuery(reUsuarioMe).
		WithArgs(desligUserID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(desligUserID, "Desligado", "d@test.com", hash, "normal", desligVendedorID, true, false, now, now, nil, "Vendedor X"))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(desligUserID).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
	mock.ExpectQuery(`FROM senha_historico sh`).
		WithArgs(desligUserID, 2, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "usuario_id", "resetado_por_id", "senha_hash_anterior", "ip_origem", "user_agent", "tipo_reset", "created_at", "usuario_nome", "resetado_por_nome"}))
	mock.ExpectExec(`INSERT INTO senha_historico`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), false, desligUserID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at`).WillReturnResult(sqlmock.NewResult(0, 0))

	status, body := doReqEscopo(t, server, generateToken(t, cfg, desligUserID, "normal"), rotaEscopo{
		"POST", "/api/auth/reset-password",
		`{"senha_atual":"senha-atual","nova_senha":"nova-senha-123","captchaToken":"token-valido-de-teste"}`,
	})

	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, body["success"])
	assert.NoError(t, mock.ExpectationsWereMet())
}
