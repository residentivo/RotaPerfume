// SEC-13: migração transparente do hash de senha no login. Um hash bcrypt
// legado (ou Argon2id com parâmetros antigos) é regravado como Argon2id com
// pepper logo após a senha ser confirmada, sem cortar sessões
// (tokens_validos_desde não muda) e sem afetar o login se o UPDATE falhar.
package handlers_test

import (
	"database/sql/driver"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/services"
)

const reRehash = `UPDATE usuarios SET password_hash = \? WHERE id = \? AND password_hash = \?`

// argon2idDe casa só com um hash Argon2id atual (parâmetros de testCfg) da senha.
type argon2idDe string

func (s argon2idDe) Match(v driver.Value) bool {
	h, ok := v.(string)
	cfg := testCfg()
	auth := services.NewAuthService()
	return ok && auth.VerifyPassword(cfg, h, string(s)) && !auth.NeedsRehash(cfg, h)
}

func esperarUsuarioLogin(mock sqlmock.Sqlmock, hash string) {
	mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores`).
		WithArgs("admin@test.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
			AddRow(int64(1), "Admin User", "admin@test.com", hash, "admin", nil, true, false, time.Now(), time.Now(), nil, nil))
}

func esperarRestoDoLogin(mock sqlmock.Sqlmock) {
	mock.ExpectExec(`INSERT INTO refresh_tokens`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE usuarios SET ultimo_login_at = \? WHERE id = \?`).
		WithArgs(sqlmock.AnyArg(), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func postLogin(t *testing.T, url, senha string) int {
	t.Helper()
	resp, err := http.Post(url+"/api/auth/login", "application/json",
		makeJSON(map[string]string{"email": "admin@test.com", "password": senha, "captchaToken": "token-valido-de-teste"}))
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// hashArgon2Antigo gera um Argon2id da senha com parâmetros diferentes dos atuais.
func hashArgon2Antigo(t *testing.T, senha string) string {
	t.Helper()
	antigo := testCfg().HashSenha
	antigo.MemoriaKiB = 128
	h, err := services.GerarHashSenha(antigo, senha)
	require.NoError(t, err)
	return h
}

func TestSEC13_Login_RehashParaArgon2id(t *testing.T) {
	legado, err := bcrypt.GenerateFromPassword([]byte("senha-correta"), bcrypt.MinCost)
	require.NoError(t, err)

	casos := map[string]string{
		"bcrypt legado":                   string(legado),
		"argon2id com parâmetros antigos": hashArgon2Antigo(t, "senha-correta"),
	}
	for nome, hashAntigo := range casos {
		t.Run(nome, func(t *testing.T) {
			server, db, mock := setupTestServer(t)
			defer server.Close()
			defer db.Close()

			esperarUsuarioLogin(mock, hashAntigo)
			mock.ExpectExec(reRehash).
				WithArgs(argon2idDe("senha-correta"), int64(1), hashAntigo).
				WillReturnResult(sqlmock.NewResult(0, 1))
			esperarRestoDoLogin(mock)

			assert.Equal(t, http.StatusOK, postLogin(t, server.URL, "senha-correta"))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSEC13_Login_RehashFalhaNaoDerrubaLogin(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	legado, err := bcrypt.GenerateFromPassword([]byte("senha-correta"), bcrypt.MinCost)
	require.NoError(t, err)
	esperarUsuarioLogin(mock, string(legado))
	mock.ExpectExec(reRehash).WillReturnError(errors.New("falha simulada"))
	esperarRestoDoLogin(mock)

	assert.Equal(t, http.StatusOK, postLogin(t, server.URL, "senha-correta"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Senha errada contra hash legado: 401 e nenhum UPDATE.
func TestSEC13_Login_BcryptLegadoSenhaErrada(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	legado, err := bcrypt.GenerateFromPassword([]byte("senha-correta"), bcrypt.MinCost)
	require.NoError(t, err)
	esperarUsuarioLogin(mock, string(legado))

	assert.Equal(t, http.StatusUnauthorized, postLogin(t, server.URL, "senha-errada"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Hash atual: login normal, sem UPDATE de rehash (sqlmock falha com SQL inesperado).
func TestSEC13_Login_HashAtualNaoRegrava(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	hash, err := services.GerarHashSenha(testCfg().HashSenha, "senha-correta")
	require.NoError(t, err)
	esperarUsuarioLogin(mock, hash)
	esperarRestoDoLogin(mock)

	assert.Equal(t, http.StatusOK, postLogin(t, server.URL, "senha-correta"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// Um hash gerado com outro pepper não autentica (o pepper faz parte do hash).
func TestSEC13_Login_PepperDiferenteRecusa(t *testing.T) {
	server, db, mock := setupTestServer(t)
	defer server.Close()
	defer db.Close()

	outro := config.HashSenha{Pepper: "um-pepper-de-outro-ambiente-32-bytes!!", MemoriaKiB: 64, Iteracoes: 1, Paralelismo: 1}
	hash, err := services.GerarHashSenha(outro, "senha-correta")
	require.NoError(t, err)
	esperarUsuarioLogin(mock, hash)

	assert.Equal(t, http.StatusUnauthorized, postLogin(t, server.URL, "senha-correta"))
	assert.NoError(t, mock.ExpectationsWereMet())
}
