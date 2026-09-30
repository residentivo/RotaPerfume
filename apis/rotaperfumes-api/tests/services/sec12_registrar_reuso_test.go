package services_test

// SEC-12 (Lote 12): RefreshTokenService.RegistrarReuso/DesfazerReuso com
// relógio injetado.
//   - agora = now() truncado ao segundo (coluna DATETIME);
//     limite = agora - janela; args do UPDATE: (agora, tokenID, limite).
//   - janela padrão 30m; SetReuseSuppressWindow(d <= 0) mantém o padrão.
//   - 1 linha → (true, agora); 0 → (false, zero); erro → (false, zero, err).

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
)

const (
	sec12MarcarSQL   = `UPDATE refresh_tokens SET reuso_detectado_em = \?\s+WHERE id = \? AND revoked_reason = 'rotacao'\s+AND \(reuso_detectado_em IS NULL OR reuso_detectado_em <= \?\)`
	sec12DesfazerSQL = `UPDATE refresh_tokens SET reuso_detectado_em = NULL WHERE id = \? AND reuso_detectado_em = \?`
)

// tempoIgual casa um argumento time.Time pelo instante (Equal), sem depender
// de monotonic/Location.
type tempoIgual struct{ want time.Time }

func (m tempoIgual) Match(v driver.Value) bool {
	got, ok := v.(time.Time)
	return ok && got.Equal(m.want)
}

// sec12T0 tem fração de segundo para provar o Truncate.
var sec12T0 = time.Date(2026, 9, 27, 14, 5, 9, 870_000_000, time.Local)

func TestSEC12_RefreshReuseSuppressWindowPadrao(t *testing.T) {
	assert.Equal(t, 30*time.Minute, services.RefreshReuseSuppressWindowPadrao)
}

func TestSEC12_RegistrarReuso_JanelaELimite(t *testing.T) {
	agora := sec12T0.Truncate(time.Second)
	casos := []struct {
		nome       string
		configurar func(s *services.RefreshTokenService)
		janela     time.Duration
	}{
		{"padrão (sem Set) → 30m", func(*services.RefreshTokenService) {}, 30 * time.Minute},
		{"Set(10m)", func(s *services.RefreshTokenService) { s.SetReuseSuppressWindow(10 * time.Minute) }, 10 * time.Minute},
		{"Set(1m)", func(s *services.RefreshTokenService) { s.SetReuseSuppressWindow(time.Minute) }, time.Minute},
		{"Set(24h)", func(s *services.RefreshTokenService) { s.SetReuseSuppressWindow(24 * time.Hour) }, 24 * time.Hour},
		{"Set(0) mantém o padrão", func(s *services.RefreshTokenService) { s.SetReuseSuppressWindow(0) }, 30 * time.Minute},
		{"Set(negativo) mantém o padrão", func(s *services.RefreshTokenService) { s.SetReuseSuppressWindow(-time.Hour) }, 30 * time.Minute},
		{"Set(10m) e depois Set(0) volta ao padrão", func(s *services.RefreshTokenService) {
			s.SetReuseSuppressWindow(10 * time.Minute)
			s.SetReuseSuppressWindow(0)
		}, 30 * time.Minute},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newRefreshTokenTestDB(t)
			mock.ExpectExec(sec12MarcarSQL).
				WithArgs(tempoIgual{agora}, int64(42), tempoIgual{agora.Add(-c.janela)}).
				WillReturnResult(sqlmock.NewResult(0, 1))

			svc := services.NewRefreshTokenServiceWithClock(func() time.Time { return sec12T0 })
			c.configurar(svc)
			cortar, marca, err := svc.RegistrarReuso(context.Background(), db, 42)
			require.NoError(t, err)
			assert.True(t, cortar)
			assert.True(t, marca.Equal(agora), "marca=%s", marca)
			assert.Zero(t, marca.Nanosecond(), "marca truncada ao segundo")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSEC12_RegistrarReuso_Resultados(t *testing.T) {
	agora := sec12T0.Truncate(time.Second)
	errBanco := errors.New("banco caiu")
	casos := []struct {
		nome       string
		setup      func(e *sqlmock.ExpectedExec)
		wantCortar bool
		wantMarca  time.Time
		wantErr    error
	}{
		{"primeiro reuso (1 linha) → corta", func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 1)) }, true, agora, nil},
		{"reuso repetido na janela (0 linhas) → suprimido", func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 0)) }, false, time.Time{}, nil},
		{"erro no UPDATE → erro, sem marca", func(e *sqlmock.ExpectedExec) { e.WillReturnError(errBanco) }, false, time.Time{}, errBanco},
		{"erro no RowsAffected → erro, sem marca", func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewErrorResult(errBanco)) }, false, time.Time{}, errBanco},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newRefreshTokenTestDB(t)
			c.setup(mock.ExpectExec(sec12MarcarSQL).WithArgs(tempoIgual{agora}, int64(10), tempoIgual{agora.Add(-30 * time.Minute)}))

			svc := services.NewRefreshTokenServiceWithClock(func() time.Time { return sec12T0 })
			cortar, marca, err := svc.RegistrarReuso(context.Background(), db, 10)
			assert.Equal(t, c.wantCortar, cortar)
			assert.True(t, marca.Equal(c.wantMarca), "marca=%s", marca)
			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				assert.True(t, marca.IsZero())
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// O relógio é sempre o do Go: cada chamada lê now() de novo.
func TestSEC12_RegistrarReuso_UsaRelogioACadaChamada(t *testing.T) {
	db, mock := newRefreshTokenTestDB(t)
	instantes := []time.Time{sec12T0, sec12T0.Add(31 * time.Minute)}
	i := 0
	svc := services.NewRefreshTokenServiceWithClock(func() time.Time { v := instantes[i]; return v })
	for _, ins := range instantes {
		a := ins.Truncate(time.Second)
		mock.ExpectExec(sec12MarcarSQL).WithArgs(tempoIgual{a}, int64(10), tempoIgual{a.Add(-30 * time.Minute)}).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	for k := range instantes {
		i = k
		_, marca, err := svc.RegistrarReuso(context.Background(), db, 10)
		require.NoError(t, err)
		assert.True(t, marca.Equal(instantes[k].Truncate(time.Second)))
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC12_DesfazerReuso(t *testing.T) {
	marca := sec12T0.Truncate(time.Second)
	casos := []struct {
		nome    string
		setup   func(e *sqlmock.ExpectedExec)
		wantErr bool
	}{
		{"limpa a marca", func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 1)) }, false},
		{"marca já trocada (0 linhas) não é erro", func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 0)) }, false},
		{"erro propagado", func(e *sqlmock.ExpectedExec) { e.WillReturnError(fmt.Errorf("falhou")) }, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := newRefreshTokenTestDB(t)
			c.setup(mock.ExpectExec(sec12DesfazerSQL).WithArgs(int64(10), tempoIgual{marca}))

			err := services.NewRefreshTokenService().DesfazerReuso(context.Background(), db, 10, marca)
			if c.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
