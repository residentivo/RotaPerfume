package handlers_test

// Lote 31 (TestBrain), ponta a ponta pelo router real:
//   - BUG-24: o UPDATE condicional da senha (WHERE id = ? AND password_hash = ?)
//     sem linha afetada vira 409 "a senha foi alterada por outra operação;
//     tente novamente" (usuário existe), com rollback do histórico e sem
//     revogar sessões nem expirar cookies. Usuário que sumiu no meio da troca
//     da PRÓPRIA senha continua 500 "erro interno" (comportamento anterior,
//     gravarNovaSenha; no reset pelo admin é 404, coberto em services).
//   - SEC-33: no modo seguro, o refresh só com o cookie ANTIGO
//     (refresh_token) não é aceito: o backend não cai para o nome antigo e
//     não emite tokens novos.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
)

func TestBUG24_HTTP_ResetPassword_UpdateSemLinha(t *testing.T) {
	casos := []struct {
		nome      string
		releitura *sqlmock.Rows
		status    int
		msg       string
	}{
		{"usuário existe → 409", sqlmock.NewRows([]string{"1"}).AddRow(1), http.StatusConflict,
			"a senha foi alterada por outra operação; tente novamente"},
		{"usuário sumiu → 500 (inalterado)", sqlmock.NewRows([]string{"1"}), http.StatusInternalServerError, "erro interno"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			server, mock, cfg := sec2xServidor(t, func(*config.Config) {})
			hash, err := gerarHashSenha(cfg.HashSenha, "senha-atual")
			require.NoError(t, err)
			mock.ExpectQuery(usuarioByIDSQL).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows(usuarioCols).
				AddRow(int64(2), "User 2", "user2@test.com", hash, "normal", nil, true, true, time.Now(), time.Now(), nil, nil))
			mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).WithArgs(int64(2)).
				WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(0))
			mock.ExpectQuery(`FROM senha_historico sh`).WillReturnRows(sqlmock.NewRows([]string{"id", "usuario_id", "resetado_por_id",
				"senha_hash_anterior", "ip_origem", "user_agent", "tipo_reset", "created_at", "usuario_nome", "resetado_por_nome"}))
			mock.ExpectBegin()
			mock.ExpectExec(`INSERT INTO senha_historico`).WillReturnResult(sqlmock.NewResult(1, 1))
			// O hash lido no início vai no WHERE (BUG-24).
			mock.ExpectExec(`UPDATE usuarios SET password_hash = \?.* WHERE id = \? AND password_hash = \?`).
				WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int64(2), hash).
				WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectQuery(`SELECT 1 FROM usuarios WHERE id = \?`).WithArgs(int64(2)).WillReturnRows(c.releitura)
			// Sem UPDATE refresh_tokens (revogação) e sem commit: rollback.
			mock.ExpectRollback()

			resp := sec2xPost(t, server.URL+"/api/auth/reset-password", "", generateToken(t, cfg, 2, "normal"),
				map[string]any{"senha_atual": "senha-atual", "nova_senha": "nova-senha-123", "captchaToken": "ok"})
			raw := readBody(t, resp)
			require.Equal(t, c.status, resp.StatusCode, "%s", raw)
			if c.msg != "" {
				var body map[string]any
				require.NoError(t, json.Unmarshal(raw, &body))
				assert.Equal(t, c.msg, body["error"])
			}
			assert.Empty(t, resp.Header.Values("Set-Cookie"), "falha não expira cookies nem emite sessão")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSEC33_HTTP_RefreshModoSeguro_SoCookieAntigo_Recusado(t *testing.T) {
	for nome, cookie := range map[string]*http.Cookie{
		"só o nome antigo": {Name: "refresh_token", Value: refreshTokenTexto},
		"nenhum cookie":    nil,
	} {
		t.Run(nome, func(t *testing.T) {
			server, mock, _ := sec2xServidor(t, func(c *config.Config) { c.CookieSecure = true })
			var cookies []*http.Cookie
			if cookie != nil {
				cookies = append(cookies, cookie)
			}
			resp := sec2xPost(t, server.URL+"/api/auth/refresh", "", "", map[string]string{}, cookies...)
			raw := readBody(t, resp)
			assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnauthorized}, resp.StatusCode, "%s", raw)
			for _, sc := range resp.Header.Values("Set-Cookie") {
				nomeValor, _, _ := strings.Cut(sc, ";")
				_, valor, _ := strings.Cut(nomeValor, "=")
				assert.Empty(t, valor, "nenhum token emitido: %s", sc)
			}
			assert.NoError(t, mock.ExpectationsWereMet(), "o token do cookie antigo nem chega ao banco")
		})
	}
}
