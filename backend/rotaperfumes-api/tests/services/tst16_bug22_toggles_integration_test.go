package services_test

// TST-16 (Lote 36): integração (INTEGRATION=1 + DB_USUARIO/DB_SENHA; senão
// t.Skip; SEC-50: integracao.Abrir recusa a produção antes de conectar) do
// BUG-22 contra um MariaDB real: toggles SIMULTÂNEOS do PATCH /inativar de
// cliente (admin e vendedor na carteira, com o lock do cliente e a carteira
// conferida sob o lock) e de usuário (UPDATE atômico + revogação e corte de
// sessão na mesma tx) se compõem: número par de toggles volta ao valor
// original, ímpar inverte (com ler-inverter-gravar, dois toggles podiam se
// anular). Nenhum toggle pode falhar (deadlock/lock wait viraria erro).
//
// Dados: vendedor/cliente "ZZ-TEST-TST16-*" e usuário
// "zz-test-tst16-<n>@teste.local" (com um refresh token), apagados no
// t.Cleanup.

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
)

// casosToggleTST16 são as quantidades de toggles simultâneos e o valor de
// partida de cada rodada.
var casosToggleTST16 = []struct {
	toggles  int
	original bool
}{{2, true}, {2, false}, {8, true}, {7, true}, {7, false}}

// emParalelo dispara n chamadas de f ao mesmo tempo (barreira) e devolve os
// erros.
func emParalelo(n int, f func() error) []error {
	inicio := make(chan struct{})
	var wg sync.WaitGroup
	erros := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-inicio
			erros[i] = f()
		}(i)
	}
	close(inicio)
	wg.Wait()
	return erros
}

func esperadoTST16(original bool, toggles int) bool {
	if toggles%2 == 1 {
		return !original
	}
	return original
}

func clienteTST16(t *testing.T, ctx context.Context, db *sql.DB, vendedorID int64, ativo bool) int64 {
	t.Helper()
	d := comercial.DadosCliente{
		CNPJ: cnpjValidoTeste(time.Now().UnixNano()), RazaoSocial: fmt.Sprintf("ZZ-TEST-TST16-%d", time.Now().UnixNano()),
		Segmento: "Teste", Cidade: "Curitiba", UF: "PR",
	}
	p := principalAdmin()
	if vendedorID > 0 {
		p = principalRestrito()
	}
	c, err := casosComercial(db, vendedorID).CriarCliente(ctx, p, fixo(d))
	require.NoError(t, err)
	id := c.ClienteIDOrigem
	t.Cleanup(func() {
		// carteiras tem ON DELETE CASCADE a partir de clientes.
		_, err := db.Exec("DELETE FROM clientes WHERE cliente_id_origem = ? AND razao_social LIKE 'ZZ-TEST-TST16-%'", id)
		assert.NoError(t, err)
	})
	_, err = db.Exec("UPDATE clientes SET ativo = ? WHERE cliente_id_origem = ?", ativo, id)
	require.NoError(t, err)
	return id
}

func ativoClienteTST16(t *testing.T, db *sql.DB, id int64) bool {
	t.Helper()
	var ativo bool
	require.NoError(t, db.QueryRow("SELECT ativo FROM clientes WHERE cliente_id_origem = ?", id).Scan(&ativo))
	return ativo
}

func TestIntegracaoTST16_BUG22_TogglesConcorrentesCliente(t *testing.T) {
	db := abrirDBIntegracao(t)
	ctx := context.Background()
	vendID := criarVendedorTemp(t, ctx, db, "TST16")
	for _, escopo := range []struct {
		nome       string
		vendedorID int64 // 0 = admin (sem carteira); >0 = vendedor na carteira
	}{{"admin", 0}, {"vendedor na carteira", vendID}} {
		for _, c := range casosToggleTST16 {
			t.Run(fmt.Sprintf("%s: %d toggles a partir de ativo=%t", escopo.nome, c.toggles, c.original), func(t *testing.T) {
				id := clienteTST16(t, ctx, db, escopo.vendedorID, c.original)
				casos := casosComercial(db, escopo.vendedorID)
				p := principalAdmin()
				if escopo.vendedorID > 0 {
					p = principalRestrito()
				}
				for _, err := range emParalelo(c.toggles, func() error {
					_, err := casos.AlternarAtivoCliente(ctx, p, id, fixo[*bool](nil))
					return err
				}) {
					require.NoError(t, err)
				}
				assert.Equal(t, esperadoTST16(c.original, c.toggles), ativoClienteTST16(t, db, id), "cada toggle conta exatamente uma vez")
			})
		}
	}
}

