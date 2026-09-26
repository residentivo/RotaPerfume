package handlers_test

// Integração HTTP ponta a ponta (router real + MySQL local) dos cards do
// Lote 5: SEC-04 (janela de graça do refresh) e NEG-02 (CNPJ alfanumérico).
// Mesma infra do Lote 4 (itCtx): só roda com INTEGRATION=1, os dados levam o
// marcador ZZ-TEST-HTTP-<sufixo> e são apagados no t.Cleanup.

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apisvc "github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/cnpj"
)

// ---------------------------------------------------------------------------
// SEC-04: janela de graça com o relógio real (DATETIME do MySQL + loc=Local)
// ---------------------------------------------------------------------------

// refreshIT faz um refresh e confere que um 401 não emite nenhum token.
func refreshIT(t *testing.T, c *itCtx, tok string) (int, string, string) {
	t.Helper()
	resp := postRefresh(t, c.srv.URL+"/api/auth/refresh", map[string]any{"refresh_token": tok}, "")
	defer resp.Body.Close()
	access, novo := refreshCookies(resp)
	raw := string(readBody(t, resp))
	if resp.StatusCode != http.StatusOK {
		assert.Empty(t, access, "%d não pode emitir access_token", resp.StatusCode)
		assert.Empty(t, novo, "%d não pode emitir refresh_token", resp.StatusCode)
	}
	return resp.StatusCode, raw, novo
}

func TestIntegracaoHTTP_SEC04_JanelaDeGraca(t *testing.T) {
	c := novoItCtx(t)
	usu := c.usuario("USEC04", 0, true)
	svc := apisvc.NewRefreshTokenService()
	novoToken := func(t *testing.T) string {
		tok, err := svc.GenerateRefreshToken(context.Background(), c.db, usu, "127.0.0.1", "go-test")
		require.NoError(t, err)
		return tok
	}
	revogarHa := func(t *testing.T, tok string, segundos int) {
		_, err := c.db.Exec(`UPDATE refresh_tokens SET revoked_at = NOW() - INTERVAL ? SECOND WHERE token_hash = ?`,
			segundos, hashRefresh(tok))
		require.NoError(t, err)
	}
	ativos := func() int {
		return c.contar("SELECT COUNT(*) FROM refresh_tokens WHERE usuario_id = ? AND revoked_at IS NULL", usu)
	}

	var corpoDentro string

	// A ordem importa: o limiter é do servidor desta execução e as falhas
	// acumulam. Os casos que não podem contar vêm antes.
	t.Run("aba atrasada: reuso sequencial do token recém-rotacionado (revoked_at gravado pela API) não conta", func(t *testing.T) {
		tok := novoToken(t)
		st, _, novo := refreshIT(t, c, tok)
		require.Equal(t, http.StatusOK, st)
		require.NotEmpty(t, novo)
		antes := ativos()
		for i := 0; i < 15; i++ {
			st, raw, _ := refreshIT(t, c, tok)
			require.Equal(t, http.StatusUnauthorized, st, "tentativa %d: %s", i+1, raw)
			corpoDentro = raw
		}
		assert.Equal(t, antes, ativos(), "nenhum token novo emitido nos 401")
		st, _, _ = refreshIT(t, c, novo)
		assert.Equal(t, http.StatusOK, st, "a aba vencedora segue com o token novo, sem 429")
	})

	t.Run("revogado há 5s pelo relógio do banco: 15 tentativas sem 429", func(t *testing.T) {
		tok := novoToken(t)
		revogarHa(t, tok, 5)
		for i := 0; i < 15; i++ {
			st, raw, _ := refreshIT(t, c, tok)
			require.Equal(t, http.StatusUnauthorized, st, "tentativa %d: %s", i+1, raw)
		}
	})

	t.Run("revogado há 25s (borda interna) não conta", func(t *testing.T) {
		tok := novoToken(t)
		revogarHa(t, tok, 25)
		for i := 0; i < 11; i++ {
			st, raw, _ := refreshIT(t, c, tok)
			require.Equal(t, http.StatusUnauthorized, st, "tentativa %d: %s", i+1, raw)
		}
	})

	t.Run("fora da janela (há 2min): mesma resposta, conta, e a 11ª recebe 429", func(t *testing.T) {
		tok := novoToken(t)
		revogarHa(t, tok, 120)
		for i := 0; i < 10; i++ {
			st, raw, _ := refreshIT(t, c, tok)
			require.Equal(t, http.StatusUnauthorized, st, "tentativa %d: %s", i+1, raw)
			assert.Equal(t, corpoDentro, raw, "sem oráculo: corpo idêntico ao da janela de graça")
		}
		st, _, _ := refreshIT(t, c, tok)
		assert.Equal(t, http.StatusTooManyRequests, st)

		// Com o IP bloqueado, nem um token válido passa (e nada é emitido).
		st, _, _ = refreshIT(t, c, novoToken(t))
		assert.Equal(t, http.StatusTooManyRequests, st)
	})
}

