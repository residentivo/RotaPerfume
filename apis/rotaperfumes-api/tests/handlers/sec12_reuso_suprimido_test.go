package handlers_test

// SEC-12 (Lote 12): POST /api/auth/refresh com reuso de token já rotacionado.
//   (1) primeiro reuso (marca = 1 linha) → corta (revogação em massa + corte
//       de sessão) e notifica 1×;
//   (2) reuso repetido dentro da janela (marca = 0 linhas) → sem
//       RevokeAll/InvalidarSessoes/Notificar; 401 com corpo idêntico, sem
//       Set-Cookie; log "reuso repetido ... sessões NÃO revogadas de novo";
//       continua contando no rate limit;
//   (3) depois da janela (marca = 1 de novo) → corta de novo;
//   (4) erro ao marcar → fail-closed: corta, sem DesfazerReuso;
//   (5) RevokeAll e/ou InvalidarSessoes falham → DesfazerReuso;
//   (6) motivo logout/senha/inativacao/revogacao_massa/NULL → não marca;
//   (7) token dentro da janela de graça (Recent) → não marca.
// A janela usada no UPDATE vem de cfg.RefreshReuseSuppressWindow
// (NewAuthHandler → SetReuseSuppressWindow; <= 0 → 30m).

import (
	"database/sql/driver"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/handlers"
)

const (
	logCorteSEC12      = "revogando todas as sessões"
	logSuprimidoSEC12  = "reuso repetido de token rotacionado dentro da janela de supressão — sessões NÃO revogadas de novo"
	logFailClosedSEC12 = "falha ao registrar reuso (fail-closed, corta assim mesmo): token_id=10"
	logDesfeitaSEC12   = "corte incompleto — marca de reuso desfeita"
	logFalhaDesfazer   = "falha ao desfazer a marca de reuso após corte incompleto: token_id=10"
)

