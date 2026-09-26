package middleware_test

// SEC-06: JWTMiddlewareWithUserCheck consulta ativo/role do usuário no banco
// (via UserStatusChecker) a cada request protegido, com regras fail-closed.

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
)

// mockChecker registra as chamadas e devolve o status/erro configurado.
type mockChecker struct {
	status middleware.UserStatus
	err    error
	calls  int
	lastID int64
}

func (m *mockChecker) CheckUserStatus(_ context.Context, userID int64) (middleware.UserStatus, error) {
	m.calls++
	m.lastID = userID
	return m.status, m.err
}

// serveWithCheck executa o middleware com checagem e informa se o handler
// downstream foi chamado.
func serveWithCheck(t *testing.T, checker middleware.UserStatusChecker, protected, requireAdmin bool, token string) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		dummyHandler(w, r)
	})
	req := httptest.NewRequest("GET", "/protected", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	middleware.JWTMiddlewareWithUserCheck(testCfg(), checker, protected, requireAdmin)(next).ServeHTTP(w, req)
	return w, called
}

func TestUserCheck_UsuarioAtivo_200ComRoleDoBanco(t *testing.T) {
	cfg := testCfg()
	chk := &mockChecker{status: middleware.UserStatus{Ativo: true, Role: "normal"}}
	w, called := serveWithCheck(t, chk, true, false, generateToken(t, cfg, 7, "normal", time.Hour, ""))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, called)
	assert.Equal(t, 1, chk.calls)
	assert.Equal(t, int64(7), chk.lastID)
	assert.Equal(t, "7", w.Header().Get("X-User-ID"))
	assert.Equal(t, "normal", w.Header().Get("X-User-Role"))
}

func TestUserCheck_UsuarioInativo_401(t *testing.T) {
	cfg := testCfg()
	chk := &mockChecker{status: middleware.UserStatus{Ativo: false, Role: "normal"}}
	w, called := serveWithCheck(t, chk, true, false, generateToken(t, cfg, 7, "normal", time.Hour, ""))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, called, "handler não pode ser chamado para usuário inativo")
	assert.Contains(t, w.Body.String(), "usuário inativo")
}

func TestUserCheck_UsuarioInexistente_401(t *testing.T) {
	cfg := testCfg()
	chk := &mockChecker{err: middleware.ErrUserNotFound}
	w, called := serveWithCheck(t, chk, true, false, generateToken(t, cfg, 99, "admin", time.Hour, ""))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, called)
	assert.Contains(t, w.Body.String(), "usuário inativo")
}

func TestUserCheck_ErroDeBanco_500SemChamarHandler(t *testing.T) {
	cfg := testCfg()
	chk := &mockChecker{err: errors.New("db down")}
	w, called := serveWithCheck(t, chk, true, false, generateToken(t, cfg, 7, "normal", time.Hour, ""))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.False(t, called)
	assert.NotContains(t, w.Body.String(), "db down", "detalhe do erro não pode vazar")
}

func TestUserCheck_AdminRebaixado_403EmRotaAdmin(t *testing.T) {
	cfg := testCfg()
	chk := &mockChecker{status: middleware.UserStatus{Ativo: true, Role: "normal"}}
	w, called := serveWithCheck(t, chk, true, true, generateToken(t, cfg, 1, "admin", time.Hour, ""))

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, called)
	assert.Contains(t, w.Body.String(), "acesso restrito a administradores")
}

func TestUserCheck_AdminRebaixado_RotaComum_ContextoComRoleDoBanco(t *testing.T) {
	cfg := testCfg()
	chk := &mockChecker{status: middleware.UserStatus{Ativo: true, Role: "normal"}}
	w, called := serveWithCheck(t, chk, true, false, generateToken(t, cfg, 1, "admin", time.Hour, ""))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, called)
	assert.Equal(t, "normal", w.Header().Get("X-User-Role"), "role do banco prevalece sobre o do token")
}

