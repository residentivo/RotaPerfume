package handlers_test

// ARQ-03: contrato HTTP de Identidade byte a byte contra o envelope legado
// (map + Encoder), montagem dos handlers e os ramos que só um emissor de JWT
// falho ou um vínculo órfão alcançam.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/rotaperfumes-api/internal/identidadehttp"
	"github.com/rotaperfumes/shared/adapters/ratelimit"
	shareddb "github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"github.com/rotaperfumes/shared/kernel"
)

// envelopeMapLegado serializa como o writeJSON legado (map ordenado pelo
// encoder), para comparar byte a byte.
func envelopeMapLegado(t *testing.T, data map[string]any) string {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, json.NewEncoder(&b).Encode(map[string]any{"success": true, "data": data}))
	return b.String()
}

func TestARQ03_ContratoJSON_Me_ByteAByte(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	criado := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	vend := int64(4)
	mock.ExpectQuery(usuarioByIDSQL).WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows(usuarioCols).
		AddRow(int64(3), "João", "joao@test.com", "hash", "normal", vend, true, false, criado, criado, nil, "Vend"))
	// Vínculo órfão: vendedor inexistente não é "desligado" (sem erro).
	mock.ExpectQuery(`SELECT data_desligamento IS NOT NULL FROM vendedores WHERE id = \? LIMIT 1`).WithArgs(vend).
		WillReturnRows(sqlmock.NewRows([]string{"d"}))

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 3, "normal"))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, envelopeMapLegado(t, map[string]any{
		"id": int64(3), "nome": "João", "email": "joao@test.com", "role": "normal", "ativo": true,
		"id_vendedor": &vend, "created_at": criado, "ultimo_login_at": (*time.Time)(nil), "vendedor_desligado": false,
	}), string(readBody(t, resp)))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestARQ03_ContratoJSON_ListUsuarios_ByteAByte(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	criado := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM usuarios`).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1))
	mock.ExpectQuery(`FROM usuarios u`).WillReturnRows(sqlmock.NewRows(usuarioCols).
		AddRow(int64(2), "Ana", "ana@test.com", "hash", "normal", nil, true, false, criado, criado, nil, nil))

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/usuarios", nil)
	req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), 1, "admin"))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var b bytes.Buffer
	require.NoError(t, json.NewEncoder(&b).Encode(map[string]any{
		"success": true,
		"data": []map[string]any{{"id": int64(2), "id_vendedor": (*int64)(nil), "vendedor_nome": "", "email": "ana@test.com",
			"role": "normal", "ativo": true, "nome": "Ana", "created_at": criado, "ultimo_login_at": (*time.Time)(nil)}},
		"pagination": map[string]any{"page": 1, "limit": 20, "total": 1, "pages": 1},
	}))
	assert.Equal(t, b.String(), string(readBody(t, resp)))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestARQ03_Handlers_MontagemSemCasosPanica(t *testing.T) {
	assert.Panics(t, func() { identidadehttp.NovoAuthHandler(nil, time.Hour, false, true) })
	assert.Panics(t, func() { identidadehttp.NovoUsuarioHandler(nil, false) })
	assert.Panics(t, func() { identidadehttp.NovoSenhaHistoricoHandler(nil) })
}

func TestARQ03_BodyInvalido_ResetPasswordECreateUsuario(t *testing.T) {
	server, db, _ := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	for _, c := range []struct{ path, uidRole string }{{"/api/auth/reset-password", "normal"}, {"/api/usuarios", "admin"}, {"/api/admin/reset-password", "admin"}} {
		uid := int64(2)
		if c.uidRole == "admin" {
			uid = 1
		}
		req, _ := http.NewRequest(http.MethodPost, server.URL+c.path, strings.NewReader("{"))
		req.Header.Set("Authorization", "Bearer "+generateToken(t, testCfg(), uid, c.uidRole))
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, c.path)
		assert.Equal(t, "body JSON inválido", decodeResponse(t, readBody(t, resp))["error"])
		resp.Body.Close()
	}
}

// emissorFalho: o HS256 da biblioteca não falha na prática; o 500 com
// mensagem própria do legado ("erro ao gerar token") só aparece assim.
type emissorFalho struct{}

func (emissorFalho) Emitir(int64, kernel.Role) (string, error) {
	return "", errors.New("falha simulada")
}

type captchaOK struct{}

func (captchaOK) Verificar(_ context.Context, _, _ string) error { return nil }

func TestARQ03_FalhaAoEmitirJWT_MensagensDoLegado(t *testing.T) {
	cfg := testCfg()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	casos := identidadeapp.Novo(identidadeapp.Deps{
		Usuarios: identidademysql.NovoUsuarios(db), Tokens: identidademysql.NovoRefreshTokens(db), Historico: identidademysql.NovoHistoricoSenhas(db),
		Rotacao: shareddb.NewTransactor(db, func(q shareddb.Querier) identidadeapp.TxRotacao {
			return identidadeapp.TxRotacao{Tokens: identidademysql.NovoRefreshTokens(q)}
		}),
		TrocaSenha: shareddb.NewTransactor(db, func(q shareddb.Querier) identidadeapp.TxSenha {
			return identidadeapp.TxSenha{Usuarios: identidademysql.NovoUsuarios(q), Historico: identidademysql.NovoHistoricoSenhas(q),
				Tokens: identidademysql.NovoRefreshTokens(q)}
		}),
		Hasher: identidadecrypto.NovoHasher(cfg.HashSenha), Emissor: emissorFalho{}, Refresh: identidadecrypto.GeradorRefresh{},
		Senhas: identidadecrypto.GeradorSenha{}, Captcha: captchaOK{},
		Limites:    identidadeapp.Limites{Login: ratelimit.Nova(5, time.Minute, time.Minute), Refresh: ratelimit.Nova(5, time.Minute, time.Minute), Reset: ratelimit.Nova(5, time.Minute, time.Minute)},
		Publicador: identidadeapp.SemPublicador{}, Email: &fakeEmailService{}, Vendedores: vendedoresNenhum{}, Relogio: time.Now,
	}, identidadeapp.Opcoes{})
	h := identidadehttp.NovoAuthHandler(casos, cfg.JWTTTL, false, cfg.CookieSecure)

	hash, err := gerarHashSenha(cfg.HashSenha, "senha-correta")
	require.NoError(t, err)
	mock.ExpectQuery(`WHERE\s+u\.email\s+=\s+\?`).WillReturnRows(sqlmock.NewRows(usuarioCols).
		AddRow(int64(1), "A", "a@test.com", hash, "admin", nil, true, false, time.Now(), time.Now(), nil, nil))
	w := httptest.NewRecorder()
	h.Login(w, httptest.NewRequest(http.MethodPost, "/api/auth/login", makeJSON(map[string]string{"email": "a@test.com", "password": "senha-correta", "captchaToken": "x"})))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "erro ao gerar token", decodeResponse(t, w.Body.Bytes())["error"])
	assert.Empty(t, w.Result().Cookies())

	expectRefreshValido(mock, 5)
	expectUsuarioAtivo(mock, 5)
	mock.ExpectBegin()
	mock.ExpectExec(revokeCondSQL).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()
	w = httptest.NewRecorder()
	h.Refresh(w, httptest.NewRequest(http.MethodPost, "/api/auth/refresh", makeJSON(map[string]string{"refresh_token": refreshTokenTexto})))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "erro ao gerar access token", decodeResponse(t, w.Body.Bytes())["error"])
	assert.Empty(t, w.Result().Cookies(), "rollback: nenhum token sai")
	assert.NoError(t, mock.ExpectationsWereMet())
}

type vendedoresNenhum struct{}

func (vendedoresNenhum) Existe(context.Context, int64) (bool, error)    { return false, nil }
func (vendedoresNenhum) Desligado(context.Context, int64) (bool, error) { return false, nil }

type emailQueFalha struct{}

func (emailQueFalha) EnviarSenhaInicial(context.Context, string, string, string) error {
	return errors.New("smtp fora")
}

// Criação com e-mail que falha: 201 com email_enviado=false e o aviso no log
// (a criação não é desfeita; a senha nunca aparece).
func TestARQ03_CreateUsuario_EmailFalhou(t *testing.T) {
	logs := capturarLog(t)
	cfg := testCfg()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	casos := bootstrap.NovaIdentidade(db, cfg, bootstrap.ExtIdentidade{Captcha: captchaOK{}, Publicador: identidadeapp.SemPublicador{}, Email: emailQueFalha{}})
	server := httptest.NewServer(bootstrap.HandlerHTTP(cfg, fakeUserStatusChecker{}, bootstrap.NovosHandlersCom(db, cfg, casos)))
	defer server.Close()

	mock.ExpectExec(`INSERT INTO usuarios`).WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectQuery(`WHERE u\.id = \?`).WillReturnError(sql.ErrConnDone)
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/usuarios", makeJSON(validCreateUsuarioPayload()))
	req.Header.Set("Authorization", "Bearer "+generateToken(t, cfg, 1, "admin"))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, false, decodeResponse(t, readBody(t, resp))["data"].(map[string]any)["email_enviado"])
	assert.Contains(t, logs.String(), "email com senha inicial NÃO foi enviado")
	assert.NoError(t, mock.ExpectationsWereMet())
}
