package handlers_test

// TST-05 (pendência do LOG-02): teste de runtime com VERBOSE ligado
// (vlog.SetEnabled(true)) nos handlers de login e reset de senha, fluxos de
// sucesso e de falha. Usa o router real (httptest) com DB mockado (sqlmock) e
// captcha fake, captura a saída do pacote log e verifica que nenhum valor
// sensível usado no teste vai ao log (docs/logs-verbose.md, seção 5): senha,
// hash (atual, novo e do histórico), JWT, refresh token (claro e hash), token
// do captcha, valores de cookie, header Authorization, User-Agent, pepper,
// segredo JWT e e-mail sem máscara. O e-mail só pode sair como
// vlog.MaskEmail(email).

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/adapters/turnstile"
	"github.com/rotaperfumes/shared/vlog"
)

const (
	tst05Email       = "fulano.sensivel@empresa-teste.com"
	tst05Senha       = "SenhaSuperSecreta#2026"
	tst05SenhaErrada = "SenhaErradaSecreta#2026"
	tst05NovaSenha   = "NovaSenhaForteSecreta#77"
	tst05SenhaAntiga = "SenhaDoHistorico#Antiga55"
	tst05Captcha     = "captcha-token-sensivel-XYZ123abc"
	tst05UserAgent   = "UA-Sensivel/9.9 (tst05-vlog-runtime)"
)

// tst05Buffer é um io.Writer seguro para concorrência: o handler roda na
// goroutine do httptest.Server enquanto o teste lê o buffer.
type tst05Buffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *tst05Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *tst05Buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// tst05CapturarLog liga o vlog, redireciona o log para um buffer e restaura
// saída, flags e estado do vlog no Cleanup.
func tst05CapturarLog(t *testing.T) *tst05Buffer {
	t.Helper()
	buf := &tst05Buffer{}
	prevOut, prevFlags, prevEnabled := log.Writer(), log.Flags(), vlog.Enabled()
	log.SetOutput(buf)
	log.SetFlags(0)
	vlog.SetEnabled(true)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
		vlog.SetEnabled(prevEnabled)
	})
	return buf
}

// tst05ArgCaptor é um sqlmock.Argument que aceita qualquer valor e guarda as
// strings recebidas (ex.: hash do refresh token, novo hash de senha) para
// conferir depois que não foram ao log.
type tst05ArgCaptor struct {
	mu   *sync.Mutex
	vals *[]string
}

func (c tst05ArgCaptor) Match(v driver.Value) bool {
	switch s := v.(type) {
	case string:
		c.mu.Lock()
		*c.vals = append(*c.vals, s)
		c.mu.Unlock()
	case []byte:
		c.mu.Lock()
		*c.vals = append(*c.vals, string(s))
		c.mu.Unlock()
	}
	return true
}

// tst05Ctx guarda os valores sensíveis acumulados durante um caso.
type tst05Ctx struct {
	t        *testing.T
	mock     sqlmock.Sqlmock
	auth     *authHandlerTeste
	url      string
	mu       sync.Mutex
	dbArgs   []string
	secretos []string
	cookies  int // cookies com valor não vazio recebidos
}

func (c *tst05Ctx) captor() tst05ArgCaptor { return tst05ArgCaptor{mu: &c.mu, vals: &c.dbArgs} }

func (c *tst05Ctx) anyArgs(n int) []driver.Value {
	out := make([]driver.Value, n)
	for i := range out {
		out[i] = c.captor()
	}
	return out
}

// hash gera um hash Argon2id e o registra (inteiro e por segmento: salt e
// digest) como valor proibido no log.
func (c *tst05Ctx) hash(senha string) string {
	h, err := novoAuthCrypto().HashPassword(testCfg(), senha)
	require.NoError(c.t, err)
	c.addHash(h)
	return h
}

func (c *tst05Ctx) addHash(h string) {
	c.secretos = append(c.secretos, h)
	for _, parte := range strings.Split(h, "$") {
		if len(parte) >= 12 {
			c.secretos = append(c.secretos, parte)
		}
	}
}

