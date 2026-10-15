package repositories_test

// ARQ-05 (cobertura do portão): ramos de scan ainda não testados do legado
// de Identidade que segue em produção até o ARQ-07 (GetByID é usado pelo
// notificador de alertas SEC-09; FindByUsuario fica com os testes).

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/repositories"
)

func TestARQ05_UsuarioGetByID_ComUltimoLoginEVendedor(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()
	agora := time.Now()
	mock.ExpectQuery(`WHERE u\.id = \?`).WithArgs(int64(4)).WillReturnRows(sqlmock.NewRows(
		[]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
		AddRow(int64(4), "Ana", "ana@rp.test", "h", "normal", int64(3), true, true, agora, agora, agora, "Vendedora"))

	u, err := novoUsuarioRepo().GetByID(context.Background(), db, 4)

	require.NoError(t, err)
	require.NotNil(t, u.IDVendedor)
	assert.Equal(t, int64(3), *u.IDVendedor)
	require.NotNil(t, u.UltimoLoginAt)
	require.NotNil(t, u.VendedorNome)
	assert.Equal(t, "Vendedora", *u.VendedorNome)
	assert.True(t, u.DeveTrocarSenha)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestARQ05_RefreshFindByUsuario_MotivoEFalhaDeScan(t *testing.T) {
	colunas := []string{"id", "usuario_id", "token_hash", "expires_at", "revoked_at", "ip_origem", "user_agent", "revoked_reason"}
	agora := time.Now()

	db, mock := newMock(t)
	defer db.Close()
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM refresh_tokens`).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1))
	mock.ExpectQuery(`FROM refresh_tokens`).WillReturnRows(sqlmock.NewRows(colunas).
		AddRow(int64(1), int64(4), "hash", agora, agora, "ip", "ua", "logout"))
	tokens, total, err := novoRefreshRepo().FindByUsuario(context.Background(), db, 4, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, tokens, 1)
	assert.Equal(t, repositories.RevokeReason("logout"), tokens[0].RevokedReason)

	db2, mock2 := newMock(t)
	defer db2.Close()
	mock2.ExpectQuery(`SELECT COUNT\(\*\) FROM refresh_tokens`).WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(1))
	mock2.ExpectQuery(`FROM refresh_tokens`).WillReturnRows(sqlmock.NewRows(colunas).
		AddRow("x", int64(4), "hash", agora, nil, "ip", "ua", nil))
	_, _, err = novoRefreshRepo().FindByUsuario(context.Background(), db2, 4, 1, 20)
	assert.ErrorContains(t, err, "repositories: scan refresh_token")
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.NoError(t, mock2.ExpectationsWereMet())
}
