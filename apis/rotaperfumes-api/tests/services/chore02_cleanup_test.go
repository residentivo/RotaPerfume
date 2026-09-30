package services_test

// CHORE-02 (Lote 12): RefreshTokenService.CleanupExpired com relógio fixo.
//   - corte = now() - retenção; retenção padrão 720h; SetRetencao(d <= 0)
//     mantém o padrão; retenção < 24h vira 24h dentro do CleanupExpired.
//   - DELETE em lotes de RefreshCleanupLote (1000) até um lote vir
//     incompleto; args exatamente (corte, 1000).
//   - erro no N-ésimo lote → (soma parcial, erro);
//   - ctx cancelado → (soma parcial, ctx.Err()), sem novo DELETE;
//   - o log "[refresh] cleanup: N tokens removidos (corte=...)" sai sempre,
//     mesmo com 0.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

const chore02DeleteSQL = `DELETE FROM refresh_tokens WHERE expires_at < \? ORDER BY id LIMIT \?`

var chore02T0 = time.Date(2026, 9, 27, 3, 0, 0, 0, time.Local)

func chore02Svc() *services.RefreshTokenService {
	return services.NewRefreshTokenServiceWithClock(func() time.Time { return chore02T0 })
}

func TestCHORE02_Constantes(t *testing.T) {
	assert.Equal(t, 720*time.Hour, services.RefreshTokenRetencaoPadrao)
	assert.Equal(t, 24*time.Hour, services.RefreshTokenRetencaoMinima)
	assert.Equal(t, 1000, services.RefreshCleanupLote)
}

func TestCHORE02_CleanupExpired_Corte(t *testing.T) {
	casos := []struct {
		nome       string
		configurar func(s *services.RefreshTokenService)
		retencao   time.Duration
	}{
		{"padrão (sem Set) → 720h", func(*services.RefreshTokenService) {}, 720 * time.Hour},
		{"Set(48h)", func(s *services.RefreshTokenService) { s.SetRetencao(48 * time.Hour) }, 48 * time.Hour},
		{"Set(24h) no mínimo", func(s *services.RefreshTokenService) { s.SetRetencao(24 * time.Hour) }, 24 * time.Hour},
		{"Set(8760h)", func(s *services.RefreshTokenService) { s.SetRetencao(8760 * time.Hour) }, 8760 * time.Hour},
		{"Set(1h) abaixo do mínimo vira 24h", func(s *services.RefreshTokenService) { s.SetRetencao(time.Hour) }, 24 * time.Hour},
		{"Set(1ns) abaixo do mínimo vira 24h", func(s *services.RefreshTokenService) { s.SetRetencao(time.Nanosecond) }, 24 * time.Hour},
		{"Set(0) mantém o padrão", func(s *services.RefreshTokenService) { s.SetRetencao(0) }, 720 * time.Hour},
		{"Set(negativo) mantém o padrão", func(s *services.RefreshTokenService) { s.SetRetencao(-time.Hour) }, 720 * time.Hour},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newRefreshTokenTestDB(t)
			corte := chore02T0.Add(-c.retencao)
			mock.ExpectExec(chore02DeleteSQL).WithArgs(tempoIgual{corte}, services.RefreshCleanupLote).
				WillReturnResult(sqlmock.NewResult(0, 0))

			svc := chore02Svc()
			c.configurar(svc)
			n, err := svc.CleanupExpired(context.Background(), db)
			require.NoError(t, err)
			assert.Zero(t, n)
			assert.False(t, corte.After(chore02T0.Add(-24*time.Hour)), "invariante: nunca apaga token com expires_at >= agora")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// resultadoLote: RowsAffected de cada DELETE, ou erro.
type resultadoLote struct {
	n   int64
	err error
}

func TestCHORE02_CleanupExpired_Lotes(t *testing.T) {
	errBanco := errors.New("deadlock")
	casos := []struct {
		nome      string
		lotes     []resultadoLote
		wantTotal int64
		wantErr   error
		wantLog   string
	}{
		{"1000, 1000, 3 → 3 DELETEs, total 2003", []resultadoLote{{n: 1000}, {n: 1000}, {n: 3}}, 2003, nil, "[refresh] cleanup: 2003 tokens removidos (corte="},
		{"1000, 0 → 2 DELETEs, total 1000", []resultadoLote{{n: 1000}, {n: 0}}, 1000, nil, "[refresh] cleanup: 1000 tokens removidos"},
		{"999 → 1 DELETE", []resultadoLote{{n: 999}}, 999, nil, "[refresh] cleanup: 999 tokens removidos"},
		{"0 → 1 DELETE e o log sai mesmo assim", []resultadoLote{{n: 0}}, 0, nil, "[refresh] cleanup: 0 tokens removidos (corte="},
		{"erro no 1º lote → (0, erro)", []resultadoLote{{err: errBanco}}, 0, errBanco, "[refresh] cleanup: falha após 0 tokens removidos"},
		{"erro no 2º lote → (1000, erro)", []resultadoLote{{n: 1000}, {err: errBanco}}, 1000, errBanco, "[refresh] cleanup: falha após 1000 tokens removidos"},
		{"erro no 3º lote → (2000, erro)", []resultadoLote{{n: 1000}, {n: 1000}, {err: errBanco}}, 2000, errBanco, "falha após 2000 tokens removidos"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			logs := sec09CapturarLog(t)
			db, mock := newRefreshTokenTestDB(t)
			corte := chore02T0.Add(-720 * time.Hour)
			for _, l := range c.lotes {
				e := mock.ExpectExec(chore02DeleteSQL).WithArgs(tempoIgual{corte}, 1000)
				if l.err != nil {
					e.WillReturnError(l.err)
				} else {
					e.WillReturnResult(sqlmock.NewResult(0, l.n))
				}
			}

			n, err := chore02Svc().CleanupExpired(context.Background(), db)
			assert.Equal(t, c.wantTotal, n)
			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet(), "número exato de DELETEs")
			out := logs.String()
			assert.Contains(t, out, c.wantLog)
			assert.Contains(t, out, "corte="+corte.Format(time.RFC3339))
			assert.Equal(t, 1, strings.Count(out, "[refresh] cleanup:"), "um único log por execução")
		})
	}
}

