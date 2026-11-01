package catalogo_test

// Integração (INTEGRATION=1 + DB_USUARIO/DB_SENHA; senão t.Skip) do BUG-17
// e do BUG-21 contra o MariaDB do .env.
//
//   - BUG-17: toggles simultâneos de ativo (UPDATE … SET ativo = NOT ativo)
//     se compõem: um número par de toggles volta ao valor original, um
//     ímpar inverte (com ler-inverter-gravar, dois toggles podiam se anular).
//   - BUG-21: o upsert da baixa por faturamento calcula a ruptura sobre o
//     saldo RESULTANTE (sem aplicar a baixa duas vezes): saldo 10, baixa 6 →
//     saldo 4, ruptura 0; saldo 5, baixa 6 → saldo -1, ruptura 1.
//
// Dados: produto(s) com SKU "ZZ-TEST-L25-<n>" e os registros de estoque
// deles, apagados no t.Cleanup.

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
	"github.com/rotaperfumes/shared/catalogo/catalogomysql"
	"github.com/rotaperfumes/shared/config"
	shareddb "github.com/rotaperfumes/shared/db"
)

func lote25EnvOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func abrirDBLote25(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("INTEGRATION") != "1" {
		t.Skip("teste de integração: defina INTEGRATION=1 (requer MariaDB local)")
	}
	usuario, senha := os.Getenv("DB_USUARIO"), os.Getenv("DB_SENHA")
	if usuario == "" || senha == "" {
		t.Skip("teste de integração: defina DB_USUARIO/DB_SENHA (SEC-11: sem default)")
	}
	cfg := &config.Config{
		DBHost: lote25EnvOr("DB_HOST", "localhost"), DBPort: lote25EnvOr("DB_PORT", "3306"),
		DBName: lote25EnvOr("DB_NAME", "rotaperfumes"), DBUsuario: usuario, DBSenha: senha,
	}
	db, err := shareddb.Open(cfg.DSN())
	require.NoError(t, err, "MariaDB indisponível")
	t.Cleanup(func() { db.Close() })
	return db
}

// produtoLote25 cria um produto de teste e agenda a limpeza dele e do estoque.
func produtoLote25(t *testing.T, db *sql.DB, ativo bool) (int64, string) {
	t.Helper()
	sku := fmt.Sprintf("ZZ-TEST-L25-%d", time.Now().UnixNano())
	res, err := db.Exec(`INSERT INTO produtos (sku, descricao, categoria, marca, preco_tabela, custo_unitario, unidade, ativo)
		VALUES (?, 'ZZ-TEST Lote 25', 'Teste', 'Teste', 1, 1, 'UN', ?)`, sku, ativo)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.Exec(`DELETE FROM estoque WHERE sku = ?`, sku)
		assert.NoError(t, err)
		_, err = db.Exec(`DELETE FROM produtos WHERE id = ?`, id)
		assert.NoError(t, err)
	})
	return id, sku
}

func ativoLote25(t *testing.T, db *sql.DB, id int64) bool {
	t.Helper()
	var ativo bool
	require.NoError(t, db.QueryRow(`SELECT ativo FROM produtos WHERE id = ?`, id).Scan(&ativo))
	return ativo
}

func TestIntegracaoBUG17_TogglesConcorrentes(t *testing.T) {
	db := abrirDBLote25(t)
	casos := catalogoapp.Novo(catalogomysql.NovoProdutos(db), catalogomysql.NovoEstoques(db))
	for _, c := range []struct {
		toggles  int
		original bool
	}{{2, true}, {2, false}, {10, true}, {7, true}, {7, false}} {
		t.Run(fmt.Sprintf("%d toggles a partir de ativo=%t", c.toggles, c.original), func(t *testing.T) {
			id, _ := produtoLote25(t, db, c.original)
			inicio := make(chan struct{})
			var wg sync.WaitGroup
			erros := make(chan error, c.toggles)
			for i := 0; i < c.toggles; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-inicio
					_, err := casos.AlternarAtivoProduto(context.Background(), id, nil)
					erros <- err
				}()
			}
			close(inicio)
			wg.Wait()
			close(erros)
			for err := range erros {
				require.NoError(t, err)
			}
			want := c.original
			if c.toggles%2 == 1 {
				want = !c.original
			}
			assert.Equal(t, want, ativoLote25(t, db, id), "cada toggle conta exatamente uma vez")
		})
	}
}

func TestIntegracaoBUG17_ToggleInexistente_404(t *testing.T) {
	db := abrirDBLote25(t)
	err := catalogomysql.NovoProdutos(db).AlternarAtivo(context.Background(), -1)
	assert.ErrorIs(t, err, catalogo.ErrProdutoNaoEncontrado, "0 linhas (clientFoundRows=true) → 404")
}

func TestIntegracaoBUG21_UpsertDaBaixa_RupturaSobreSaldoResultante(t *testing.T) {
	db := abrirDBLote25(t)
	dia := time.Date(2001, 2, 3, 12, 0, 0, 0, time.Local) // data isolada, só deste teste
	casos := []struct {
		nome               string
		saldoInicial       *int // nil = sem registro do dia (ramo INSERT)
		baixa              int
		saldoFinal         int
		rupturaFinal       bool
		rupturaInicialFlag bool
	}{
		{"saldo 10, baixa 6 → 4 sem ruptura", ptrInt(10), 6, 4, false, false},
		{"saldo 5, baixa 6 → -1 com ruptura", ptrInt(5), 6, -1, true, false},
		{"saldo 6, baixa 6 → 0 com ruptura", ptrInt(6), 6, 0, true, false},
		{"saldo 7, baixa 6 → 1 sem ruptura (limpa ruptura antiga)", ptrInt(7), 6, 1, false, true},
		{"sem registro do dia, baixa 6 → -6 com ruptura", nil, 6, -6, true, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			_, sku := produtoLote25(t, db, true)
			if c.saldoInicial != nil {
				_, err := db.Exec(`INSERT INTO estoque (data_snapshot, sku, saldo, ruptura) VALUES (?, ?, ?, ?)`,
					dia.Format("2006-01-02"), sku, *c.saldoInicial, c.rupturaInicialFlag)
				require.NoError(t, err)
			}
			tx, err := db.Begin()
			require.NoError(t, err)
			err = catalogomysql.NovoEstoques(tx).AjustarSaldoPorFaturamento(context.Background(), sku, dia, c.baixa)
			if err != nil {
				_ = tx.Rollback()
				require.NoError(t, err)
			}
			require.NoError(t, tx.Commit())

			var saldo int
			var ruptura bool
			require.NoError(t, db.QueryRow(`SELECT saldo, ruptura FROM estoque WHERE data_snapshot = ? AND sku = ?`,
				dia.Format("2006-01-02"), sku).Scan(&saldo, &ruptura))
			assert.Equal(t, c.saldoFinal, saldo, "a baixa é aplicada uma única vez")
			assert.Equal(t, c.rupturaFinal, ruptura, "ruptura = saldo resultante <= 0")
		})
	}
}

func ptrInt(v int) *int { return &v }
