package handlers_test

// Integração HTTP ponta a ponta (router real routes.NewMux + handlers reais +
// MySQL local) dos cards SEC-01, BUG-04 e RISCO-01.
//
// Só roda com INTEGRATION=1. Credenciais via DB_HOST/DB_PORT/DB_NAME/
// DB_USUARIO/DB_SENHA (defaults: localhost:3306/rotaperfumes, golang/golang).
//
// Dados: todo registro criado usa o marcador da execução
//   - nome/razao_social/sku/descricao: "ZZ-TEST-HTTP-<sufixo>-..."
//   - e-mail: "zz-test-http-<sufixo>-...@teste.local"
// e é apagado no t.Cleanup (limpezaIntegracao), em ordem compatível com as
// FKs. Nenhum dado pré-existente é alterado.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/rotaperfumes-api/routes"
	apisvc "github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
	shareddb "github.com/rotaperfumes/shared/db"
)

// ---------------------------------------------------------------------------
// Infra
// ---------------------------------------------------------------------------

type itCtx struct {
	t      *testing.T
	db     *sql.DB
	srv    *httptest.Server
	prefix string // ZZ-TEST-HTTP-<sufixo>-
	email  string // zz-test-http-<sufixo>-
	admin  string // token admin
	seq    atomic.Int64
}

func envOrIT(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// novoItCtx abre o banco, sobe o servidor real e registra a limpeza.
func novoItCtx(t *testing.T) *itCtx {
	t.Helper()
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("teste de integração: defina INTEGRATION=1 (requer MySQL local)")
	}
	dbCfg := &config.Config{
		DBHost:    envOrIT("DB_HOST", "localhost"),
		DBPort:    envOrIT("DB_PORT", "3306"),
		DBName:    envOrIT("DB_NAME", "rotaperfumes"),
		DBUsuario: envOrIT("DB_USUARIO", "golang"),
		DBSenha:   envOrIT("DB_SENHA", "golang"),
	}
	db, err := shareddb.Open(dbCfg.DSN())
	require.NoError(t, err, "MySQL local indisponível")
	t.Cleanup(func() { db.Close() })

	cfg := testCfg()
	authH := handlers.NewAuthHandler(db, cfg)
	authH.SetCaptchaVerifier(&fakeCaptchaVerifier{})
	mux := routes.NewMux(cfg, middleware.NewDBUserStatusChecker(db), authH,
		handlers.NewUsuarioHandler(db, cfg, &fakeEmailService{}),
		handlers.NewDashboardHandler(db, cfg),
		handlers.NewSenhaHistoricoHandler(db),
		handlers.NewVendedorHandler(db, cfg),
		handlers.NewClienteHandler(db, cfg),
		handlers.NewProdutoHandler(db, cfg),
		handlers.NewPedidoHandler(db, cfg),
		handlers.NewPagamentoHandler(db, cfg),
		handlers.NewOportunidadeHandler(db, cfg),
		handlers.NewVisitaHandler(db, cfg),
		handlers.NewEstoqueHandler(db, cfg),
	)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	suf := fmt.Sprintf("%d", time.Now().UnixNano()%1e10)
	c := &itCtx{
		t: t, db: db, srv: srv,
		prefix: "ZZ-TEST-HTTP-" + suf + "-",
		email:  "zz-test-http-" + suf + "-",
		admin:  generateToken(t, cfg, 1, "admin"),
	}
	t.Cleanup(c.limpar) // roda antes de db.Close (LIFO)
	return c
}

