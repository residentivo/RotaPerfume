// OPS-01: reset em massa das senhas dos usuários ativos. Confere, por
// usuário, que a senha enviada por e-mail é a mesma gravada no banco como
// Argon2id com pepper, com troca obrigatória, histórico e sessões revogadas.
package services_test

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/models"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

const reListAtivos = usuarioColunasRegex + `\s+WHERE u\.ativo = 1\s+ORDER BY u\.id ASC`

// emailPorDestino guarda a senha enviada a cada destinatário; falharEm faz o
// envio para aquele endereço devolver erro.
type emailPorDestino struct {
	senhas   map[string]string
	falharEm string
}

func (e *emailPorDestino) EnviarSenhaInicial(_ context.Context, destinatario, _, senha string) error {
	if destinatario == e.falharEm {
		return errors.New("smtp fora do ar")
	}
	e.senhas[destinatario] = senha
	return nil
}

// capturaHash é um sqlmock.Argument que aceita qualquer string e a guarda.
type capturaHash struct{ destino *string }

func (c capturaHash) Match(v driver.Value) bool {
	s, ok := v.(string)
	*c.destino = s
	return ok
}

type usuarioOPS01 struct {
	id    int64
	email string
	role  string
	hash  string
}

var ativosOPS01 = []usuarioOPS01{
	{1, "admin@test.com", models.RoleAdmin, "$2a$12$hashantigoadmin"},
	{2, "ana@test.com", models.RoleNormal, "$2a$12$hashantigoana"},
}

func linhaUsuario(rows *sqlmock.Rows, u usuarioOPS01) *sqlmock.Rows {
	return rows.AddRow(u.id, "Nome "+u.email, u.email, u.hash, u.role, nil, true, false, time.Now(), time.Now(), nil, nil)
}

func esperarListaAtivos(mock sqlmock.Sqlmock) {
	rows := sqlmock.NewRows(usuarioColunasHeader())
	for _, u := range ativosOPS01 {
		linhaUsuario(rows, u)
	}
	mock.ExpectQuery(reListAtivos).WillReturnRows(rows)
}

// esperarReset registra os SQLs do reset de um usuário e devolve onde o hash
// novo gravado será capturado.
func esperarReset(mock sqlmock.Sqlmock, u usuarioOPS01, comPosProcessamento bool) *string {
	novo := new(string)
	mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).WithArgs(u.id).
		WillReturnRows(linhaUsuario(sqlmock.NewRows(usuarioColunasHeader()), u))
	mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \?, tokens_validos_desde = \? WHERE id = \?`).
		WithArgs(capturaHash{novo}, true, sqlmock.AnyArg(), u.id).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if comPosProcessamento {
		mock.ExpectExec(`INSERT INTO senha_historico`).
			WithArgs(u.id, sqlmock.AnyArg(), u.hash, services.ResetMassaIPOrigem, services.ResetMassaUserAgent, "admin").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at = \?, revoked_reason = \? WHERE usuario_id = \? AND revoked_at IS NULL`).
			WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), u.id).
			WillReturnResult(sqlmock.NewResult(0, 2))
	}
	return novo
}