func TestUserCheck_PromovidoNoBanco_PassaEmRotaAdmin(t *testing.T) {
	cfg := testCfg()
	chk := &mockChecker{status: middleware.UserStatus{Ativo: true, Role: "admin"}}
	w, called := serveWithCheck(t, chk, true, true, generateToken(t, cfg, 5, "normal", time.Hour, ""))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, called)
	assert.Equal(t, "admin", w.Header().Get("X-User-Role"))
}

func TestUserCheck_JWTInvalidoOuAusente_401SemConsultarBanco(t *testing.T) {
	cfg := testCfg()
	casos := map[string]string{
		"ausente":    "",
		"assinatura": generateToken(t, cfg, 7, "normal", time.Hour, "outro-segredo"),
		"expirado":   generateToken(t, cfg, 7, "normal", -time.Minute, ""),
		"lixo":       "nao.e.jwt",
	}
	for nome, tok := range casos {
		t.Run(nome, func(t *testing.T) {
			chk := &mockChecker{status: middleware.UserStatus{Ativo: true, Role: "admin"}}
			w, called := serveWithCheck(t, chk, true, false, tok)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.False(t, called)
			assert.Equal(t, 0, chk.calls, "JWT inválido não deve consultar o banco")
		})
	}
}

func TestUserCheck_RotaPublica_NaoConsultaBanco(t *testing.T) {
	cfg := testCfg()
	chk := &mockChecker{err: errors.New("não deveria ser chamado")}
	w, called := serveWithCheck(t, chk, false, false, generateToken(t, cfg, 7, "normal", time.Hour, ""))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, called)
	assert.Equal(t, 0, chk.calls)
}

func TestUserCheck_SemCache_ConsultaACadaRequest(t *testing.T) {
	cfg := testCfg()
	chk := &mockChecker{status: middleware.UserStatus{Ativo: true, Role: "normal"}}
	tok := generateToken(t, cfg, 7, "normal", time.Hour, "")

	w1, _ := serveWithCheck(t, chk, true, false, tok)
	assert.Equal(t, http.StatusOK, w1.Code)

	chk.status.Ativo = false // inativado entre os dois requests
	w2, called := serveWithCheck(t, chk, true, false, tok)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
	assert.False(t, called)
	assert.Equal(t, 2, chk.calls)
}

func TestUserCheck_CheckerNil_Panica(t *testing.T) {
	assert.Panics(t, func() {
		middleware.JWTMiddlewareWithUserCheck(testCfg(), nil, true, false)
	})
}

// ------------------------------------------------------------------
// DBUserStatusChecker (sqlmock)
// ------------------------------------------------------------------

const statusQueryRegex = `SELECT ativo, role FROM usuarios WHERE id = \? LIMIT 1`

func newCheckerDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func TestDBUserStatusChecker_Encontrado(t *testing.T) {
	db, mock := newCheckerDB(t)
	mock.ExpectQuery(statusQueryRegex).WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"ativo", "role"}).AddRow(false, "normal"))

	st, err := middleware.NewDBUserStatusChecker(db).CheckUserStatus(context.Background(), 3)

	require.NoError(t, err)
	assert.Equal(t, middleware.UserStatus{Ativo: false, Role: "normal"}, st)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDBUserStatusChecker_Inexistente_ErrUserNotFound(t *testing.T) {
	db, mock := newCheckerDB(t)
	mock.ExpectQuery(statusQueryRegex).WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"ativo", "role"}))

	_, err := middleware.NewDBUserStatusChecker(db).CheckUserStatus(context.Background(), 3)

	assert.ErrorIs(t, err, middleware.ErrUserNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDBUserStatusChecker_ErroDeBanco_Repassado(t *testing.T) {
	db, mock := newCheckerDB(t)
	mock.ExpectQuery(statusQueryRegex).WithArgs(int64(3)).WillReturnError(sql.ErrConnDone)

	_, err := middleware.NewDBUserStatusChecker(db).CheckUserStatus(context.Background(), 3)

	assert.ErrorIs(t, err, sql.ErrConnDone)
	assert.NotErrorIs(t, err, middleware.ErrUserNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}
