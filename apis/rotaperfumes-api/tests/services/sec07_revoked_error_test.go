package services_test

// SEC-07: o token revogado carrega id, usuário, motivo e instante da
// revogação; a classificação (recente × fora da janela) e o sinal de reuso
// de rotação vêm do domínio. ARQ-07: o RevokedTokenError era montado pelo
// repassador de teste; agora as asserções são sobre o próprio
// identidade.RefreshToken (adapter real) e identidade.EstadoRefresh
// (Avaliar/ReusoDeRotacao, as regras que o RenovarSessao aplica).

import (
	"context"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const findRefreshSEC07 = `SELECT id, usuario_id, token_hash, expires_at, revoked_at, ip_origem, user_agent, revoked_reason\s+FROM refresh_tokens\s+WHERE token_hash = \?\s+LIMIT 1`

func TestSEC07_ValidateRefreshToken_RevokedTokenError(t *testing.T) {
	agora := time.Date(2026, 9, 26, 12, 0, 0, 0, time.Local)
	casos := []struct {
		nome       string
		revokedAt  time.Time
		motivo     any
		wantReason identidade.MotivoRevogacao
		wantRecent bool
		wantReuso  bool
	}{
		{"rotacao fora da janela = reuso", agora.Add(-time.Minute), "rotacao", identidade.MotivoRotacao, false, true},
		{"rotacao dentro da janela = corrida", agora.Add(-5 * time.Second), "rotacao", identidade.MotivoRotacao, true, false},
		{"logout fora da janela", agora.Add(-time.Minute), "logout", identidade.MotivoLogout, false, false},
		{"inativacao fora da janela", agora.Add(-time.Minute), "inativacao", identidade.MotivoInativacao, false, false},
		{"senha fora da janela", agora.Add(-time.Minute), "senha", identidade.MotivoSenha, false, false},
		{"revogacao_massa fora da janela", agora.Add(-time.Minute), "revogacao_massa", identidade.MotivoRevogacaoMassa, false, false},
		{"NULL legado fora da janela (conservador)", agora.Add(-time.Minute), nil, identidade.MotivoDesconhecido, false, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			defer db.Close()
			token := "tok-sec07"
			mock.ExpectQuery(findRefreshSEC07).WithArgs(hashOf(token)).
				WillReturnRows(sqlmock.NewRows(refreshTokenColunas()).
					AddRow(int64(44), int64(9), hashOf(token), agora.Add(time.Hour), c.revokedAt, "ip", "ua", c.motivo))

			rt, estado, err := buscarEAvaliar(db, token, agora)

			require.NoError(t, err)
			assert.Equal(t, int64(44), rt.ID)
			assert.Equal(t, int64(9), rt.UsuarioID)
			assert.Equal(t, c.wantReason, rt.Motivo)
			require.NotNil(t, rt.RevogadoEm)
			assert.True(t, rt.RevogadoEm.Equal(c.revokedAt))
			assert.Equal(t, c.wantRecent, estado == identidade.RefreshRevogadoRecente)
			assert.Equal(t, c.wantReuso, rt.ReusoDeRotacao(estado))

			// revogado (recente ou não): nunca válido nem expirado.
			assert.Contains(t, []identidade.EstadoRefresh{identidade.RefreshRevogado, identidade.RefreshRevogadoRecente}, estado)
			if !c.wantRecent {
				assert.Equal(t, identidade.RefreshRevogado, estado)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// BeginRotation grava revoked_reason = "rotacao". ARQ-07: a revogação da
// rotação é a do caso de uso RenovarSessao (UoW real); o UPDATE condicional
// recusa (0 linhas) e a tx é desfeita, como o Rollback do teste antigo.
func TestSEC07_BeginRotation_GravaMotivoRotacao(t *testing.T) {
	_, err, mock := renovar(t, func(m sqlmock.Sqlmock) {
		m.ExpectBegin()
		m.ExpectExec(revokeCondicionalSQL).
			WithArgs(sqlmock.AnyArg(), "rotacao", int64(7)).WillReturnResult(sqlmock.NewResult(0, 0))
		m.ExpectRollback()
	})
	assert.ErrorIs(t, err, identidade.ErrRefreshRevogado)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// RevokeAllUserTokens grava exatamente o motivo informado pelo chamador.
func TestSEC07_RevokeAllUserTokens_GravaMotivoInformado(t *testing.T) {
	for _, motivo := range []identidade.MotivoRevogacao{
		identidade.MotivoSenha,
		identidade.MotivoInativacao,
		identidade.MotivoRevogacaoMassa,
	} {
		t.Run(string(motivo), func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE usuario_id = \? AND revoked_at IS NULL`).
				WithArgs(sqlmock.AnyArg(), string(motivo), int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))

			err = identidademysql.NovoRefreshTokens(db).RevogarTodosDoUsuario(context.Background(), 3, motivo)
			require.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
