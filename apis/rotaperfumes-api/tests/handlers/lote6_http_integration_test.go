package handlers_test

// Integração HTTP ponta a ponta (router real + MySQL local) dos cards do
// Lote 6: SEC-06 (usuário inativado perde a sessão na hora) e SEC-07 (reuso
// de refresh token rotacionado fora da janela revoga todas as sessões).
// Mesma infra do Lote 4 (itCtx): só roda com INTEGRATION=1, os dados levam o
// marcador ZZ-TEST-HTTP-<sufixo> e são apagados no t.Cleanup (usuarios →
// refresh_tokens em cascata). Nenhum dado pré-existente é alterado.

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apisvc "github.com/rotaperfumes/rotaperfumes-api/services"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

const senhaIT = "Senha-IT-Lote6@1"

// usuarioComSenha cria um usuário de teste com senha real (bcrypt) para
// passar pelo POST /api/auth/login.
func (c *itCtx) usuarioComSenha(tag, role string, vendedorID int64) (int64, string) {
	c.t.Helper()
	id := c.usuario(tag, vendedorID, true)
	hash, err := sharedsvc.NewAuthService().HashPassword(testCfg(), senhaIT)
	require.NoError(c.t, err)
	_, err = c.db.Exec(`UPDATE usuarios SET password_hash = ?, role = ?, deve_trocar_senha = 0 WHERE id = ?`, hash, role, id)
	require.NoError(c.t, err)
	var email string
	require.NoError(c.t, c.db.QueryRow(`SELECT email FROM usuarios WHERE id = ?`, id).Scan(&email))
	return id, email
}

// loginIT faz o login real e devolve access e refresh dos cookies.
func (c *itCtx) loginIT(email string) (int, string, string) {
	c.t.Helper()
	req, err := http.NewRequest(http.MethodPost, c.srv.URL+"/api/auth/login",
		makeJSON(map[string]string{"email": email, "password": senhaIT, "captchaToken": "token-valido-de-teste"}))
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer resp.Body.Close()
	access, refresh := refreshCookies(resp)
	return resp.StatusCode, access, refresh
}

// motivos devolve a contagem de refresh tokens do usuário por motivo
// ("ativo" para revoked_at NULL; "NULL" para revogado sem motivo).
func (c *itCtx) motivos(uid int64) map[string]int {
	c.t.Helper()
	rows, err := c.db.Query(`SELECT CASE WHEN revoked_at IS NULL THEN 'ativo'
		ELSE IFNULL(revoked_reason, 'NULL') END m, COUNT(*) FROM refresh_tokens WHERE usuario_id = ? GROUP BY m`, uid)
	require.NoError(c.t, err)
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var m string
		var n int
		require.NoError(c.t, rows.Scan(&m, &n))
		out[m] = n
	}
	require.NoError(c.t, rows.Err())
	return out
}

// ---------------------------------------------------------------------------
// SEC-06
// ---------------------------------------------------------------------------