// cancelaNoRowsAffected cancela o ctx quando o repositório lê o
// RowsAffected — ou seja, depois do DELETE e antes do próximo teste do laço.
type cancelaNoRowsAffected struct {
	n      int64
	cancel context.CancelFunc
}

func (r cancelaNoRowsAffected) LastInsertId() (int64, error) { return 0, nil }
func (r cancelaNoRowsAffected) RowsAffected() (int64, error) {
	r.cancel()
	return r.n, nil
}

func TestCHORE02_CleanupExpired_ContextoCancelado(t *testing.T) {
	t.Run("cancelado antes de começar → (0, Canceled), nenhum DELETE", func(t *testing.T) {
		logs := sec09CapturarLog(t)
		db, mock := newRefreshTokenTestDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		n, err := chore02Svc().CleanupExpired(ctx, db)
		assert.Zero(t, n)
		assert.ErrorIs(t, err, context.Canceled)
		assert.NoError(t, mock.ExpectationsWereMet())
		assert.Contains(t, logs.String(), "[refresh] cleanup: interrompido após 0 tokens removidos")
	})

	casos := []struct {
		nome        string
		lotesAntes  int // lotes cheios antes do que cancela
		wantTotal   int64
		wantLogFrag string
	}{
		{"cancelado durante o 1º lote cheio → (1000, Canceled)", 0, 1000, "interrompido após 1000 tokens removidos"},
		{"cancelado durante o 3º lote cheio → (3000, Canceled)", 2, 3000, "interrompido após 3000 tokens removidos"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			logs := sec09CapturarLog(t)
			db, mock := newRefreshTokenTestDB(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			for i := 0; i < c.lotesAntes; i++ {
				mock.ExpectExec(chore02DeleteSQL).WillReturnResult(sqlmock.NewResult(0, 1000))
			}
			mock.ExpectExec(chore02DeleteSQL).WillReturnResult(cancelaNoRowsAffected{n: 1000, cancel: cancel})
			// Nenhum DELETE depois do cancelamento: um DELETE extra não
			// programado viraria erro do sqlmock e mudaria o retorno.

			n, err := chore02Svc().CleanupExpired(ctx, db)
			assert.Equal(t, c.wantTotal, n, "devolve o total parcial")
			assert.ErrorIs(t, err, context.Canceled)
			assert.NoError(t, mock.ExpectationsWereMet())
			assert.Contains(t, logs.String(), c.wantLogFrag)
		})
	}

	t.Run("deadline estourado → DeadlineExceeded", func(t *testing.T) {
		sec09CapturarLog(t)
		db, mock := newRefreshTokenTestDB(t)
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		defer cancel()
		n, err := chore02Svc().CleanupExpired(ctx, db)
		assert.Zero(t, n)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// Muitos lotes: o laço não tem limite artificial de iterações.
func TestCHORE02_CleanupExpired_MuitosLotes(t *testing.T) {
	sec09CapturarLog(t)
	db, mock := newRefreshTokenTestDB(t)
	const cheios = 12
	for i := 0; i < cheios; i++ {
		mock.ExpectExec(chore02DeleteSQL).WillReturnResult(sqlmock.NewResult(0, 1000))
	}
	mock.ExpectExec(chore02DeleteSQL).WillReturnResult(sqlmock.NewResult(0, 1))
	n, err := chore02Svc().CleanupExpired(context.Background(), db)
	require.NoError(t, err)
	assert.Equal(t, int64(cheios*1000+1), n, fmt.Sprintf("%d lotes", cheios+1))
	assert.NoError(t, mock.ExpectationsWereMet())
}