// Valor explícito concorrente (ativo=false em paralelo): idempotente, sem
// erro, termina inativo.
func TestIntegracaoTST16_BUG22_DefinirInativoConcorrenteCliente(t *testing.T) {
	db := abrirDBIntegracao(t)
	ctx := context.Background()
	id := clienteTST16(t, ctx, db, 0, true)
	falso := false
	casos := casosComercial(db, 0)
	for _, err := range emParalelo(6, func() error {
		_, err := casos.AlternarAtivoCliente(ctx, principalAdmin(), id, fixo(&falso))
		return err
	}) {
		require.NoError(t, err)
	}
	assert.False(t, ativoClienteTST16(t, db, id))
}

func usuarioTST16(t *testing.T, db *sql.DB, ativo bool) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO usuarios (nome, email, password_hash, role, ativo) VALUES ('ZZ-TEST-TST16', ?, 'x', 'normal', ?)`,
		fmt.Sprintf("zz-test-tst16-%d@teste.local", time.Now().UnixNano()), ativo)
	require.NoError(t, err)
	uid, err := res.LastInsertId()
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.Exec(`DELETE FROM refresh_tokens WHERE usuario_id = ?`, uid)
		assert.NoError(t, err)
		_, err = db.Exec(`DELETE FROM usuarios WHERE id = ? AND email LIKE 'zz-test-tst16-%'`, uid)
		assert.NoError(t, err)
	})
	g := identidadecrypto.GeradorRefresh{}
	require.NoError(t, identidademysql.NovoRefreshTokens(db).Inserir(context.Background(),
		identidade.NovoRefreshToken(uid, g.Hash(g.Novo()), time.Now(), "127.0.0.1", "go-test-tst16")))
	return uid
}

func TestIntegracaoTST16_BUG22_TogglesConcorrentesUsuario(t *testing.T) {
	db := abrirDBIntegracao(t)
	ctx := context.Background()
	casos := novosCasos(db, nil, nil, time.Now, identidadeapp.Opcoes{})
	for _, c := range casosToggleTST16 {
		t.Run(fmt.Sprintf("%d toggles a partir de ativo=%t", c.toggles, c.original), func(t *testing.T) {
			uid := usuarioTST16(t, db, c.original)
			for _, err := range emParalelo(c.toggles, func() error {
				_, err := casos.AlternarAtivoUsuario(ctx, adminTeste(), uid, nil)
				return err
			}) {
				require.NoError(t, err)
			}
			want := esperadoTST16(c.original, c.toggles)
			var ativo bool
			var corte sql.NullTime
			require.NoError(t, db.QueryRow(`SELECT ativo, tokens_validos_desde FROM usuarios WHERE id = ?`, uid).Scan(&ativo, &corte))
			assert.Equal(t, want, ativo, "cada toggle conta exatamente uma vez")

			var abertos, inativacao int
			require.NoError(t, db.QueryRow(`SELECT COALESCE(SUM(revoked_at IS NULL), 0), COALESCE(SUM(revoked_reason = 'inativacao'), 0)
				FROM refresh_tokens WHERE usuario_id = ?`, uid).Scan(&abertos, &inativacao))
			// Com 2 ou mais toggles, algum deles inativou o usuário (I-09/
			// SEC-08): o token sai revogado com o motivo "inativacao" e o
			// corte de sessão fica gravado, mesmo que ele termine ativo.
			require.GreaterOrEqual(t, c.toggles, 2)
			assert.Zero(t, abertos, "refresh token revogado na inativação")
			assert.Equal(t, 1, inativacao)
			assert.True(t, corte.Valid, "corte de sessão gravado")
		})
	}
}
