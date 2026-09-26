package resetpassword_test

// TST-03 (Lote 7): tools/resetpassword com o banco em sqlmock, a saída em
// bytes.Buffer e o gerador de senha injetado. Os hashes gravados são
// conferidos com bcrypt (senha certa e custo 12), não só com AnyArg.

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"log"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/rotaperfumes/shared/tools/resetpassword"
)

var (
	errBanco  = errors.New("falha simulada no banco")
	errRandom = errors.New("sem entropia")
	ctx       = context.Background()
)

const senhaGerada = "GeradaXYZ0123456"

var (
	reUpdateAdmin   = regexp.QuoteMeta("UPDATE usuarios SET password_hash = ?, ativo = 1 WHERE email = ?")
	reUpdateEmail   = regexp.QuoteMeta("UPDATE usuarios SET password_hash = ? WHERE email = ?")
	reUpdateAll     = regexp.QuoteMeta("UPDATE usuarios SET password_hash = ? WHERE password_hash LIKE '%PLACEHOLDER%'")
	reInsert        = "INSERT INTO usuarios"
	reExisteAdmin   = regexp.QuoteMeta("SELECT EXISTS(SELECT 1 FROM usuarios WHERE email = 'admin@rotaperfumes.com.br')")
	reListar        = "SELECT id, nome, email, role, ativo"
	senhaMuitoLonga = strings.Repeat("x", 73) // bcrypt recusa > 72 bytes
)

var colunasLista = []string{"id", "nome", "email", "role", "ativo", "preview", "status"}

// hashDe é um sqlmock.Argument que só casa com um hash bcrypt de custo
// BcryptCost da senha informada.
type hashDe string

func (s hashDe) Match(v driver.Value) bool {
	h, ok := v.(string)
	if !ok || bcrypt.CompareHashAndPassword([]byte(h), []byte(s)) != nil {
		return false
	}
	custo, err := bcrypt.Cost([]byte(h))
	return err == nil && custo == resetpassword.BcryptCost
}

func novoMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

func silenciarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	saida, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(saida); log.SetFlags(flags) })
	return &buf
}

func gerador(t *testing.T) resetpassword.PasswordGenerator {
	return func(n int) (string, error) {
		assert.Equal(t, 16, n, "senha gerada tem 16 caracteres")
		return senhaGerada, nil
	}
}

func geradorQuebrado(int) (string, error) { return "", errRandom }

func geradorProibido(t *testing.T) resetpassword.PasswordGenerator {
	return func(int) (string, error) {
		t.Fatal("não deveria gerar senha")
		return "", nil
	}
}

func listaComAdmin() *sqlmock.Rows {
	return sqlmock.NewRows(colunasLista).
		AddRow(1, "Admin", resetpassword.AdminEmail, "admin", 1, "$2a$12$abc", "OK").
		AddRow(2, "Vendedor", "vend@x.com", "normal", 1, "$2a$12$XXXXPLACEHOLDER", "PLACEHOLDER")
}

// ---------------------------------------------------------------------------
// ListUsers
// ---------------------------------------------------------------------------