func TestOPS01_Simulacao_NaoGravaNemEnvia(t *testing.T) {
	db, mock := newUsuarioTestDB(t)
	esperarListaAtivos(mock) // só o SELECT: qualquer outro SQL falha o teste
	email := &emailPorDestino{senhas: map[string]string{}}
	svc := services.NewResetSenhasAtivosService(services.NewUsuarioService(db, usuarioTestCfg(false), email))

	itens, err := svc.Executar(context.Background(), db, true)
	require.NoError(t, err)
	require.Len(t, itens, 2)
	for i, it := range itens {
		assert.Equal(t, ativosOPS01[i].id, it.ID)
		assert.Equal(t, "simulado", it.Status)
	}
	assert.Empty(t, email.senhas)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOPS01_Executar_SenhaEnviadaEhOHashArgon2idGravado(t *testing.T) {
	db, mock := newUsuarioTestDB(t)
	cfg := usuarioTestCfg(false)
	esperarListaAtivos(mock)
	hashes := make([]*string, len(ativosOPS01))
	for i, u := range ativosOPS01 {
		hashes[i] = esperarReset(mock, u, true)
	}
	email := &emailPorDestino{senhas: map[string]string{}}
	svc := services.NewResetSenhasAtivosService(services.NewUsuarioService(db, cfg, email))

	itens, err := svc.Executar(context.Background(), db, false)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())

	senhasVistas := map[string]bool{}
	for i, u := range ativosOPS01 {
		assert.Equal(t, "resetado", itens[i].Status)
		senha := email.senhas[u.email]
		require.Len(t, senha, 16, "senha aleatória de 16 caracteres para %s", u.email)
		assert.False(t, senhasVistas[senha], "cada usuário recebe uma senha diferente")
		senhasVistas[senha] = true

		hash := *hashes[i]
		assert.True(t, strings.HasPrefix(hash, "$argon2id$"), "hash gravado deve ser argon2id: %s", hash)
		assert.True(t, sharedsvc.VerificarSenha(cfg.HashSenha, hash, senha), "a senha enviada valida contra o hash gravado")
		assert.False(t, sharedsvc.PrecisaRehash(cfg.HashSenha, hash))
		assert.False(t, sharedsvc.VerificarSenha(cfg.HashSenha, u.hash, senha), "a senha antiga deixa de valer")
	}
}

func TestOPS01_Executar_ParaNoPrimeiroEmailQueFalha(t *testing.T) {
	db, mock := newUsuarioTestDB(t)
	esperarListaAtivos(mock)
	esperarReset(mock, ativosOPS01[0], true) // o 2º usuário nunca é tocado
	email := &emailPorDestino{senhas: map[string]string{}, falharEm: ativosOPS01[0].email}
	svc := services.NewResetSenhasAtivosService(services.NewUsuarioService(db, usuarioTestCfg(false), email))

	itens, err := svc.Executar(context.Background(), db, false)
	require.ErrorIs(t, err, services.ErrEmailResetFalhou)
	require.Len(t, itens, 1)
	assert.Equal(t, "falha_email", itens[0].Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOPS01_Executar_ErrosDoBanco(t *testing.T) {
	errBanco := errors.New("banco fora do ar")

	t.Run("listagem falha", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectQuery(reListAtivos).WillReturnError(errBanco)
		svc := services.NewResetSenhasAtivosService(services.NewUsuarioService(db, usuarioTestCfg(false), &emailPorDestino{senhas: map[string]string{}}))
		itens, err := svc.Executar(context.Background(), db, false)
		assert.ErrorIs(t, err, errBanco)
		assert.Nil(t, itens)
	})

	t.Run("gravação da senha falha: para sem enviar e-mail", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		esperarListaAtivos(mock)
		u := ativosOPS01[0]
		mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).WithArgs(u.id).
			WillReturnRows(linhaUsuario(sqlmock.NewRows(usuarioColunasHeader()), u))
		mock.ExpectExec(`UPDATE usuarios SET password_hash`).WillReturnError(errBanco)
		email := &emailPorDestino{senhas: map[string]string{}}
		svc := services.NewResetSenhasAtivosService(services.NewUsuarioService(db, usuarioTestCfg(false), email))

		itens, err := svc.Executar(context.Background(), db, false)
		assert.ErrorIs(t, err, errBanco)
		assert.Empty(t, itens)
		assert.Empty(t, email.senhas)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("histórico e revogação falham: só log, segue", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		esperarListaAtivos(mock)
		for _, u := range ativosOPS01 {
			esperarReset(mock, u, false)
			mock.ExpectExec(`INSERT INTO senha_historico`).WillReturnError(errBanco)
			mock.ExpectExec(`UPDATE refresh_tokens`).WillReturnError(errBanco)
		}
		email := &emailPorDestino{senhas: map[string]string{}}
		svc := services.NewResetSenhasAtivosService(services.NewUsuarioService(db, usuarioTestCfg(false), email))

		itens, err := svc.Executar(context.Background(), db, false)
		require.NoError(t, err)
		assert.Len(t, itens, 2)
		assert.Len(t, email.senhas, 2)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
