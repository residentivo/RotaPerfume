package db_test

// Integração (INTEGRATION=1 + DB_USUARIO/DB_SENHA; senão t.Skip) do schema
// do banco de dev depois do Lote 12. Só lê o information_schema.
//   - DB-01: estoque sem a coluna origem (17 novo ou migração 18 aplicada).
//   - SEC-12: refresh_tokens.reuso_detectado_em DATETIME NULL DEFAULT NULL,
//     logo depois de revoked_reason (migração 23 / 06 novo).

import (
	"database/sql"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/db"
)

func abrirSchemaIT(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("teste de integração: defina INTEGRATION=1 (requer MySQL local)")
	}
	if os.Getenv("DB_USUARIO") == "" || os.Getenv("DB_SENHA") == "" {
		t.Skip("teste de integração: defina DB_USUARIO/DB_SENHA (SEC-11: sem default)")
	}
	t.Setenv("JWT_SECRET", "integracao")
	cfg, err := config.Load()
	require.NoError(t, err)
	conn, err := db.Open(cfg.DSN())
	require.NoError(t, err, "MySQL local indisponível")
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestIntegracaoDB01_SEC12_Schema(t *testing.T) {
	conn := abrirSchemaIT(t)

	colunas := func(t *testing.T, tabela string) map[string]int {
		t.Helper()
		rows, err := conn.Query(`SELECT COLUMN_NAME, ORDINAL_POSITION FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`, tabela)
		require.NoError(t, err)
		defer rows.Close()
		out := map[string]int{}
		for rows.Next() {
			var nome string
			var pos int
			require.NoError(t, rows.Scan(&nome, &pos))
			out[nome] = pos
		}
		require.NoError(t, rows.Err())
		require.NotEmpty(t, out, "tabela %s ausente", tabela)
		return out
	}

	t.Run("DB-01: estoque sem origem", func(t *testing.T) {
		cols := colunas(t, "estoque")
		assert.NotContains(t, cols, "origem")
		for _, c := range []string{"id", "data_snapshot", "sku", "saldo", "ruptura", "created_at", "updated_at"} {
			assert.Contains(t, cols, c)
		}
	})

	t.Run("SEC-12: refresh_tokens.reuso_detectado_em", func(t *testing.T) {
		cols := colunas(t, "refresh_tokens")
		require.Contains(t, cols, "reuso_detectado_em")
		assert.Equal(t, cols["revoked_reason"]+1, cols["reuso_detectado_em"], "logo depois de revoked_reason")

		var tipo, nulo string
		var padrao sql.NullString
		require.NoError(t, conn.QueryRow(`SELECT DATA_TYPE, IS_NULLABLE, COLUMN_DEFAULT FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refresh_tokens' AND COLUMN_NAME = 'reuso_detectado_em'`).Scan(&tipo, &nulo, &padrao))
		assert.Equal(t, "datetime", tipo)
		assert.Equal(t, "YES", nulo)
		assert.False(t, padrao.Valid, "DEFAULT NULL")
	})
}
