package handlers_test

// DEPLOY-07 (Lote 33, TestBrain): em produção os 42 vendedores ficam com o
// hash PLACEHOLDER dos seeds (o seed-producao.sh não roda -all-users). Login
// com esse hash, com a senha que for, responde 401 "credenciais inválidas"
// (igual a senha errada), nunca 500 e nunca sessão. Os placeholders são lidos
// dos próprios sql/02 e sql/03, para o teste acompanhar o arquivo.

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DATA-DOG/go-sqlmock"
)

var rePlaceholderSQL = regexp.MustCompile(`'(\$2a\$12\$XXXXPLACEHOLDER[A-Z0-9_]+XXXX)'`)

func placeholderDoSeed(t *testing.T, arquivo string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "sql", arquivo))
	require.NoError(t, err)
	m := rePlaceholderSQL.FindStringSubmatch(string(b))
	require.NotNil(t, m, "placeholder em sql/%s", arquivo)
	return m[1]
}

func TestDEPLOY07_Login_HashPlaceholder_401(t *testing.T) {
	casos := []struct {
		nome, arquivo, email, role string
		idVendedor                 any
		ativo                      bool
		senha                      string
	}{
		{"admin do 02", "02_seed_admin.sql", "admin@rotaperfumes.com.br", "admin", nil, true, "Admin@123"},
		{"admin do 02, senha igual a trecho do placeholder", "02_seed_admin.sql", "admin@rotaperfumes.com.br", "admin", nil, true, "XXXXPLACEHOLDER"},
		{"vendedor do 03 (ativo em dev)", "03_seed_vendedores.sql", "carla.carvalho@rotaperfumes.com.br", "normal", int64(2), true, "Mudar@123"},
		{"vendedor do 03 com o próprio placeholder como senha", "03_seed_vendedores.sql", "carla.carvalho@rotaperfumes.com.br", "normal", int64(2), true, "$2a$12$XXXXPLACEHOLDER_MUDAR123_SERA_SUBSTITUIDO_PELO_GOXXXX"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			hash := placeholderDoSeed(t, c.arquivo)
			server, db, mock, _ := setupTestServerWithAuthHandler(t)
			defer server.Close()
			defer db.Close()

			mock.ExpectQuery(`FROM\s+usuarios\s+u\s+LEFT\s+JOIN\s+vendedores\s+v\s+ON\s+v\.id\s+=\s+u\.id_vendedor\s+WHERE\s+u\.email\s+=\s+\?\s+LIMIT\s+1`).
				WithArgs(c.email).
				WillReturnRows(sqlmock.NewRows([]string{"id", "nome", "email", "password_hash", "role", "id_vendedor", "ativo", "deve_trocar_senha", "created_at", "updated_at", "ultimo_login_at", "vendedor_nome"}).
					AddRow(int64(7), "Seed", c.email, hash, c.role, c.idVendedor, c.ativo, true, time.Now(), time.Now(), nil, nil))

			resp, err := http.Post(server.URL+"/api/auth/login", "application/json",
				makeJSON(map[string]string{"email": c.email, "password": c.senha, "captchaToken": "token-valido-de-teste"}))
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "placeholder = credenciais inválidas, nunca 500")
			body := decodeResponse(t, readBody(t, resp))
			assert.False(t, como[bool](t, body["success"]))
			assert.Equal(t, "credenciais inválidas", body["error"])
			for _, ck := range resp.Cookies() {
				assert.Empty(t, ck.Value, "nenhum cookie de sessão emitido (%s)", ck.Name)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
