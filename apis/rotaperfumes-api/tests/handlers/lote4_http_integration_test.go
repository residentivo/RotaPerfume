package handlers_test

// Integração HTTP ponta a ponta (router real + MySQL local) dos cards do
// Lote 4: SEC-02, SEC-03, BUG-06, NEG-01 e NEG-04. Reaproveita a infra de
// sec01_bug04_risco01_http_integration_test.go (itCtx): só roda com
// INTEGRATION=1, todo dado criado carrega o marcador ZZ-TEST-HTTP-<sufixo>
// e é apagado no t.Cleanup. Nenhum dado pré-existente é alterado.
//
// NEG-01 409: depende do índice UNIQUE uq_clientes_cnpj (migração
// sql/19_alter_clientes_cnpj_unique.sql). Sem o índice, o subteste é pulado
// com a indicação "pendente: migração 19".

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apisvc "github.com/rotaperfumes/rotaperfumes-api/services"
)

// ---------------------------------------------------------------------------
// SEC-02: corrida no refresh com o banco real (lock de linha do InnoDB)
// ---------------------------------------------------------------------------

func TestIntegracaoHTTP_SEC02_RefreshConcorrente(t *testing.T) {
	c := novoItCtx(t)
	usu := c.usuario("USEC02", 0, true)
	svc := apisvc.NewRefreshTokenService()
	ctx := context.Background()

	// SEC-04: 15 refreshes paralelos com o mesmo token (abas concorrentes)
	// resultam em exatamente 1×200 e o resto 401, sem nenhum 429 — a corrida
	// no BeginRotation e o token revogado dentro da janela de graça não contam
	// no refreshLimiter. Várias rodadas somam bem mais de 10 falhas do mesmo
	// IP; se contassem, as rodadas seguintes receberiam 429.
	const rodadas = 3
	const paralelos = 15
	for i := 0; i < rodadas; i++ {
		t.Run(fmt.Sprintf("rodada %d", i+1), func(t *testing.T) {
			tok, err := svc.GenerateRefreshToken(ctx, c.db, usu, "127.0.0.1", "go-test")
			require.NoError(t, err)
			ativosAntes := c.contar("SELECT COUNT(*) FROM refresh_tokens WHERE usuario_id = ? AND revoked_at IS NULL", usu)

			var wg sync.WaitGroup
			start := make(chan struct{})
			status := make([]int, paralelos)
			novos := make([]string, paralelos)
			for g := 0; g < paralelos; g++ {
				wg.Add(1)
				go func(g int) {
					defer wg.Done()
					<-start
					resp := postRefresh(t, c.srv.URL+"/api/auth/refresh", map[string]any{"refresh_token": tok}, "")
					defer resp.Body.Close()
					_, novos[g] = refreshCookies(resp)
					status[g] = resp.StatusCode
				}(g)
			}
			close(start)
			wg.Wait()

			got := map[int]int{}
			novoTok := ""
			for g, s := range status {
				got[s]++
				if s == http.StatusOK {
					novoTok = novos[g]
				} else {
					assert.Empty(t, novos[g], "401 não pode emitir refresh token")
				}
			}
			assert.Equal(t, map[int]int{http.StatusOK: 1, http.StatusUnauthorized: paralelos - 1}, got)
			assert.Zero(t, got[http.StatusTooManyRequests], "corrida entre abas não pode gerar 429")

			// O token antigo foi revogado e só UM novo foi emitido.
			var revogado bool
			require.NoError(t, c.db.QueryRow("SELECT revoked_at IS NOT NULL FROM refresh_tokens WHERE token_hash = ?",
				hashRefresh(tok)).Scan(&revogado))
			assert.True(t, revogado)
			ativosDepois := c.contar("SELECT COUNT(*) FROM refresh_tokens WHERE usuario_id = ? AND revoked_at IS NULL", usu)
			assert.Equal(t, ativosAntes, ativosDepois, "revoga 1 e emite 1: o total de ativos não muda")

			// A aba vencedora segue funcionando com o token novo.
			require.NotEmpty(t, novoTok, "o 200 deve trazer o novo refresh token em Set-Cookie")
			st, body := c.req("POST", "/api/auth/refresh", "", map[string]any{"refresh_token": novoTok})
			assert.Equal(t, http.StatusOK, st, "refresh com o token novo: %v", body)
		})
	}

	t.Run("reuso do token já rotacionado -> 401", func(t *testing.T) {
		tok, err := svc.GenerateRefreshToken(ctx, c.db, usu, "127.0.0.1", "go-test")
		require.NoError(t, err)
		st, _ := c.req("POST", "/api/auth/refresh", "", map[string]any{"refresh_token": tok})
		require.Equal(t, http.StatusOK, st)
		st, _ = c.req("POST", "/api/auth/refresh", "", map[string]any{"refresh_token": tok})
		assert.Equal(t, http.StatusUnauthorized, st)
	})
}