// TestIntegracaoHTTP_SEC06_InativarUsuarioDerrubaSessao: com a sessão aberta
// (login real), o admin inativa o usuário → o próximo request protegido e o
// refresh recebem 401. Reativado, o refresh token antigo continua inválido e
// só um novo login devolve a sessão.
func TestIntegracaoHTTP_SEC06_InativarUsuarioDerrubaSessao(t *testing.T) {
	c := novoItCtx(t)
	uid, email := c.usuarioComSenha("USEC06", "normal", 0)
	path := fmt.Sprintf("/api/usuarios/%d/inativar", uid)

	st, access, refresh := c.loginIT(email)
	require.Equal(t, http.StatusOK, st)
	require.NotEmpty(t, access)
	require.NotEmpty(t, refresh)

	// Sessão aberta: segunda aba com outra sessão, request protegido e um refresh OK.
	_, _, refresh2 := c.loginIT(email)
	require.NotEmpty(t, refresh2)
	st, _ = c.req("GET", "/api/auth/me", access, nil)
	require.Equal(t, http.StatusOK, st)
	st, _, refresh = refreshIT(t, c, refresh)
	require.Equal(t, http.StatusOK, st)
	require.NotEmpty(t, refresh)
	require.Equal(t, 2, c.motivos(uid)["ativo"])

	// O admin inativa.
	st, body := c.req("PATCH", path, c.admin, map[string]any{"ativo": false})
	require.Equal(t, http.StatusOK, st, body)
	assert.Equal(t, false, dataMap(t, body)["ativo"])

	m := c.motivos(uid)
	assert.Zero(t, m["ativo"], "nenhum refresh token pode sobrar ativo: %v", m)
	assert.Equal(t, 2, m["inativacao"], "as duas sessões revogadas com motivo inativacao: %v", m)
	assert.Equal(t, 1, m["rotacao"], "o token rotacionado mantém o motivo original: %v", m)

	for _, rota := range []string{"/api/auth/me", "/api/clientes", "/api/dashboard/metrics"} {
		t.Run("request protegido após inativar: "+rota, func(t *testing.T) {
			st, body := c.req("GET", rota, access, nil)
			assert.Equal(t, http.StatusUnauthorized, st)
			assert.Equal(t, "usuário inativo", body["error"])
		})
	}
	t.Run("refresh das duas sessões após inativar -> 401 sem emitir token", func(t *testing.T) {
		for _, tok := range []string{refresh, refresh2} {
			st, raw, novo := refreshIT(t, c, tok)
			assert.Equal(t, http.StatusUnauthorized, st, raw)
			assert.Empty(t, novo)
		}
	})
	t.Run("login com usuário inativo -> 401", func(t *testing.T) {
		st, a, r := c.loginIT(email)
		assert.Equal(t, http.StatusUnauthorized, st)
		assert.Empty(t, a)
		assert.Empty(t, r)
	})
	// Motivo inativacao não é reuso de rotação: sem revogação em massa extra.
	assert.Equal(t, 2, c.motivos(uid)["inativacao"])
	assert.Zero(t, c.motivos(uid)["revogacao_massa"])

	// Reativa (restaura o estado) e confere os tokens antigos.
	st, body = c.req("PATCH", path, c.admin, map[string]any{"ativo": true})
	require.Equal(t, http.StatusOK, st, body)
	require.Equal(t, 1, c.contar("SELECT ativo FROM usuarios WHERE id = ?", uid))

	t.Run("reativado: refresh tokens antigos continuam inválidos", func(t *testing.T) {
		for _, tok := range []string{refresh, refresh2} {
			st, raw, novo := refreshIT(t, c, tok)
			assert.Equal(t, http.StatusUnauthorized, st, raw)
			assert.Empty(t, novo)
		}
		assert.Zero(t, c.motivos(uid)["ativo"], "reativar não ressuscita sessão")
	})
	t.Run("reativado: access token antigo (JWT ainda no prazo)", func(t *testing.T) {
		// Achado do TestBrain (Lote 6): o SEC-06 revoga só os refresh tokens.
		// O access token JWT emitido antes da inativação não tem lista de
		// revogação; com o usuário reativado ele volta a ser aceito até expirar
		// (JWT_TTL). Registrado aqui sem travar o comportamento.
		st, _ := c.req("GET", "/api/auth/me", access, nil)
		t.Logf("access token antigo após reativar: HTTP %d (200 = volta a valer até o TTL)", st)
	})
	t.Run("reativado: novo login restabelece a sessão", func(t *testing.T) {
		st, a, r := c.loginIT(email)
		require.Equal(t, http.StatusOK, st)
		require.NotEmpty(t, r)
		st, _ = c.req("GET", "/api/auth/me", a, nil)
		assert.Equal(t, http.StatusOK, st)
		st, _, _ = refreshIT(t, c, r)
		assert.Equal(t, http.StatusOK, st)
	})
}