func TestListUsers(t *testing.T) {
	t.Run("com admin", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
		var out bytes.Buffer
		require.NoError(t, resetpassword.ListUsers(ctx, db, &out))
		s := out.String()
		assert.Contains(t, s, "EMAIL")
		assert.Contains(t, s, resetpassword.AdminEmail)
		assert.Contains(t, s, "vend@x.com")
		assert.Contains(t, s, "Total: 2 usuarios | 1 OK | 1 PLACEHOLDER | admin existe: true")
		assert.NotContains(t, s, "ATENÇÃO")
	})
	t.Run("sem admin avisa", func(t *testing.T) {
		db, mock := novoMock(t)
		longo := strings.Repeat("a", 60) + "@x.com"
		mock.ExpectQuery(reListar).WillReturnRows(sqlmock.NewRows(colunasLista).
			AddRow(3, "X", longo, "normal", 0, "hash", "UNKNOWN"))
		var out bytes.Buffer
		require.NoError(t, resetpassword.ListUsers(ctx, db, &out))
		s := out.String()
		assert.Contains(t, s, resetpassword.Truncate(longo, 50), "e-mail longo é truncado")
		assert.NotContains(t, s, longo)
		assert.Contains(t, s, "Total: 1 usuarios | 0 OK | 0 PLACEHOLDER | admin existe: false")
		assert.Contains(t, s, "NÃO EXISTE")
	})
	casos := []struct {
		nome string
		prep func(m sqlmock.Sqlmock)
		sub  string
	}{
		{"erro na query", func(m sqlmock.Sqlmock) { m.ExpectQuery(reListar).WillReturnError(errBanco) }, "list query"},
		{"erro no scan", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reListar).WillReturnRows(sqlmock.NewRows(colunasLista).AddRow("x", "n", "e", "r", 1, "p", "OK"))
		}, "list scan"},
		{"erro na iteração", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reListar).WillReturnRows(listaComAdmin().RowError(1, errBanco))
		}, errBanco.Error()},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock := novoMock(t)
			c.prep(mock)
			err := resetpassword.ListUsers(ctx, db, &bytes.Buffer{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.sub)
		})
	}
}

// ---------------------------------------------------------------------------
// Run: cada ação
// ---------------------------------------------------------------------------

func TestRun_List(t *testing.T) {
	db, mock := novoMock(t)
	mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
	var out bytes.Buffer
	require.NoError(t, resetpassword.Run(ctx, db, resetpassword.Options{List: true}, &out, geradorProibido(t)))
	assert.Contains(t, out.String(), "admin existe: true")

	db, mock = novoMock(t)
	mock.ExpectQuery(reListar).WillReturnError(errBanco)
	err := resetpassword.Run(ctx, db, resetpassword.Options{List: true}, &out, geradorProibido(t))
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "resetpassword: list:"), err.Error())
}

func TestRun_SemAcao(t *testing.T) {
	db, _ := novoMock(t)
	err := resetpassword.Run(ctx, db, resetpassword.Options{Password: "x"}, &bytes.Buffer{}, geradorProibido(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "informe uma ação")
}

func TestRun_CreateAdmin(t *testing.T) {
	t.Run("senha da flag; admin já existe -> só UPDATE", func(t *testing.T) {
		silenciarLog(t)
		t.Setenv("SEED_ADMIN_PASSWORD", "NaoUsar")
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateAdmin).WithArgs(hashDe("Flag@123"), resetpassword.AdminEmail).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
		var out bytes.Buffer
		require.NoError(t, resetpassword.Run(ctx, db, resetpassword.Options{CreateAdmin: true, Password: "Flag@123"}, &out, geradorProibido(t)))
		require.NoError(t, mock.ExpectationsWereMet())
		assert.NotContains(t, out.String(), "Senha gerada")
	})
	t.Run("senha do .env; admin não existe -> INSERT", func(t *testing.T) {
		logs := silenciarLog(t)
		t.Setenv("SEED_ADMIN_PASSWORD", "Env@123")
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateAdmin).WithArgs(hashDe("Env@123"), resetpassword.AdminEmail).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(reInsert).WithArgs("Administrador Principal", resetpassword.AdminEmail, hashDe("Env@123")).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
		require.NoError(t, resetpassword.Run(ctx, db, resetpassword.Options{CreateAdmin: true}, &bytes.Buffer{}, geradorProibido(t)))
		require.NoError(t, mock.ExpectationsWereMet())
		assert.Contains(t, logs.String(), "usando SEED_ADMIN_PASSWORD do .env")
		assert.Contains(t, logs.String(), "admin criado (INSERT)")
	})
	t.Run("sem flag nem .env -> senha gerada é impressa e usada", func(t *testing.T) {
		silenciarLog(t)
		t.Setenv("SEED_ADMIN_PASSWORD", "")
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateAdmin).WithArgs(hashDe(senhaGerada), resetpassword.AdminEmail).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(reListar).WillReturnError(errBanco) // lista final só loga
		var out bytes.Buffer
		require.NoError(t, resetpassword.Run(ctx, db, resetpassword.Options{CreateAdmin: true}, &out, gerador(t)))
		require.NoError(t, mock.ExpectationsWereMet())
		assert.Contains(t, out.String(), "ADMIN_PASSWORD="+senhaGerada)
	})
	t.Run("gerador falha", func(t *testing.T) {
		t.Setenv("SEED_ADMIN_PASSWORD", "")
		db, _ := novoMock(t)
		err := resetpassword.Run(ctx, db, resetpassword.Options{CreateAdmin: true}, &bytes.Buffer{}, geradorQuebrado)
		require.ErrorIs(t, err, errRandom)
		assert.Contains(t, err.Error(), "gerar senha")
	})
	t.Run("UPDATE falha", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateAdmin).WillReturnError(errBanco)
		err := resetpassword.Run(ctx, db, resetpassword.Options{CreateAdmin: true, Password: "x"}, &bytes.Buffer{}, geradorProibido(t))
		require.ErrorIs(t, err, errBanco)
		assert.Contains(t, err.Error(), "resetpassword: create-admin: update admin")
	})
}

