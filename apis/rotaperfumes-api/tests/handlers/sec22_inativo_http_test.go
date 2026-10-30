package handlers_test

// SEC-22 (Lote 24, TestBrain), ponta a ponta pelo router real:
//   Login: inexistente, inativo com senha errada, inativo com senha certa e
//   ativo com senha errada respondem o MESMO corpo, byte a byte
//   (401 {"error":"credenciais inválidas","success":false}), sem Set-Cookie
//   e sem "inativo" em lugar nenhum da resposta. Inativo com senha certa não
//   grava refresh nem ultimo_login (sqlmock estrito). 5 falhas de inativo
//   (senha certa) → 429 na 6ª, sem consultar o banco.
//   Refresh de inativo: 401 "refresh token revogado" sem Set-Cookie de
//   token; UPDATE de revogação com motivo "inativacao"; a segunda chamada
//   idêntica responde igual; falha na revogação não muda a resposta; conta
//   no rate limit por IP (10 falhas → 429 na 11ª).

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

// Envelope de erro do httpx (mapa: chaves em ordem alfabética).
const sec22CorpoCredenciais = `{"error":"credenciais inválidas","success":false}`

func sec22LinhaUsuario(id int64, email, hash string, ativo bool) *sqlmock.Rows {
	return sqlmock.NewRows(usuarioCols).
		AddRow(id, "Fulano", email, hash, "normal", nil, ativo, false, time.Now(), time.Now(), nil, nil)
}

// sec22RespostaSemSegredo confere que a resposta não revela inatividade
// nem emite cookie.
func sec22RespostaSemSegredo(t *testing.T, resp *http.Response, corpo []byte) {
	t.Helper()
	assert.NotContains(t, strings.ToLower(string(corpo)), "inativ")
	for k, vs := range resp.Header {
		for _, v := range vs {
			assert.NotContains(t, strings.ToLower(v), "inativ", "header %s", k)
		}
	}
	assert.Empty(t, resp.Header.Values("Set-Cookie"), "nenhum cookie em login recusado")
}

func TestSEC22_HTTP_LoginCorpoIdenticoNosQuatroCasos(t *testing.T) {
	hs := testCfg().HashSenha
	hash, err := gerarHashSenha(hs, "senha-correta")
	require.NoError(t, err)

	casos := []struct {
		nome         string
		email, senha string
		setup        func(mock sqlmock.Sqlmock)
	}{
		{"e-mail inexistente", "ninguem@test.com", "senha-correta", func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery(loginPorEmailSQL).WithArgs("ninguem@test.com").WillReturnError(sql.ErrNoRows)
		}},
		{"inativo com senha errada", "inativo@test.com", "senha-errada", func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery(loginPorEmailSQL).WithArgs("inativo@test.com").WillReturnRows(sec22LinhaUsuario(3, "inativo@test.com", hash, false))
		}},
		{"inativo com senha certa", "inativo@test.com", "senha-correta", func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery(loginPorEmailSQL).WithArgs("inativo@test.com").WillReturnRows(sec22LinhaUsuario(3, "inativo@test.com", hash, false))
		}},
		{"ativo com senha errada", "ativo@test.com", "senha-errada", func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery(loginPorEmailSQL).WithArgs("ativo@test.com").WillReturnRows(sec22LinhaUsuario(4, "ativo@test.com", hash, true))
		}},
	}
	corpos := map[string]string{}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, mock, _ := sec2xServidor(t, func(*config.Config) {})
			c.setup(mock)
			resp := sec2xPost(t, server.URL+"/api/auth/login", "", "",
				map[string]string{"email": c.email, "password": c.senha, "captchaToken": "ok"})
			corpo := readBody(t, resp)

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			assert.Equal(t, sec22CorpoCredenciais, strings.TrimSuffix(string(corpo), "\n"))
			sec22RespostaSemSegredo(t, resp, corpo)
			assert.NoError(t, mock.ExpectationsWereMet(), "nenhum INSERT de refresh nem UPDATE de ultimo_login")
			corpos[c.nome] = string(corpo)
		})
	}
	require.Len(t, corpos, len(casos))
	ref := corpos[casos[0].nome]
	for nome, corpo := range corpos {
		assert.Equal(t, ref, corpo, "%s: corpo idêntico byte a byte", nome)
	}
}

