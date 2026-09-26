package handlers_test

// Integração HTTP ponta a ponta (router real + MySQL local) do SEC-08: corte
// de sessão (usuarios.tokens_validos_desde) derruba os access tokens JWT já
// emitidos. Mesma infra do Lote 4/6 (itCtx): só roda com INTEGRATION=1, os
// dados levam o marcador ZZ-TEST-HTTP-<sufixo> e são apagados no t.Cleanup.
//
// Regressão do card: login -> inativar -> reativar -> token antigo em
// /api/auth/me devolvia 200 (achado do Lote 6); agora 401.
//
// A regra do middleware é iat <= corte -> 401 (em segundos). Um login feito
// no mesmo segundo do corte também seria recusado, então antes do login novo
// o teste espera virar o segundo (esperarProximoSegundo) — sem isso o caso
// "login novo -> 200" ficaria instável.

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sharedsvc "github.com/rotaperfumes/shared/services"
)

const (
	msgSessaoEncerradaIT = "sessão encerrada — faça login novamente"
	novaSenhaSEC08       = "Nova-Senha-SEC08@2"
)

// esperarProximoSegundo dorme até o início do próximo segundo: qualquer
// corte gravado antes desta chamada fica estritamente anterior ao iat dos
// tokens emitidos depois dela.
func esperarProximoSegundo() {
	agora := time.Now()
	time.Sleep(agora.Truncate(time.Second).Add(time.Second + 20*time.Millisecond).Sub(agora))
}

// corteDe devolve o corte de sessão gravado para o usuário (nil = NULL).
func (c *itCtx) corteDe(uid int64) *time.Time {
	c.t.Helper()
	var v *time.Time
	require.NoError(c.t, c.db.QueryRow(`SELECT tokens_validos_desde FROM usuarios WHERE id = ?`, uid).Scan(&v))
	return v
}

// loginComSenha é o loginIT com senha arbitrária.
func (c *itCtx) loginComSenha(email, senha string) (int, string, string) {
	c.t.Helper()
	req, err := http.NewRequest(http.MethodPost, c.srv.URL+"/api/auth/login",
		makeJSON(map[string]string{"email": email, "password": senha, "captchaToken": "token-valido-de-teste"}))
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer resp.Body.Close()
	access, refresh := refreshCookies(resp)
	return resp.StatusCode, access, refresh
}

// assertSessaoEncerrada confere que o access token antigo recebe 401 com a
// mensagem do corte de sessão.
func (c *itCtx) assertSessaoEncerrada(t *testing.T, access string) {
	t.Helper()
	for _, rota := range []string{"/api/auth/me", "/api/clientes"} {
		st, body := c.req("GET", rota, access, nil)
		assert.Equal(t, http.StatusUnauthorized, st, "%s: %v", rota, body)
		assert.Equal(t, msgSessaoEncerradaIT, body["error"], rota)
	}
}

// assertLoginNovoOK: depois do corte (e da virada do segundo), um login novo
// devolve uma sessão que funciona.
func (c *itCtx) assertLoginNovoOK(t *testing.T, email, senha string) {
	t.Helper()
	esperarProximoSegundo()
	st, a, r := c.loginComSenha(email, senha)
	require.Equal(t, http.StatusOK, st)
	require.NotEmpty(t, a)
	require.NotEmpty(t, r)
	st, body := c.req("GET", "/api/auth/me", a, nil)
	assert.Equal(t, http.StatusOK, st, "%v", body)
}

