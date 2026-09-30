package handlers_test

// Integração HTTP ponta a ponta (router real + MySQL local) do SEC-12. Mesma
// infra do Lote 4/6 (itCtx): só roda com INTEGRATION=1 (e DB_USUARIO/
// DB_SENHA), os dados levam o marcador ZZ-TEST-HTTP-<sufixo> e são apagados
// no t.Cleanup (refresh_tokens em cascata com o usuário).
//
// Cenário: login → rotação → o refresh antigo sai da janela de graça e é
// reusado por duas requisições concorrentes → exatamente um corte (uma
// revogação em massa e um tokens_validos_desde); a vítima faz login de novo
// e sobrevive a novos replays dentro da janela (30 min); passada a janela
// (simulada recuando reuso_detectado_em), o replay corta de novo.
//
// Rate limit: o teste soma 5 falhas contadas no mesmo IP (limite 10).

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// refreshCru faz o POST /refresh sem helpers do testing (seguro em
// goroutine) e devolve status, corpo e se veio algum Set-Cookie.
func refreshCru(url, tok string) (int, string, bool, error) {
	resp, err := http.Post(url, "application/json", bytes.NewBufferString(`{"refresh_token":"`+tok+`"}`))
	if err != nil {
		return 0, "", false, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), len(resp.Header.Values("Set-Cookie")) > 0, err
}

func (c *itCtx) reusoDetectadoEm(tok string) *time.Time {
	c.t.Helper()
	var v *time.Time
	require.NoError(c.t, c.db.QueryRow(`SELECT reuso_detectado_em FROM refresh_tokens WHERE token_hash = ?`, hashRefresh(tok)).Scan(&v))
	return v
}

func TestIntegracaoHTTP_SEC12_ReusoRepetido(t *testing.T) {
	c := novoItCtx(t)
	logs := capturarLog(t)
	url := c.srv.URL + "/api/auth/refresh"
	uid, email := c.usuarioComSenha("USEC12", "normal", 0)

	cortes := func() int {
		return strings.Count(logs.String(), "revogando todas as sessões: user_id="+strconv.FormatInt(uid, 10)+" ")
	}
	suprimidos := func() int {
		return strings.Count(logs.String(), "sessões NÃO revogadas de novo: user_id="+strconv.FormatInt(uid, 10)+" ")
	}

	st, _, refresh1 := c.loginIT(email)
	require.Equal(t, http.StatusOK, st)
	st, raw, refresh2 := refreshIT(t, c, refresh1)
	require.Equal(t, http.StatusOK, st, raw)
	require.NotEmpty(t, refresh2)
	_, err := c.db.Exec(`UPDATE refresh_tokens SET revoked_at = NOW() - INTERVAL 120 SECOND WHERE token_hash = ?`, hashRefresh(refresh1))
	require.NoError(t, err)
	require.Nil(t, c.reusoDetectadoEm(refresh1))

	var corpoCorte string
	t.Run("duas requisições concorrentes com o token reusado → um corte", func(t *testing.T) {
		type resultado struct {
			st     int
			corpo  string
			cookie bool
			err    error
		}
		res := make([]resultado, 2)
		var wg sync.WaitGroup
		inicio := make(chan struct{})
		for i := range res {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-inicio
				st, corpo, cookie, err := refreshCru(url, refresh1)
				res[i] = resultado{st, corpo, cookie, err}
			}(i)
		}
		close(inicio)
		wg.Wait()

		for i, r := range res {
			require.NoError(t, r.err)
			assert.Equal(t, http.StatusUnauthorized, r.st, "req %d", i)
			assert.False(t, r.cookie, "req %d sem Set-Cookie", i)
		}
		assert.Equal(t, res[0].corpo, res[1].corpo, "corpo idêntico no corte e na supressão")
		corpoCorte = res[0].corpo
		assert.Equal(t, 1, cortes(), "exatamente uma revogação em massa")
		assert.Equal(t, 1, suprimidos(), "a outra requisição é suprimida")
		require.NotNil(t, c.reusoDetectadoEm(refresh1))
		require.NotNil(t, c.corteDe(uid), "tokens_validos_desde gravado")
		assert.Equal(t, 1, c.motivos(uid)["revogacao_massa"], "só o refresh2 foi revogado em massa")
		assert.Zero(t, c.motivos(uid)["ativo"])
	})

	// Re-login da vítima, num segundo posterior ao corte.
	esperarProximoSegundo()
	st, access3, refresh3 := c.loginIT(email)
	require.Equal(t, http.StatusOK, st)
	corteAntes := c.corteDe(uid)
	require.NotNil(t, corteAntes)

	t.Run("re-login sobrevive a novos replays dentro da janela", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			st, corpo, cookie, err := refreshCru(url, refresh1)
			require.NoError(t, err)
			assert.Equal(t, http.StatusUnauthorized, st)
			assert.False(t, cookie)
			assert.Equal(t, corpoCorte, corpo)
		}
		assert.Equal(t, 1, cortes(), "nenhum corte novo")
		assert.Equal(t, 3, suprimidos())
		assert.True(t, corteAntes.Equal(*c.corteDe(uid)), "tokens_validos_desde não foi regravado")

		st, body := c.req("GET", "/api/auth/me", access3, nil)
		assert.Equal(t, http.StatusOK, st, "o access do re-login continua válido: %v", body)
		assert.Equal(t, 1, c.motivos(uid)["ativo"], "o refresh do re-login continua ativo")
	})

	var refresh4 string
	t.Run("o refresh do re-login continua rotacionando", func(t *testing.T) {
		st, raw, novo := refreshIT(t, c, refresh3)
		require.Equal(t, http.StatusOK, st, raw)
		require.NotEmpty(t, novo)
		refresh4 = novo
	})

	t.Run("passada a janela, o replay corta de novo", func(t *testing.T) {
		_, err := c.db.Exec(`UPDATE refresh_tokens SET reuso_detectado_em = reuso_detectado_em - INTERVAL 31 MINUTE WHERE token_hash = ?`, hashRefresh(refresh1))
		require.NoError(t, err)
		esperarProximoSegundo()

		st, corpo, cookie, err := refreshCru(url, refresh1)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, st)
		assert.False(t, cookie)
		assert.Equal(t, corpoCorte, corpo)
		assert.Equal(t, 2, cortes(), "corte de novo depois da janela")
		assert.Equal(t, 3, suprimidos())
		assert.True(t, c.corteDe(uid).After(*corteAntes), "tokens_validos_desde avançou")

		c.assertSessaoEncerrada(t, access3)
		st, raw, _ := refreshIT(t, c, refresh4)
		assert.Equal(t, http.StatusUnauthorized, st, raw)
		assert.Zero(t, c.motivos(uid)["ativo"])
	})
}

