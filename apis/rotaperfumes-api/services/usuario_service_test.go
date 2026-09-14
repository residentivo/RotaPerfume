package services_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
)

func usuarioTestCfg(verbose bool) *config.Config {
	return &config.Config{Verbose: verbose, BCryptCost: 4}
}

func newUsuarioTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

// fakeEmailService é um mock de sharedsvc.EmailService — nunca golpeia rede
// real. Permite simular sucesso ou falha no envio e capturar os argumentos
// recebidos para asserção.
type fakeEmailService struct {
	err            error
	chamadas       int
	ultimoDestino  string
	ultimoNomeUsr  string
	ultimoSenhaEnv string
}

func (f *fakeEmailService) EnviarSenhaInicial(ctx context.Context, destinatario, nomeUsuario, senha string) error {
	f.chamadas++
	f.ultimoDestino = destinatario
	f.ultimoNomeUsr = nomeUsuario
	f.ultimoSenhaEnv = senha
	return f.err
}

const usuarioColunasRegex = `u\.id, u\.nome, u\.email, u\.password_hash, u\.role, u\.id_vendedor, u\.ativo,\s+u\.deve_trocar_senha, u\.created_at, u\.updated_at, u\.ultimo_login_at, v\.nome\s+FROM usuarios u\s+LEFT JOIN vendedores v ON v\.id = u\.id_vendedor`

func usuarioColunasHeader() []string {
	return []string{
		"id", "nome", "email", "password_hash", "role",
		"id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome",
	}
}

func usuarioRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(usuarioColunasHeader()).
		AddRow(int64(1), "Fulano", "fulano@test.com", "hash-bcrypt", models.RoleNormal, nil, true, false, now, now, nil, nil)
}

func emptyUsuarioRows() *sqlmock.Rows {
	return sqlmock.NewRows(usuarioColunasHeader())
}

// ---------------------------------------------------------------------------
// GetUsuarioByEmail / GetUsuarioByID
// ---------------------------------------------------------------------------

