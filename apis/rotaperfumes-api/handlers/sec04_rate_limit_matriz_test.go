package handlers_test

// SEC-04 (TestBrain, Lote 5): matriz "conta / não conta" do refreshLimiter por
// IP. Para cada tipo de falha, 10 tentativas seguidas e depois:
//   - se conta: a 11ª recebe 429 com Retry-After, sem consultar o banco;
//   - se não conta: um refresh válido em seguida recebe 200.
// Em todos os casos o 401 tem o mesmo formato e não emite token.

import (
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefresh_SEC04_MatrizRateLimit(t *testing.T) {
	expectToken := func(expiresAt time.Time, revokedAt any) func(sqlmock.Sqlmock) {
		return func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery(findRefreshSQL).
				WithArgs(hashRefresh(refreshTokenTexto)).
				WillReturnRows(sqlmock.NewRows(refreshTokenCols).
					AddRow(int64(10), int64(5), hashRefresh(refreshTokenTexto), expiresAt, revokedAt, "127.0.0.1", "go-test"))
		}
	}
	futuro := func() time.Time { return time.Now().Add(time.Hour) }

	cases := []struct {
		nome    string
		setup   func() func(sqlmock.Sqlmock)
		wantErr string
		conta   bool
	}{
		{"não encontrado conta", func() func(sqlmock.Sqlmock) {
			return func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(findRefreshSQL).WillReturnRows(sqlmock.NewRows(refreshTokenCols))
			}
		}, "", true},
		{"expirado conta", func() func(sqlmock.Sqlmock) {
			return expectToken(time.Now().Add(-time.Second), nil)
		}, "refresh token expirado", true},
		{"expirado e revogado há 5s conta (expiração tem precedência)", func() func(sqlmock.Sqlmock) {
			return expectToken(time.Now().Add(-time.Second), time.Now().Add(-5*time.Second))
		}, "refresh token expirado", true},
		{"revogado há 31s conta", func() func(sqlmock.Sqlmock) {
			return expectToken(futuro(), time.Now().Add(-31*time.Second))
		}, "refresh token revogado", true},
		{"revogado há 1h conta", func() func(sqlmock.Sqlmock) {
			return expectToken(futuro(), time.Now().Add(-time.Hour))
		}, "refresh token revogado", true},
		{"revoked_at 2min no futuro (relógio muito adiantado) conta", func() func(sqlmock.Sqlmock) {
			return expectToken(futuro(), time.Now().Add(2*time.Minute))
		}, "refresh token revogado", true},
		{"revogado agora não conta", func() func(sqlmock.Sqlmock) {
			return expectToken(futuro(), time.Now())
		}, "refresh token revogado", false},
		{"revogado há 25s não conta", func() func(sqlmock.Sqlmock) {
			return expectToken(futuro(), time.Now().Add(-25*time.Second))
		}, "refresh token revogado", false},
		{"revoked_at 10s no futuro (DATETIME arredondado / relógio do banco) não conta", func() func(sqlmock.Sqlmock) {
			return expectToken(futuro(), time.Now().Add(10*time.Second))
		}, "refresh token revogado", false},
	}

	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()
			url := server.URL + "/api/auth/refresh"

			for i := 0; i < 10; i++ {
				tc.setup()(mock)
				status, raw := refreshSemEmissao(t, url)
				require.Equal(t, http.StatusUnauthorized, status, "tentativa %d: %s", i+1, raw)
				if tc.wantErr != "" {
					assert.Equal(t, tc.wantErr, decodeResponse(t, []byte(raw))["error"])
				}
			}
			require.NoError(t, mock.ExpectationsWereMet())

			if tc.conta {
				status, _ := refreshSemEmissao(t, url)
				assert.Equal(t, http.StatusTooManyRequests, status)
			} else {
				expectRefreshSucesso(mock)
				assertRefreshOK(t, url)
			}
			assert.NoError(t, mock.ExpectationsWereMet(), "com IP bloqueado o banco não é consultado")
		})
	}
}
