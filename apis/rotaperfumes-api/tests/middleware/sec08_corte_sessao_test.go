package middleware_test

// SEC-08: corte de sessão (usuarios.tokens_validos_desde). O middleware com
// checagem de usuário recusa (401 "sessão encerrada — faça login novamente")
// access tokens com iat <= corte; sem corte (NULL) o token vale; token sem
// iat com corte preenchido é recusado (fail-closed). O iat é controlado
// explicitamente para não depender do relógio.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/shared/services"
)

const msgSessaoEncerradaEsperada = "sessão encerrada — faça login novamente"

// tokenComIat gera um JWT válido (exp em 1h) com o iat informado; iat nil
// omite o claim.
func tokenComIat(t *testing.T, uid int64, role string, iat *time.Time) string {
	t.Helper()
	cfg := testCfg()
	now := time.Now()
	rc := jwt.RegisteredClaims{
		Issuer:    cfg.JWTIssuer,
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		Subject:   fmt.Sprintf("%d", uid),
	}
	if iat != nil {
		rc.IssuedAt = jwt.NewNumericDate(*iat)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, services.AuthClaims{UserID: uid, Role: role, RegisteredClaims: rc})
	s, err := tok.SignedString([]byte(cfg.JWTSecret))
	require.NoError(t, err)
	return s
}

func ptrTime(t time.Time) *time.Time { return &t }

// serveSemChecker executa o JWTMiddleware (sem consulta ao banco) numa rota
// protegida e devolve o status.
func serveSemChecker(t *testing.T, token string, called *bool) int {
	t.Helper()
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		dummyHandler(w, r)
	})
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	middleware.JWTMiddleware(testCfg(), true, false)(next).ServeHTTP(w, req)
	return w.Code
}

func TestSEC08_CorteDeSessao_Middleware(t *testing.T) {
	// Instante base com fração de segundo, para provar que a comparação é
	// feita em segundos (iat do JWT não tem fração).
	corte := time.Date(2026, 9, 26, 10, 0, 0, 0, time.Local)
	casos := []struct {
		nome       string
		corte      *time.Time
		iat        *time.Time
		wantStatus int
		wantMsg    string
		wantCalled bool
	}{
		{"corte NULL -> 200", nil, ptrTime(corte), 200, "", true},
		{"corte NULL e iat ausente -> 200", nil, nil, 200, "", true},
		{"iat 1h antes do corte -> 401", ptrTime(corte), ptrTime(corte.Add(-time.Hour)), 401, msgSessaoEncerradaEsperada, false},
		{"iat 1s antes do corte -> 401", ptrTime(corte), ptrTime(corte.Add(-time.Second)), 401, msgSessaoEncerradaEsperada, false},
		{"iat == corte (mesmo segundo) -> 401", ptrTime(corte), ptrTime(corte), 401, msgSessaoEncerradaEsperada, false},
		{"iat no mesmo segundo, corte com fração -> 401", ptrTime(corte.Add(900 * time.Millisecond)), ptrTime(corte), 401, msgSessaoEncerradaEsperada, false},
		{"iat 1s depois do corte -> 200", ptrTime(corte), ptrTime(corte.Add(time.Second)), 200, "", true},
		{"iat 1h depois do corte -> 200", ptrTime(corte), ptrTime(corte.Add(time.Hour)), 200, "", true},
		{"corte preenchido e iat ausente -> 401 (fail-closed)", ptrTime(corte), nil, 401, msgSessaoEncerradaEsperada, false},
		{"corte em outro fuso, mesmo instante -> 401", ptrTime(corte.UTC()), ptrTime(corte), 401, msgSessaoEncerradaEsperada, false},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			chk := &mockChecker{status: middleware.UserStatus{Ativo: true, Role: "normal", TokensValidosDesde: tc.corte}}
			w, called := serveWithCheck(t, chk, true, false, tokenComIat(t, 7, "normal", tc.iat))

			assert.Equal(t, tc.wantStatus, w.Code, w.Body.String())
			assert.Equal(t, tc.wantCalled, called)
			assert.Equal(t, 1, chk.calls)
			if tc.wantMsg != "" {
				assert.JSONEq(t, `{"success":false,"error":"`+tc.wantMsg+`"}`, w.Body.String())
			} else {
				assert.Equal(t, "7", w.Header().Get("X-User-ID"))
			}
		})
	}
}

