package handlers_test

// Lote 36 (ressalva BAIXA do SecBrain): o novoItCtx apaga, antes de criar,
// os restos ZZ-TEST-HTTP-% / zz-test-http-%@teste.local de execuções que
// morreram antes do t.Cleanup (pânico do -timeout, SIGKILL), com os
// dependentes. Só roda com INTEGRATION=1 (integracao.Abrir; recusa a produção).

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/testkit/integracao"
)

// restoMorto simula o que uma execução morta deixaria no banco.
type restoMorto struct {
	usuarioID, vendedorID int64
}

func plantarRestoMorto(t *testing.T, db *sql.DB, suf string, comVendedor bool) restoMorto {
	t.Helper()
	var r restoMorto
	var vend any
	if comVendedor {
		res, err := db.Exec(`INSERT INTO vendedores (nome, regiao, uf, data_admissao, meta_mensal)
			VALUES (?, 'Teste', 'PR', '2024-01-01', 1)`, "ZZ-TEST-HTTP-"+suf+"-VEND-1")
		require.NoError(t, err)
		r.vendedorID, _ = res.LastInsertId()
		vend = r.vendedorID
	}
	// O admin perigoso: ativo, sem troca de senha pendente.
	res, err := db.Exec(`INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha)
		VALUES (?, ?, 'x-hash-de-teste-nao-usado', 'admin', ?, 1, 0)`,
		"ZZ-TEST-HTTP-"+suf+"-ADM-1", "zz-test-http-"+suf+"-adm-1@teste.local", vend)
	require.NoError(t, err)
	r.usuarioID, _ = res.LastInsertId()
	_, err = db.Exec(`INSERT INTO refresh_tokens (usuario_id, token_hash, expires_at) VALUES (?, ?, ?)`,
		r.usuarioID, fmt.Sprintf("l36-hash-%s-%d", suf, r.usuarioID), time.Now().Add(time.Hour))
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO senha_historico (usuario_id, senha_hash_anterior, tipo_reset) VALUES (?, 'x', 'admin')`,
		r.usuarioID)
	require.NoError(t, err)
	return r
}

func contar(t *testing.T, db *sql.DB, q string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(q, args...).Scan(&n))
	return n
}

func TestLote36_NovoItCtx_PurgaRestosDeExecucaoMorta(t *testing.T) {
	casos := []struct {
		nome        string
		comVendedor bool
	}{
		{"admin ativo sem dependentes de vendedor", false},
		{"admin ativo vinculado a vendedor do marcador", true},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			db := integracao.Abrir(t)
			suf := fmt.Sprintf("morto%d", time.Now().UnixNano()%1e9)
			r := plantarRestoMorto(t, db, suf, tc.comVendedor)

			// Controle fora do marcador: não pode ser tocado.
			emailCtl := fmt.Sprintf("l36-controle-%s@teste.local", suf)
			res, err := db.Exec(`INSERT INTO usuarios (nome, email, password_hash, role, ativo)
				VALUES ('L36 controle', ?, 'x', 'normal', 0)`, emailCtl)
			require.NoError(t, err)
			ctlID, _ := res.LastInsertId()
			t.Cleanup(func() { _, _ = db.Exec(`DELETE FROM usuarios WHERE id = ?`, ctlID) })

			c := novoItCtx(t)

			assert.Zero(t, contar(t, db, `SELECT COUNT(*) FROM usuarios WHERE id = ?`, r.usuarioID), "admin morto apagado")
			assert.Zero(t, contar(t, db, `SELECT COUNT(*) FROM refresh_tokens WHERE usuario_id = ?`, r.usuarioID))
			assert.Zero(t, contar(t, db, `SELECT COUNT(*) FROM senha_historico WHERE usuario_id = ?`, r.usuarioID))
			if tc.comVendedor {
				assert.Zero(t, contar(t, db, `SELECT COUNT(*) FROM vendedores WHERE id = ?`, r.vendedorID))
			}
			assert.Zero(t, contar(t, db, `SELECT COUNT(*) FROM usuarios WHERE email LIKE ? AND email NOT LIKE ?`,
				marcadorEmailHTTP, c.email+"%"), "só o admin desta execução tem o marcador")
			assert.Equal(t, 1, contar(t, db, `SELECT COUNT(*) FROM usuarios WHERE id = ?`, ctlID), "controle intacto")
			assert.Equal(t, 1, contar(t, db, `SELECT COUNT(*) FROM usuarios WHERE email LIKE ?`, c.email+"%"),
				"admin da execução criado depois da purga")
		})
	}
}
