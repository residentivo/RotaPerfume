package handlers_test

// SEC-09: POST /api/auth/refresh com reuso de token já rotacionado (fora da
// janela de graça) chama AlertaSegurancaNotificador.Notificar uma vez, com
// user_id, token_id, IP e User-Agent, depois da revogação em massa e do corte
// de sessão. Outros motivos, NULL e reuso dentro da janela não notificam. A
// resposta (401 "refresh token revogado") não muda; sem notificador (nil) o
// comportamento é o do SEC-07/08.

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
)

type sec09NotificarCall struct {
	userID, tokenID int64
	ip, ua          string
	dbConcluido     bool // revogação em massa + corte já executados
}

type sec09FakeNotificador struct {
	mu    sync.Mutex
	mock  sqlmock.Sqlmock
	calls []sec09NotificarCall
}

func (f *sec09FakeNotificador) Notificar(userID, tokenID int64, ip, ua string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, sec09NotificarCall{userID: userID, tokenID: tokenID, ip: ip, ua: ua,
		dbConcluido: f.mock.ExpectationsWereMet() == nil})
}

func (f *sec09FakeNotificador) chamadas() []sec09NotificarCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sec09NotificarCall(nil), f.calls...)
}

var _ handlers.AlertaSegurancaNotificador = (*sec09FakeNotificador)(nil)

const sec09UA = "Mozilla/5.0 (SEC-09 teste)"

// sec09PostRefresh faz o refresh com User-Agent definido e devolve status e
// campo "error" do corpo.
func sec09PostRefresh(t *testing.T, url string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, makeJSON(map[string]string{"refresh_token": refreshTokenTexto}))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", sec09UA)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	access, refresh := refreshCookies(resp)
	assert.Empty(t, access)
	assert.Empty(t, refresh)
	body := decodeResponse(t, readBody(t, resp))
	e, _ := body["error"].(string)
	return resp.StatusCode, e
}

func TestSEC09_Refresh_ReusoNotificaAlerta(t *testing.T) {
	casos := []struct {
		nome      string
		revokedAt time.Duration // relativo a agora
		motivo    any
		setupDB   func(mock sqlmock.Sqlmock)
		wantCalls int
	}{
		{
			nome: "rotacao fora da janela → notifica 1×", revokedAt: -time.Minute, motivo: "rotacao",
			setupDB: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(revokeAllUsuarioSQL).WithArgs(sqlmock.AnyArg(), "revogacao_massa", int64(5)).WillReturnResult(sqlmock.NewResult(0, 2))
				mock.ExpectExec(invalidarSessoesSQL).WithArgs(sqlmock.AnyArg(), int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			wantCalls: 1,
		},
		{
			nome: "rotacao com falha na revogação e no corte → ainda notifica", revokedAt: -time.Hour, motivo: "rotacao",
			setupDB: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(revokeAllUsuarioSQL).WillReturnError(sqlmock.ErrCancelled)
				mock.ExpectExec(invalidarSessoesSQL).WillReturnError(sqlmock.ErrCancelled)
			},
			wantCalls: 1,
		},
		{nome: "logout não notifica", revokedAt: -time.Minute, motivo: "logout"},
		{nome: "inativacao não notifica", revokedAt: -time.Minute, motivo: "inativacao"},
		{nome: "senha não notifica", revokedAt: -time.Minute, motivo: "senha"},
		{nome: "revogacao_massa não notifica", revokedAt: -time.Minute, motivo: "revogacao_massa"},
		{nome: "NULL (legado) não notifica", revokedAt: -time.Minute, motivo: nil},
		{nome: "rotacao dentro da janela (Recently) não notifica", revokedAt: -5 * time.Second, motivo: "rotacao"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock, h := setupTestServerWithAuthHandler(t)
			defer server.Close()
			defer db.Close()
			capturarLog(t)
			fake := &sec09FakeNotificador{mock: mock}
			h.SetAlertaSeguranca(fake)

			expectRefreshRevogadoComMotivo(mock, time.Now().Add(c.revokedAt), c.motivo)
			if c.setupDB != nil {
				c.setupDB(mock)
			}

			status, msg := sec09PostRefresh(t, server.URL+"/api/auth/refresh")
			assert.Equal(t, http.StatusUnauthorized, status)
			assert.Equal(t, "refresh token revogado", msg)
			assert.NoError(t, mock.ExpectationsWereMet())

			calls := fake.chamadas()
			require.Len(t, calls, c.wantCalls)
			if c.wantCalls == 1 {
				assert.Equal(t, int64(5), calls[0].userID)
				assert.Equal(t, int64(10), calls[0].tokenID)
				assert.Equal(t, "127.0.0.1", calls[0].ip)
				assert.Equal(t, sec09UA, calls[0].ua)
				assert.True(t, calls[0].dbConcluido, "Notificar só depois da revogação em massa e do corte de sessão")
			}
		})
	}
}

// Replays sucessivos chamam Notificar a cada vez — a supressão (dedup/teto)
// é responsabilidade do notifier, não do handler.
func TestSEC09_Refresh_ReusoRepetido_NotificaCadaVez(t *testing.T) {
	server, db, mock, h := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()
	capturarLog(t)
	fake := &sec09FakeNotificador{mock: mock}
	h.SetAlertaSeguranca(fake)

	for i := 0; i < 3; i++ {
		expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "rotacao")
		mock.ExpectExec(revokeAllUsuarioSQL).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(invalidarSessoesSQL).WillReturnResult(sqlmock.NewResult(0, 1))
		status, msg := sec09PostRefresh(t, server.URL+"/api/auth/refresh")
		assert.Equal(t, http.StatusUnauthorized, status)
		assert.Equal(t, "refresh token revogado", msg)
	}
	assert.Len(t, fake.chamadas(), 3)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Sem notificador (nunca configurado ou redefinido para nil): mesma resposta,
// revogação e corte continuam, sem pânico.
func TestSEC09_Refresh_AlertaNil_MantemComportamento(t *testing.T) {
	casos := []struct {
		nome  string
		setup func(h *handlers.AuthHandler, mock sqlmock.Sqlmock) *sec09FakeNotificador
	}{
		{"nunca configurado", func(*handlers.AuthHandler, sqlmock.Sqlmock) *sec09FakeNotificador { return nil }},
		{"redefinido para nil", func(h *handlers.AuthHandler, mock sqlmock.Sqlmock) *sec09FakeNotificador {
			f := &sec09FakeNotificador{mock: mock}
			h.SetAlertaSeguranca(f)
			h.SetAlertaSeguranca(nil)
			return f
		}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock, h := setupTestServerWithAuthHandler(t)
			defer server.Close()
			defer db.Close()
			logs := capturarLog(t)
			f := c.setup(h, mock)

			expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "rotacao")
			mock.ExpectExec(revokeAllUsuarioSQL).WithArgs(sqlmock.AnyArg(), "revogacao_massa", int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec(invalidarSessoesSQL).WithArgs(sqlmock.AnyArg(), int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))

			status, msg := sec09PostRefresh(t, server.URL+"/api/auth/refresh")
			assert.Equal(t, http.StatusUnauthorized, status)
			assert.Equal(t, "refresh token revogado", msg)
			assert.NoError(t, mock.ExpectationsWereMet())
			assert.Contains(t, logs.String(), "[auth][seguranca] refresh: reuso de refresh token já rotacionado")
			if f != nil {
				assert.Empty(t, f.chamadas())
			}
		})
	}
}
