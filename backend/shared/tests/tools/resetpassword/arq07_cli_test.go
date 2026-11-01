package resetpassword_test

// ARQ-07: o cmd/resetpassword virou main fino; o CLI (flags → fonte da senha
// → .env → pepper → DSN → conexão → ação) roda aqui com terminal, banco e
// gerador injetados.

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tools/resetpassword"
)

type execucaoCLI struct {
	saida, erro bytes.Buffer
	logs        *bytes.Buffer
	codigo      int
}

func rodarCLI(t *testing.T, deps resetpassword.DepsCLI, entrada string, args ...string) *execucaoCLI {
	t.Helper()
	ex := &execucaoCLI{logs: silenciarLog(t)}
	amb := cmdutil.Ambiente{Entrada: strings.NewReader(entrada), Saida: &ex.saida, Erro: &ex.erro, CarregarEnv: func() {}}
	ex.codigo = cmdutil.Executar("", resetpassword.CLI(deps), args, amb)
	return ex
}

func depsCom(t *testing.T, conn resetpassword.Conexao, abrirErr error) resetpassword.DepsCLI {
	return resetpassword.DepsCLI{
		Terminal:      resetpassword.Terminal{EhTerminal: func() bool { return false }, LerSemEco: func() ([]byte, error) { return nil, nil }},
		AbrirBanco:    func(string) (resetpassword.Conexao, error) { return conn, abrirErr },
		LoadHashSenha: func() (config.HashSenha, error) { return config.HashSenha{}, errors.New("PASSWORD_PEPPER ausente") },
		Gerador:       geradorProibido(t),
	}
}

func TestARQ07_CLIResetpassword_ListSemPepper(t *testing.T) {
	t.Setenv("DB_USUARIO", "u")
	t.Setenv("DB_SENHA", "s")
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp), sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	mock.ExpectPing()
	mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
	mock.ExpectClose()

	ex := rodarCLI(t, depsCom(t, db, nil), "", "-list")
	require.Equal(t, cmdutil.SaidaOK, ex.codigo, ex.logs.String())
	assert.Contains(t, ex.saida.String(), "admin existe: true")
	assert.NoError(t, mock.ExpectationsWereMet(), "-list não exige PASSWORD_PEPPER e fecha a conexão")
}

func TestARQ07_CLIResetpassword_Falhas(t *testing.T) {
	t.Setenv("DB_USUARIO", "u")
	t.Setenv("DB_SENHA", "s")
	t.Run("duas fontes de senha", func(t *testing.T) {
		ex := rodarCLI(t, depsCom(t, nil, nil), "", "-password", "x", "-password-stdin", "-list")
		assert.Equal(t, cmdutil.SaidaErro, ex.codigo)
		assert.Contains(t, ex.logs.String(), "resetpassword: uso: informe no máximo uma fonte de senha")
	})
	t.Run("-password-prompt sem terminal", func(t *testing.T) {
		ex := rodarCLI(t, depsCom(t, nil, nil), "", "-password-prompt", "-create-admin")
		assert.Equal(t, cmdutil.SaidaErro, ex.codigo)
		assert.Contains(t, ex.logs.String(), "-password-prompt exige um terminal interativo")
	})
	t.Run("sem PASSWORD_PEPPER para gravar", func(t *testing.T) {
		ex := rodarCLI(t, depsCom(t, nil, nil), "Senha@1\n", "-password-stdin", "-create-admin")
		assert.Equal(t, cmdutil.SaidaErro, ex.codigo)
		assert.Contains(t, ex.logs.String(), "resetpassword: PASSWORD_PEPPER ausente")
	})
	t.Run("sem credenciais do banco", func(t *testing.T) {
		t.Setenv("DB_SENHA", "")
		ex := rodarCLI(t, depsCom(t, nil, nil), "", "-list")
		assert.Equal(t, cmdutil.SaidaErro, ex.codigo)
		assert.Contains(t, ex.logs.String(), "resetpassword: defina DB_USUARIO/DB_SENHA no .env")
	})
	t.Run("sql.Open e ping", func(t *testing.T) {
		ex := rodarCLI(t, depsCom(t, nil, errors.New("driver")), "", "-list")
		assert.Equal(t, cmdutil.SaidaErro, ex.codigo)
		assert.Contains(t, ex.logs.String(), "resetpassword: sql.Open: driver")

		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		mock.ExpectPing().WillReturnError(errors.New("recusada"))
		mock.ExpectClose()
		ex = rodarCLI(t, depsCom(t, db, nil), "", "-list")
		assert.Equal(t, cmdutil.SaidaErro, ex.codigo)
		assert.Contains(t, ex.logs.String(), "resetpassword: ping: recusada")
		assert.NoError(t, mock.ExpectationsWereMet(), "a conexão é fechada mesmo com o ping falhando")
	})
	t.Run("flags", func(t *testing.T) {
		assert.Equal(t, cmdutil.SaidaUso, rodarCLI(t, depsCom(t, nil, nil), "", "-nao-existe").codigo)
		assert.Equal(t, cmdutil.SaidaOK, rodarCLI(t, depsCom(t, nil, nil), "", "-h").codigo)
	})
}

func TestARQ07_CLIResetpassword_DepsPadrao(t *testing.T) {
	d := resetpassword.DepsPadrao()
	assert.False(t, d.Terminal.EhTerminal(), "o stdin do go test não é terminal")
	assert.NotNil(t, d.Terminal.LerSemEco)
	assert.NotNil(t, d.LoadHashSenha)
	assert.NotNil(t, d.Gerador)
	conn, err := d.AbrirBanco("u:s@tcp(127.0.0.1:1)/x")
	require.NoError(t, err, "sql.Open não conecta")
	require.NoError(t, conn.Close())
}