// expectMarcarReusoSuprimido: reuso repetido dentro da janela (0 linhas).
func expectMarcarReusoSuprimido(mock sqlmock.Sqlmock) {
	mock.ExpectExec(marcarReusoSQL).
		WithArgs(sqlmock.AnyArg(), int64(10), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
}

// expectCorteOK: revogação em massa e corte de sessão do usuário 5.
func expectCorteOK(mock sqlmock.Sqlmock) {
	mock.ExpectExec(revokeAllUsuarioSQL).WithArgs(sqlmock.AnyArg(), "revogacao_massa", int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(invalidarSessoesSQL).WithArgs(sqlmock.AnyArg(), int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
}

// sec12Refresh faz o refresh e devolve status, corpo cru e se houve algum
// Set-Cookie de token.
func sec12Refresh(t *testing.T, url string) (int, string) {
	t.Helper()
	resp := postRefresh(t, url, map[string]string{"refresh_token": refreshTokenTexto}, "")
	defer resp.Body.Close()
	access, refresh := refreshCookies(resp)
	assert.Empty(t, access, "nenhum access_token")
	assert.Empty(t, refresh, "nenhum refresh_token")
	assert.Empty(t, resp.Header.Values("Set-Cookie"), "sem Set-Cookie")
	return resp.StatusCode, string(readBody(t, resp))
}

func TestSEC12_Refresh_ReusoRepetido(t *testing.T) {
	// Cada passo é um POST /refresh com o token id=10 (usuário 5) revogado
	// por rotação há 1 min.
	type passo struct {
		setupDB   func(mock sqlmock.Sqlmock)
		wantCorte bool
		wantNotif int // notificações acumuladas depois do passo
		wantLog   []string
		wantNoLog []string
	}
	casos := []struct {
		nome   string
		passos []passo
	}{
		{
			nome: "(1)+(2)+(3): corta, suprime dentro da janela, corta depois da janela",
			passos: []passo{
				{setupDB: func(m sqlmock.Sqlmock) { expectMarcarReuso(m); expectCorteOK(m) }, wantCorte: true, wantNotif: 1,
					wantLog: []string{logCorteSEC12}},
				{setupDB: expectMarcarReusoSuprimido, wantNotif: 1,
					wantLog: []string{logSuprimidoSEC12, "user_id=5 token_id=10", "ip=127.0.0.1", "ua="}, wantNoLog: []string{logCorteSEC12}},
				{setupDB: expectMarcarReusoSuprimido, wantNotif: 1, wantLog: []string{logSuprimidoSEC12}, wantNoLog: []string{logCorteSEC12}},
				{setupDB: func(m sqlmock.Sqlmock) { expectMarcarReuso(m); expectCorteOK(m) }, wantCorte: true, wantNotif: 2,
					wantLog: []string{logCorteSEC12}, wantNoLog: []string{logSuprimidoSEC12}},
			},
		},
		{
			nome: "(4) erro ao marcar → fail-closed corta e notifica",
			passos: []passo{
				{setupDB: func(m sqlmock.Sqlmock) {
					m.ExpectExec(marcarReusoSQL).WillReturnError(sqlmock.ErrCancelled)
					expectCorteOK(m)
				}, wantCorte: true, wantNotif: 1, wantLog: []string{logFailClosedSEC12, logCorteSEC12}, wantNoLog: []string{logDesfeitaSEC12}},
			},
		},
		{
			nome: "(4) erro ao marcar e corte falha → sem DesfazerReuso (não há marca)",
			passos: []passo{
				{setupDB: func(m sqlmock.Sqlmock) {
					m.ExpectExec(marcarReusoSQL).WillReturnError(sqlmock.ErrCancelled)
					m.ExpectExec(revokeAllUsuarioSQL).WillReturnError(sqlmock.ErrCancelled)
					m.ExpectExec(invalidarSessoesSQL).WillReturnError(sqlmock.ErrCancelled)
				}, wantCorte: true, wantNotif: 1, wantLog: []string{logFailClosedSEC12}, wantNoLog: []string{logDesfeitaSEC12, logFalhaDesfazer}},
			},
		},
		{
			nome: "(5) RevokeAll falha → DesfazerReuso",
			passos: []passo{
				{setupDB: func(m sqlmock.Sqlmock) {
					expectMarcarReuso(m)
					m.ExpectExec(revokeAllUsuarioSQL).WillReturnError(sqlmock.ErrCancelled)
					m.ExpectExec(invalidarSessoesSQL).WillReturnResult(sqlmock.NewResult(0, 1))
					expectDesfazerReuso(m)
				}, wantCorte: true, wantNotif: 1, wantLog: []string{"falha na revogação em massa após reuso", logDesfeitaSEC12}},
			},
		},
		{
			nome: "(5) InvalidarSessoes falha → DesfazerReuso",
			passos: []passo{
				{setupDB: func(m sqlmock.Sqlmock) {
					expectMarcarReuso(m)
					m.ExpectExec(revokeAllUsuarioSQL).WillReturnResult(sqlmock.NewResult(0, 1))
					m.ExpectExec(invalidarSessoesSQL).WillReturnError(sqlmock.ErrCancelled)
					expectDesfazerReuso(m)
				}, wantCorte: true, wantNotif: 1, wantLog: []string{"falha ao invalidar access tokens após reuso", logDesfeitaSEC12}},
			},
		},
		{
			nome: "(5) os dois falham → DesfazerReuso uma vez; o próximo reuso corta de novo",
			passos: []passo{
				{setupDB: func(m sqlmock.Sqlmock) {
					expectMarcarReuso(m)
					m.ExpectExec(revokeAllUsuarioSQL).WillReturnError(sqlmock.ErrCancelled)
					m.ExpectExec(invalidarSessoesSQL).WillReturnError(sqlmock.ErrCancelled)
					expectDesfazerReuso(m)
				}, wantCorte: true, wantNotif: 1, wantLog: []string{logDesfeitaSEC12}},
				{setupDB: func(m sqlmock.Sqlmock) { expectMarcarReuso(m); expectCorteOK(m) }, wantCorte: true, wantNotif: 2, wantLog: []string{logCorteSEC12}},
			},
		},
		{
			nome: "(5) DesfazerReuso falha → só log",
			passos: []passo{
				{setupDB: func(m sqlmock.Sqlmock) {
					expectMarcarReuso(m)
					m.ExpectExec(revokeAllUsuarioSQL).WillReturnError(sqlmock.ErrCancelled)
					m.ExpectExec(invalidarSessoesSQL).WillReturnResult(sqlmock.NewResult(0, 1))
					m.ExpectExec(desfazerReusoSQL).WillReturnError(sqlmock.ErrCancelled)
				}, wantCorte: true, wantNotif: 1, wantLog: []string{logFalhaDesfazer}, wantNoLog: []string{logDesfeitaSEC12}},
			},
		},
		{
			nome: "corte OK não desfaz a marca",
			passos: []passo{
				{setupDB: func(m sqlmock.Sqlmock) { expectMarcarReuso(m); expectCorteOK(m) }, wantCorte: true, wantNotif: 1,
					wantNoLog: []string{logDesfeitaSEC12, logFalhaDesfazer}},
			},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock, h := setupTestServerWithAuthHandler(t)
			defer server.Close()
			defer db.Close()
			logs := capturarLog(t)
			fake := &sec09FakeNotificador{mock: mock}
			h.SetAlertaSeguranca(fake)
			url := server.URL + "/api/auth/refresh"

			var primeiroCorpo string
			for i, p := range c.passos {
				logs.Reset()
				expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "rotacao")
				p.setupDB(mock)

				status, corpo := sec12Refresh(t, url)
				assert.Equal(t, http.StatusUnauthorized, status, "passo %d", i)
				assert.Equal(t, "refresh token revogado", decodeResponse(t, []byte(corpo))["error"], "passo %d", i)
				if i == 0 {
					primeiroCorpo = corpo
				} else {
					assert.Equal(t, primeiroCorpo, corpo, "passo %d: corpo idêntico entre corte e supressão", i)
				}
				require.NoError(t, mock.ExpectationsWereMet(), "passo %d", i)
				assert.Len(t, fake.chamadas(), p.wantNotif, "passo %d: notificações", i)

				out := logs.String()
				for _, s := range p.wantLog {
					assert.Contains(t, out, s, "passo %d", i)
				}
				for _, s := range p.wantNoLog {
					assert.NotContains(t, out, s, "passo %d", i)
				}
				assert.Equal(t, p.wantCorte, strings.Contains(out, logCorteSEC12), "passo %d: corte", i)
			}
		})
	}
}

// (2) O reuso suprimido continua contando no rate limit: 10 suprimidos → 429.
func TestSEC12_Refresh_ReusoSuprimido_ContaNoRateLimit(t *testing.T) {
	server, db, mock, h := setupTestServerWithAuthHandler(t)
	defer server.Close()
	defer db.Close()
	logs := capturarLog(t)
	fake := &sec09FakeNotificador{mock: mock}
	h.SetAlertaSeguranca(fake)
	url := server.URL + "/api/auth/refresh"

	for i := 0; i < 10; i++ {
		expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "rotacao")
		expectMarcarReusoSuprimido(mock)
		status, _ := sec12Refresh(t, url)
		require.Equal(t, http.StatusUnauthorized, status, "tentativa %d", i)
	}
	require.NoError(t, mock.ExpectationsWereMet())

	resp := postRefresh(t, url, map[string]string{"refresh_token": refreshTokenTexto}, "")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.NoError(t, mock.ExpectationsWereMet(), "bloqueado sem consultar o banco")
	assert.Empty(t, fake.chamadas(), "supressão nunca notifica")
	assert.Equal(t, 10, strings.Count(logs.String(), "sessões NÃO revogadas de novo"))
	assert.NotContains(t, logs.String(), logCorteSEC12)
}

