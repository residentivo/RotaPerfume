package services_test

// AuthService: operações que delegam ao UsuarioRepository (GetUsuarioByEmail,
// ResetPassword), testadas com sqlmock, e o caminho de erro de HashPassword.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/repositories"
	"github.com/rotaperfumes/shared/services"
)

var usuarioCols = []string{
	"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo",
	"deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "v_nome",
}

func TestAuthService_GetUsuarioByEmail(t *testing.T) {
	agora := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	erroBanco := errors.New("conexão perdida")

	casos := []struct {
		nome      string
		prepara   func(sqlmock.Sqlmock)
		wantErrIs error
		wantErr   bool
		confere   func(t *testing.T, email string, idVend *int64, deveTrocar bool)
	}{
		{
			nome: "encontrado com vendedor vinculado",
			prepara: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`WHERE u.email = \?`).WithArgs("ana@rp.com").
					WillReturnRows(sqlmock.NewRows(usuarioCols).AddRow(
						1, "Ana", "ana@rp.com", "hash", "normal", 7, true, true, agora, agora, agora, "Vendedor 7"))
			},
			confere: func(t *testing.T, email string, idVend *int64, deveTrocar bool) {
				assert.Equal(t, "ana@rp.com", email)
				require.NotNil(t, idVend)
				assert.Equal(t, int64(7), *idVend)
				assert.True(t, deveTrocar)
			},
		},
		{
			nome: "encontrado sem vendedor (admin)",
			prepara: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`WHERE u.email = \?`).WithArgs("ana@rp.com").
					WillReturnRows(sqlmock.NewRows(usuarioCols).AddRow(
						2, "Adm", "ana@rp.com", "hash", "admin", nil, true, nil, agora, agora, nil, nil))
			},
			confere: func(t *testing.T, email string, idVend *int64, deveTrocar bool) {
				assert.Nil(t, idVend)
				assert.False(t, deveTrocar)
			},
		},
		{
			nome: "não encontrado",
			prepara: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`WHERE u.email = \?`).WithArgs("ana@rp.com").
					WillReturnRows(sqlmock.NewRows(usuarioCols))
			},
			wantErrIs: repositories.ErrNotFound,
		},
		{
			nome: "erro do banco",
			prepara: func(m sqlmock.Sqlmock) {
				m.ExpectQuery(`WHERE u.email = \?`).WithArgs("ana@rp.com").WillReturnError(erroBanco)
			},
			wantErrIs: erroBanco,
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			c.prepara(mock)

			u, err := services.NewAuthService().GetUsuarioByEmail(context.Background(), db, "ana@rp.com")
			if c.wantErrIs != nil {
				assert.ErrorIs(t, err, c.wantErrIs)
				assert.Nil(t, u)
			} else {
				require.NoError(t, err)
				require.NotNil(t, u)
				c.confere(t, u.Email, u.IDVendedor, u.DeveTrocarSenha)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAuthService_ResetPassword(t *testing.T) {
	erroBanco := errors.New("deadlock")

	casos := []struct {
		nome       string
		deveTrocar bool
		resultado  func(*sqlmock.ExpectedExec)
		wantErrIs  error
	}{
		{"sucesso forçando troca", true, func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 1)) }, nil},
		{"sucesso sem forçar troca", false, func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 1)) }, nil},
		{"usuário inexistente", true, func(e *sqlmock.ExpectedExec) { e.WillReturnResult(sqlmock.NewResult(0, 0)) }, repositories.ErrNotFound},
		{"erro do banco", false, func(e *sqlmock.ExpectedExec) { e.WillReturnError(erroBanco) }, erroBanco},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			c.resultado(mock.ExpectExec(`UPDATE usuarios SET password_hash = \?, deve_trocar_senha = \? WHERE id = \?`).
				WithArgs("novo-hash", c.deveTrocar, int64(9)))

			err = services.NewAuthService().ResetPassword(context.Background(), db, 9, "novo-hash", c.deveTrocar)
			if c.wantErrIs != nil {
				assert.ErrorIs(t, err, c.wantErrIs)
			} else {
				assert.NoError(t, err)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAuthService_HashPassword_Erros(t *testing.T) {
	casos := []struct {
		nome  string
		cost  int
		senha string
	}{
		{"cost acima do máximo do bcrypt", 99, "senha"},
		{"senha acima de 72 bytes", 4, string(make([]byte, 73))},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			hash, err := services.NewAuthService().HashPassword(&config.Config{BCryptCost: c.cost}, c.senha)
			require.Error(t, err)
			assert.Empty(t, hash)
			assert.Contains(t, err.Error(), "auth: hash falhou")
		})
	}
}