// TestIntegracaoHTTP_SEC06_DesligarVendedorDerrubaSessao: DELETE do vendedor
// inativa os usuários dele e revoga os refresh tokens na mesma transação.
func TestIntegracaoHTTP_SEC06_DesligarVendedorDerrubaSessao(t *testing.T) {
	c := novoItCtx(t)
	vend := c.vendedor("VSEC06", "")
	uid, email := c.usuarioComSenha("USEC06V", "normal", vend)
	outro, emailOutro := c.usuarioComSenha("USEC06O", "normal", 0) // não pode ser afetado

	st, access, refresh := c.loginIT(email)
	require.Equal(t, http.StatusOK, st)
	_, accessOutro, refreshOutro := c.loginIT(emailOutro)
	st, _ = c.req("GET", "/api/auth/me", access, nil)
	require.Equal(t, http.StatusOK, st)

	st, body := c.req("DELETE", fmt.Sprintf("/api/vendedores/%d", vend), c.admin, nil)
	require.Equal(t, http.StatusOK, st, body)

	assert.Equal(t, 0, c.contar("SELECT ativo FROM usuarios WHERE id = ?", uid))
	assert.Equal(t, map[string]int{"inativacao": 1}, c.motivos(uid))
	assert.Equal(t, map[string]int{"ativo": 1}, c.motivos(outro), "usuário de fora do vendedor intacto")

	st, body = c.req("GET", "/api/auth/me", access, nil)
	assert.Equal(t, http.StatusUnauthorized, st)
	assert.Equal(t, "usuário inativo", body["error"])
	st, raw, novo := refreshIT(t, c, refresh)
	assert.Equal(t, http.StatusUnauthorized, st, raw)
	assert.Empty(t, novo)

	st, _ = c.req("GET", "/api/auth/me", accessOutro, nil)
	assert.Equal(t, http.StatusOK, st)
	st, _, _ = refreshIT(t, c, refreshOutro)
	assert.Equal(t, http.StatusOK, st)

	// Segundo DELETE (idempotente) não quebra e não mexe em mais nada.
	st, _ = c.req("DELETE", fmt.Sprintf("/api/vendedores/%d", vend), c.admin, nil)
	assert.Contains(t, []int{http.StatusOK, http.StatusConflict}, st)
	assert.Equal(t, map[string]int{"inativacao": 1}, c.motivos(uid))
}

// TestIntegracaoHTTP_SEC06_RoleEInexistente: rebaixar admin→normal vale no
// request seguinte (role do banco) e token de usuário apagado recebe 401.
func TestIntegracaoHTTP_SEC06_RoleEInexistente(t *testing.T) {
	c := novoItCtx(t)
	adm, _ := c.usuarioComSenha("ASEC06", "admin", 0)
	tokAdm := generateToken(t, testCfg(), adm, "admin")

	st, _ := c.req("GET", "/api/usuarios", tokAdm, nil)
	require.Equal(t, http.StatusOK, st)

	_, err := c.db.Exec(`UPDATE usuarios SET role = 'normal' WHERE id = ?`, adm)
	require.NoError(t, err)
	st, body := c.req("GET", "/api/usuarios", tokAdm, nil)
	assert.Equal(t, http.StatusForbidden, st, "role do banco prevalece sobre o do JWT")
	assert.Equal(t, "acesso restrito a administradores", body["error"])

	st, _ = c.req("GET", "/api/auth/me", tokAdm, nil)
	assert.Equal(t, http.StatusOK, st, "rebaixado continua autenticado como normal")

	t.Run("normal promovido a admin no banco ganha acesso sem novo token", func(t *testing.T) {
		tokNormal := generateToken(t, testCfg(), adm, "normal")
		_, err := c.db.Exec(`UPDATE usuarios SET role = 'admin' WHERE id = ?`, adm)
		require.NoError(t, err)
		st, _ := c.req("GET", "/api/usuarios", tokNormal, nil)
		assert.Equal(t, http.StatusOK, st)
	})

	t.Run("usuário inexistente -> 401 com a mesma mensagem do inativo", func(t *testing.T) {
		var maxID int64
		require.NoError(t, c.db.QueryRow(`SELECT IFNULL(MAX(id), 0) FROM usuarios`).Scan(&maxID))
		fantasma := generateToken(t, testCfg(), maxID+100000, "admin")
		for _, rota := range []string{"/api/auth/me", "/api/usuarios"} {
			st, body := c.req("GET", rota, fantasma, nil)
			assert.Equal(t, http.StatusUnauthorized, st, rota)
			assert.Equal(t, "usuário inativo", body["error"], rota)
		}
	})

	t.Run("rotas públicas não consultam o usuário (refresh sem token -> 400)", func(t *testing.T) {
		st, raw, _ := refreshIT(t, c, "")
		assert.Equal(t, http.StatusBadRequest, st, raw)
	})
}

