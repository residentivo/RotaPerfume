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
)

func newSenhaHistoricoTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, mock
}

func senhaHistoricoColunas() []string {
	return []string{
		"id", "usuario_id", "resetado_por_id", "senha_hash_anterior",
		"ip_origem", "user_agent", "tipo_reset", "created_at",
	}
}

func senhaHistoricoRows() *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(senhaHistoricoColunas()).
		AddRow(int64(1), int64(10), nil, "hash-antigo", "127.0.0.1", "curl/8.0", "usuario", now)
}

// ---------------------------------------------------------------------------
// Registrar
// ---------------------------------------------------------------------------

func TestSenhaHistoricoService_Registrar(t *testing.T) {
	adminID := int64(99)

	testCases := []struct {
		nome          string
		resetadoPorID *int64
		tipo          string
		mock          func(mock sqlmock.Sqlmock)
		wantErr       bool
	}{
		{
			nome:          "sucesso - reset feito pelo próprio usuário (resetadoPorID nil)",
			resetadoPorID: nil,
			tipo:          services.TipoResetUsuario,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO senha_historico \(usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset\)`).
					WithArgs(int64(10), nil, "hash-antigo", "127.0.0.1", "curl/8.0", services.TipoResetUsuario).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			nome:          "sucesso - reset feito por admin",
			resetadoPorID: &adminID,
			tipo:          services.TipoResetAdmin,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO senha_historico \(usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset\)`).
					WithArgs(int64(10), adminID, "hash-antigo", "127.0.0.1", "curl/8.0", services.TipoResetAdmin).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			nome:          "sucesso - tipo primeiro acesso",
			resetadoPorID: nil,
			tipo:          services.TipoResetPrimeiroAcesso,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO senha_historico \(usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset\)`).
					WithArgs(int64(10), nil, "hash-antigo", "127.0.0.1", "curl/8.0", services.TipoResetPrimeiroAcesso).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			nome:          "sucesso - tipo esquecimento",
			resetadoPorID: nil,
			tipo:          services.TipoResetEsquecimento,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO senha_historico \(usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset\)`).
					WithArgs(int64(10), nil, "hash-antigo", "127.0.0.1", "curl/8.0", services.TipoResetEsquecimento).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			nome:          "erro do repo é propagado",
			resetadoPorID: nil,
			tipo:          services.TipoResetUsuario,
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO senha_historico \(usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset\)`).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newSenhaHistoricoTestDB(t)
			tc.mock(mock)

			svc := services.NewSenhaHistoricoService()
			err := svc.Registrar(context.Background(), db, 10, tc.resetadoPorID, "hash-antigo", "127.0.0.1", "curl/8.0", tc.tipo)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// ListarPorUsuario
// ---------------------------------------------------------------------------

func TestSenhaHistoricoService_ListarPorUsuario(t *testing.T) {
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
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
					WithArgs(int64(10)).
					WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(1))
				mock.ExpectQuery(`SELECT id, usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset, created_at\s+FROM senha_historico\s+WHERE usuario_id = \?\s+ORDER BY id DESC\s+LIMIT \? OFFSET \?`).
					WithArgs(int64(10), 20, 0).
					WillReturnRows(senhaHistoricoRows())
			},
			wantLen: 1,
			wantTot: 1,
		},
		{
			nome: "erro no count é propagado",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico WHERE usuario_id = \?`).
					WithArgs(int64(10)).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newSenhaHistoricoTestDB(t)
			tc.mock(mock)

			svc := services.NewSenhaHistoricoService()
			historico, total, err := svc.ListarPorUsuario(context.Background(), db, 10, 1, 20, "", "")

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, historico, tc.wantLen)
				assert.Equal(t, tc.wantTot, total)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// ListarTodos
// ---------------------------------------------------------------------------

func TestSenhaHistoricoService_ListarTodos(t *testing.T) {
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
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico`).
					WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))
				mock.ExpectQuery(`SELECT id, usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset, created_at\s+FROM senha_historico\s+ORDER BY id DESC\s+LIMIT \? OFFSET \?`).
					WithArgs(20, 0).
					WillReturnRows(senhaHistoricoRows())
			},
			wantLen: 1,
			wantTot: 2,
		},
		{
			nome: "erro no list é propagado",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM senha_historico`).
					WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(2))
				mock.ExpectQuery(`SELECT id, usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset, created_at\s+FROM senha_historico\s+ORDER BY id DESC\s+LIMIT \? OFFSET \?`).
					WithArgs(20, 0).
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			db, mock := newSenhaHistoricoTestDB(t)
			tc.mock(mock)

			svc := services.NewSenhaHistoricoService()
			historico, total, err := svc.ListarTodos(context.Background(), db, 1, 20, "", "")

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, historico, tc.wantLen)
				assert.Equal(t, tc.wantTot, total)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