// ---------------------------------------------------------------------------
// SEC-03: escopo de GET /api/vendedores
// ---------------------------------------------------------------------------

func TestIntegracaoHTTP_SEC03_ListVendedores(t *testing.T) {
	c := novoItCtx(t)
	vendA := c.vendedor("S3A", "")
	vendD := c.vendedor("S3D", "2024-06-01")
	userA := c.usuario("S3UA", vendA, true)
	userSem := c.usuario("S3USEM", 0, true)
	userD := c.usuario("S3UD", vendD, true)

	ids := func(t *testing.T, body map[string]any) []int64 {
		t.Helper()
		lista, ok := body["data"].([]any)
		require.True(t, ok, "data deve ser array: %v", body)
		out := []int64{}
		for _, it := range lista {
			out = append(out, int64(it.(map[string]any)["id"].(float64)))
		}
		return out
	}

	t.Run("admin vê todos (ativos e desligados)", func(t *testing.T) {
		st, body := c.req("GET", "/api/vendedores", c.admin, nil)
		require.Equal(t, http.StatusOK, st, body)
		got := ids(t, body)
		assert.Len(t, got, c.contar("SELECT COUNT(*) FROM vendedores"))
		assert.Contains(t, got, vendA)
		assert.Contains(t, got, vendD)
	})

	t.Run("normal com vínculo vê só o próprio", func(t *testing.T) {
		st, body := c.req("GET", "/api/vendedores", c.tokenNormal(userA), nil)
		require.Equal(t, http.StatusOK, st, body)
		assert.Equal(t, []int64{vendA}, ids(t, body))
	})

	t.Run("normal sem vínculo recebe []", func(t *testing.T) {
		st, body := c.req("GET", "/api/vendedores", c.tokenNormal(userSem), nil)
		require.Equal(t, http.StatusOK, st, body)
		assert.Equal(t, []int64{}, ids(t, body))
	})

	t.Run("normal com vendedor desligado recebe 403", func(t *testing.T) {
		st, body := c.req("GET", "/api/vendedores", c.tokenNormal(userD), nil)
		assert.Equal(t, http.StatusForbidden, st)
		assert.Equal(t, "acesso bloqueado: vendedor desligado", body["error"])
	})
}

// ---------------------------------------------------------------------------
// BUG-06: PATCH .../inativar com body inválido
// ---------------------------------------------------------------------------