// ---------------------------------------------------------------------------
// SEC-07
// ---------------------------------------------------------------------------

// TestIntegracaoHTTP_SEC07_ReusoForaDaJanela: só o reuso de token revogado
// com motivo 'rotacao' fora da janela de graça revoga todas as sessões do
// usuário (revogacao_massa). logout, NULL (legado) e reuso dentro da janela
// não mexem nas outras sessões. Cada subteste usa um usuário próprio e o
// servidor desta execução (limiter novo); o total de 401 que contam fica
// abaixo do limite de 10.
func TestIntegracaoHTTP_SEC07_ReusoForaDaJanela(t *testing.T) {
	c := novoItCtx(t)
	svc := apisvc.NewRefreshTokenService()
	novoToken := func(t *testing.T, uid int64) string {
		tok, err := svc.GenerateRefreshToken(context.Background(), c.db, uid, "127.0.0.1", "go-test")
		require.NoError(t, err)
		return tok
	}
	envelhecer := func(t *testing.T, tok string, segundos int) {
		res, err := c.db.Exec(`UPDATE refresh_tokens SET revoked_at = NOW() - INTERVAL ? SECOND
			WHERE token_hash = ? AND revoked_at IS NOT NULL`, segundos, hashRefresh(tok))
		require.NoError(t, err)
		n, _ := res.RowsAffected()
		require.EqualValues(t, 1, n, "o token precisa estar revogado para envelhecer")
	}

	t.Run("rotacao fora da janela: 401 e revoga todas as sessões (revogacao_massa)", func(t *testing.T) {
		uid := c.usuario("USEC07R", 0, true)
		tok := novoToken(t, uid)
		outraSessao := novoToken(t, uid)
		st, _, rotacionado := refreshIT(t, c, tok) // revoga tok com motivo rotacao
		require.Equal(t, http.StatusOK, st)
		require.Equal(t, map[string]int{"ativo": 2, "rotacao": 1}, c.motivos(uid))
		envelhecer(t, tok, 120)

		st, raw, novo := refreshIT(t, c, tok)
		assert.Equal(t, http.StatusUnauthorized, st, raw)
		assert.Empty(t, novo)
		assert.Equal(t, map[string]int{"rotacao": 1, "revogacao_massa": 2}, c.motivos(uid),
			"a outra sessão e o token rotacionado caem; o reusado mantém 'rotacao'")

		for _, t2 := range []string{outraSessao, rotacionado} {
			st, raw, _ := refreshIT(t, c, t2)
			assert.Equal(t, http.StatusUnauthorized, st, raw)
		}
		// Reuso de token 'revogacao_massa' não dispara nova revogação em massa.
		assert.Equal(t, map[string]int{"rotacao": 1, "revogacao_massa": 2}, c.motivos(uid))
	})

	t.Run("rotacao dentro da janela (corrida entre abas): não revoga as outras sessões", func(t *testing.T) {
		uid := c.usuario("USEC07J", 0, true)
		tok := novoToken(t, uid)
		novoToken(t, uid)
		st, _, _ := refreshIT(t, c, tok)
		require.Equal(t, http.StatusOK, st)
		st, raw, _ := refreshIT(t, c, tok)
		assert.Equal(t, http.StatusUnauthorized, st, raw)
		assert.Equal(t, map[string]int{"ativo": 2, "rotacao": 1}, c.motivos(uid))
	})

	t.Run("logout fora da janela: 401 e NÃO revoga as outras sessões", func(t *testing.T) {
		uid := c.usuario("USEC07L", 0, true)
		tok := novoToken(t, uid)
		outraSessao := novoToken(t, uid)
		resp := postRefresh(t, c.srv.URL+"/api/auth/logout", map[string]any{"refresh_token": tok}, "")
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, map[string]int{"ativo": 1, "logout": 1}, c.motivos(uid))
		envelhecer(t, tok, 120)

		st, raw, novo := refreshIT(t, c, tok)
		assert.Equal(t, http.StatusUnauthorized, st, raw)
		assert.Empty(t, novo)
		assert.Equal(t, map[string]int{"ativo": 1, "logout": 1}, c.motivos(uid))
		st, _, _ = refreshIT(t, c, outraSessao)
		assert.Equal(t, http.StatusOK, st, "a outra sessão segue válida")
	})

	t.Run("motivo NULL (legado) fora da janela: NÃO revoga as outras sessões", func(t *testing.T) {
		uid := c.usuario("USEC07N", 0, true)
		tok := novoToken(t, uid)
		novoToken(t, uid)
		_, err := c.db.Exec(`UPDATE refresh_tokens SET revoked_at = NOW() - INTERVAL 120 SECOND, revoked_reason = NULL
			WHERE token_hash = ?`, hashRefresh(tok))
		require.NoError(t, err)
		st, raw, _ := refreshIT(t, c, tok)
		assert.Equal(t, http.StatusUnauthorized, st, raw)
		assert.Equal(t, map[string]int{"ativo": 1, "NULL": 1}, c.motivos(uid))
	})

	t.Run("senha e inativacao fora da janela: sem revogação em massa", func(t *testing.T) {
		for _, motivo := range []string{"senha", "inativacao"} {
			t.Run(motivo, func(t *testing.T) {
				uid := c.usuario("USEC07"+motivo, 0, true)
				tok := novoToken(t, uid)
				novoToken(t, uid)
				_, err := c.db.Exec(`UPDATE refresh_tokens SET revoked_at = NOW() - INTERVAL 120 SECOND, revoked_reason = ?
					WHERE token_hash = ?`, motivo, hashRefresh(tok))
				require.NoError(t, err)
				st, raw, _ := refreshIT(t, c, tok)
				assert.Equal(t, http.StatusUnauthorized, st, raw)
				assert.Equal(t, map[string]int{"ativo": 1, motivo: 1}, c.motivos(uid))
			})
		}
	})
}