func (c *tst05Ctx) post(path string, body map[string]any, bearer string) *http.Response {
	req, err := http.NewRequest(http.MethodPost, c.url+path, makeJSON(body))
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", tst05UserAgent)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
		c.secretos = append(c.secretos, bearer, "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	readBody(c.t, resp)
	resp.Body.Close()
	for _, ck := range resp.Cookies() {
		if ck.Value != "" {
			c.cookies++
			c.secretos = append(c.secretos, ck.Value)
		}
	}
	return resp
}

var tst05UsuarioCols = []string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}

const (
	tst05QueryPorEmail = `FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.email\s+=\s+\?\s+LIMIT\s+1`
	tst05QueryPorID    = `FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.id\s+=\s+\?\s+LIMIT\s+1`
)

func (c *tst05Ctx) mockUsuarioPorEmail(hash string, ativo bool) {
	c.mock.ExpectQuery(tst05QueryPorEmail).
		WithArgs(tst05Email).
		WillReturnRows(sqlmock.NewRows(tst05UsuarioCols).
			AddRow(int64(2), "Fulano Sensivel", tst05Email, hash, "normal", nil, ativo, false, time.Now(), time.Now(), nil, nil))
}

func (c *tst05Ctx) mockUsuarioPorID(hash string) {
	c.mock.ExpectQuery(tst05QueryPorID).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows(tst05UsuarioCols).
			AddRow(int64(2), "Fulano Sensivel", tst05Email, hash, "normal", nil, true, false, time.Now(), time.Now(), nil, nil))
}

func (c *tst05Ctx) mockHistorico(hashes ...string) {
	c.mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(len(hashes)))
	rows := sqlmock.NewRows([]string{"id", "usuario_id", "resetado_por_id", "senha_hash_anterior", "ip_origem", "user_agent", "tipo_reset", "created_at", "usuario_nome", "resetado_por_nome"})
	for i, h := range hashes {
		rows.AddRow(int64(i+1), int64(2), nil, h, "127.0.0.1", tst05UserAgent, "usuario", time.Now(), "Fulano Sensivel", nil)
	}
	c.mock.ExpectQuery(`FROM senha_historico sh`).
		WithArgs(int64(2), 2, 0).
		WillReturnRows(rows)
}

type tst05Caso struct {
	nome       string
	funcao     string // função do handler cuja linha verbose deve aparecer
	statusEsp  int
	emailNoLog bool // o fluxo loga o e-mail (mascarado)
	cookiesMin int  // mínimo de cookies (access/refresh) que o caso deve coletar
	dbArgsMin  int  // mínimo de strings capturadas nos INSERT/UPDATE (hashes, UA)
	exec       func(c *tst05Ctx) *http.Response
}