func TestIntegracaoHTTP_BUG06_InativarBody(t *testing.T) {
	c := novoItCtx(t)

	prodSKU := c.nome("SKU")
	res, err := c.db.Exec(`INSERT INTO produtos (sku, descricao, categoria, marca, preco_tabela, custo_unitario, unidade, ativo)
		VALUES (?, ?, 'Teste', 'Teste', 1, 1, 'UN', 1)`, prodSKU, c.nome("PROD"))
	require.NoError(t, err)
	prodID, _ := res.LastInsertId()

	recursos := []struct {
		nome   string
		path   string
		tabela string
		chave  string
		id     int64
	}{
		{"clientes", "/api/clientes/%d/inativar", "clientes", "cliente_id_origem", c.cliente("B06")},
		{"produtos", "/api/produtos/%d/inativar", "produtos", "id", prodID},
		{"usuarios", "/api/usuarios/%d/inativar", "usuarios", "id", c.usuario("B06U", 0, true)},
	}
	ativo := func(t *testing.T, tabela, chave string, id int64) bool {
		t.Helper()
		return c.contar(fmt.Sprintf("SELECT ativo FROM %s WHERE %s = ?", tabela, chave), id) == 1
	}

	invalidos := []string{`{"ativo":"false"}`, `{"ativo":1}`, `{"ativo":`, `[true]`, `{"ativo":true} lixo`, `"x"`}
	for _, r := range recursos {
		t.Run(r.nome, func(t *testing.T) {
			path := fmt.Sprintf(r.path, r.id)
			for _, b := range invalidos {
				t.Run("inválido "+b, func(t *testing.T) {
					antes := ativo(t, r.tabela, r.chave, r.id)
					st, body := c.req("PATCH", path, c.admin, b)
					assert.Equal(t, http.StatusBadRequest, st)
					assert.Equal(t, "body JSON inválido", body["error"])
					assert.Equal(t, antes, ativo(t, r.tabela, r.chave, r.id), "estado não pode mudar")
				})
			}
			for _, b := range []any{nil, "null", "{}", `{"ativo":null}`} {
				t.Run(fmt.Sprintf("toggle %v", b), func(t *testing.T) {
					antes := ativo(t, r.tabela, r.chave, r.id)
					st, body := c.req("PATCH", path, c.admin, b)
					require.Equal(t, http.StatusOK, st, body)
					assert.Equal(t, !antes, ativo(t, r.tabela, r.chave, r.id), "body vazio/null/{} alterna")
				})
			}
			for _, v := range []bool{false, false, true, true} {
				st, body := c.req("PATCH", path, c.admin, map[string]any{"ativo": v})
				require.Equal(t, http.StatusOK, st, body)
				assert.Equal(t, v, ativo(t, r.tabela, r.chave, r.id), "define explicitamente (idempotente)")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NEG-01: CNPJ
// ---------------------------------------------------------------------------

func mascararCNPJ(d string) string {
	return d[0:2] + "." + d[2:5] + "." + d[5:8] + "/" + d[8:12] + "-" + d[12:14]
}

func TestIntegracaoHTTP_NEG01_CNPJ(t *testing.T) {
	c := novoItCtx(t)
	vend := c.vendedor("N01V", "")
	userA := c.usuario("N01U", vend, true)
	tokA := c.tokenNormal(userA)

	payload := func(cnpj, razao string) map[string]any {
		p := c.payloadCliente(razao)
		p["cnpj"] = cnpj
		return p
	}

	t.Run("CNPJ inválido -> 400 sem insert", func(t *testing.T) {
		valido := c.cnpj()
		dvErrado := valido[:13] + fmt.Sprint((int(valido[13]-'0')+1)%10)
		for _, cnpj := range []string{"123", "1234567890123", "123456789012345", "11.222.333/0001-8X",
			"00000000000000", "11111111111111", dvErrado} {
			t.Run(cnpj, func(t *testing.T) {
				razao := c.nome("N01INV")
				st, body := c.req("POST", "/api/clientes", c.admin, payload(cnpj, razao))
				assert.Equal(t, http.StatusBadRequest, st)
				assert.Equal(t, "cnpj inválido", body["error"])
				assert.Zero(t, c.contar("SELECT COUNT(*) FROM clientes WHERE razao_social = ?", razao))
			})
		}
	})

	// NEG-04: o PUT abaixo reenvia o mesmo CNPJ, que tem DV VÁLIDO (c.cnpj());
	// por isso continua 200. O caso do CNPJ legado com DV inválido está em
	// TestIntegracaoHTTP_NEG04_CNPJLegadoDVInvalido.
	t.Run("CNPJ mascarado é normalizado (sem máscara, 14 posições em maiúsculas); Update com o mesmo CNPJ (DV válido) -> 200", func(t *testing.T) {
		digitos := c.cnpj()
		razao := c.nome("N01MASC")
		st, body := c.req("POST", "/api/clientes", tokA, payload(mascararCNPJ(digitos), razao))
		require.Equal(t, http.StatusCreated, st, body)
		id := idDe(t, body, "cliente_id_origem")
		assert.Equal(t, digitos, dataMap(t, body)["cnpj"])

		var gravado string
		require.NoError(t, c.db.QueryRow("SELECT cnpj FROM clientes WHERE cliente_id_origem = ?", id).Scan(&gravado))
		assert.Equal(t, digitos, gravado)

		for _, cnpj := range []string{digitos, mascararCNPJ(digitos)} {
			st, body = c.req("PUT", fmt.Sprintf("/api/clientes/%d", id), tokA, payload(cnpj, c.nome("N01EDIT")))
			assert.Equal(t, http.StatusOK, st, body)
		}
		st, body = c.req("PUT", fmt.Sprintf("/api/clientes/%d", id), tokA, payload("123", razao))
		assert.Equal(t, http.StatusBadRequest, st)
		assert.Equal(t, "cnpj inválido", body["error"])
	})

	t.Run("CNPJ duplicado -> 409 genérico sem vazar dono", func(t *testing.T) {
		if c.contar(`SELECT COUNT(*) FROM information_schema.statistics
			WHERE table_schema = DATABASE() AND table_name = 'clientes' AND index_name = 'uq_clientes_cnpj'`) == 0 {
			t.Skip("pendente: migração 19 (uq_clientes_cnpj) ainda não aplicada")
		}
		// Cliente existente pertence a OUTRO vendedor.
		outroVend := c.vendedor("N01OUTRO", "")
		existente := c.cliente("N01DONO")
		c.carteira(existente, outroVend, "")
		var cnpj, razaoDono, nomeVend string
		require.NoError(t, c.db.QueryRow("SELECT cnpj, razao_social FROM clientes WHERE cliente_id_origem = ?", existente).
			Scan(&cnpj, &razaoDono))
		require.NoError(t, c.db.QueryRow("SELECT nome FROM vendedores WHERE id = ?", outroVend).Scan(&nomeVend))

		for _, tc := range []struct{ nome, token string }{{"normal", tokA}, {"admin", c.admin}} {
			t.Run("create "+tc.nome, func(t *testing.T) {
				razao := c.nome("N01DUP")
				st, body := c.req("POST", "/api/clientes", tc.token, payload(mascararCNPJ(cnpj), razao))
				assert.Equal(t, http.StatusConflict, st)
				assert.Equal(t, "cnpj já cadastrado", body["error"])
				raw := fmt.Sprint(body)
				for _, vaz := range []string{razaoDono, nomeVend, fmt.Sprint(existente), fmt.Sprint(outroVend)} {
					assert.False(t, strings.Contains(raw, vaz), "409 vazou %q: %s", vaz, raw)
				}
				assert.Zero(t, c.contar("SELECT COUNT(*) FROM clientes WHERE razao_social = ?", razao))
			})
		}
		t.Run("update para CNPJ de outro cliente", func(t *testing.T) {
			outro := c.cliente("N01ALVO")
			st, body := c.req("PUT", fmt.Sprintf("/api/clientes/%d", outro), c.admin, payload(cnpj, c.nome("N01ALVO")))
			assert.Equal(t, http.StatusConflict, st)
			assert.Equal(t, "cnpj já cadastrado", body["error"])
		})
	})
}

// ---------------------------------------------------------------------------
// NEG-04: DV do CNPJ exigido em toda gravação
// ---------------------------------------------------------------------------

// TestIntegracaoHTTP_NEG04_CNPJLegadoDVInvalido: cliente legado (inserido por
// SQL, sem passar pela API) com CNPJ de 14 dígitos e DV inválido.
//   - PUT com o mesmo CNPJ (com e sem máscara) → 400 "cnpj inválido" e o
//     registro fica intacto (inclusive updated_at);
//   - PATCH /inativar não grava CNPJ → 200 e só `ativo` muda.
func TestIntegracaoHTTP_NEG04_CNPJLegadoDVInvalido(t *testing.T) {
	c := novoItCtx(t)

	valido := c.cnpj()
	legado := valido[:13] + fmt.Sprint((int(valido[13]-'0')+1)%10) // DV errado, único (deriva de c.cnpj)
	require.NotEqual(t, valido, legado)
	razao := c.nome("N04LEG")
	res, err := c.db.Exec(`INSERT INTO clientes (cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo)
		VALUES (?, ?, 'Teste', 'Curitiba', 'PR', 'Centro', '2024-01-01', 1)`, legado, razao)
	require.NoError(t, err)
	id, _ := res.LastInsertId()
	path := fmt.Sprintf("/api/clientes/%d", id)

	t.Run("PUT com o mesmo CNPJ legado -> 400 e banco inalterado", func(t *testing.T) {
		for _, cnpj := range []string{legado, mascararCNPJ(legado)} {
			t.Run(cnpj, func(t *testing.T) {
				antes := c.snapshotCliente(id)
				p := c.payloadCliente(c.nome("N04EDIT"))
				p["cnpj"] = cnpj
				st, body := c.req("PUT", path, c.admin, p)
				assert.Equal(t, http.StatusBadRequest, st, body)
				assert.Equal(t, "cnpj inválido", body["error"])
				assert.Equal(t, antes, c.snapshotCliente(id), "PUT recusado não pode alterar o cliente")
			})
		}
		var cnpjGravado, razaoGravada string
		require.NoError(t, c.db.QueryRow("SELECT cnpj, razao_social FROM clientes WHERE cliente_id_origem = ?", id).
			Scan(&cnpjGravado, &razaoGravada))
		assert.Equal(t, legado, cnpjGravado)
		assert.Equal(t, razao, razaoGravada)
	})

	t.Run("PATCH /inativar do cliente legado -> 200", func(t *testing.T) {
		st, body := c.req("PATCH", path+"/inativar", c.admin, map[string]any{"ativo": false})
		require.Equal(t, http.StatusOK, st, body)
		assert.Equal(t, false, dataMap(t, body)["ativo"])
		assert.Zero(t, c.contar("SELECT ativo FROM clientes WHERE cliente_id_origem = ?", id))

		st, body = c.req("PATCH", path+"/inativar", c.admin, nil) // toggle de volta
		require.Equal(t, http.StatusOK, st, body)
		assert.Equal(t, 1, c.contar("SELECT ativo FROM clientes WHERE cliente_id_origem = ?", id))

		var cnpjGravado string
		require.NoError(t, c.db.QueryRow("SELECT cnpj FROM clientes WHERE cliente_id_origem = ?", id).Scan(&cnpjGravado))
		assert.Equal(t, legado, cnpjGravado, "toggle não mexe no CNPJ")
	})
}