func TestRun_AllUsers(t *testing.T) {
	t.Run("troca placeholders; admin existe", func(t *testing.T) {
		logs := silenciarLog(t)
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateAll).WithArgs(hashDe("Todos@123")).WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectQuery(reExisteAdmin).WillReturnRows(sqlmock.NewRows([]string{"e"}).AddRow(true))
		mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
		require.NoError(t, resetpassword.Run(ctx, db, resetpassword.Options{AllUsers: true, Password: "Todos@123"}, &bytes.Buffer{}, geradorProibido(t)))
		require.NoError(t, mock.ExpectationsWereMet())
		assert.Contains(t, logs.String(), "3 linha(s) com placeholder atualizada(s)")
	})
	t.Run("senha do .env; admin ausente é criado inativo", func(t *testing.T) {
		silenciarLog(t)
		t.Setenv("SEED_USER_PASSWORD", "EnvUser@1")
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateAll).WithArgs(hashDe("EnvUser@1")).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(reExisteAdmin).WillReturnRows(sqlmock.NewRows([]string{"e"}).AddRow(false))
		mock.ExpectExec(`INSERT INTO usuarios .*NEEDS_RESET.*'admin', NULL, 0\)`).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
		require.NoError(t, resetpassword.Run(ctx, db, resetpassword.Options{AllUsers: true}, &bytes.Buffer{}, geradorProibido(t)))
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("senha gerada", func(t *testing.T) {
		silenciarLog(t)
		t.Setenv("SEED_USER_PASSWORD", "")
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateAll).WithArgs(hashDe(senhaGerada)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(reExisteAdmin).WillReturnError(errBanco) // ensure-admin só loga
		mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
		var out bytes.Buffer
		require.NoError(t, resetpassword.Run(ctx, db, resetpassword.Options{AllUsers: true}, &out, gerador(t)))
		assert.Contains(t, out.String(), "USER_PASSWORD="+senhaGerada)
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("gerador falha", func(t *testing.T) {
		t.Setenv("SEED_USER_PASSWORD", "")
		db, _ := novoMock(t)
		err := resetpassword.Run(ctx, db, resetpassword.Options{AllUsers: true}, &bytes.Buffer{}, geradorQuebrado)
		require.ErrorIs(t, err, errRandom)
	})
	t.Run("UPDATE falha", func(t *testing.T) {
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateAll).WillReturnError(errBanco)
		err := resetpassword.Run(ctx, db, resetpassword.Options{AllUsers: true, Password: "x"}, &bytes.Buffer{}, geradorProibido(t))
		require.ErrorIs(t, err, errBanco)
		assert.Contains(t, err.Error(), "resetpassword: update all:")
	})
}

func TestRun_Email(t *testing.T) {
	const email = "novo@x.com"
	t.Run("existente -> UPDATE", func(t *testing.T) {
		silenciarLog(t)
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateEmail).WithArgs(hashDe("Nova@123"), email).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
		require.NoError(t, resetpassword.Run(ctx, db, resetpassword.Options{Email: email, Password: "Nova@123"}, &bytes.Buffer{}, geradorProibido(t)))
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("novo sem nome nem vendedor -> INSERT com defaults", func(t *testing.T) {
		silenciarLog(t)
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateEmail).WithArgs(hashDe(senhaGerada), email).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(reInsert).WithArgs("Usuário "+email, email, hashDe(senhaGerada), "normal", nil).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
		var out bytes.Buffer
		opts := resetpassword.Options{Email: email, Role: "normal"}
		require.NoError(t, resetpassword.Run(ctx, db, opts, &out, gerador(t)))
		require.NoError(t, mock.ExpectationsWereMet())
		assert.Contains(t, out.String(), "PASSWORD="+senhaGerada)
	})
	t.Run("novo com nome e vendedor", func(t *testing.T) {
		silenciarLog(t)
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateEmail).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(reInsert).WithArgs("Maria", email, hashDe("M@ria123"), "admin", int64(5)).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery(reListar).WillReturnRows(listaComAdmin())
		opts := resetpassword.Options{Email: email, Password: "M@ria123", Role: "admin", Nome: "Maria", IDVendedor: 5}
		require.NoError(t, resetpassword.Run(ctx, db, opts, &bytes.Buffer{}, geradorProibido(t)))
		require.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("gerador falha", func(t *testing.T) {
		db, _ := novoMock(t)
		err := resetpassword.Run(ctx, db, resetpassword.Options{Email: email}, &bytes.Buffer{}, geradorQuebrado)
		require.ErrorIs(t, err, errRandom)
	})
	t.Run("INSERT falha", func(t *testing.T) {
		silenciarLog(t)
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateEmail).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(reInsert).WillReturnError(errBanco)
		err := resetpassword.Run(ctx, db, resetpassword.Options{Email: email, Password: "x"}, &bytes.Buffer{}, geradorProibido(t))
		require.ErrorIs(t, err, errBanco)
		assert.Contains(t, err.Error(), "resetpassword: upsert: insert")
	})
}

// ---------------------------------------------------------------------------
// Funções de gravação: erros diretos
// ---------------------------------------------------------------------------

func TestGravacao_SenhaAcimaDe72BytesNaoToca(t *testing.T) {
	db, mock := novoMock(t) // nenhuma expectativa: não pode haver SQL
	casos := map[string]func() error{
		"UpsertAdmin":           func() error { return resetpassword.UpsertAdmin(ctx, db, senhaMuitoLonga) },
		"UpsertByEmail":         func() error { return resetpassword.UpsertByEmail(ctx, db, "a@b", senhaMuitoLonga, "normal", "", 0) },
		"UpdateAllPlaceholders": func() error { return resetpassword.UpdateAllPlaceholders(ctx, db, senhaMuitoLonga) },
	}
	for nome, fn := range casos {
		t.Run(nome, func(t *testing.T) {
			err := fn()
			require.ErrorIs(t, err, bcrypt.ErrPasswordTooLong)
			assert.Contains(t, err.Error(), "bcrypt")
		})
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpsertAdmin_InsertFalha(t *testing.T) {
	silenciarLog(t)
	db, mock := novoMock(t)
	mock.ExpectExec(reUpdateAdmin).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(reInsert).WillReturnError(errBanco)
	err := resetpassword.UpsertAdmin(ctx, db, "x")
	require.ErrorIs(t, err, errBanco)
	assert.Contains(t, err.Error(), "insert admin")
}

func TestUpsertByEmail_UpdateFalha(t *testing.T) {
	db, mock := novoMock(t)
	mock.ExpectExec(reUpdateEmail).WillReturnError(errBanco)
	err := resetpassword.UpsertByEmail(ctx, db, "a@b", "x", "normal", "", 0)
	require.ErrorIs(t, err, errBanco)
	assert.Contains(t, err.Error(), "update:")
}

func TestEnsureAdmin(t *testing.T) {
	casos := []struct {
		nome string
		prep func(m sqlmock.Sqlmock)
		err  error
	}{
		{"existe: nada a fazer", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reExisteAdmin).WillReturnRows(sqlmock.NewRows([]string{"e"}).AddRow(true))
		}, nil},
		{"não existe: INSERT inativo", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reExisteAdmin).WillReturnRows(sqlmock.NewRows([]string{"e"}).AddRow(false))
			m.ExpectExec(reInsert).WillReturnResult(sqlmock.NewResult(1, 1))
		}, nil},
		{"consulta falha", func(m sqlmock.Sqlmock) { m.ExpectQuery(reExisteAdmin).WillReturnError(errBanco) }, errBanco},
		{"INSERT falha", func(m sqlmock.Sqlmock) {
			m.ExpectQuery(reExisteAdmin).WillReturnRows(sqlmock.NewRows([]string{"e"}).AddRow(false))
			m.ExpectExec(reInsert).WillReturnError(errBanco)
		}, errBanco},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			silenciarLog(t)
			db, mock := novoMock(t)
			c.prep(mock)
			err := resetpassword.EnsureAdmin(ctx, db)
			if c.err != nil {
				require.ErrorIs(t, err, c.err)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// Utilitários
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	casos := []struct {
		s    string
		n    int
		want string
	}{
		{"curto", 10, "curto"},
		{"exato", 5, "exato"},
		{"abcdefghij", 8, "abcde..."},
		{"", 4, ""},
	}
	for _, c := range casos {
		t.Run(c.s, func(t *testing.T) {
			got := resetpassword.Truncate(c.s, c.n)
			assert.Equal(t, c.want, got)
			assert.LessOrEqual(t, len(got), c.n)
		})
	}
}

func TestGenerateRandomPassword(t *testing.T) {
	alfabeto := regexp.MustCompile(`^[a-zA-Z0-9]*$`)
	vistas := map[string]bool{}
	for _, n := range []int{0, 1, 16, 64} {
		pwd, err := resetpassword.GenerateRandomPassword(n)
		require.NoError(t, err)
		assert.Len(t, pwd, n)
		assert.Regexp(t, alfabeto, pwd)
		if n >= 16 {
			assert.False(t, vistas[pwd], "senhas não se repetem")
			vistas[pwd] = true
		}
	}
}

func TestGetEnvEDSN(t *testing.T) {
	t.Setenv("RP_TESTE_VAZIA", "")
	assert.Equal(t, "padrão", resetpassword.GetEnv("RP_TESTE_VAZIA", "padrão"))
	t.Setenv("RP_TESTE_CHEIA", "valor")
	assert.Equal(t, "valor", resetpassword.GetEnv("RP_TESTE_CHEIA", "padrão"))

	for _, k := range []string{"DB_USUARIO", "DB_SENHA", "DB_HOST", "DB_PORT", "DB_NAME"} {
		t.Setenv(k, "")
	}
	assert.Equal(t, "golang:golang@tcp(localhost:3306)/rotaperfumes?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local&clientFoundRows=true", resetpassword.DSN())

	t.Setenv("DB_USUARIO", "u")
	t.Setenv("DB_SENHA", "s")
	t.Setenv("DB_HOST", "db")
	t.Setenv("DB_PORT", "3307")
	t.Setenv("DB_NAME", "rp")
	dsn := resetpassword.DSN()
	assert.True(t, strings.HasPrefix(dsn, "u:s@tcp(db:3307)/rp?"), dsn)
	assert.Contains(t, dsn, "clientFoundRows=true", "mesma regra de config.DSN (BUG-04)")
	assert.Contains(t, dsn, "loc=Local")
}
