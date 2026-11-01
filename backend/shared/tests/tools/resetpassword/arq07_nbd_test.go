package resetpassword_test

// ARQ-07 NB-D: a listagem não imprime parte do hash (só o prefixo do
// algoritmo) e o UPSERT por e-mail loga o e-mail mascarado.

import (
	"bytes"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/tools/resetpassword"
)

func TestARQ07_NBD_ListUsersSoPrefixoDoAlgoritmo(t *testing.T) {
	db, mock := novoMock(t)
	mock.ExpectQuery(regexp.QuoteMeta("LEFT(SUBSTRING_INDEX(password_hash, '$', 2), 12) AS algoritmo")).
		WillReturnRows(sqlmock.NewRows(colunasLista).
			AddRow(1, "Admin", resetpassword.AdminEmail, "admin", 1, "$argon2id", "OK").
			AddRow(2, "Lixo", "lixo@x.com", "normal", 1, "senhaEmClaro", "UNKNOWN"))
	var out bytes.Buffer
	require.NoError(t, resetpassword.ListUsers(ctx, db, &out))
	s := out.String()
	assert.Contains(t, s, "ALGORITMO")
	assert.NotContains(t, s, "HASH_PREVIEW")
	assert.Contains(t, s, "$argon2id")
	assert.NotContains(t, s, "senhaEmClaro", "valor sem forma de prefixo não é impresso")
	assert.NotContains(t, s, "LEFT(password_hash")
}

func TestARQ07_NBD_AlgoritmoDoHash(t *testing.T) {
	for entrada, esperado := range map[string]string{
		"$argon2id": "$argon2id", "$2a": "$2a", "$2y": "$2y",
		"": "?", "hash": "?", "$argon2id$v=19": "?", "$ABC": "?", "$senha-muito-longa": "?",
	} {
		assert.Equal(t, esperado, resetpassword.AlgoritmoDoHash(entrada), "%q", entrada)
	}
}

func TestARQ07_NBD_UpsertLogaEmailMascarado(t *testing.T) {
	const email = "joana.souza@x.com"
	t.Run("UPDATE", func(t *testing.T) {
		logs := silenciarLog(t)
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateEmail).WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, resetpassword.UpsertByEmail(ctx, db, hsTeste, email, "Nova@123", "normal", "J", 0))
		assert.Contains(t, logs.String(), "resetpassword: j***@x.com atualizado (UPDATE)")
		assert.NotContains(t, logs.String(), email)
	})
	t.Run("INSERT", func(t *testing.T) {
		logs := silenciarLog(t)
		db, mock := novoMock(t)
		mock.ExpectExec(reUpdateEmail).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(reInsert).WillReturnResult(sqlmock.NewResult(1, 1))
		require.NoError(t, resetpassword.UpsertByEmail(ctx, db, hsTeste, email, "Nova@123", "normal", "J", 0))
		assert.Contains(t, logs.String(), "resetpassword: j***@x.com criado (INSERT)")
		assert.NotContains(t, logs.String(), email)
	})
}