func tst05Casos() []tst05Caso {
	login := func(senha string) map[string]any {
		return map[string]any{"email": tst05Email, "password": senha, "captchaToken": tst05Captcha}
	}
	reset := func(atual, nova string) map[string]any {
		return map[string]any{"senha_atual": atual, "nova_senha": nova, "captchaToken": tst05Captcha}
	}
	jwtUsuario := func(c *tst05Ctx) string { return generateToken(c.t, testCfg(), 2, "normal") }

	return []tst05Caso{
		{"login sucesso", "AuthHandler.Login", http.StatusOK, true, 2, 3, func(c *tst05Ctx) *http.Response {
			c.mockUsuarioPorEmail(c.hash(tst05Senha), true)
			c.mock.ExpectExec(`INSERT INTO refresh_tokens`).WithArgs(c.anyArgs(5)...).WillReturnResult(sqlmock.NewResult(1, 1))
			c.mock.ExpectExec(`UPDATE usuarios SET ultimo_login_at = \? WHERE id = \?`).
				WithArgs(sqlmock.AnyArg(), int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
			return c.post("/api/auth/login", login(tst05Senha), "")
		}},
		{"login senha incorreta", "AuthHandler.Login", http.StatusUnauthorized, true, 0, 0, func(c *tst05Ctx) *http.Response {
			c.mockUsuarioPorEmail(c.hash(tst05Senha), true)
			return c.post("/api/auth/login", login(tst05SenhaErrada), "")
		}},
		{"login usuario inexistente", "AuthHandler.Login", http.StatusUnauthorized, true, 0, 0, func(c *tst05Ctx) *http.Response {
			c.mock.ExpectQuery(tst05QueryPorEmail).WithArgs(tst05Email).WillReturnError(sql.ErrNoRows)
			return c.post("/api/auth/login", login(tst05Senha), "")
		}},
		{"login usuario inativo", "AuthHandler.Login", http.StatusUnauthorized, true, 0, 0, func(c *tst05Ctx) *http.Response {
			c.mockUsuarioPorEmail(c.hash(tst05Senha), false)
			return c.post("/api/auth/login", login(tst05Senha), "")
		}},
		{"login erro de banco", "AuthHandler.Login", http.StatusInternalServerError, true, 0, 0, func(c *tst05Ctx) *http.Response {
			c.mock.ExpectQuery(tst05QueryPorEmail).WithArgs(tst05Email).WillReturnError(fmt.Errorf("falha simulada do driver"))
			return c.post("/api/auth/login", login(tst05Senha), "")
		}},
		{"login captcha invalido", "AuthHandler.Login", http.StatusBadRequest, true, 0, 0, func(c *tst05Ctx) *http.Response {
			c.auth.SetCaptchaVerifier(&fakeCaptchaVerifier{err: turnstile.ErrCaptchaInvalido})
			return c.post("/api/auth/login", login(tst05Senha), "")
		}},
		{"reset sucesso", "AuthHandler.ResetPassword", http.StatusOK, false, 0, 4, func(c *tst05Ctx) *http.Response {
			c.mockUsuarioPorID(c.hash(tst05Senha))
			c.mockHistorico(c.hash(tst05SenhaAntiga))
			c.mock.ExpectExec(`INSERT INTO senha_historico`).WithArgs(c.anyArgs(6)...).WillReturnResult(sqlmock.NewResult(1, 1))
			c.mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \?, tokens_validos_desde = \? WHERE id = \?`).
				WithArgs(c.captor(), false, sqlmock.AnyArg(), int64(2)).WillReturnResult(sqlmock.NewResult(0, 1))
			c.mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE usuario_id = \?`).
				WithArgs(sqlmock.AnyArg(), "senha", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 0))
			return c.post("/api/auth/reset-password", reset(tst05Senha, tst05NovaSenha), jwtUsuario(c))
		}},
		{"reset senha atual incorreta", "AuthHandler.ResetPassword", http.StatusBadRequest, false, 0, 0, func(c *tst05Ctx) *http.Response {
			c.mockUsuarioPorID(c.hash(tst05Senha))
			return c.post("/api/auth/reset-password", reset(tst05SenhaErrada, tst05NovaSenha), jwtUsuario(c))
		}},
		{"reset nova senha reutilizada do historico", "AuthHandler.ResetPassword", http.StatusBadRequest, false, 0, 0, func(c *tst05Ctx) *http.Response {
			c.mockUsuarioPorID(c.hash(tst05Senha))
			c.mockHistorico(c.hash(tst05SenhaAntiga))
			return c.post("/api/auth/reset-password", reset(tst05Senha, tst05SenhaAntiga), jwtUsuario(c))
		}},
		{"reset nova senha fraca", "AuthHandler.ResetPassword", http.StatusBadRequest, false, 0, 0, func(c *tst05Ctx) *http.Response {
			c.mockUsuarioPorID(c.hash(tst05Senha))
			return c.post("/api/auth/reset-password", reset(tst05Senha, "fraquinha"), jwtUsuario(c))
		}},
		{"reset captcha invalido", "AuthHandler.ResetPassword", http.StatusBadRequest, false, 0, 0, func(c *tst05Ctx) *http.Response {
			c.auth.SetCaptchaVerifier(&fakeCaptchaVerifier{err: turnstile.ErrCaptchaInvalido})
			return c.post("/api/auth/reset-password", reset(tst05Senha, tst05NovaSenha), jwtUsuario(c))
		}},
	}
}

