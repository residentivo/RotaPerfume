package resetsenhas_test

// ARQ-07: o cmd/resetsenhas (OPS-01) saiu do main para internal/resetsenhas.
// O comando inteiro roda sobre o sqlmock e um e-mail falso: simula por
// padrão (M-03), exige SMTP, para no primeiro e-mail que falha e imprime os
// e-mails mascarados (NB-7). O caso de uso em si (hash, histórico,
// revogação) é coberto por tests/services/ops01_reset_senhas_ativos_test.go.

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/internal/resetsenhas"
	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/notificacao"
)

var colunasUsuario = []string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo",
	"deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}

const reAtivos = `FROM usuarios u\s+LEFT JOIN vendedores v ON v\.id = u\.id_vendedor\s+WHERE u\.ativo = 1\s+ORDER BY u\.id ASC`

func cfgTeste() *config.Config {
	return &config.Config{
		DBName: "rotaperfumes", DBHost: "db.local", DBPort: "3306",
		SMTPHost: "smtp.exemplo.test", SMTPFrom: "remetente@rotaperfumes.com.br",
		JWTSecret: "segredo", JWTTTL: time.Hour,
		HashSenha: config.HashSenha{Pepper: "pepper-de-teste-com-pelo-menos-32-bytes", MemoriaKiB: 64, Iteracoes: 1, Paralelismo: 1},
	}
}

type emailFalso struct{ err error }

func (e emailFalso) EnviarSenhaInicial(context.Context, string, string, string) error { return e.err }

type execucao struct {
	saida, erro, logs bytes.Buffer
	codigo            int
	envCarregado      bool
}

func rodar(t *testing.T, db *sql.DB, deps resetsenhas.Deps, cfgErr error, args ...string) *execucao {
	t.Helper()
	ex := &execucao{}
	anterior := log.Writer()
	log.SetOutput(&ex.logs)
	t.Cleanup(func() { log.SetOutput(anterior) })
	if deps.AbrirBanco == nil {
		deps.AbrirBanco = func(string) (*sql.DB, error) { return db, nil }
	}
	if deps.Email == nil {
		deps.Email = func(*config.Config) (identidadeapp.EnviadorSenhaInicial, error) { return emailFalso{}, nil }
	}
	amb := cmdutil.Ambiente{
		Saida: &ex.saida, Erro: &ex.erro,
		CarregarEnv: func() { ex.envCarregado = true },
		Config:      func() (*config.Config, error) { return cfgTeste(), cfgErr },
	}
	ex.codigo = cmdutil.Executar(resetsenhas.Tag, resetsenhas.CLI(deps), args, amb)
	return ex
}

func novoMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func esperarAtivos(mock sqlmock.Sqlmock) {
	mock.ExpectQuery(reAtivos).WillReturnRows(sqlmock.NewRows(colunasUsuario).
		AddRow(int64(1), "Admin", "admin@rotaperfumes.com.br", "$2a$12$x", "admin", nil, true, false, time.Now(), time.Now(), nil, nil).
		AddRow(int64(2), "Ana", "ana.silva@exemplo.com", "$2a$12$y", "normal", nil, true, false, time.Now(), time.Now(), nil, nil))
}