// limpar apaga tudo que carrega o marcador desta execução, respeitando FKs.
func (c *itCtx) limpar() {
	like := c.prefix + "%"
	likeEmail := c.email + "%"
	stmts := []struct {
		q    string
		args []any
	}{
		{`DELETE pg FROM pagamentos pg JOIN pedidos p ON p.pedido_id_origem = pg.pedido_id
		   JOIN vendedores v ON v.id = p.vendedor_id WHERE v.nome LIKE ?`, []any{like}},
		{`DELETE p FROM pedidos p JOIN vendedores v ON v.id = p.vendedor_id WHERE v.nome LIKE ?`, []any{like}},
		{`DELETE FROM oportunidades WHERE vendedor_id IN (SELECT id FROM vendedores WHERE nome LIKE ?)`, []any{like}},
		{`DELETE FROM visitas WHERE vendedor_id IN (SELECT id FROM vendedores WHERE nome LIKE ?)`, []any{like}},
		{`DELETE FROM clientes WHERE razao_social LIKE ?`, []any{like}}, // carteiras em cascata
		{`DELETE FROM carteiras WHERE vendedor_id IN (SELECT id FROM vendedores WHERE nome LIKE ?)`, []any{like}},
		{`DELETE FROM estoque WHERE sku LIKE ?`, []any{like}},
		{`DELETE FROM produtos WHERE sku LIKE ?`, []any{like}},
		{`DELETE FROM usuarios WHERE email LIKE ?`, []any{likeEmail}}, // refresh_tokens/senha_historico em cascata
		{`DELETE FROM vendedores WHERE nome LIKE ?`, []any{like}},
	}
	for _, s := range stmts {
		if _, err := c.db.Exec(s.q, s.args...); err != nil {
			c.t.Errorf("limpeza falhou (%s): %v", strings.Fields(s.q)[0:3], err)
		}
	}
	// Garantia: nada remanescente desta execução.
	var n int
	err := c.db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM clientes WHERE razao_social LIKE ?) +
		(SELECT COUNT(*) FROM vendedores WHERE nome LIKE ?) +
		(SELECT COUNT(*) FROM produtos WHERE sku LIKE ?) +
		(SELECT COUNT(*) FROM estoque WHERE sku LIKE ?) +
		(SELECT COUNT(*) FROM usuarios WHERE email LIKE ?)`, like, like, like, like, likeEmail).Scan(&n)
	if err != nil || n != 0 {
		c.t.Errorf("registros de teste remanescentes: n=%d err=%v", n, err)
	}
}

func (c *itCtx) nome(tag string) string {
	return fmt.Sprintf("%s%s-%d", c.prefix, tag, c.seq.Add(1))
}

func (c *itCtx) cnpj() string {
	return cnpjValidoTeste(9e11 + (time.Now().UnixNano()/1000+c.seq.Add(1))%1e11)
}

// req faz a chamada HTTP. body pode ser nil, string (cru) ou qualquer valor
// serializável em JSON.
func (c *itCtx) req(method, path, token string, body any) (int, map[string]any) {
	c.t.Helper()
	var rdr *bytes.Buffer
	switch b := body.(type) {
	case nil:
		rdr = &bytes.Buffer{}
	case string:
		rdr = bytes.NewBufferString(b)
	default:
		raw, err := json.Marshal(b)
		require.NoError(c.t, err)
		rdr = bytes.NewBuffer(raw)
	}
	r, err := http.NewRequest(method, c.srv.URL+path, rdr)
	require.NoError(c.t, err)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(r)
	require.NoError(c.t, err)
	defer resp.Body.Close()
	out := map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func dataMap(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	d, ok := body["data"].(map[string]any)
	require.True(t, ok, "resposta sem data: %v", body)
	return d
}

func idDe(t *testing.T, body map[string]any, campo string) int64 {
	t.Helper()
	v, ok := dataMap(t, body)[campo].(float64)
	require.True(t, ok, "campo %s ausente em %v", campo, body)
	return int64(v)
}

// --- fixtures diretas no banco (determinísticas, sem passar pela API) ---

func (c *itCtx) vendedor(tag string, desligadoEm string) int64 {
	c.t.Helper()
	var dd any
	if desligadoEm != "" {
		dd = desligadoEm
	}
	res, err := c.db.Exec(`INSERT INTO vendedores (nome, regiao, uf, data_admissao, data_desligamento, meta_mensal)
		VALUES (?, 'Teste', 'PR', '2024-01-01', ?, 1)`, c.nome(tag), dd)
	require.NoError(c.t, err)
	id, _ := res.LastInsertId()
	return id
}

func (c *itCtx) usuario(tag string, vendedorID int64, ativo bool) int64 {
	c.t.Helper()
	var v any
	if vendedorID > 0 {
		v = vendedorID
	}
	res, err := c.db.Exec(`INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo)
		VALUES (?, ?, 'x-hash-de-teste-nao-usado', 'normal', ?, ?)`,
		c.nome(tag), fmt.Sprintf("%s%s-%d@teste.local", c.email, strings.ToLower(tag), c.seq.Add(1)), v, ativo)
	require.NoError(c.t, err)
	id, _ := res.LastInsertId()
	return id
}

func (c *itCtx) cliente(tag string) int64 {
	c.t.Helper()
	res, err := c.db.Exec(`INSERT INTO clientes (cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo)
		VALUES (?, ?, 'Teste', 'Curitiba', 'PR', 'Centro', '2024-01-01', 1)`, c.cnpj(), c.nome(tag))
	require.NoError(c.t, err)
	id, _ := res.LastInsertId()
	return id
}

func (c *itCtx) carteira(clienteID, vendedorID int64, dataFim string) {
	c.t.Helper()
	var fim any
	if dataFim != "" {
		fim = dataFim
	}
	_, err := c.db.Exec(`INSERT INTO carteiras (cliente_id, vendedor_id, data_inicio, data_fim) VALUES (?, ?, '2024-01-01', ?)`,
		clienteID, vendedorID, fim)
	require.NoError(c.t, err)
}

func (c *itCtx) tokenNormal(uid int64) string {
	return generateToken(c.t, testCfg(), uid, "normal")
}

// snapshotCliente devolve o estado persistido do cliente + suas carteiras.
func (c *itCtx) snapshotCliente(id int64) string {
	c.t.Helper()
	var cli, cart sql.NullString
	_ = c.db.QueryRow(`SELECT CONCAT_WS('|', cnpj, razao_social, segmento, cidade, uf, IFNULL(bairro,''),
		data_cadastro, ativo, updated_at) FROM clientes WHERE cliente_id_origem = ?`, id).Scan(&cli)
	require.NoError(c.t, c.db.QueryRow(`SELECT IFNULL(GROUP_CONCAT(CONCAT_WS('|', carteira_id_origem, vendedor_id,
		data_inicio, IFNULL(data_fim,'-'), updated_at) ORDER BY carteira_id_origem SEPARATOR ';'), '')
		FROM carteiras WHERE cliente_id = ?`, id).Scan(&cart))
	return cli.String + "#" + cart.String
}

func (c *itCtx) payloadCliente(razao string) map[string]any {
	return map[string]any{
		"cnpj": c.cnpj(), "razao_social": razao, "segmento": "Teste",
		"cidade": "Londrina", "uf": "PR", "bairro": "Centro", "data_cadastro": "2024-05-05",
	}
}

func (c *itCtx) contar(q string, args ...any) int {
	c.t.Helper()
	var n int
	require.NoError(c.t, c.db.QueryRow(q, args...).Scan(&n))
	return n
}

// ---------------------------------------------------------------------------
// SEC-01
// ---------------------------------------------------------------------------

func TestIntegracaoHTTP_SEC01_EscritaDeClientes(t *testing.T) {
	c := novoItCtx(t)

	vendA := c.vendedor("VA", "")
	vendB := c.vendedor("VB", "")
	vendD := c.vendedor("VD", "2024-06-01")

	userA := c.usuario("UA", vendA, true)
	userSem := c.usuario("USEM", 0, true)
	userD := c.usuario("UD", vendD, true)

	cliProprio := c.cliente("PROPRIO")
	c.carteira(cliProprio, vendA, "")
	cliOutro := c.cliente("OUTRO")
	c.carteira(cliOutro, vendB, "")
	cliEncerrado := c.cliente("ENCERRADO")
	c.carteira(cliEncerrado, vendA, "2024-06-01")
	cliDeslig := c.cliente("DESLIG")
	c.carteira(cliDeslig, vendD, "")
	var maxID int64
	require.NoError(t, c.db.QueryRow("SELECT IFNULL(MAX(cliente_id_origem),0) FROM clientes").Scan(&maxID))
	inexistente := maxID + 1_000_000

	tokA, tokSem, tokD := c.tokenNormal(userA), c.tokenNormal(userSem), c.tokenNormal(userD)

	t.Run("normal sem acesso ao cliente -> 404 e banco inalterado", func(t *testing.T) {
		alvos := []struct {
			nome  string
			token string
			id    int64
		}{
			{"fora da carteira", tokA, cliOutro},
			{"inexistente", tokA, inexistente},
			{"vinculo encerrado (data_fim preenchida)", tokA, cliEncerrado},
			{"sem vendedor vinculado", tokSem, cliProprio},
		}
		rotas := []struct {
			nome, method, sufixo string
			body                 any
		}{
			{"PUT valido", "PUT", "", c.payloadCliente(c.prefix + "HACK")},
			{"PUT body invalido", "PUT", "", `{"razao_social":`},
			{"PUT com vendedor_id", "PUT", "", map[string]any{"razao_social": c.prefix + "HACK", "vendedor_id": vendA}},
			{"PATCH ativo=false", "PATCH", "/inativar", map[string]any{"ativo": false}},
			{"PATCH sem body (toggle)", "PATCH", "/inativar", nil},
			{"PATCH body invalido", "PATCH", "/inativar", `{"ativo":`},
		}
		for _, a := range alvos {
			for _, r := range rotas {
				t.Run(a.nome+"/"+r.nome, func(t *testing.T) {
					antes := c.snapshotCliente(a.id)
					status, body := c.req(r.method, fmt.Sprintf("/api/clientes/%d%s", a.id, r.sufixo), a.token, r.body)
					assert.Equal(t, http.StatusNotFound, status)
					assert.Equal(t, "cliente não encontrado", body["error"])
					assert.Equal(t, antes, c.snapshotCliente(a.id), "banco não pode mudar num 404")
				})
			}
		}
	})

	t.Run("vendedor desligado -> 403 sem escrita", func(t *testing.T) {
		for _, r := range []struct {
			method, path string
			body         any
		}{
			{"PUT", fmt.Sprintf("/api/clientes/%d", cliDeslig), c.payloadCliente(c.prefix + "HACK")},
			{"PATCH", fmt.Sprintf("/api/clientes/%d/inativar", cliDeslig), map[string]any{"ativo": false}},
		} {
			t.Run(r.method, func(t *testing.T) {
				antes := c.snapshotCliente(cliDeslig)
				status, body := c.req(r.method, r.path, tokD, r.body)
				assert.Equal(t, http.StatusForbidden, status)
				assert.Equal(t, msgVendedorDesligadoH, body["error"])
				assert.Equal(t, antes, c.snapshotCliente(cliDeslig))
			})
		}
		t.Run("POST", func(t *testing.T) {
			razao := c.nome("CRIADO-DESLIG")
			status, body := c.req("POST", "/api/clientes", tokD, c.payloadCliente(razao))
			assert.Equal(t, http.StatusForbidden, status)
			assert.Equal(t, msgVendedorDesligadoH, body["error"])
			assert.Zero(t, c.contar("SELECT COUNT(*) FROM clientes WHERE razao_social = ?", razao))
		})
	})

	t.Run("propria carteira -> 200", func(t *testing.T) {
		cartAntes := c.contar("SELECT COUNT(*) FROM carteiras WHERE cliente_id = ?", cliProprio)
		var cartSnap string
		require.NoError(t, c.db.QueryRow(`SELECT GROUP_CONCAT(CONCAT_WS('|', carteira_id_origem, vendedor_id, data_inicio, IFNULL(data_fim,'-')))
			FROM carteiras WHERE cliente_id = ?`, cliProprio).Scan(&cartSnap))

		novaRazao := c.nome("PROPRIO-EDITADO")
		payload := c.payloadCliente(novaRazao)
		payload["vendedor_id"] = vendB
		payload["carteira_id"] = 1
		status, body := c.req("PUT", fmt.Sprintf("/api/clientes/%d", cliProprio), tokA, payload)
		require.Equal(t, http.StatusOK, status, body)
		assert.Equal(t, novaRazao, dataMap(t, body)["razao_social"])

		var cartDepois string
		require.NoError(t, c.db.QueryRow(`SELECT GROUP_CONCAT(CONCAT_WS('|', carteira_id_origem, vendedor_id, data_inicio, IFNULL(data_fim,'-')))
			FROM carteiras WHERE cliente_id = ?`, cliProprio).Scan(&cartDepois))
		assert.Equal(t, cartSnap, cartDepois, "vendedor_id/carteira_id do body não podem mexer na carteira")
		assert.Equal(t, cartAntes, c.contar("SELECT COUNT(*) FROM carteiras WHERE cliente_id = ?", cliProprio))

		for _, ativo := range []bool{false, true} {
			status, body = c.req("PATCH", fmt.Sprintf("/api/clientes/%d/inativar", cliProprio), tokA, map[string]any{"ativo": ativo})
			require.Equal(t, http.StatusOK, status, body)
			assert.Equal(t, ativo, dataMap(t, body)["ativo"])
		}
	})

	t.Run("create normal com vendedor -> 201 + carteira + visivel em GET", func(t *testing.T) {
		razao := c.nome("CRIADO-NORMAL")
		status, body := c.req("POST", "/api/clientes", tokA, c.payloadCliente(razao))
		require.Equal(t, http.StatusCreated, status, body)
		id := idDe(t, body, "cliente_id_origem")

		var vend int64
		var inicio string
		var fim sql.NullString
		require.NoError(t, c.db.QueryRow(`SELECT vendedor_id, DATE_FORMAT(data_inicio,'%Y-%m-%d'), data_fim
			FROM carteiras WHERE cliente_id = ?`, id).Scan(&vend, &inicio, &fim))
		assert.Equal(t, vendA, vend)
		assert.Equal(t, time.Now().Format("2006-01-02"), inicio)
		assert.False(t, fim.Valid)
		assert.Equal(t, 1, c.contar("SELECT COUNT(*) FROM carteiras WHERE cliente_id = ?", id))

		status, body = c.req("GET", fmt.Sprintf("/api/clientes/%d", id), tokA, nil)
		require.Equal(t, http.StatusOK, status, body)
		assert.Equal(t, razao, dataMap(t, body)["razao_social"])

		status, body = c.req("GET", "/api/clientes?q="+razao, tokA, nil)
		require.Equal(t, http.StatusOK, status, body)
		lista, _ := body["data"].([]any)
		require.Len(t, lista, 1)
		assert.Equal(t, float64(id), lista[0].(map[string]any)["cliente_id_origem"])

		// Outro vendedor não enxerga; e o próprio consegue editar.
		tokB := c.tokenNormal(c.usuario("UB", vendB, true))
		status, _ = c.req("GET", fmt.Sprintf("/api/clientes/%d", id), tokB, nil)
		assert.Equal(t, http.StatusNotFound, status)
		status, _ = c.req("PUT", fmt.Sprintf("/api/clientes/%d", id), tokA, c.payloadCliente(razao))
		assert.Equal(t, http.StatusOK, status)
	})

	t.Run("create normal sem vendedor -> 403 sem insert", func(t *testing.T) {
		razao := c.nome("CRIADO-SEMVEND")
		status, body := c.req("POST", "/api/clientes", tokSem, c.payloadCliente(razao))
		assert.Equal(t, http.StatusForbidden, status)
		assert.Equal(t, "usuário sem vendedor vinculado", body["error"])
		assert.Zero(t, c.contar("SELECT COUNT(*) FROM clientes WHERE razao_social = ?", razao))
	})

	t.Run("admin: create sem carteira, update/toggle de qualquer cliente", func(t *testing.T) {
		razao := c.nome("CRIADO-ADMIN")
		status, body := c.req("POST", "/api/clientes", c.admin, c.payloadCliente(razao))
		require.Equal(t, http.StatusCreated, status, body)
		id := idDe(t, body, "cliente_id_origem")
		assert.Zero(t, c.contar("SELECT COUNT(*) FROM carteiras WHERE cliente_id = ?", id), "admin não gera carteira")

		for _, alvo := range []int64{cliOutro, cliEncerrado, cliDeslig, id} {
			nova := c.nome("ADMIN-EDIT")
			status, body = c.req("PUT", fmt.Sprintf("/api/clientes/%d", alvo), c.admin, c.payloadCliente(nova))
			require.Equal(t, http.StatusOK, status, body)
			assert.Equal(t, nova, dataMap(t, body)["razao_social"])
			for _, ativo := range []bool{false, true} {
				status, body = c.req("PATCH", fmt.Sprintf("/api/clientes/%d/inativar", alvo), c.admin, map[string]any{"ativo": ativo})
				require.Equal(t, http.StatusOK, status, body)
			}
		}
		status, _ = c.req("PUT", fmt.Sprintf("/api/clientes/%d", inexistente), c.admin, c.payloadCliente(c.nome("X")))
		assert.Equal(t, http.StatusNotFound, status)
	})
}

// ---------------------------------------------------------------------------
// BUG-04
// ---------------------------------------------------------------------------

func TestIntegracaoHTTP_BUG04_PutSemAlteracao(t *testing.T) {
	c := novoItCtx(t)
	adm := c.admin

	// criar via API e devolver o id.
	criar := func(t *testing.T, path, campoID string, body map[string]any) int64 {
		t.Helper()
		status, resp := c.req("POST", path, adm, body)
		require.Equal(t, http.StatusCreated, status, "POST %s: %v", path, resp)
		return idDe(t, resp, campoID)
	}
	// PUT idêntico duas vezes (a 1ª pode já ser no-op; a 2ª certamente é).
	putIgual := func(t *testing.T, path string, body map[string]any) {
		t.Helper()
		for i := 1; i <= 2; i++ {
			status, resp := c.req("PUT", path, adm, body)
			assert.Equal(t, http.StatusOK, status, "PUT sem alteração #%d em %s: %v", i, path, resp)
		}
	}

	vendBody := map[string]any{"nome": c.nome("VEND"), "regiao": "Teste", "uf": "PR", "data_admissao": "2024-01-01", "meta_mensal": 10}
	vendID := criar(t, "/api/vendedores", "id", vendBody)

	cliBody := c.payloadCliente(c.nome("CLI"))
	cliID := criar(t, "/api/clientes", "cliente_id_origem", cliBody)

	sku := c.nome("SKU")
	prodBody := map[string]any{
		"sku": sku, "descricao": c.nome("PROD"), "categoria": "Teste", "marca": "Teste", "nota_olfativa": "Floral",
		"preco_tabela": 10.5, "custo_unitario": 5.25, "unidade": "UN", "data_lancamento": "2024-01-01",
	}
	prodID := criar(t, "/api/produtos", "id", prodBody)

	pedBody := map[string]any{
		"cliente_id": cliID, "vendedor_id": vendID, "data_pedido": "2024-03-10", "canal": "App", "status": "Em separação",
		"itens": []map[string]any{{"produto_id": prodID, "quantidade": 2, "preco_praticado": 10.5, "desconto_pct": 0}},
	}
	pedID := criar(t, "/api/pedidos", "pedido_id_origem", pedBody)

	pagBody := map[string]any{
		"pedido_id": pedID, "forma_pagamento": "PIX", "parcelas": 1, "valor": 21, "taxa_pct": 0, "valor_liquido": 21,
		"data_vencimento": "2024-03-20", "status_pagamento": "Em aberto",
	}
	pagID := criar(t, "/api/pagamentos", "pagamento_id", pagBody)

	ciclo := 10
	opBody := map[string]any{
		"cliente_id": cliID, "vendedor_id": vendID, "origem": "Indicação", "data_abertura": "2024-03-01",
		"etapa": "Prospecção", "probabilidade_pct": 25, "valor_estimado": 1000, "ciclo_dias": ciclo,
	}
	opID := criar(t, "/api/oportunidades", "oportunidade_id", opBody)

	visBody := map[string]any{"cliente_id": cliID, "vendedor_id": vendID, "data_visita": "2024-03-05", "resultado": "Pedido", "duracao_min": 30}
	visID := criar(t, "/api/visitas", "visita_id", visBody)

	estBody := map[string]any{"sku": sku, "data_snapshot": "2024-03-01", "saldo": 7}
	estID := criar(t, "/api/estoque", "id", estBody)

	usuBody := map[string]any{"nome": c.nome("USU"), "email": c.email + "usu@teste.local", "role": "normal", "id_vendedor": vendID}
	usuID := criar(t, "/api/usuarios", "id", usuBody)

	t.Run("PUT sem alteracao -> 200", func(t *testing.T) {
		casos := []struct {
			nome, path string
			body       map[string]any
		}{
			{"cliente", fmt.Sprintf("/api/clientes/%d", cliID), cliBody},
			{"vendedor", fmt.Sprintf("/api/vendedores/%d", vendID), vendBody},
			{"produto", fmt.Sprintf("/api/produtos/%d", prodID), prodBody},
			{"pedido", fmt.Sprintf("/api/pedidos/%d", pedID), pedBody},
			{"pagamento", fmt.Sprintf("/api/pagamentos/%d", pagID), pagBody},
			{"oportunidade", fmt.Sprintf("/api/oportunidades/%d", opID), opBody},
			{"visita", fmt.Sprintf("/api/visitas/%d", visID), visBody},
			{"estoque", fmt.Sprintf("/api/estoque/%d", estID), map[string]any{"saldo": 7}},
			{"usuario", fmt.Sprintf("/api/usuarios/%d", usuID), map[string]any{"nome": usuBody["nome"], "role": "normal", "id_vendedor": vendID}},
		}
		for _, tc := range casos {
			t.Run(tc.nome, func(t *testing.T) { putIgual(t, tc.path, tc.body) })
		}
	})

	t.Run("PATCH inativar com o mesmo valor -> 200", func(t *testing.T) {
		for _, tc := range []struct{ nome, path string }{
			{"cliente", fmt.Sprintf("/api/clientes/%d/inativar", cliID)},
			{"produto", fmt.Sprintf("/api/produtos/%d/inativar", prodID)},
			{"usuario", fmt.Sprintf("/api/usuarios/%d/inativar", usuID)},
		} {
			t.Run(tc.nome, func(t *testing.T) {
				for _, ativo := range []bool{true, true, false, false, true} {
					status, body := c.req("PATCH", tc.path, adm, map[string]any{"ativo": ativo})
					require.Equal(t, http.StatusOK, status, "ativo=%t: %v", ativo, body)
					assert.Equal(t, ativo, dataMap(t, body)["ativo"])
				}
			})
		}
	})

	t.Run("id inexistente continua 404", func(t *testing.T) {
		const inex = int64(9_000_000_000_000)
		casos := []struct {
			nome, method, path string
			body               any
		}{
			{"PUT cliente", "PUT", fmt.Sprintf("/api/clientes/%d", inex), cliBody},
			{"PUT vendedor", "PUT", fmt.Sprintf("/api/vendedores/%d", inex), vendBody},
			{"PUT produto", "PUT", fmt.Sprintf("/api/produtos/%d", inex), prodBody},
			{"PUT pedido", "PUT", fmt.Sprintf("/api/pedidos/%d", inex), pedBody},
			{"PUT pagamento", "PUT", fmt.Sprintf("/api/pagamentos/%d", inex), pagBody},
			{"PUT oportunidade", "PUT", fmt.Sprintf("/api/oportunidades/%d", inex), opBody},
			{"PUT visita", "PUT", fmt.Sprintf("/api/visitas/%d", inex), visBody},
			{"PUT estoque", "PUT", fmt.Sprintf("/api/estoque/%d", inex), map[string]any{"saldo": 7}},
			{"PUT usuario", "PUT", fmt.Sprintf("/api/usuarios/%d", inex), map[string]any{"nome": "x", "role": "normal"}},
			{"PATCH cliente", "PATCH", fmt.Sprintf("/api/clientes/%d/inativar", inex), map[string]any{"ativo": true}},
			{"PATCH produto", "PATCH", fmt.Sprintf("/api/produtos/%d/inativar", inex), map[string]any{"ativo": true}},
			{"PATCH usuario", "PATCH", fmt.Sprintf("/api/usuarios/%d/inativar", inex), map[string]any{"ativo": true}},
			{"DELETE vendedor", "DELETE", fmt.Sprintf("/api/vendedores/%d", inex), nil},
			{"DELETE pedido", "DELETE", fmt.Sprintf("/api/pedidos/%d", inex), nil},
			{"DELETE pagamento", "DELETE", fmt.Sprintf("/api/pagamentos/%d", inex), nil},
			{"DELETE oportunidade", "DELETE", fmt.Sprintf("/api/oportunidades/%d", inex), nil},
			{"DELETE visita", "DELETE", fmt.Sprintf("/api/visitas/%d", inex), nil},
			{"POST reativar vendedor", "POST", fmt.Sprintf("/api/vendedores/%d/reativar", inex), nil},
		}
		for _, tc := range casos {
			t.Run(tc.nome, func(t *testing.T) {
				status, body := c.req(tc.method, tc.path, adm, tc.body)
				assert.Equal(t, http.StatusNotFound, status, "%v", body)
			})
		}
	})

	t.Run("DELETE existentes -> 204 e segundo DELETE -> 404", func(t *testing.T) {
		for _, tc := range []struct{ nome, path string }{
			{"pagamento", fmt.Sprintf("/api/pagamentos/%d", pagID)},
			{"pedido", fmt.Sprintf("/api/pedidos/%d", pedID)},
			{"oportunidade", fmt.Sprintf("/api/oportunidades/%d", opID)},
			{"visita", fmt.Sprintf("/api/visitas/%d", visID)},
		} {
			t.Run(tc.nome, func(t *testing.T) {
				status, body := c.req("DELETE", tc.path, adm, nil)
				require.Equal(t, http.StatusNoContent, status, "%v", body)
				status, _ = c.req("DELETE", tc.path, adm, nil)
				assert.Equal(t, http.StatusNotFound, status)
			})
		}
	})
}

// DELETE vendedor 2x no mesmo dia, com o usuário reativado entre as chamadas:
// ambas 200, o usuário volta a ativo=0 e a data de desligamento é preservada.
func TestIntegracaoHTTP_BUG04_DeleteVendedorDuasVezes(t *testing.T) {
	c := novoItCtx(t)
	vend := c.vendedor("VDEL", "")
	usu := c.usuario("UDEL", vend, true)
	controle := c.usuario("UCTRL", c.vendedor("VCTRL", ""), true)
	path := fmt.Sprintf("/api/vendedores/%d", vend)

	ativo := func(id int64) int {
		return c.contar("SELECT ativo FROM usuarios WHERE id = ?", id)
	}
	data := func() string {
		var s sql.NullString
		require.NoError(t, c.db.QueryRow("SELECT DATE_FORMAT(data_desligamento,'%Y-%m-%d') FROM vendedores WHERE id = ?", vend).Scan(&s))
		return s.String
	}

	status, body := c.req("DELETE", path, c.admin, nil)
	require.Equal(t, http.StatusOK, status, "%v", body)
	hoje := time.Now().Format("2006-01-02")
	require.Equal(t, hoje, data())
	assert.Equal(t, 0, ativo(usu))
	assert.True(t, strings.HasPrefix(dataMap(t, body)["data_desligamento"].(string), hoje))

	status, body = c.req("PATCH", fmt.Sprintf("/api/usuarios/%d/inativar", usu), c.admin, map[string]any{"ativo": true})
	require.Equal(t, http.StatusOK, status, "%v", body)
	require.Equal(t, 1, ativo(usu))

	status, body = c.req("DELETE", path, c.admin, nil)
	assert.Equal(t, http.StatusOK, status, "%v", body)
	assert.Equal(t, 0, ativo(usu), "segundo DELETE deve inativar de novo o usuário reativado")
	assert.Equal(t, hoje, data(), "data original preservada")
	assert.Equal(t, 1, ativo(controle), "usuário de outro vendedor intocado")

	// Com data retroativa: novo DELETE preserva a data antiga (COALESCE).
	_, err := c.db.Exec("UPDATE vendedores SET data_desligamento = '2020-05-05' WHERE id = ?", vend)
	require.NoError(t, err)
	status, _ = c.req("DELETE", path, c.admin, nil)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "2020-05-05", data())
}

// ---------------------------------------------------------------------------
// RISCO-01
// ---------------------------------------------------------------------------

func TestIntegracaoHTTP_RISCO01_TimeLocalFixo(t *testing.T) {
	c := novoItCtx(t)
	_, off := time.Now().Zone()
	assert.Equal(t, -10800, off, "time.Local deve ser -03 (import de config -> tz)")

	var sessTZ string
	require.NoError(t, c.db.QueryRow("SELECT @@session.time_zone").Scan(&sessTZ))
	t.Logf("MySQL session time_zone=%s", sessTZ)
}

// 2018-11-04 (antigo início do horário de verão em America/Sao_Paulo) grava
// e lê igual, via API, e re-salvar não desloca.
func TestIntegracaoHTTP_RISCO01_Data20181104(t *testing.T) {
	c := novoItCtx(t)
	const d = "2018-11-04"

	casos := []struct {
		nome, post, campoID, tabela, colID, colData, campoData string
		body                                                   func() map[string]any
		put                                                    func(body map[string]any) map[string]any
	}{
		{
			nome: "vendedor.data_admissao", post: "/api/vendedores", campoID: "id",
			tabela: "vendedores", colID: "id", colData: "data_admissao", campoData: "data_admissao",
			body: func() map[string]any {
				return map[string]any{"nome": c.nome("V1811"), "regiao": "Teste", "uf": "PR", "data_admissao": d, "meta_mensal": 1}
			},
			put: func(b map[string]any) map[string]any { return b },
		},
		{
			nome: "cliente.data_cadastro", post: "/api/clientes", campoID: "cliente_id_origem",
			tabela: "clientes", colID: "cliente_id_origem", colData: "data_cadastro", campoData: "data_cadastro",
			body: func() map[string]any {
				b := c.payloadCliente(c.nome("C1811"))
				b["data_cadastro"] = d
				return b
			},
			put: func(b map[string]any) map[string]any { return b },
		},
		{
			nome: "produto.data_lancamento", post: "/api/produtos", campoID: "id",
			tabela: "produtos", colID: "id", colData: "data_lancamento", campoData: "data_lancamento",
			body: func() map[string]any {
				return map[string]any{
					"sku": c.nome("P1811"), "descricao": c.nome("P1811"), "categoria": "Teste", "marca": "Teste",
					"preco_tabela": 1, "custo_unitario": 1, "unidade": "UN", "data_lancamento": d,
				}
			},
			put: func(b map[string]any) map[string]any { return b },
		},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			body := tc.body()
			status, resp := c.req("POST", tc.post, c.admin, body)
			require.Equal(t, http.StatusCreated, status, "%v", resp)
			id := idDe(t, resp, tc.campoID)
			path := fmt.Sprintf("%s/%d", tc.post, id)

			noBanco := func() string {
				var s string
				q := fmt.Sprintf("SELECT DATE_FORMAT(%s,'%%Y-%%m-%%d') FROM %s WHERE %s = ?", tc.colData, tc.tabela, tc.colID)
				require.NoError(t, c.db.QueryRow(q, id).Scan(&s))
				return s
			}
			assert.Equal(t, d, noBanco(), "gravação")

			for ciclo := 1; ciclo <= 3; ciclo++ {
				status, resp = c.req("GET", path, c.admin, nil)
				require.Equal(t, http.StatusOK, status, "%v", resp)
				lido, _ := dataMap(t, resp)[tc.campoData].(string)
				require.GreaterOrEqual(t, len(lido), 10, "%v", resp)
				assert.Equal(t, d, lido[:10], "leitura ciclo %d", ciclo)
				assert.True(t, strings.HasSuffix(lido, "-03:00"), "JSON deve sair em -03:00: %s", lido)

				body[tc.campoData] = lido[:10]
				status, resp = c.req("PUT", path, c.admin, tc.put(body))
				require.Equal(t, http.StatusOK, status, "%v", resp)
				assert.Equal(t, d, noBanco(), "re-save ciclo %d", ciclo)
			}
		})
	}
}

// Regressão do refresh token após fixar o fuso: expires_at é DATETIME
// gravado/lido com loc=Local. Um deslocamento de fuso faria tokens recém
// expirados parecerem válidos (ou o contrário).
func TestIntegracaoHTTP_RISCO01_RefreshToken(t *testing.T) {
	c := novoItCtx(t)
	usu := c.usuario("UREFRESH", 0, true)
	svc := apisvc.NewRefreshTokenService()
	ctx := context.Background()

	novoToken := func(t *testing.T, expira time.Time) string {
		t.Helper()
		tok, err := svc.GenerateRefreshToken(ctx, c.db, usu, "127.0.0.1", "go-test")
		require.NoError(t, err)
		_, err = c.db.Exec("UPDATE refresh_tokens SET expires_at = ? WHERE token_hash = ?", expira, hashRefresh(tok))
		require.NoError(t, err)
		return tok
	}

	t.Run("expires_at gravado no fuso -03", func(t *testing.T) {
		antes := time.Now()
		tok, err := svc.GenerateRefreshToken(ctx, c.db, usu, "127.0.0.1", "go-test")
		require.NoError(t, err)
		var gravado string
		require.NoError(t, c.db.QueryRow("SELECT DATE_FORMAT(expires_at,'%Y-%m-%d %H:%i') FROM refresh_tokens WHERE token_hash = ?",
			hashRefresh(tok)).Scan(&gravado))
		esperado := antes.Add(apisvc.RefreshTokenTTL).In(time.FixedZone("-03", -10800))
		depois := time.Now().Add(apisvc.RefreshTokenTTL).In(time.FixedZone("-03", -10800))
		assert.Contains(t, []string{esperado.Format("2006-01-02 15:04"), depois.Format("2006-01-02 15:04")}, gravado)
	})

	casos := []struct {
		nome   string
		expira time.Duration
		status int
		msg    string
	}{
		{"expirado ha 1 minuto -> 401", -time.Minute, http.StatusUnauthorized, "refresh token expirado"},
		{"expirado ha 2 horas -> 401", -2 * time.Hour, http.StatusUnauthorized, "refresh token expirado"},
		{"expirado ha 4 horas -> 401", -4 * time.Hour, http.StatusUnauthorized, "refresh token expirado"},
		{"valido por mais 2 minutos -> 200", 2 * time.Minute, http.StatusOK, ""},
		{"valido por mais 7 dias -> 200", apisvc.RefreshTokenTTL, http.StatusOK, ""},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			tok := novoToken(t, time.Now().Add(tc.expira))
			status, body := c.req("POST", "/api/auth/refresh", "", map[string]any{"refresh_token": tok})
			assert.Equal(t, tc.status, status, "%v", body)
			if tc.msg != "" {
				assert.Equal(t, tc.msg, body["error"])
			}
		})
	}
}