// emailCru casa qualquer e-mail cujo usuário tenha ao menos um caractere
// não mascarado antes do "@" (vlog.MaskEmail produz "x***@dominio", que não
// casa porque "*" não está na classe).
var tst05EmailCru = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+\.[A-Za-z]`)

func TestTST05_VlogRuntime_LoginReset_NaoVazaSensiveis(t *testing.T) {
	for _, tc := range tst05Casos() {
		t.Run(tc.nome, func(t *testing.T) {
			buf := tst05CapturarLog(t)

			server, db, mock, authHandler := setupTestServerWithAuthHandler(t)
			defer db.Close()

			cfg := testCfg()
			c := &tst05Ctx{t: t, mock: mock, auth: authHandler, url: server.URL}
			c.secretos = append(c.secretos,
				tst05Senha, tst05SenhaErrada, tst05NovaSenha, tst05SenhaAntiga, "fraquinha",
				tst05Captcha, tst05UserAgent, tst05Email,
				cfg.JWTSecret, cfg.HashSenha.Pepper,
			)

			resp := tc.exec(c)
			// Close bloqueia até as requisições em curso terminarem: toda a
			// saída de log do handler/middlewares já está no buffer.
			server.Close()

			assert.Equal(t, tc.statusEsp, resp.StatusCode)
			require.NoError(t, mock.ExpectationsWereMet())

			// Garante que os segredos dinâmicos foram de fato coletados (senão
			// o NotContains abaixo passaria por vacuidade).
			assert.GreaterOrEqual(t, c.cookies, tc.cookiesMin, "cookies de sessão coletados")
			assert.GreaterOrEqual(t, len(c.dbArgs), tc.dbArgsMin, "argumentos sensíveis do banco coletados")

			// Valores gravados no banco (hash do refresh token, novo hash,
			// hash anterior no histórico, UA): nenhum pode ir ao log.
			for _, v := range c.dbArgs {
				if len(v) >= 12 {
					c.secretos = append(c.secretos, v)
				}
			}

			saida := buf.String()
			require.NotEmpty(t, saida, "com VERBOSE ligado o fluxo deve gerar log")
			assert.Contains(t, saida, "[auth_handler.go] ["+tc.funcao+"]",
				"linha verbose do handler deve aparecer com vlog ligado")

			for _, s := range c.secretos {
				if s == "" {
					continue
				}
				assert.NotContains(t, saida, s, "valor sensível vazou para o log")
			}

			assert.Empty(t, tst05EmailCru.FindAllString(saida, -1), "e-mail sem máscara no log")
			if tc.emailNoLog {
				assert.Contains(t, saida, vlog.MaskEmail(tst05Email), "e-mail deve sair mascarado via vlog.MaskEmail")
			}

			if t.Failed() {
				t.Logf("saída capturada:\n%s", saida)
			}
		})
	}
}

// TestTST05_DetectorDeVazamento_ControleNegativo garante que o detector usado
// acima não passa por vacuidade: uma linha com e-mail cru é detectada e a
// forma mascarada não.
func TestTST05_DetectorDeVazamento_ControleNegativo(t *testing.T) {
	casos := []struct {
		nome    string
		linha   string
		detecta bool
	}{
		{"e-mail cru", "[auth] login: " + tst05Email, true},
		{"e-mail cru no meio", "x=1 email=ana.silva@empresa.com y=2", true},
		{"e-mail mascarado", "[auth] login: " + vlog.MaskEmail(tst05Email), false},
		{"sem e-mail", "[auth] login OK: user_id=2 role=normal", false},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			buf := tst05CapturarLog(t)
			log.Print(tc.linha)
			assert.Equal(t, tc.detecta, tst05EmailCru.MatchString(buf.String()))
		})
	}
}