func TestARQ07_ResetSenhas_SimulaPorPadraoEMascaraEmails(t *testing.T) {
	db, mock := novoMock(t)
	mock.ExpectPing()
	esperarAtivos(mock) // só o SELECT: qualquer escrita falharia o teste
	ex := rodar(t, db, resetsenhas.Deps{}, nil)

	require.Equal(t, cmdutil.SaidaOK, ex.codigo, ex.logs.String())
	assert.True(t, ex.envCarregado, "o .env é carregado antes da config")
	out := ex.saida.String()
	assert.Contains(t, out, "Banco: rotaperfumes@db.local:3306 | SMTP: smtp.exemplo.test (from r***@rotaperfumes.com.br) | modo: simulação")
	assert.Contains(t, out, "a***@rotaperfumes.com.br")
	assert.Contains(t, out, "a***@exemplo.com")
	assert.NotContains(t, out, "ana.silva@exemplo.com", "NB-7: e-mail sem máscara no stdout")
	assert.NotContains(t, out, "remetente@rotaperfumes.com.br")
	assert.Contains(t, out, "Total: 2 | resetados: 0 | simulados: 2 | falha de e-mail: 0")
	assert.Contains(t, out, "Simulação: nada foi gravado nem enviado.")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestARQ07_ResetSenhas_ExecutarParaNoPrimeiroEmailQueFalha(t *testing.T) {
	db, mock := novoMock(t)
	mock.ExpectPing()
	esperarAtivos(mock)
	mock.ExpectQuery(`WHERE u\.id = \?\s+LIMIT 1`).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows(colunasUsuario).
		AddRow(int64(1), "Admin", "admin@rotaperfumes.com.br", "$2a$12$x", "admin", nil, true, false, time.Now(), time.Now(), nil, nil))
	mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \?, tokens_validos_desde = \? WHERE id = \?`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO senha_historico`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE refresh_tokens SET revoked_at`).WillReturnResult(sqlmock.NewResult(0, 0))

	smtpFora := resetsenhas.Deps{Email: func(*config.Config) (identidadeapp.EnviadorSenhaInicial, error) {
		return emailFalso{err: errors.New("smtp fora do ar")}, nil
	}}
	ex := rodar(t, db, smtpFora, nil, "-executar")

	assert.Equal(t, cmdutil.SaidaErro, ex.codigo)
	assert.Contains(t, ex.saida.String(), "modo: EXECUTAR")
	assert.Contains(t, ex.saida.String(), "Total: 1 | resetados: 0 | simulados: 0 | falha de e-mail: 1")
	assert.Contains(t, ex.erro.String(), "PARADO: o e-mail do último usuário falhou.")
	assert.Contains(t, ex.logs.String(), "resetsenhas: reset em massa: e-mail com a nova senha não foi enviado: usuario_id=1")
	assert.NotContains(t, ex.saida.String(), "Simulação:")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestARQ07_ResetSenhas_Falhas(t *testing.T) {
	errInfra := errors.New("fora do ar")
	t.Run("config", func(t *testing.T) {
		ex := rodar(t, nil, resetsenhas.Deps{}, errInfra)
		assert.Equal(t, cmdutil.SaidaErro, ex.codigo)
		assert.Contains(t, ex.logs.String(), "resetsenhas: config: fora do ar")
	})
	t.Run("sem SMTP recusa rodar (a senha só chega por e-mail)", func(t *testing.T) {
		semSMTP := resetsenhas.Deps{
			Email: func(*config.Config) (identidadeapp.EnviadorSenhaInicial, error) {
				return nil, notificacao.ErrSMTPNaoConfigurado
			},
			AbrirBanco: func(string) (*sql.DB, error) { t.Fatal("não abre o banco sem SMTP"); return nil, nil },
		}
		ex := rodar(t, nil, semSMTP, nil, "-executar")
		assert.Equal(t, cmdutil.SaidaErro, ex.codigo)
		assert.Contains(t, ex.logs.String(), "resetsenhas: SMTP obrigatório (a senha só chega ao usuário por e-mail)")
	})
	t.Run("abrir o banco e ping", func(t *testing.T) {
		ex := rodar(t, nil, resetsenhas.Deps{AbrirBanco: func(string) (*sql.DB, error) { return nil, errInfra }}, nil)
		assert.Contains(t, ex.logs.String(), "resetsenhas: db: fora do ar")

		db, mock := novoMock(t)
		mock.ExpectPing().WillReturnError(errInfra)
		ex = rodar(t, db, resetsenhas.Deps{}, nil)
		assert.Equal(t, cmdutil.SaidaErro, ex.codigo)
		assert.Contains(t, ex.logs.String(), "resetsenhas: db ping: fora do ar")
	})
	t.Run("listar ativos falha", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectPing()
		mock.ExpectQuery(reAtivos).WillReturnError(errInfra)
		ex := rodar(t, db, resetsenhas.Deps{}, nil)
		assert.Equal(t, cmdutil.SaidaErro, ex.codigo)
		assert.Empty(t, ex.erro.String(), "só o e-mail falho mostra o aviso PARADO")
	})
	t.Run("flags", func(t *testing.T) {
		assert.Equal(t, cmdutil.SaidaUso, rodar(t, nil, resetsenhas.Deps{}, nil, "-nao-existe").codigo)
		assert.Equal(t, cmdutil.SaidaOK, rodar(t, nil, resetsenhas.Deps{}, nil, "-h").codigo)
	})
}

func TestARQ07_ResetSenhas_DepsPadrao(t *testing.T) {
	d := resetsenhas.DepsPadrao()
	_, err := d.Email(&config.Config{})
	assert.ErrorIs(t, err, notificacao.ErrSMTPNaoConfigurado)
	email, err := d.Email(cfgCompleta())
	require.NoError(t, err)
	assert.NotNil(t, email)
	assert.NotNil(t, d.AbrirBanco)
}

func cfgCompleta() *config.Config {
	c := cfgTeste()
	c.SMTPUser, c.SMTPPassword, c.SMTPPort = "u", "p", "587"
	return c
}

func TestARQ07_ResetSenhas_Imprimir(t *testing.T) {
	var out strings.Builder
	resetsenhas.Imprimir(&out, []identidadeapp.ItemResetMassa{
		{ID: 1, Email: "admin@x.com", Role: "admin", Status: identidadeapp.StatusResetado},
		{ID: 2, Email: "sem-arroba", Role: "normal", Status: identidadeapp.StatusFalhaEmail},
	})
	assert.Contains(t, out.String(), "a***@x.com")
	assert.Contains(t, out.String(), "***")
	assert.NotContains(t, out.String(), "sem-arroba")
	assert.Contains(t, out.String(), "Total: 2 | resetados: 1 | simulados: 0 | falha de e-mail: 1")
}
