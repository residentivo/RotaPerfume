package handlers

// Testes unitários (white-box) de resolverVendedorScope,
// resolverVendedorScopeBase e responderErroEscopo. O contexto (userID/role)
// é injetado pelo JWTMiddleware real, como em produção.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/shared/config"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

const (
	reScopeUsuario  = `SELECT id_vendedor FROM usuarios WHERE id = \? LIMIT 1`
	reScopeDeslig   = `SELECT data_desligamento IS NOT NULL FROM vendedores WHERE id = \? LIMIT 1`
	scopeTestUserID = int64(7)
)

func scopeCfg() *config.Config {
	return &config.Config{JWTSecret: "scope-internal-secret", JWTIssuer: "scope-test", JWTTTL: time.Hour, BCryptCost: 4}
}

// runComJWT executa fn dentro de uma requisição autenticada pelo
// JWTMiddleware real (role/uid no token).
func runComJWT(t *testing.T, role string, fn func(w http.ResponseWriter, r *http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	cfg := scopeCfg()
	tok, err := sharedsvc.NewAuthService().GenerateJWT(cfg, scopeTestUserID, role)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/teste", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	middleware.JWTMiddleware(cfg, true, false)(http.HandlerFunc(fn)).ServeHTTP(rec, req)
	return rec
}

func TestResolverVendedorScope_Matriz(t *testing.T) {
	dbErr := errors.New("db down")

	casos := []struct {
		nome       string
		role       string
		setup      func(m sqlmock.Sqlmock)
		want       vendedorScope
		wantErr    error // errVendedorDesligado ou nil
		wantErrAny bool  // erro genérico (500)
	}{
		{
			nome:  "admin: sem nenhuma consulta",
			role:  "admin",
			setup: func(m sqlmock.Sqlmock) {},
			want:  vendedorScope{Restrito: false},
		},
		{
			nome: "normal ativo",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reScopeUsuario).WithArgs(scopeTestUserID).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(10))
				m.ExpectQuery(reScopeDeslig).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"d"}).AddRow(0))
			},
			want: vendedorScope{Restrito: true, VendedorID: 10},
		},
		{
			nome: "normal desligado → errVendedorDesligado",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reScopeUsuario).WithArgs(scopeTestUserID).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(10))
				m.ExpectQuery(reScopeDeslig).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"d"}).AddRow(1))
			},
			wantErr: errVendedorDesligado,
		},
		{
			nome: "vinculo orfao (IsDesligado ErrNotFound) → sem vendedor",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reScopeUsuario).WithArgs(scopeTestUserID).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(10))
				m.ExpectQuery(reScopeDeslig).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"d"}))
			},
			want: vendedorScope{Restrito: true, VendedorID: 0},
		},
		{
			nome: "id_vendedor NULL → sem vendedor, sem consultar desligado",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reScopeUsuario).WithArgs(scopeTestUserID).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(nil))
			},
			want: vendedorScope{Restrito: true, VendedorID: 0},
		},
		{
			nome: "id_vendedor <= 0 → sem vendedor, sem consultar desligado",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reScopeUsuario).WithArgs(scopeTestUserID).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(0))
			},
			want: vendedorScope{Restrito: true, VendedorID: 0},
		},
		{
			nome: "usuario inexistente → sem vendedor",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reScopeUsuario).WithArgs(scopeTestUserID).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}))
			},
			want: vendedorScope{Restrito: true, VendedorID: 0},
		},
		{
			nome: "erro ao ler id_vendedor → erro (500)",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reScopeUsuario).WithArgs(scopeTestUserID).WillReturnError(dbErr)
			},
			wantErrAny: true,
		},
		{
			nome: "erro ao checar desligado → erro (500)",
			role: "normal",
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(reScopeUsuario).WithArgs(scopeTestUserID).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(10))
				m.ExpectQuery(reScopeDeslig).WithArgs(int64(10)).WillReturnError(dbErr)
			},
			wantErrAny: true,
		},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			defer db.Close()
			c.setup(mock)

			var got vendedorScope
			var gotErr error
			rec := runComJWT(t, c.role, func(w http.ResponseWriter, r *http.Request) {
				got, gotErr = resolverVendedorScope(r, db)
			})
			require.Equal(t, http.StatusOK, rec.Code, "middleware deve deixar passar")

			switch {
			case c.wantErr != nil:
				assert.ErrorIs(t, gotErr, c.wantErr)
				assert.Equal(t, vendedorScope{}, got)
			case c.wantErrAny:
				require.Error(t, gotErr)
				assert.NotErrorIs(t, gotErr, errVendedorDesligado)
				assert.ErrorIs(t, gotErr, dbErr)
				assert.Equal(t, vendedorScope{}, got)
			default:
				assert.NoError(t, gotErr)
				assert.Equal(t, c.want, got)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestResolverVendedorScopeBase_NaoChecaDesligado garante que a variante do
// Dashboard NÃO consulta data_desligamento (o Dashboard trata o desligado
// por conta própria, com 200 zerado).
func TestResolverVendedorScopeBase_NaoChecaDesligado(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery(reScopeUsuario).WithArgs(scopeTestUserID).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(10))

	var got vendedorScope
	var gotErr error
	runComJWT(t, "normal", func(w http.ResponseWriter, r *http.Request) {
		got, gotErr = resolverVendedorScopeBase(r.Context(), db)
	})
	assert.NoError(t, gotErr)
	assert.Equal(t, vendedorScope{Restrito: true, VendedorID: 10}, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestResolverVendedorScope_SemCache chama o resolver repetidas vezes na
// mesma instância de DB, alternando o estado do vendedor: cada chamada
// consulta o banco e reflete o estado atual.
func TestResolverVendedorScope_SemCache(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	estados := []bool{true, false, true, false}
	for i, desligado := range estados {
		v := 0
		if desligado {
			v = 1
		}
		mock.ExpectQuery(reScopeUsuario).WithArgs(scopeTestUserID).WillReturnRows(sqlmock.NewRows([]string{"id_vendedor"}).AddRow(10))
		mock.ExpectQuery(reScopeDeslig).WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"d"}).AddRow(v))

		var gotErr error
		runComJWT(t, "normal", func(w http.ResponseWriter, r *http.Request) {
			_, gotErr = resolverVendedorScope(r, db)
		})
		if desligado {
			assert.ErrorIs(t, gotErr, errVendedorDesligado, "chamada %d", i+1)
		} else {
			assert.NoError(t, gotErr, "chamada %d", i+1)
		}
		require.NoError(t, mock.ExpectationsWereMet(), "chamada %d deve consultar o banco", i+1)
	}
}

