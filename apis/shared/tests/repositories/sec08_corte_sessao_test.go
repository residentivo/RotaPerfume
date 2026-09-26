package repositories_test

// SEC-08: corte de sessão (usuarios.tokens_validos_desde) no
// UsuarioRepository. O valor gravado é gerado pelo Go, truncado em segundos
// (mesma precisão do iat do JWT), e fica entre o instante anterior e o
// posterior à chamada. SetAtivo(true) NÃO mexe na coluna.
//
// O teste de integração (INTEGRATION=1, MySQL local, DSN loc=Local) confere
// no banco que o valor gravado não sofre deslocamento de fuso nem
// arredondamento para o segundo seguinte.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	shareddb "github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/repositories"
)

// argCorte casa com um time.Time sem fração de segundo, dentro de
// [antes truncado, depois].
type argCorte struct{ antes, depois time.Time }

func novoArgCorte() *argCorte { return &argCorte{antes: time.Now()} }

func (a *argCorte) fechar() { a.depois = time.Now() }

func (a *argCorte) Match(v driver.Value) bool {
	t, ok := v.(time.Time)
	if !ok || t.Nanosecond() != 0 {
		return false
	}
	depois := a.depois
	if depois.IsZero() {
		depois = time.Now()
	}
	return !t.Before(a.antes.Truncate(time.Second)) && !t.After(depois)
}

func TestSEC08_ArgCorte_Sanidade(t *testing.T) {
	a := novoArgCorte()
	a.fechar()
	agora := a.antes.Truncate(time.Second)
	assert.True(t, a.Match(agora))
	assert.False(t, a.Match(agora.Add(time.Millisecond)), "com fração não casa")
	assert.False(t, a.Match(agora.Add(-time.Second)), "antes da janela não casa")
	assert.False(t, a.Match(agora.Add(time.Hour)), "depois da janela não casa")
	assert.False(t, a.Match("2026-01-01"), "tipo errado não casa")
}

const (
	reSetAtivoInativar = `^UPDATE usuarios SET ativo = \?, tokens_validos_desde = \? WHERE id = \?$`
	reSetAtivoAtivar   = `^UPDATE usuarios SET ativo = \? WHERE id = \?$`
	reInvalidar        = `^UPDATE usuarios SET tokens_validos_desde = \? WHERE id = \?$`
	reUpdateSenha      = `^UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \?, tokens_validos_desde = \? WHERE id = \?$`
	reInativarVendedor = `^UPDATE usuarios SET ativo = 0, tokens_validos_desde = \? WHERE id_vendedor = \? AND ativo = 1$`
)