func TestIntegracaoHTTP_SEC08_CorteDeSessao(t *testing.T) {
	c := novoItCtx(t)

	t.Run("regressão: inativar -> reativar -> token antigo 401", func(t *testing.T) {
		uid, email := c.usuarioComSenha("USEC08A", "normal", 0)
		require.Nil(t, c.corteDe(uid), "usuário novo sem corte")
		st, access, refresh := c.loginIT(email)
		require.Equal(t, http.StatusOK, st)
		st, _ = c.req("GET", "/api/auth/me", access, nil)
		require.Equal(t, http.StatusOK, st, "sem corte o token vale")

		path := fmt.Sprintf("/api/usuarios/%d/inativar", uid)
		st, body := c.req("PATCH", path, c.admin, map[string]any{"ativo": false})
		require.Equal(t, http.StatusOK, st, body)
		corte := c.corteDe(uid)
		require.NotNil(t, corte, "inativar grava o corte")

		st, body = c.req("GET", "/api/auth/me", access, nil)
		assert.Equal(t, http.StatusUnauthorized, st)
		assert.Equal(t, "usuário inativo", body["error"], "inativo: mensagem de inativo tem precedência")

		st, body = c.req("PATCH", path, c.admin, map[string]any{"ativo": true})
		require.Equal(t, http.StatusOK, st, body)
		require.NotNil(t, c.corteDe(uid))
		assert.True(t, corte.Equal(*c.corteDe(uid)), "reativar não mexe no corte")

		c.assertSessaoEncerrada(t, access) // antes do SEC-08: 200
		st, raw, _ := refreshIT(t, c, refresh)
		assert.Equal(t, http.StatusUnauthorized, st, raw)

		c.assertLoginNovoOK(t, email, senhaIT)
	})

	t.Run("troca de senha pelo próprio usuário", func(t *testing.T) {
		uid, email := c.usuarioComSenha("USEC08S", "normal", 0)
		st, access, _ := c.loginIT(email)
		require.Equal(t, http.StatusOK, st)
		_, outraAba, _ := c.loginIT(email)

		st, body := c.req("POST", "/api/auth/reset-password", access, map[string]any{
			"senha_atual": senhaIT, "nova_senha": novaSenhaSEC08, "captchaToken": "token-valido-de-teste"})
		require.Equal(t, http.StatusOK, st, body)
		require.NotNil(t, c.corteDe(uid))

		c.assertSessaoEncerrada(t, access)
		c.assertSessaoEncerrada(t, outraAba)
		c.assertLoginNovoOK(t, email, novaSenhaSEC08)
	})

	t.Run("reset de senha pelo admin", func(t *testing.T) {
		uid, email := c.usuarioComSenha("USEC08R", "normal", 0)
		st, access, _ := c.loginIT(email)
		require.Equal(t, http.StatusOK, st)

		st, body := c.req("POST", "/api/admin/reset-password", c.admin, map[string]any{"usuario_id": uid})
		require.Equal(t, http.StatusOK, st, body)
		require.NotNil(t, c.corteDe(uid))

		c.assertSessaoEncerrada(t, access)

		// A senha nova é aleatória (enviada por e-mail); o teste regrava um
		// hash conhecido direto no banco (sem mexer no corte) para o login.
		hash, err := sharedsvc.NewAuthService().HashPassword(testCfg(), senhaIT)
		require.NoError(t, err)
		_, err = c.db.Exec(`UPDATE usuarios SET password_hash = ?, deve_trocar_senha = 0 WHERE id = ?`, hash, uid)
		require.NoError(t, err)
		c.assertLoginNovoOK(t, email, senhaIT)
	})

	t.Run("desligamento do vendedor", func(t *testing.T) {
		vend := c.vendedor("VSEC08", "")
		uid, email := c.usuarioComSenha("USEC08V", "normal", vend)
		st, access, _ := c.loginIT(email)
		require.Equal(t, http.StatusOK, st)

		st, body := c.req("DELETE", fmt.Sprintf("/api/vendedores/%d", vend), c.admin, nil)
		require.Equal(t, http.StatusOK, st, body)
		require.NotNil(t, c.corteDe(uid), "InativarByVendedorID grava o corte")

		st, body = c.req("GET", "/api/auth/me", access, nil)
		assert.Equal(t, http.StatusUnauthorized, st)
		assert.Equal(t, "usuário inativo", body["error"])

		// Mesmo se o admin reativar o usuário, o token antigo não volta.
		st, body = c.req("PATCH", fmt.Sprintf("/api/usuarios/%d/inativar", uid), c.admin, map[string]any{"ativo": true})
		require.Equal(t, http.StatusOK, st, body)
		c.assertSessaoEncerrada(t, access)
		c.assertLoginNovoOK(t, email, senhaIT)
	})

	t.Run("reuso de refresh rotacionado (SEC-07) derruba os access tokens", func(t *testing.T) {
		uid, email := c.usuarioComSenha("USEC08T", "normal", 0)
		st, access1, refresh1 := c.loginIT(email)
		require.Equal(t, http.StatusOK, st)
		_, accessOutraAba, _ := c.loginIT(email)

		resp := postRefresh(t, c.srv.URL+"/api/auth/refresh", map[string]any{"refresh_token": refresh1}, "")
		access2, refresh2 := refreshCookies(resp)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.NotEmpty(t, access2)
		require.NotEmpty(t, refresh2)
		require.Nil(t, c.corteDe(uid), "rotação normal não grava corte")

		// Tira o token rotacionado da janela de graça e o reusa.
		_, err := c.db.Exec(`UPDATE refresh_tokens SET revoked_at = NOW() - INTERVAL 120 SECOND
			WHERE token_hash = ? AND revoked_at IS NOT NULL`, hashRefresh(refresh1))
		require.NoError(t, err)
		st, raw, _ := refreshIT(t, c, refresh1)
		require.Equal(t, http.StatusUnauthorized, st, raw)
		require.NotNil(t, c.corteDe(uid), "reuso grava o corte")

		for nome, tok := range map[string]string{"access do login": access1, "access da outra aba": accessOutraAba, "access do refresh": access2} {
			t.Run(nome, func(t *testing.T) { c.assertSessaoEncerrada(t, tok) })
		}
		st, raw, _ = refreshIT(t, c, refresh2)
		assert.Equal(t, http.StatusUnauthorized, st, raw)

		c.assertLoginNovoOK(t, email, senhaIT)
	})

	t.Run("sem evento de corte o token segue válido (sem falso positivo)", func(t *testing.T) {
		uid, email := c.usuarioComSenha("USEC08N", "normal", 0)
		st, access, _ := c.loginIT(email)
		require.Equal(t, http.StatusOK, st)
		// Edição comum do usuário não é evento de corte.
		st, body := c.req("PUT", fmt.Sprintf("/api/usuarios/%d", uid), c.admin, map[string]any{
			"nome": c.nome("USEC08N2"), "email": email, "role": "normal"})
		require.Equal(t, http.StatusOK, st, body)
		assert.Nil(t, c.corteDe(uid))
		st, _ = c.req("GET", "/api/auth/me", access, nil)
		assert.Equal(t, http.StatusOK, st)
	})

	t.Run("corte gravado pelo Go sem deslocamento de fuso", func(t *testing.T) {
		uid, _ := c.usuarioComSenha("USEC08F", "normal", 0)
		antes := time.Now()
		st, body := c.req("PATCH", fmt.Sprintf("/api/usuarios/%d/inativar", uid), c.admin, map[string]any{"ativo": false})
		require.Equal(t, http.StatusOK, st, body)
		depois := time.Now()
		var texto string
		require.NoError(t, c.db.QueryRow(`SELECT DATE_FORMAT(tokens_validos_desde, '%Y-%m-%d %H:%i:%s') FROM usuarios WHERE id = ?`, uid).Scan(&texto))
		gravado, err := time.ParseInLocation("2006-01-02 15:04:05", texto, time.Local)
		require.NoError(t, err)
		assert.False(t, gravado.Before(antes.Truncate(time.Second)), "banco=%s antes=%s", texto, antes)
		assert.False(t, gravado.After(depois), "banco=%s depois=%s", texto, depois)
	})

}
