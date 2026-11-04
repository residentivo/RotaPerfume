package resetpassword_test

// DEPLOY-07 (Lote 33): -create-admin e -email gravam deve_trocar_senha = 1
// (UPDATE e INSERT); -password-out ARQ grava a senha GERADA num arquivo novo
// (O_EXCL, 0600), nunca no stdout, e o apaga se a gravação no banco falhar.

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cmdutil"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/tools/resetpassword"
)

var (
	reInsertAdminTrocaSenha = regexp.QuoteMeta("INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha)") +
		`\s+` + regexp.QuoteMeta("VALUES (?, ?, ?, 'admin', NULL, 1, 1)")
	reInsertEmailTrocaSenha = regexp.QuoteMeta("INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha)") +
		`\s+` + regexp.QuoteMeta("VALUES (?, ?, ?, ?, ?, 1, 1)")
)

func TestDEPLOY07_Upserts_GravamDeveTrocarSenha(t *testing.T) {
	t.Run("UpsertAdmin UPDATE", func(t *testing.T) {
		silenciarLog(t)
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateAdmin).WithArgs(hashDe("Adm@1234"), sqlmock.AnyArg(), resetpassword.AdminEmail).WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, resetpassword.UpsertAdmin(ctx, db, hsTeste, "Adm@1234"))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("UpsertAdmin INSERT", func(t *testing.T) {
		silenciarLog(t)
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateAdmin).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(reInsertAdminTrocaSenha).WithArgs("Administrador Principal", resetpassword.AdminEmail, hashDe("Adm@1234")).WillReturnResult(sqlmock.NewResult(1, 1))
		require.NoError(t, resetpassword.UpsertAdmin(ctx, db, hsTeste, "Adm@1234"))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("UpsertByEmail UPDATE", func(t *testing.T) {
		silenciarLog(t)
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateEmail).WithArgs(hashDe("Nova@123"), sqlmock.AnyArg(), "a@x.com").WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, resetpassword.UpsertByEmail(ctx, db, hsTeste, "a@x.com", "Nova@123", "normal", "A", 0))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("UpsertByEmail INSERT", func(t *testing.T) {
		silenciarLog(t)
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateEmail).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(reInsertEmailTrocaSenha).WithArgs("A", "a@x.com", hashDe("Nova@123"), "normal", nil).WillReturnResult(sqlmock.NewResult(1, 1))
		require.NoError(t, resetpassword.UpsertByEmail(ctx, db, hsTeste, "a@x.com", "Nova@123", "normal", "A", 0))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func lerArquivo(t *testing.T, caminho string) string {
	t.Helper()
	b, err := os.ReadFile(caminho)
	require.NoError(t, err)
	return string(b)
}

func TestDEPLOY07_PasswordOut_CreateAdmin(t *testing.T) {
	silenciarLog(t)
	t.Setenv("SEED_ADMIN_PASSWORD", "NaoUsarEnv") // -password-out ignora a env
	arq := filepath.Join(t.TempDir(), "admin.senha")
	db, mock := novoMock(t)
	mock.ExpectExec(reUpdateAdmin).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(reInsertAdminTrocaSenha).WithArgs("Administrador Principal", resetpassword.AdminEmail, hashDe(senhaGerada)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())

	var out bytes.Buffer
	opts := resetpassword.Options{HashSenha: hsTeste, CreateAdmin: true, PasswordOut: arq}
	require.NoError(t, resetpassword.Run(ctx, db, opts, &out, gerador(t)))
	require.NoError(t, mock.ExpectationsWereMet())

	assert.Equal(t, senhaGerada+"\n", lerArquivo(t, arq))
	assert.NotContains(t, out.String(), senhaGerada, "a senha nunca vai para o stdout")
	assert.NotContains(t, out.String(), "ADMIN_PASSWORD=")
	assert.Contains(t, out.String(), "senha gravada em "+arq)
	if runtime.GOOS != "windows" {
		st, err := os.Stat(arq)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	}
}

func TestDEPLOY07_PasswordOut_EmailEAllUsers(t *testing.T) {
	t.Run("-email", func(t *testing.T) {
		silenciarLog(t)
		arq := filepath.Join(t.TempDir(), "u.senha")
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateEmail).WithArgs(hashDe(senhaGerada), sqlmock.AnyArg(), "u@x.com").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
		var out bytes.Buffer
		opts := resetpassword.Options{HashSenha: hsTeste, Email: "u@x.com", PasswordOut: arq}
		require.NoError(t, resetpassword.Run(ctx, db, opts, &out, gerador(t)))
		assert.Equal(t, senhaGerada+"\n", lerArquivo(t, arq))
		assert.NotContains(t, out.String(), senhaGerada)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("-all-users", func(t *testing.T) {
		silenciarLog(t)
		t.Setenv("SEED_USER_PASSWORD", "NaoUsarEnv")
		arq := filepath.Join(t.TempDir(), "todos.senha")
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateAll).WithArgs(hashDe(senhaGerada)).WillReturnResult(sqlmock.NewResult(0, 2))
		mock.ExpectQuery(reExisteAdmin).WillReturnRows(sqlmock.NewRows([]string{"e"}).AddRow(true))
		mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
		var out bytes.Buffer
		opts := resetpassword.Options{HashSenha: hsTeste, AllUsers: true, PasswordOut: arq}
		require.NoError(t, resetpassword.Run(ctx, db, opts, &out, gerador(t)))
		assert.Equal(t, senhaGerada+"\n", lerArquivo(t, arq))
		assert.NotContains(t, out.String(), senhaGerada)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestDEPLOY07_PasswordOut_ArquivoExistenteNaoTocaOBanco(t *testing.T) {
	silenciarLog(t)
	arq := filepath.Join(t.TempDir(), "admin.senha")
	require.NoError(t, os.WriteFile(arq, []byte("antiga\n"), 0o600))
	db, mock := novoMock(t) // nenhuma expectativa: o banco não pode ser chamado
	err := resetpassword.Run(ctx, db, resetpassword.Options{HashSenha: hsTeste, CreateAdmin: true, PasswordOut: arq}, &bytes.Buffer{}, gerador(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "-password-out")
	assert.Equal(t, "antiga\n", lerArquivo(t, arq), "arquivo existente não é sobrescrito")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDEPLOY07_PasswordOut_DiretorioInexistente(t *testing.T) {
	silenciarLog(t)
	arq := filepath.Join(t.TempDir(), "nao", "existe", "admin.senha")
	db, mock := novoMock(t)
	err := resetpassword.Run(ctx, db, resetpassword.Options{HashSenha: hsTeste, CreateAdmin: true, PasswordOut: arq}, &bytes.Buffer{}, gerador(t))
	require.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDEPLOY07_PasswordOut_FalhaNoBancoApagaArquivo(t *testing.T) {
	silenciarLog(t)
	arq := filepath.Join(t.TempDir(), "admin.senha")
	db, mock := novoMock(t)
	mock.ExpectExec(reUpdateAdmin).WillReturnError(errBanco)
	var out bytes.Buffer
	err := resetpassword.Run(ctx, db, resetpassword.Options{HashSenha: hsTeste, CreateAdmin: true, PasswordOut: arq}, &out, gerador(t))
	require.ErrorIs(t, err, errBanco)
	_, errStat := os.Stat(arq)
	assert.True(t, os.IsNotExist(errStat), "senha que não foi gravada no banco não fica no disco")
	assert.NotContains(t, out.String(), "senha gravada")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDEPLOY07_PasswordOut_GeradorQuebrado(t *testing.T) {
	silenciarLog(t)
	arq := filepath.Join(t.TempDir(), "admin.senha")
	db, mock := novoMock(t)
	err := resetpassword.Run(ctx, db, resetpassword.Options{HashSenha: hsTeste, CreateAdmin: true, PasswordOut: arq}, &bytes.Buffer{}, geradorQuebrado)
	require.ErrorIs(t, err, errRandom)
	_, errStat := os.Stat(arq)
	assert.True(t, os.IsNotExist(errStat))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestDEPLOY07_ResolverSenha_PasswordOutNaoCombina(t *testing.T) {
	isT, ler := proibidoTerminal(t)
	casos := []struct {
		nome string
		opts resetpassword.Options
		msg  string
	}{
		{"com -password", resetpassword.Options{PasswordOut: "x", Password: "a"}, "senha GERADA"},
		{"com -password-stdin", resetpassword.Options{PasswordOut: "x", PasswordStdin: true}, "senha GERADA"},
		{"com -password-prompt", resetpassword.Options{PasswordOut: "x", PasswordPrompt: true}, "senha GERADA"},
		{"com -list", resetpassword.Options{PasswordOut: "x", List: true}, "-list"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, err := resetpassword.ResolverSenha(c.opts, leitorProibido{t}, isT, ler, &bytes.Buffer{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.msg)
		})
	}
	senha, err := resetpassword.ResolverSenha(resetpassword.Options{PasswordOut: "x", CreateAdmin: true}, leitorProibido{t}, isT, ler, &bytes.Buffer{})
	require.NoError(t, err)
	assert.Empty(t, senha, "sem fonte: o Run gera a senha")
}

// Pela CLI: a flag -password-out existe, a senha não sai no stdout/stderr.
func TestDEPLOY07_CLI_PasswordOut(t *testing.T) {
	t.Setenv("DB_USUARIO", "u")
	t.Setenv("DB_SENHA", "s")
	arq07SemTLSHerdado(t)
	arq := filepath.Join(t.TempDir(), "admin.senha")
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp), sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	mock.ExpectPing()
	mock.ExpectExec(reUpdateAdmin).WithArgs(hashDe(senhaGerada), sqlmock.AnyArg(), resetpassword.AdminEmail).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
	mock.ExpectClose()

	deps := depsCom(t, db, nil)
	deps.LoadHashSenha = func() (config.HashSenha, error) { return hsTeste, nil }
	deps.Gerador = gerador(t)
	ex := rodarCLI(t, deps, "", "-create-admin", "-password-out", arq)
	require.Equal(t, cmdutil.SaidaOK, ex.codigo, ex.erro.String()+ex.logs.String())
	assert.Equal(t, senhaGerada+"\n", lerArquivo(t, arq))
	for _, s := range []string{ex.saida.String(), ex.erro.String(), ex.logs.String()} {
		assert.NotContains(t, s, senhaGerada)
	}
	assert.True(t, strings.Contains(ex.saida.String(), "senha gravada em "+arq))
	assert.NoError(t, mock.ExpectationsWereMet())
}
