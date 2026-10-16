package repositories_test

// ARQ-05 (cobertura do portão): ramos de scan de Identidade. ARQ-07: o
// legado (repositories.UsuarioRepository.GetByID e
// RefreshTokenRepository.FindByUsuario) saiu; os testes, com os mesmos
// nomes, exercitam o scan equivalente de identidademysql (Usuarios.PorID,
// mesmo SQL do GetByID, e RefreshTokens.PorHash, mesmo scan de refresh
// token, com motivo e falha de scan).

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
)

func TestARQ05_UsuarioGetByID_ComUltimoLoginEVendedor(t *testing.T) {
	db, mock := newMock(t)
	defer db.Close()
	agora := time.Now()
	mock.ExpectQuery(`FROM usuarios u\s+LEFT JOIN vendedores v ON v\.id = u\.id_vendedor\s+WHERE u\.id = \?\s+LIMIT 1`).WithArgs(int64(4)).WillReturnRows(sqlmock.NewRows(
		[]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
		AddRow(int64(4), "Ana", "ana@rp.test", "h", "normal", int64(3), true, true, agora, agora, agora, "Vendedora"))

	u, err := identidademysql.NovoUsuarios(db).PorID(context.Background(), 4)

	require.NoError(t, err)
	require.NotNil(t, u.IDVendedor)
	assert.Equal(t, int64(3), *u.IDVendedor)
	require.NotNil(t, u.UltimoLoginEm)
	assert.True(t, agora.Equal(*u.UltimoLoginEm))
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
	mock.ExpectQuery(`FROM refresh_tokens\s+WHERE token_hash = \?`).WithArgs("hash").WillReturnRows(sqlmock.NewRows(colunas).
		AddRow(int64(1), int64(4), "hash", agora, agora, "ip", "ua", "logout"))
	rt, err := identidademysql.NovoRefreshTokens(db).PorHash(context.Background(), "hash")
	require.NoError(t, err)
	assert.Equal(t, int64(4), rt.UsuarioID)
	require.NotNil(t, rt.RevogadoEm)
	assert.Equal(t, identidade.MotivoLogout, rt.Motivo)

	db2, mock2 := newMock(t)
	defer db2.Close()
	mock2.ExpectQuery(`FROM refresh_tokens\s+WHERE token_hash = \?`).WillReturnRows(sqlmock.NewRows(colunas).
		AddRow("x", int64(4), "hash", agora, nil, "ip", "ua", nil))
	_, err = identidademysql.NovoRefreshTokens(db2).PorHash(context.Background(), "hash")
	assert.ErrorContains(t, err, "identidademysql: scan refresh_token")
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.NoError(t, mock2.ExpectationsWereMet())
}