// (2) O cliente não distingue corte, supressão e outros motivos.
func TestSEC12_Refresh_RespostaIgualCorteSupressaoLogout(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()
	capturarLog(t)
	url := server.URL + "/api/auth/refresh"

	expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "rotacao")
	expectMarcarReuso(mock)
	expectCorteOK(mock)
	stCorte, corpoCorte := sec12Refresh(t, url)

	expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "rotacao")
	expectMarcarReusoSuprimido(mock)
	stSup, corpoSup := sec12Refresh(t, url)

	expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "logout")
	stLogout, corpoLogout := sec12Refresh(t, url)

	assert.Equal(t, http.StatusUnauthorized, stCorte)
	assert.Equal(t, stCorte, stSup)
	assert.Equal(t, stCorte, stLogout)
	assert.Equal(t, corpoCorte, corpoSup)
	assert.Equal(t, corpoCorte, corpoLogout)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// (6) e (7): outros motivos e a janela de graça nunca chegam ao Marcar.
func TestSEC12_Refresh_NaoMarcaForaDoReusoDeRotacao(t *testing.T) {
	casos := []struct {
		nome      string
		revokedAt time.Duration
		motivo    any
	}{
		{"(6) logout", -time.Minute, "logout"},
		{"(6) senha", -time.Minute, "senha"},
		{"(6) inativacao", -time.Minute, "inativacao"},
		{"(6) revogacao_massa", -time.Minute, "revogacao_massa"},
		{"(6) NULL (legado)", -time.Minute, nil},
		{"(7) rotacao dentro da janela de graça", -5 * time.Second, "rotacao"},
		{"(7) logout dentro da janela de graça", -5 * time.Second, "logout"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, db, mock, h := setupTestServerWithAuthHandler(t)
			defer server.Close()
			defer db.Close()
			logs := capturarLog(t)
			fake := &sec09FakeNotificador{mock: mock}
			h.SetAlertaSeguranca(fake)

			expectRefreshRevogadoComMotivo(mock, time.Now().Add(c.revokedAt), c.motivo)
			// Sem ExpectExec: qualquer UPDATE (marca/corte) seria inesperado.

			status, _ := sec12Refresh(t, server.URL+"/api/auth/refresh")
			assert.Equal(t, http.StatusUnauthorized, status)
			assert.NoError(t, mock.ExpectationsWereMet())
			assert.Empty(t, fake.chamadas())
			out := logs.String()
			assert.NotContains(t, out, "[auth][seguranca]")
			assert.NotContains(t, out, "reuso repetido")
		})
	}
}