// ---------------------------------------------------------------------------
// BUG-08: POST/PUT /api/clientes devolvem created_at/updated_at do banco
// ---------------------------------------------------------------------------

func TestIntegracaoHTTP_BUG08_TimestampsDoCliente(t *testing.T) {
	c := novoItCtx(t)
	vend := c.vendedor("VBUG08", "")
	uid := c.usuario("UBUG08", vend, true)
	tokNormal := c.tokenNormal(uid)
	const zero = "0001-01-01T00:00:00Z"

	conferir := func(t *testing.T, body map[string]any) int64 {
		t.Helper()
		d := dataMap(t, body)
		for _, campo := range []string{"created_at", "updated_at"} {
			v, ok := d[campo].(string)
			require.True(t, ok, "%s ausente: %v", campo, d)
			assert.NotEqual(t, zero, v, "%s zerado", campo)
			assert.NotEmpty(t, v)
		}
		return idDe(t, body, "cliente_id_origem")
	}

	for _, tc := range []struct{ nome, token string }{{"admin (create simples)", c.admin}, {"normal (create na carteira)", tokNormal}} {
		t.Run("POST "+tc.nome, func(t *testing.T) {
			st, body := c.req("POST", "/api/clientes", tc.token, c.payloadCliente(c.nome("B08")))
			require.Equal(t, http.StatusCreated, st, body)
			id := conferir(t, body)
			assert.Equal(t, 1, c.contar(`SELECT COUNT(*) FROM clientes WHERE cliente_id_origem = ? AND created_at IS NOT NULL`, id))
		})
	}
	t.Run("PUT devolve timestamps preenchidos", func(t *testing.T) {
		st, body := c.req("POST", "/api/clientes", c.admin, c.payloadCliente(c.nome("B08PUT")))
		require.Equal(t, http.StatusCreated, st, body)
		id := conferir(t, body)
		p := c.payloadCliente(c.nome("B08PUT2"))
		p["cnpj"] = dataMap(t, body)["cnpj"]
		st, body = c.req("PUT", fmt.Sprintf("/api/clientes/%d", id), c.admin, p)
		require.Equal(t, http.StatusOK, st, body)
		conferir(t, body)
	})
}