// ---------------------------------------------------------------------------
// NEG-02: CNPJ alfanumérico ponta a ponta
// ---------------------------------------------------------------------------

// cnpjAlfaTeste gera um CNPJ alfanumérico válido e único por execução.
func (c *itCtx) cnpjAlfa() string {
	const alfa = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	n := time.Now().UnixNano()/1000 + c.seq.Add(1)
	base := fmt.Sprintf("Z%c%c%09d", alfa[n%24], alfa[(n/24)%24], n%1e9)
	d1, d2 := cnpj.DigitosVerificadores(base)
	return fmt.Sprintf("%s%d%d", base, d1, d2)
}

func TestIntegracaoHTTP_NEG02_CNPJAlfanumerico(t *testing.T) {
	c := novoItCtx(t)
	doc := c.cnpjAlfa()
	require.True(t, cnpj.Valido(doc))
	mascarado := mascararCNPJ(doc)
	razao := c.nome("N02ALFA")

	p := c.payloadCliente(razao)
	p["cnpj"] = strings.ToLower(mascarado)
	st, body := c.req("POST", "/api/clientes", c.admin, p)
	require.Equal(t, http.StatusCreated, st, body)
	id := idDe(t, body, "cliente_id_origem")
	assert.Equal(t, doc, dataMap(t, body)["cnpj"], "retorno sem máscara e em maiúsculas")

	var gravado string
	require.NoError(t, c.db.QueryRow("SELECT BINARY cnpj FROM clientes WHERE cliente_id_origem = ?", id).Scan(&gravado))
	assert.Equal(t, doc, gravado, "banco grava em maiúsculas (collation _ci não pode mascarar isso)")

	buscar := func(t *testing.T, q string) []int64 {
		t.Helper()
		st, body := c.req("GET", "/api/clientes?limit=50&q="+url.QueryEscape(q), c.admin, nil)
		require.Equal(t, http.StatusOK, st, body)
		lista, ok := body["data"].([]any)
		require.True(t, ok, "data deve ser array: %v", body)
		out := []int64{}
		for _, it := range lista {
			out = append(out, int64(it.(map[string]any)["cliente_id_origem"].(float64)))
		}
		return out
	}

	t.Run("busca ?q=", func(t *testing.T) {
		for _, q := range []string{
			doc,                             // sem máscara
			strings.ToLower(doc),            // minúsculas (collation _ci)
			mascarado,                       // com máscara
			strings.ToLower(mascarado),      // máscara + minúsculas
			strings.ToLower(mascarado[:10]), // prefixo mascarado (digitando)
			doc[2:9],                        // trecho do meio sem máscara
		} {
			t.Run(q, func(t *testing.T) {
				assert.Contains(t, buscar(t, q), id)
			})
		}
	})

	t.Run("duplicado em outra caixa/máscara -> 409", func(t *testing.T) {
		for _, variante := range []string{doc, strings.ToLower(doc), mascarado} {
			r := c.nome("N02DUP")
			p := c.payloadCliente(r)
			p["cnpj"] = variante
			st, body := c.req("POST", "/api/clientes", c.admin, p)
			assert.Equal(t, http.StatusConflict, st, "%s: %v", variante, body)
			assert.Equal(t, "cnpj já cadastrado", body["error"])
			assert.Zero(t, c.contar("SELECT COUNT(*) FROM clientes WHERE razao_social = ?", r))
		}
	})

	t.Run("DV alfanumérico inválido / letra no DV -> 400", func(t *testing.T) {
		dvErrado := doc[:13] + fmt.Sprint((int(doc[13]-'0')+1)%10)
		for _, inv := range []string{dvErrado, doc[:12] + "A" + doc[13:], "12ABC34501DE36", "12ABC34501DE3#"} {
			r := c.nome("N02INV")
			p := c.payloadCliente(r)
			p["cnpj"] = inv
			st, body := c.req("POST", "/api/clientes", c.admin, p)
			assert.Equal(t, http.StatusBadRequest, st, "%s: %v", inv, body)
			assert.Equal(t, "cnpj inválido", body["error"])
		}
	})

	t.Run("PUT com o mesmo CNPJ em minúsculas -> 200 e continua maiúsculo", func(t *testing.T) {
		p := c.payloadCliente(c.nome("N02EDIT"))
		p["cnpj"] = strings.ToLower(doc)
		st, body := c.req("PUT", fmt.Sprintf("/api/clientes/%d", id), c.admin, p)
		require.Equal(t, http.StatusOK, st, body)
		assert.Equal(t, doc, dataMap(t, body)["cnpj"])
		require.NoError(t, c.db.QueryRow("SELECT BINARY cnpj FROM clientes WHERE cliente_id_origem = ?", id).Scan(&gravado))
		assert.Equal(t, doc, gravado)
	})
}
