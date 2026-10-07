package resetpassword_test

// SEC-08: UpsertAdmin e UpsertByEmail gravam o corte de sessão
// (tokens_validos_desde) no UPDATE: time.Time do Go, sem fração de segundo,
// entre o instante anterior e o posterior à chamada.

import (
	"database/sql/driver"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/tools/resetpassword"
)

type corteAgora struct{ antes time.Time }

func (c corteAgora) Match(v driver.Value) bool {
	t, ok := v.(time.Time)
	return ok && t.Nanosecond() == 0 &&
		!t.Before(c.antes.Truncate(time.Second)) && !t.After(time.Now())
}

func TestSEC08_Upserts_GravamCorte(t *testing.T) {
	casos := []struct {
		nome   string
		re     string
		args   func(c corteAgora) []driver.Value
		chamar func(db resetpassword.DB) error
	}{
		{"UpsertAdmin", reUpdateAdmin,
			func(c corteAgora) []driver.Value {
				return []driver.Value{hashDe("Adm@1234"), c, resetpassword.AdminEmail}
			},
			func(db resetpassword.DB) error { return resetpassword.UpsertAdmin(ctx, db, hsTeste, "Adm@1234") }},
		{"UpsertByEmail", reUpdateEmail,
			func(c corteAgora) []driver.Value { return []driver.Value{hashDe("Nova@123"), c, "a@x.com"} },
			func(db resetpassword.DB) error {
				return resetpassword.UpsertByEmail(ctx, db, hsTeste, "a@x.com", "Nova@123", "normal", "A", 0)
			}},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			silenciarLog(t)
			db, mock := novoMock(t)
			c := corteAgora{antes: time.Now()}
			mock.ExpectExec(tc.re).WithArgs(tc.args(c)...).WillReturnResult(sqlmock.NewResult(0, 1))
			require.NoError(t, tc.chamar(db))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSEC08_CorteAgora_Sanidade(t *testing.T) {
	c := corteAgora{antes: time.Now()}
	assert.True(t, c.Match(c.antes.Truncate(time.Second)))
	assert.False(t, c.Match(c.antes.Truncate(time.Second).Add(time.Microsecond)))
	assert.False(t, c.Match(c.antes.Add(-2*time.Second)))
	assert.False(t, c.Match("x"))
}