func TestSEC22_HTTP_InativoContaNoRateLimitDeLogin(t *testing.T) {
	hash, err := gerarHashSenha(testCfg().HashSenha, "senha-correta")
	require.NoError(t, err)
	server, mock, _ := sec2xServidor(t, func(*config.Config) {})
	for i := 0; i < 5; i++ {
		mock.ExpectQuery(loginPorEmailSQL).WithArgs("inativo@test.com").WillReturnRows(sec22LinhaUsuario(3, "inativo@test.com", hash, false))
	}
	for i := 1; i <= 6; i++ {
		resp := sec2xPost(t, server.URL+"/api/auth/login", "", "",
			map[string]string{"email": "inativo@test.com", "password": "senha-correta", "captchaToken": "ok"})
		corpo := readBody(t, resp)
		if i < 6 {
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "tentativa %d", i)
			continue
		}
		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode, "6ª tentativa: %s", corpo)
		assert.NotEmpty(t, resp.Header.Get("Retry-After"))
		sec22RespostaSemSegredo(t, resp, corpo)
	}
	assert.NoError(t, mock.ExpectationsWereMet(), "a 6ª não consulta o banco")
}

func sec22ExpectRefreshInativo(mock sqlmock.Sqlmock, errRevogar error) {
	agora := time.Now()
	expectRefreshValido(mock, 3)
	mock.ExpectQuery(usuarioByIDSQL).WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows(usuarioCols).
		AddRow(int64(3), "Inativo", "i@test.com", "hash", "normal", nil, false, false, agora, agora, nil, nil))
	exp := mock.ExpectExec(revokeCondSQL).WithArgs(sqlmock.AnyArg(), "inativacao", int64(10))
	if errRevogar != nil {
		exp.WillReturnError(errRevogar)
		return
	}
	exp.WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestSEC22_HTTP_RefreshInativo(t *testing.T) {
	casos := []struct {
		nome       string
		errRevogar error
	}{
		{"revogação ok", nil},
		{"revogação falha no banco", errors.New("db down")},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, mock, _ := sec2xServidor(t, func(*config.Config) {})
			url := server.URL + "/api/auth/refresh"
			var corpos []string
			for chamada := 1; chamada <= 2; chamada++ {
				sec22ExpectRefreshInativo(mock, c.errRevogar)
				resp := sec2xPost(t, url, "", "", map[string]string{}, &http.Cookie{Name: "refresh_token", Value: refreshTokenTexto})
				corpo := readBody(t, resp)
				assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "chamada %d", chamada)
				assert.Equal(t, "refresh token revogado", decodeResponse(t, corpo)["error"])
				access, refresh := refreshCookies(resp)
				assert.Empty(t, access, "nenhum access_token")
				assert.Empty(t, refresh, "nenhum refresh_token")
				assert.NotContains(t, strings.ToLower(string(corpo)), "inativ")
				corpos = append(corpos, string(corpo))
			}
			assert.Equal(t, corpos[0], corpos[1], "segunda chamada idêntica")
			assert.NoError(t, mock.ExpectationsWereMet(), "UPDATE com motivo inativacao em cada chamada")
		})
	}
}

// O 401 do refresh de inativo é idêntico ao de um token revogado comum.
func TestSEC22_HTTP_RefreshInativoIgualARevogado(t *testing.T) {
	server, mock, _ := sec2xServidor(t, func(*config.Config) {})
	url := server.URL + "/api/auth/refresh"

	sec22ExpectRefreshInativo(mock, nil)
	_, inativo := refreshSemEmissao(t, url)

	mock.ExpectQuery(findRefreshSQL).WillReturnRows(sqlmock.NewRows(refreshTokenCols).
		AddRow(int64(10), int64(3), hashRefresh(refreshTokenTexto), time.Now().Add(time.Hour), time.Now().Add(-time.Hour), "ip", "ua", "logout"))
	_, revogado := refreshSemEmissao(t, url)

	assert.Equal(t, revogado, inativo)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC22_HTTP_RefreshInativoContaNoRateLimit(t *testing.T) {
	server, mock, _ := sec2xServidor(t, func(*config.Config) {})
	url := server.URL + "/api/auth/refresh"
	for i := 1; i <= 10; i++ {
		sec22ExpectRefreshInativo(mock, nil)
		status, raw := refreshSemEmissao(t, url)
		require.Equal(t, http.StatusUnauthorized, status, "tentativa %d: %s", i, raw)
	}
	status, _ := refreshSemEmissao(t, url)
	assert.Equal(t, http.StatusTooManyRequests, status, "11ª bloqueada por refresh-ip")
	assert.NoError(t, mock.ExpectationsWereMet(), "a bloqueada não consulta o banco")
}
