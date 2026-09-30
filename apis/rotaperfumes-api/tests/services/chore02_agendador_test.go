package services_test

// CHORE-02 (Lote 12): IniciarLimpezaRefreshTokens.
//   - roda o CleanupExpired uma vez no início e a cada intervalo;
//   - cancel() fecha o canal done rapidamente (< 1s);
//   - intervalo <= 0 → canal já fechado, nada executa;
//   - panic no cleanup é recuperado e logado, sem derrubar o processo nem
//     parar as execuções seguintes;
//   - ctx já cancelado → não executa.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

// esperarAte faz polling de cond até o prazo.
func esperarAte(t *testing.T, prazo time.Duration, cond func() bool, msg string) {
	t.Helper()
	fim := time.Now().Add(prazo)
	for time.Now().Before(fim) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("tempo esgotado: %s", msg)
}

func fechaEm(done <-chan struct{}, prazo time.Duration) bool {
	select {
	case <-done:
		return true
	case <-time.After(prazo):
		return false
	}
}

func TestCHORE02_Agendador_IntervaloDesativado(t *testing.T) {
	for _, intervalo := range []time.Duration{0, -time.Second, -time.Hour} {
		t.Run(intervalo.String(), func(t *testing.T) {
			logs := sec09CapturarLog(t)
			db, mock := newRefreshTokenTestDB(t)
			done := services.IniciarLimpezaRefreshTokens(context.Background(), db, chore02Svc(), intervalo)
			require.True(t, fechaEm(done, time.Millisecond), "canal já fechado")
			time.Sleep(30 * time.Millisecond)
			assert.NoError(t, mock.ExpectationsWereMet())
			out := logs.String()
			assert.Contains(t, out, "limpeza periódica desativada")
			assert.NotContains(t, out, "tokens removidos", "nada executa")
		})
	}
}

func TestCHORE02_Agendador_ExecutaNoInicioEEncerraNoCancel(t *testing.T) {
	logs := sec09CapturarLog(t)
	db, mock := newRefreshTokenTestDB(t)
	mock.ExpectExec(chore02DeleteSQL).WithArgs(tempoIgual{chore02T0.Add(-720 * time.Hour)}, 1000).
		WillReturnResult(sqlmock.NewResult(0, 7))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := services.IniciarLimpezaRefreshTokens(ctx, db, chore02Svc(), time.Hour)

	esperarAte(t, 2*time.Second, func() bool { return mock.ExpectationsWereMet() == nil }, "execução imediata")
	esperarAte(t, time.Second, func() bool { return strings.Contains(logs.String(), "7 tokens removidos") }, "log da execução")
	assert.False(t, fechaEm(done, 20*time.Millisecond), "segue rodando até o cancel")

	inicio := time.Now()
	cancel()
	require.True(t, fechaEm(done, time.Second), "cancel fecha o done em < 1s")
	assert.Less(t, time.Since(inicio), time.Second)
	out := logs.String()
	assert.Contains(t, out, "limpeza periódica iniciada (intervalo=1h0m0s)")
	assert.Contains(t, out, "limpeza periódica encerrada")
	assert.Equal(t, 1, strings.Count(out, "tokens removidos"), "só a execução inicial (intervalo 1h)")
}

func TestCHORE02_Agendador_IntervaloCurtoExecutaMaisVezes(t *testing.T) {
	logs := sec09CapturarLog(t)
	db, mock := newRefreshTokenTestDB(t)
	const execucoes = 4
	for i := 0; i < execucoes; i++ {
		mock.ExpectExec(chore02DeleteSQL).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := services.IniciarLimpezaRefreshTokens(ctx, db, chore02Svc(), 20*time.Millisecond)

	esperarAte(t, 3*time.Second, func() bool { return mock.ExpectationsWereMet() == nil }, "4 execuções com intervalo de 20ms")
	cancel()
	require.True(t, fechaEm(done, time.Second))
	assert.GreaterOrEqual(t, strings.Count(logs.String(), "0 tokens removidos"), execucoes)
}

// Erro de banco numa execução é logado e as seguintes continuam.
func TestCHORE02_Agendador_ErroNaoPara(t *testing.T) {
	logs := sec09CapturarLog(t)
	db, mock := newRefreshTokenTestDB(t)
	mock.ExpectExec(chore02DeleteSQL).WillReturnError(assert.AnError)
	mock.ExpectExec(chore02DeleteSQL).WillReturnResult(sqlmock.NewResult(0, 2))
	ctx, cancel := context.WithCancel(context.Background())
	done := services.IniciarLimpezaRefreshTokens(ctx, db, chore02Svc(), 20*time.Millisecond)

	esperarAte(t, 3*time.Second, func() bool { return mock.ExpectationsWereMet() == nil }, "erro e depois sucesso")
	cancel()
	require.True(t, fechaEm(done, time.Second))
	out := logs.String()
	assert.Contains(t, out, "[refresh] cleanup: erro:")
	assert.Contains(t, out, "2 tokens removidos")
}

// svc nil faz o CleanupExpired entrar em panic (nil pointer): o agendador
// recupera, loga e continua nas próximas execuções.
func TestCHORE02_Agendador_PanicNaoDerrubaProcesso(t *testing.T) {
	logs := sec09CapturarLog(t)
	db, _ := newRefreshTokenTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := services.IniciarLimpezaRefreshTokens(ctx, db, nil, 20*time.Millisecond)

	esperarAte(t, 3*time.Second, func() bool {
		return strings.Count(logs.String(), "[refresh] cleanup: panic recuperado") >= 3
	}, "3 panics recuperados (execução inicial + ticks)")
	cancel()
	require.True(t, fechaEm(done, time.Second), "a goroutine segue viva e encerra no cancel")
	assert.Contains(t, logs.String(), "limpeza periódica encerrada")
}

func TestCHORE02_Agendador_ContextoJaCancelado(t *testing.T) {
	logs := sec09CapturarLog(t)
	db, mock := newRefreshTokenTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := services.IniciarLimpezaRefreshTokens(ctx, db, chore02Svc(), time.Hour)
	require.True(t, fechaEm(done, time.Second))
	assert.NoError(t, mock.ExpectationsWereMet(), "nenhum DELETE com ctx cancelado")
	assert.NotContains(t, logs.String(), "tokens removidos")
	assert.NotContains(t, logs.String(), "interrompido")
}