func TestSEC08_SetAtivo(t *testing.T) {
	ctx := context.Background()
	repo := repositories.NewUsuarioRepository()

	t.Run("inativar grava corte truncado em segundos", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		corte := novoArgCorte()
		mock.ExpectExec(reSetAtivoInativar).WithArgs(false, corte, int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, repo.SetAtivo(ctx, db, 5, false))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	t.Run("ativar não toca na coluna (2 args, sem tokens_validos_desde)", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectExec(reSetAtivoAtivar).WithArgs(true, int64(5)).WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, repo.SetAtivo(ctx, db, 5, true))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
	for _, ativo := range []bool{true, false} {
		re := reSetAtivoAtivar
		if !ativo {
			re = reSetAtivoInativar
		}
		t.Run(fmt.Sprintf("ativo=%v: 0 linhas -> ErrNotFound", ativo), func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			mock.ExpectExec(re).WillReturnResult(sqlmock.NewResult(0, 0))
			assert.ErrorIs(t, repo.SetAtivo(ctx, db, 5, ativo), repositories.ErrNotFound)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
		t.Run(fmt.Sprintf("ativo=%v: erro de banco embrulhado", ativo), func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			mock.ExpectExec(re).WillReturnError(sql.ErrConnDone)
			err := repo.SetAtivo(ctx, db, 5, ativo)
			assert.ErrorIs(t, err, sql.ErrConnDone)
			assert.Contains(t, err.Error(), "set ativo")
		})
	}
}

func TestSEC08_InativarByVendedorID_GravaCorte(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()
	corte := novoArgCorte()
	mock.ExpectExec(reInativarVendedor).WithArgs(corte, int64(3)).WillReturnResult(sqlmock.NewResult(0, 2))
	n, err := repositories.NewUsuarioRepository().InativarByVendedorID(context.Background(), db, 3)
	require.NoError(t, err)
	assert.EqualValues(t, 2, n)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSEC08_UpdatePasswordHash_GravaCorte(t *testing.T) {
	for _, deveTrocar := range []bool{true, false} {
		t.Run(fmt.Sprintf("deve_trocar_senha=%v", deveTrocar), func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			corte := novoArgCorte()
			mock.ExpectExec(reUpdateSenha).WithArgs("h", deveTrocar, corte, int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
			require.NoError(t, repositories.NewUsuarioRepository().UpdatePasswordHash(context.Background(), db, 9, "h", deveTrocar))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSEC08_InvalidarSessoes(t *testing.T) {
	corte := time.Date(2026, 9, 26, 10, 11, 12, 0, time.Local)
	casos := []struct {
		nome    string
		result  driver.Result
		execErr error
		wantErr error
		wantMsg string
	}{
		{"sucesso", sqlmock.NewResult(0, 1), nil, nil, ""},
		{"0 linhas -> ErrNotFound", sqlmock.NewResult(0, 0), nil, repositories.ErrNotFound, ""},
		{"erro de banco embrulhado", nil, sql.ErrConnDone, sql.ErrConnDone, "invalidar sessoes"},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			exp := mock.ExpectExec(reInvalidar).WithArgs(corte, int64(4))
			if tc.execErr != nil {
				exp.WillReturnError(tc.execErr)
			} else {
				exp.WillReturnResult(tc.result)
			}
			err := repositories.NewUsuarioRepository().InvalidarSessoes(context.Background(), db, 4, corte)
			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.wantErr)
			}
			if tc.wantMsg != "" {
				assert.Contains(t, err.Error(), tc.wantMsg)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
	t.Run("dentro de transação (Execer)", func(t *testing.T) {
		db, mock := newMock(t)
		defer db.Close()
		mock.ExpectBegin()
		mock.ExpectExec(reInvalidar).WithArgs(corte, int64(4)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		tx, err := db.Begin()
		require.NoError(t, err)
		require.NoError(t, repositories.NewUsuarioRepository().InvalidarSessoes(context.Background(), tx, 4, corte))
		require.NoError(t, tx.Commit())
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSEC08_GetStatusByID_Corte(t *testing.T) {
	corte := time.Date(2026, 9, 26, 10, 11, 12, 0, time.Local)
	casos := []struct {
		nome    string
		valor   any
		want    *time.Time
		wantErr bool
	}{
		{"NULL -> nil", nil, nil, false},
		{"preenchido -> valor", corte, &corte, false},
		{"tipo inválido -> erro embrulhado", "nao-e-data", nil, true},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newMock(t)
			defer db.Close()
			mock.ExpectQuery(reGetStatusUsuario).WithArgs(int64(4)).
				WillReturnRows(sqlmock.NewRows([]string{"ativo", "role", "tokens_validos_desde"}).AddRow(true, "normal", tc.valor))
			st, err := repositories.NewUsuarioRepository().GetStatusByID(context.Background(), db, 4)
			if tc.wantErr {
				require.Error(t, err)
				assert.False(t, errors.Is(err, repositories.ErrNotFound))
				assert.Contains(t, err.Error(), "get status usuario")
				return
			}
			require.NoError(t, err)
			if tc.want == nil {
				assert.Nil(t, st.TokensValidosDesde)
			} else {
				require.NotNil(t, st.TokensValidosDesde)
				assert.True(t, tc.want.Equal(*st.TokensValidosDesde))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Integração com o MySQL local (INTEGRATION=1)
// ---------------------------------------------------------------------------

func envOrSEC08(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func abrirDBSEC08(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("teste de integração: defina INTEGRATION=1 (requer MySQL local)")
	}
	cfg := &config.Config{
		DBHost:    envOrSEC08("DB_HOST", "localhost"),
		DBPort:    envOrSEC08("DB_PORT", "3306"),
		DBName:    envOrSEC08("DB_NAME", "rotaperfumes"),
		DBUsuario: envOrSEC08("DB_USUARIO", "golang"),
		DBSenha:   envOrSEC08("DB_SENHA", "golang"),
	}
	db, err := shareddb.Open(cfg.DSN())
	require.NoError(t, err, "MySQL local indisponível")
	t.Cleanup(func() { db.Close() })
	return db
}

// TestIntegracaoSEC08_CorteDeSessao_Repositorio: usuário temporário
// (e-mail zz-test-sec08-<n>@teste.local), apagado no Cleanup.
func TestIntegracaoSEC08_CorteDeSessao_Repositorio(t *testing.T) {
	db := abrirDBSEC08(t)
	ctx := context.Background()
	repo := repositories.NewUsuarioRepository()

	email := fmt.Sprintf("zz-test-sec08-%d@teste.local", time.Now().UnixNano())
	res, err := db.Exec(`INSERT INTO usuarios (nome, email, password_hash, role, ativo) VALUES ('ZZ-TEST-SEC08', ?, 'x', 'normal', 1)`, email)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.Exec(`DELETE FROM usuarios WHERE id = ?`, id)
		assert.NoError(t, err)
	})

	// Texto gravado no banco (sem conversão do driver).
	noBanco := func(t *testing.T) sql.NullString {
		t.Helper()
		var s sql.NullString
		require.NoError(t, db.QueryRow(`SELECT DATE_FORMAT(tokens_validos_desde, '%Y-%m-%d %H:%i:%s.%f') FROM usuarios WHERE id = ?`, id).Scan(&s))
		return s
	}
	// conferir: o valor no banco é o horário local de parede entre antes e
	// depois, sem fração, e a leitura via GetStatusByID devolve o mesmo instante.
	conferir := func(t *testing.T, antes, depois time.Time) time.Time {
		t.Helper()
		s := noBanco(t)
		require.True(t, s.Valid, "corte deveria estar preenchido")
		gravado, err := time.ParseInLocation("2006-01-02 15:04:05.000000", s.String, time.Local)
		require.NoError(t, err, s.String)
		assert.Zero(t, gravado.Nanosecond(), "sem fração de segundo: %s", s.String)
		assert.False(t, gravado.Before(antes.Truncate(time.Second)), "deslocamento para trás: banco=%s antes=%s", s.String, antes)
		assert.False(t, gravado.After(depois), "deslocamento para frente (fuso ou arredondamento): banco=%s depois=%s", s.String, depois)

		st, err := repo.GetStatusByID(ctx, db, id)
		require.NoError(t, err)
		require.NotNil(t, st.TokensValidosDesde)
		assert.Equal(t, gravado.Unix(), st.TokensValidosDesde.Unix(), "leitura pelo driver (loc=Local) sem deslocamento")
		return gravado
	}

	t.Run("recém-criado: NULL", func(t *testing.T) {
		assert.False(t, noBanco(t).Valid)
		st, err := repo.GetStatusByID(ctx, db, id)
		require.NoError(t, err)
		assert.Nil(t, st.TokensValidosDesde)
	})

	var corteInativacao time.Time
	t.Run("SetAtivo(false) grava corte", func(t *testing.T) {
		antes := time.Now()
		require.NoError(t, repo.SetAtivo(ctx, db, id, false))
		corteInativacao = conferir(t, antes, time.Now())
	})
	t.Run("SetAtivo(true) preserva o corte", func(t *testing.T) {
		require.NoError(t, repo.SetAtivo(ctx, db, id, true))
		st, err := repo.GetStatusByID(ctx, db, id)
		require.NoError(t, err)
		assert.True(t, st.Ativo)
		require.NotNil(t, st.TokensValidosDesde)
		assert.Equal(t, corteInativacao.Unix(), st.TokensValidosDesde.Unix())
	})
	t.Run("UpdatePasswordHash grava corte", func(t *testing.T) {
		antes := time.Now()
		require.NoError(t, repo.UpdatePasswordHash(ctx, db, id, "x2", false))
		conferir(t, antes, time.Now())
	})
	t.Run("InvalidarSessoes grava exatamente o instante informado", func(t *testing.T) {
		alvo := time.Now().Add(-37 * time.Minute).Truncate(time.Second)
		require.NoError(t, repo.InvalidarSessoes(ctx, db, id, alvo))
		assert.Equal(t, alvo.Format("2006-01-02 15:04:05")+".000000", noBanco(t).String)
		st, err := repo.GetStatusByID(ctx, db, id)
		require.NoError(t, err)
		assert.True(t, alvo.Equal(*st.TokensValidosDesde))
	})
	t.Run("InvalidarSessoes em id inexistente -> ErrNotFound", func(t *testing.T) {
		var maxID int64
		require.NoError(t, db.QueryRow(`SELECT IFNULL(MAX(id),0) FROM usuarios`).Scan(&maxID))
		assert.ErrorIs(t, repo.InvalidarSessoes(ctx, db, maxID+100000, time.Now().Truncate(time.Second)), repositories.ErrNotFound)
	})
	t.Run("InativarByVendedorID grava corte", func(t *testing.T) {
		vres, err := db.Exec(`INSERT INTO vendedores (nome, regiao, uf, data_admissao, meta_mensal) VALUES ('ZZ-TEST-SEC08-V', 'Teste', 'PR', '2024-01-01', 1)`)
		require.NoError(t, err)
		vid, _ := vres.LastInsertId()
		t.Cleanup(func() {
			_, _ = db.Exec(`UPDATE usuarios SET id_vendedor = NULL WHERE id = ?`, id)
			_, err := db.Exec(`DELETE FROM vendedores WHERE id = ?`, vid)
			assert.NoError(t, err)
		})
		_, err = db.Exec(`UPDATE usuarios SET id_vendedor = ?, ativo = 1 WHERE id = ?`, vid, id)
		require.NoError(t, err)

		antes := time.Now()
		n, err := repo.InativarByVendedorID(ctx, db, vid)
		require.NoError(t, err)
		assert.EqualValues(t, 1, n)
		conferir(t, antes, time.Now())
	})
}