// capturaTempo guarda o time.Time recebido como argumento do UPDATE.
type capturaTempo struct {
	mu *sync.Mutex
	v  *time.Time
}

func (c capturaTempo) Match(v driver.Value) bool {
	tm, ok := v.(time.Time)
	if ok {
		c.mu.Lock()
		*c.v = tm
		c.mu.Unlock()
	}
	return ok
}

// A janela do UPDATE vem da config (NewAuthHandler → SetReuseSuppressWindow).
func TestSEC12_NewAuthHandler_JanelaDaConfig(t *testing.T) {
	casos := []struct {
		nome   string
		janela time.Duration
		want   time.Duration
	}{
		{"config 10m", 10 * time.Minute, 10 * time.Minute},
		{"config 1m", time.Minute, time.Minute},
		{"config 24h", 24 * time.Hour, 24 * time.Hour},
		{"config zero (Config montada à mão) → padrão 30m", 0, 30 * time.Minute},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			defer db.Close()
			capturarLog(t)
			cfg := testCfg()
			cfg.RefreshReuseSuppressWindow = c.janela
			h := handlers.NewAuthHandler(db, cfg)

			var mu sync.Mutex
			var agora, limite time.Time
			expectRefreshRevogadoComMotivo(mock, time.Now().Add(-time.Minute), "rotacao")
			mock.ExpectExec(marcarReusoSQL).
				WithArgs(capturaTempo{&mu, &agora}, int64(10), capturaTempo{&mu, &limite}).
				WillReturnResult(sqlmock.NewResult(0, 0))

			antes := time.Now()
			req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", makeJSON(map[string]string{"refresh_token": refreshTokenTexto}))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.Refresh(rec, req)
			depois := time.Now()

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			require.NoError(t, mock.ExpectationsWereMet())
			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, c.want, agora.Sub(limite), "limite = agora - janela")
			assert.Zero(t, agora.Nanosecond(), "agora truncado ao segundo")
			assert.False(t, agora.Before(antes.Truncate(time.Second)), "relógio do Go")
			assert.False(t, agora.After(depois))
		})
	}
}