// A precedência das regras: erro do checker (500), inexistente/inativo
// ("usuário inativo") vêm antes do corte; o corte vem antes do requireAdmin.
func TestSEC08_CorteDeSessao_Precedencia(t *testing.T) {
	corte := time.Now().Truncate(time.Second)
	antigo := corte.Add(-time.Minute)
	casos := []struct {
		nome         string
		chk          *mockChecker
		requireAdmin bool
		wantStatus   int
		wantMsg      string
	}{
		{"erro do checker -> 500 mesmo com corte", &mockChecker{err: errors.New("db down"),
			status: middleware.UserStatus{TokensValidosDesde: &corte}}, false, 500, "erro interno"},
		{"usuário inexistente -> 401 usuário inativo", &mockChecker{err: middleware.ErrUserNotFound}, false, 401, "usuário inativo"},
		{"inativo com corte -> mensagem de inativo", &mockChecker{status: middleware.UserStatus{Ativo: false, Role: "normal",
			TokensValidosDesde: &corte}}, false, 401, "usuário inativo"},
		{"token antigo em rota admin -> 401 (não 403)", &mockChecker{status: middleware.UserStatus{Ativo: true, Role: "normal",
			TokensValidosDesde: &corte}}, true, 401, msgSessaoEncerradaEsperada},
		{"admin com token antigo em rota admin -> 401", &mockChecker{status: middleware.UserStatus{Ativo: true, Role: "admin",
			TokensValidosDesde: &corte}}, true, 401, msgSessaoEncerradaEsperada},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			w, called := serveWithCheck(t, tc.chk, true, tc.requireAdmin, tokenComIat(t, 7, "admin", &antigo))
			assert.Equal(t, tc.wantStatus, w.Code)
			assert.False(t, called)
			assert.Contains(t, w.Body.String(), tc.wantMsg)
			assert.NotContains(t, w.Body.String(), "db down")
		})
	}
}

// Rota pública não consulta o checker, então o corte não se aplica a ela.
func TestSEC08_CorteDeSessao_RotaPublicaIgnoraCorte(t *testing.T) {
	corte := time.Now().Add(time.Hour)
	antigo := time.Now().Add(-time.Hour)
	chk := &mockChecker{status: middleware.UserStatus{Ativo: true, Role: "normal", TokensValidosDesde: &corte}}
	w, called := serveWithCheck(t, chk, false, false, tokenComIat(t, 7, "normal", &antigo))
	assert.Equal(t, 200, w.Code)
	assert.True(t, called)
	assert.Equal(t, 0, chk.calls)
}

// JWTMiddleware sem checker (só JWT) não conhece o corte.
func TestSEC08_JWTMiddlewareSemChecker_NaoAplicaCorte(t *testing.T) {
	antigo := time.Now().Add(-time.Hour)
	called := false
	w := serveSemChecker(t, tokenComIat(t, 7, "normal", &antigo), &called)
	assert.Equal(t, 200, w)
	assert.True(t, called)
}

// DBUserStatusChecker repassa o corte lido do banco (NULL -> nil).
func TestSEC08_DBUserStatusChecker_RepassaCorte(t *testing.T) {
	corte := time.Date(2026, 9, 26, 10, 0, 0, 0, time.Local)
	casos := []struct {
		nome  string
		valor any
		want  *time.Time
	}{
		{"NULL -> nil", nil, nil},
		{"preenchido -> ponteiro", corte, &corte},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newCheckerDB(t)
			mock.ExpectQuery(statusQueryRegex).WithArgs(int64(3)).
				WillReturnRows(sqlmock.NewRows([]string{"ativo", "role", "tokens_validos_desde"}).AddRow(true, "admin", tc.valor))

			st, err := middleware.NewDBUserStatusChecker(db).CheckUserStatus(context.Background(), 3)
			require.NoError(t, err)
			assert.True(t, st.Ativo)
			assert.Equal(t, "admin", st.Role)
			if tc.want == nil {
				assert.Nil(t, st.TokensValidosDesde)
			} else {
				require.NotNil(t, st.TokensValidosDesde)
				assert.True(t, tc.want.Equal(*st.TokensValidosDesde))
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// Leitura com erro de scan (tipo incompatível na coluna) vira erro 500 no checker.
func TestSEC08_DBUserStatusChecker_CorteInvalido_Erro(t *testing.T) {
	db, mock := newCheckerDB(t)
	mock.ExpectQuery(statusQueryRegex).WithArgs(int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"ativo", "role", "tokens_validos_desde"}).AddRow(true, "admin", "nao-e-data"))

	_, err := middleware.NewDBUserStatusChecker(db).CheckUserStatus(context.Background(), 3)
	require.Error(t, err)
	assert.NotErrorIs(t, err, middleware.ErrUserNotFound)
	assert.NotErrorIs(t, err, sql.ErrNoRows)
}
