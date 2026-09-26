package routes_test

// SEC-06: o router real passa o usuário pelo UserStatusChecker em toda rota
// protegida (ativo + role do banco) e não o consulta em rotas públicas.

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/rotaperfumes-api/routes"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

// fakeUserStatusChecker segue a convenção dos testes (user_id 1 = admin,
// demais = normal, todos ativos); status sobrescreve por user_id e err
// força falha de infraestrutura. calls conta as consultas.
type fakeUserStatusChecker struct {
	status map[int64]middleware.UserStatus
	err    error
	calls  *int
}

func (f fakeUserStatusChecker) CheckUserStatus(_ context.Context, userID int64) (middleware.UserStatus, error) {
	if f.calls != nil {
		*f.calls++
	}
	if f.err != nil {
		return middleware.UserStatus{}, f.err
	}
	if st, ok := f.status[userID]; ok {
		return st, nil
	}
	if userID == 1 {
		return middleware.UserStatus{Ativo: true, Role: "admin"}, nil
	}
	return middleware.UserStatus{Ativo: true, Role: "normal"}, nil
}

func newRouterWithChecker(t *testing.T, checker middleware.UserStatusChecker) http.Handler {
	t.Helper()
	cfg := testCfg()
	db, _, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return routes.NewMux(cfg, checker,
		handlers.NewAuthHandler(db, cfg),
		handlers.NewUsuarioHandler(db, cfg, sharedsvc.NewNoopEmailService()),
		handlers.NewDashboardHandler(db, cfg),
		handlers.NewSenhaHistoricoHandler(db),
		handlers.NewVendedorHandler(db, cfg),
		handlers.NewClienteHandler(db, cfg),
		handlers.NewProdutoHandler(db, cfg),
		handlers.NewPedidoHandler(db, cfg),
		handlers.NewPagamentoHandler(db, cfg),
		handlers.NewOportunidadeHandler(db, cfg),
		handlers.NewVisitaHandler(db, cfg),
		handlers.NewEstoqueHandler(db, cfg),
	)
}

func TestSEC06_Router_UsuarioInativo_401(t *testing.T) {
	calls := 0
	h := newRouterWithChecker(t, fakeUserStatusChecker{
		status: map[int64]middleware.UserStatus{2: {Ativo: false, Role: "normal"}},
		calls:  &calls,
	})
	w := serve(h, "GET", "/api/clientes", token(t, 2, "normal"), "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "usuário inativo")
	assert.Equal(t, 1, calls)
}

func TestSEC06_Router_AdminRebaixado_403EmRotaAdmin(t *testing.T) {
	h := newRouterWithChecker(t, fakeUserStatusChecker{
		status: map[int64]middleware.UserStatus{1: {Ativo: true, Role: "normal"}},
	})
	w := serve(h, "GET", "/api/usuarios", token(t, 1, "admin"), "")
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "acesso restrito a administradores")
}

func TestSEC06_Router_ErroNoChecker_500(t *testing.T) {
	h := newRouterWithChecker(t, fakeUserStatusChecker{err: errors.New("db down")})
	w := serve(h, "GET", "/api/vendedores", token(t, 2, "normal"), "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSEC06_Router_RotaPublica_NaoConsultaChecker(t *testing.T) {
	calls := 0
	h := newRouterWithChecker(t, fakeUserStatusChecker{err: errors.New("não deveria ser chamado"), calls: &calls})
	// Logout é público: mesmo com token válido o checker não é consultado.
	w := serve(h, "POST", "/api/auth/logout", token(t, 2, "normal"), `{}`)
	assert.NotEqual(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, 0, calls)
}

func TestSEC06_Router_SemToken_NaoConsultaChecker(t *testing.T) {
	calls := 0
	h := newRouterWithChecker(t, fakeUserStatusChecker{calls: &calls})
	w := serve(h, "GET", "/api/clientes", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, 0, calls)
}

func TestSEC06_NewMux_CheckerNil_Panica(t *testing.T) {
	assert.Panics(t, func() { newRouterWithChecker(t, nil) })
}