// TestResolverVendedorScope_SemUsuarioNoContexto: sem JWT no contexto (não
// deveria ocorrer em rota protegida) o escopo é o mais restritivo possível,
// sem nenhuma consulta.
func TestResolverVendedorScope_SemUsuarioNoContexto(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer db.Close()

	got, err := resolverVendedorScope(httptest.NewRequest(http.MethodGet, "/x", nil), db)
	assert.NoError(t, err)
	assert.Equal(t, vendedorScope{Restrito: true, VendedorID: 0}, got)
	assert.True(t, got.SemAcesso())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestResponderErroEscopo(t *testing.T) {
	casos := []struct {
		nome       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{"desligado", errVendedorDesligado, http.StatusForbidden, msgVendedorDesligado},
		{"desligado embrulhado", fmt.Errorf("ctx: %w", errVendedorDesligado), http.StatusForbidden, msgVendedorDesligado},
		{"erro de banco", errors.New("db down"), http.StatusInternalServerError, "erro interno"},
		{"sql.ErrNoRows não vira 403", sql.ErrNoRows, http.StatusInternalServerError, "erro interno"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			rec := httptest.NewRecorder()
			responderErroEscopo(rec, "[teste]", c.err)

			assert.Equal(t, c.wantStatus, rec.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, false, body["success"])
			assert.Equal(t, c.wantMsg, body["error"])
			assert.NotContains(t, rec.Body.String(), "db down", "detalhe interno não pode vazar")
		})
	}
}

func TestMsgVendedorDesligado_Contrato(t *testing.T) {
	assert.Equal(t, "acesso bloqueado: vendedor desligado", msgVendedorDesligado)
}

func TestVendedorScope_PermiteVendedorESemAcesso(t *testing.T) {
	casos := []struct {
		nome        string
		scope       vendedorScope
		vendedorID  int64
		wantPermite bool
		wantSem     bool
	}{
		{"admin permite qualquer vendedor", vendedorScope{Restrito: false}, 99, true, false},
		{"restrito mesmo vendedor", vendedorScope{Restrito: true, VendedorID: 10}, 10, true, false},
		{"restrito outro vendedor", vendedorScope{Restrito: true, VendedorID: 10}, 11, false, false},
		{"restrito sem vendedor (id 0)", vendedorScope{Restrito: true, VendedorID: 0}, 0, false, true},
		{"restrito id negativo", vendedorScope{Restrito: true, VendedorID: -1}, -1, false, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			assert.Equal(t, c.wantPermite, c.scope.PermiteVendedor(c.vendedorID))
			assert.Equal(t, c.wantSem, c.scope.SemAcesso())
		})
	}
}