func TestUsuarioService_GetUsuarioByEmail(t *testing.T) {
	t.Run("encontrado", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.email = \?\s+LIMIT 1`).
			WithArgs("fulano@test.com").
			WillReturnRows(usuarioRows())

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		u, err := svc.GetUsuarioByEmail(context.Background(), db, "fulano@test.com")
		require.NoError(t, err)
		assert.Equal(t, "Fulano", u.Nome)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("não encontrado retorna ErrUsuarioNaoEncontrado", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.email = \?\s+LIMIT 1`).
			WithArgs("naoexiste@test.com").
			WillReturnError(sql.ErrNoRows)

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		u, err := svc.GetUsuarioByEmail(context.Background(), db, "naoexiste@test.com")
		assert.Nil(t, u)
		assert.ErrorIs(t, err, services.ErrUsuarioNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro genérico do repo é propagado", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.email = \?\s+LIMIT 1`).
			WithArgs("fulano@test.com").
			WillReturnError(sql.ErrConnDone)

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		u, err := svc.GetUsuarioByEmail(context.Background(), db, "fulano@test.com")
		assert.Nil(t, u)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestUsuarioService_GetUsuarioByID(t *testing.T) {
	t.Run("encontrado", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(usuarioRows())

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		u, err := svc.GetUsuarioByID(context.Background(), db, 1)
		require.NoError(t, err)
		assert.Equal(t, int64(1), u.ID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("não encontrado retorna ErrUsuarioNaoEncontrado", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
			WithArgs(int64(999)).
			WillReturnError(sql.ErrNoRows)

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		u, err := svc.GetUsuarioByID(context.Background(), db, 999)
		assert.Nil(t, u)
		assert.ErrorIs(t, err, services.ErrUsuarioNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// ListUsuarios
// ---------------------------------------------------------------------------

func TestUsuarioService_ListUsuarios(t *testing.T) {
	testCases := []struct {
		nome    string
		mock    func(mock sqlmock.Sqlmock)
		wantErr bool
		wantLen int
		wantTot int
	}{
		{
			nome: "sucesso",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM usuarios`).
					WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
				mock.ExpectQuery(usuarioColunasRegex+`\s+ORDER BY u\.id ASC\s+LIMIT \? OFFSET \?`).
					WithArgs(20, 0).
					WillReturnRows(usuarioRows())
			},
			wantLen: 1,
			wantTot: 1,
		},
		{
			nome: "erro no count é propagado",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM usuarios`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newUsuarioTestDB(t)
			tc.mock(mock)

			svc := services.NewUsuarioService(db, usuarioTestCfg(true), &fakeEmailService{})
			usuarios, total, err := svc.ListUsuarios(context.Background(), db, 1, 20)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, usuarios, tc.wantLen)
				assert.Equal(t, tc.wantTot, total)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// ResetSenha
// ---------------------------------------------------------------------------

func TestUsuarioService_ResetSenha(t *testing.T) {
	t.Run("sucesso", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \? WHERE id = \?`).
			WithArgs(sqlmock.AnyArg(), false, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		err := svc.ResetSenha(context.Background(), db, 1, "nova-senha-123")
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("usuário não encontrado", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \? WHERE id = \?`).
			WithArgs(sqlmock.AnyArg(), false, int64(999)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		err := svc.ResetSenha(context.Background(), db, 999, "nova-senha-123")
		assert.ErrorIs(t, err, services.ErrUsuarioNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro do repo é propagado", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \? WHERE id = \?`).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		err := svc.ResetSenha(context.Background(), db, 1, "nova-senha-123")
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// AdminResetPassword
// ---------------------------------------------------------------------------

func TestUsuarioService_AdminResetPassword(t *testing.T) {
	t.Run("sucesso - email enviado", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \? WHERE id = \?`).
			WithArgs(sqlmock.AnyArg(), true, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		email := &fakeEmailService{}
		svc := services.NewUsuarioService(db, usuarioTestCfg(true), email)
		enviado, err := svc.AdminResetPassword(context.Background(), db, 1, "fulano@test.com", "Fulano")
		require.NoError(t, err)
		assert.True(t, enviado)
		assert.Equal(t, 1, email.chamadas)
		assert.Equal(t, "fulano@test.com", email.ultimoDestino)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("sucesso - falha no envio de email não desfaz o reset", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \? WHERE id = \?`).
			WithArgs(sqlmock.AnyArg(), true, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		email := &fakeEmailService{err: assertError()}
		svc := services.NewUsuarioService(db, usuarioTestCfg(false), email)
		enviado, err := svc.AdminResetPassword(context.Background(), db, 1, "fulano@test.com", "Fulano")
		require.NoError(t, err)
		assert.False(t, enviado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("usuário não encontrado", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \? WHERE id = \?`).
			WillReturnResult(sqlmock.NewResult(0, 0))

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		enviado, err := svc.AdminResetPassword(context.Background(), db, 999, "fulano@test.com", "Fulano")
		assert.False(t, enviado)
		assert.ErrorIs(t, err, services.ErrUsuarioNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func assertError() error { return sql.ErrTxDone }

// ---------------------------------------------------------------------------
// CreateUsuario
// ---------------------------------------------------------------------------

func TestUsuarioService_CreateUsuario(t *testing.T) {
	idVendedor := int64(5)

	t.Run("nome vazio", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		u, enviado, err := svc.CreateUsuario(context.Background(), db, struct {
			Nome       string
			Email      string
			Role       string
			IDVendedor *int64
		}{Nome: "", Email: "a@test.com", Role: models.RoleNormal})
		assert.Nil(t, u)
		assert.False(t, enviado)
		assert.ErrorIs(t, err, services.ErrNomeObrigatorio)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("email vazio", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		u, enviado, err := svc.CreateUsuario(context.Background(), db, struct {
			Nome       string
			Email      string
			Role       string
			IDVendedor *int64
		}{Nome: "Fulano", Email: "", Role: models.RoleNormal})
		assert.Nil(t, u)
		assert.False(t, enviado)
		assert.ErrorIs(t, err, services.ErrEmailInvalido)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("role inválido", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		u, enviado, err := svc.CreateUsuario(context.Background(), db, struct {
			Nome       string
			Email      string
			Role       string
			IDVendedor *int64
		}{Nome: "Fulano", Email: "a@test.com", Role: "gerente"})
		assert.Nil(t, u)
		assert.False(t, enviado)
		assert.ErrorIs(t, err, services.ErrRoleInvalido)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("vendedor inexistente", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
			WithArgs(idVendedor).
			WillReturnError(sql.ErrNoRows)

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		u, enviado, err := svc.CreateUsuario(context.Background(), db, struct {
			Nome       string
			Email      string
			Role       string
			IDVendedor *int64
		}{Nome: "Fulano", Email: "a@test.com", Role: models.RoleNormal, IDVendedor: &idVendedor})
		assert.Nil(t, u)
		assert.False(t, enviado)
		assert.ErrorIs(t, err, services.ErrVendedorNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("email duplicado", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectExec(`INSERT INTO usuarios \(nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha\)`).
			WillReturnError(repositories.ErrEmailDuplicado)

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		u, enviado, err := svc.CreateUsuario(context.Background(), db, struct {
			Nome       string
			Email      string
			Role       string
			IDVendedor *int64
		}{Nome: "Fulano", Email: "a@test.com", Role: models.RoleNormal})
		assert.Nil(t, u)
		assert.False(t, enviado)
		assert.ErrorIs(t, err, services.ErrEmailDuplicado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("sucesso - email enviado", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectExec(`INSERT INTO usuarios \(nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha\)`).
			WithArgs("Fulano", "a@test.com", sqlmock.AnyArg(), models.RoleNormal, nil, true, true).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(sqlmock.NewRows(usuarioColunasHeader()).
				AddRow(int64(1), "Fulano", "a@test.com", "hash-bcrypt", models.RoleNormal, nil, true, true, time.Now(), time.Now(), nil, nil))

		email := &fakeEmailService{}
		svc := services.NewUsuarioService(db, usuarioTestCfg(true), email)
		u, enviado, err := svc.CreateUsuario(context.Background(), db, struct {
			Nome       string
			Email      string
			Role       string
			IDVendedor *int64
		}{Nome: "Fulano", Email: "A@Test.com", Role: models.RoleNormal})
		require.NoError(t, err)
		require.NotNil(t, u)
		assert.True(t, enviado)
		assert.Equal(t, 1, email.chamadas)
		assert.Equal(t, "a@test.com", email.ultimoDestino)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// UpdateUsuario
// ---------------------------------------------------------------------------

func TestUsuarioService_UpdateUsuario(t *testing.T) {
	idVendedor := int64(5)

	testCases := []struct {
		nome       string
		usuarioNom string
		role       string
		idVendedor *int64
		mock       func(mock sqlmock.Sqlmock)
		wantErr    error
	}{
		{
			nome:       "nome vazio",
			usuarioNom: "",
			role:       models.RoleNormal,
			mock:       func(mock sqlmock.Sqlmock) {},
			wantErr:    services.ErrNomeObrigatorio,
		},
		{
			nome:       "role inválido",
			usuarioNom: "Fulano",
			role:       "gerente",
			mock:       func(mock sqlmock.Sqlmock) {},
			wantErr:    services.ErrRoleInvalido,
		},
		{
			nome:       "vendedor inexistente",
			usuarioNom: "Fulano",
			role:       models.RoleNormal,
			idVendedor: &idVendedor,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT 1 FROM vendedores WHERE id = \? LIMIT 1`).
					WithArgs(idVendedor).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: services.ErrVendedorNaoEncontrado,
		},
		{
			nome:       "usuário não encontrado",
			usuarioNom: "Fulano",
			role:       models.RoleNormal,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE usuarios SET nome = \?, role = \?, id_vendedor = \? WHERE id = \?`).
					WithArgs("Fulano", models.RoleNormal, nil, int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			wantErr: services.ErrUsuarioNaoEncontrado,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newUsuarioTestDB(t)
			tc.mock(mock)

			svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
			u, err := svc.UpdateUsuario(context.Background(), db, 1, tc.usuarioNom, tc.role, tc.idVendedor)
			assert.Nil(t, u)
			assert.ErrorIs(t, err, tc.wantErr)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}

	t.Run("sucesso", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectExec(`UPDATE usuarios SET nome = \?, role = \?, id_vendedor = \? WHERE id = \?`).
			WithArgs("Fulano Editado", models.RoleAdmin, nil, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(usuarioRows())

		svc := services.NewUsuarioService(db, usuarioTestCfg(true), &fakeEmailService{})
		u, err := svc.UpdateUsuario(context.Background(), db, 1, "Fulano Editado", models.RoleAdmin, nil)
		require.NoError(t, err)
		require.NotNil(t, u)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("erro no GetByID pós-update é propagado", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectExec(`UPDATE usuarios SET nome = \?, role = \?, id_vendedor = \? WHERE id = \?`).
			WithArgs("Fulano", models.RoleNormal, nil, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnError(sql.ErrConnDone)

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		u, err := svc.UpdateUsuario(context.Background(), db, 1, "Fulano", models.RoleNormal, nil)
		assert.Nil(t, u)
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// ToggleAtivoUsuario
// ---------------------------------------------------------------------------

func TestUsuarioService_ToggleAtivoUsuario(t *testing.T) {
	t.Run("ativo nil - inverte status atual (toggle)", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(usuarioRows())
		mock.ExpectExec(`UPDATE usuarios SET ativo = \? WHERE id = \?`).
			WithArgs(false, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		svc := services.NewUsuarioService(db, usuarioTestCfg(true), &fakeEmailService{})
		u, err := svc.ToggleAtivoUsuario(context.Background(), db, 1, nil)
		require.NoError(t, err)
		assert.False(t, u.Ativo)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("ativo explícito", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(usuarioRows())
		mock.ExpectExec(`UPDATE usuarios SET ativo = \? WHERE id = \?`).
			WithArgs(true, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 1))

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		ativo := true
		u, err := svc.ToggleAtivoUsuario(context.Background(), db, 1, &ativo)
		require.NoError(t, err)
		assert.True(t, u.Ativo)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("usuário não encontrado no GetByID", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
			WithArgs(int64(999)).
			WillReturnError(sql.ErrNoRows)

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		u, err := svc.ToggleAtivoUsuario(context.Background(), db, 999, nil)
		assert.Nil(t, u)
		assert.ErrorIs(t, err, services.ErrUsuarioNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("usuário some entre GetByID e SetAtivo (corrida)", func(t *testing.T) {
		db, mock := newUsuarioTestDB(t)
		mock.ExpectQuery(usuarioColunasRegex + `\s+WHERE u\.id = \?\s+LIMIT 1`).
			WithArgs(int64(1)).
			WillReturnRows(usuarioRows())
		mock.ExpectExec(`UPDATE usuarios SET ativo = \? WHERE id = \?`).
			WithArgs(false, int64(1)).
			WillReturnResult(sqlmock.NewResult(0, 0))

		svc := services.NewUsuarioService(db, usuarioTestCfg(false), &fakeEmailService{})
		u, err := svc.ToggleAtivoUsuario(context.Background(), db, 1, nil)
		assert.Nil(t, u)
		assert.ErrorIs(t, err, services.ErrUsuarioNaoEncontrado)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// ParsePagination
// ---------------------------------------------------------------------------

func TestParsePagination(t *testing.T) {
	testCases := []struct {
		nome      string
		pageStr   string
		limitStr  string
		wantPage  int
		wantLimit int
	}{
		{"defaults quando vazio", "", "", 1, 20},
		{"valores válidos", "2", "50", 2, 50},
		{"page inválido (não numérico) vira 1", "abc", "10", 1, 10},
		{"page negativo vira 1", "-5", "10", 1, 10},
		{"limit maior que 100 é capado", "1", "500", 1, 100},
		{"limit zero vira 20", "1", "0", 1, 20},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			page, limit := services.ParsePagination(tc.pageStr, tc.limitStr)
			assert.Equal(t, tc.wantPage, page)
			assert.Equal(t, tc.wantLimit, limit)
		})
	}
}
