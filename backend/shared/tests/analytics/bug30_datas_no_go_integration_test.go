package analytics_test

// BUG-30 (Lote 38): integração (INTEGRATION=1 + DB_USUARIO/DB_SENHA; senão
// t.Skip; SEC-50: integracao.Abrir recusa a produção antes de conectar).
// Com o relógio PARADO às 22:30 -03 do dia D (01:30 UTC de D+1), um pedido
// e um cliente de D entram em "hoje" e o pedido de D+1 não, com o servidor
// em UTC ou em -03 e em qualquer hora real do teste: o SQL não usa o
// relógio do servidor.
//
// Dados: vendedor/cliente "ZZ-TEST-BUG30-*", apagados no t.Cleanup.

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsmysql"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/testkit/integracao"
	"github.com/rotaperfumes/shared/tz"
)

const (
	diaBUG30Integracao       = "2026-03-10" // D
	diaSeguinteBUG30         = "2026-03-11" // D+1
	valorPedidoDiaBUG30      = 11.0
	valorPedidoSeguinteBUG30 = 22.0
)

// dadosBUG30 cria vendedor, cliente (cadastrado em D, na carteira ativa) e
// dois pedidos faturados: um em D e outro em D+1.
func dadosBUG30(t *testing.T, db *sql.DB) (vendedorID int64) {
	t.Helper()
	n := time.Now().UnixNano()
	res, err := db.Exec(`INSERT INTO vendedores (nome, regiao, uf, data_admissao, meta_mensal) VALUES (?, 'Teste', 'PR', '2024-01-01', 1000)`,
		fmt.Sprintf("ZZ-TEST-BUG30-%d", n))
	require.NoError(t, err)
	vendedorID, err = res.LastInsertId()
	require.NoError(t, err)
	res, err = db.Exec(`INSERT INTO clientes (cnpj, razao_social, segmento, cidade, uf, data_cadastro) VALUES (?, ?, 'ZZ-TEST-BUG30', 'Curitiba', 'PR', ?)`,
		fmt.Sprintf("%014d", (n+1)%1e14), fmt.Sprintf("ZZ-TEST-BUG30-%d", n), diaBUG30Integracao)
	require.NoError(t, err)
	clienteID, err := res.LastInsertId()
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, q := range []string{
			"DELETE FROM pedidos WHERE vendedor_id = ?",
			"DELETE FROM carteiras WHERE vendedor_id = ?",
		} {
			_, err := db.Exec(q, vendedorID)
			assert.NoError(t, err)
		}
		_, err := db.Exec("DELETE FROM clientes WHERE cliente_id_origem = ? AND razao_social LIKE 'ZZ-TEST-BUG30-%'", clienteID)
		assert.NoError(t, err)
		_, err = db.Exec("DELETE FROM vendedores WHERE id = ? AND nome LIKE 'ZZ-TEST-BUG30-%'", vendedorID)
		assert.NoError(t, err)
	})
	_, err = db.Exec(`INSERT INTO carteiras (cliente_id, vendedor_id, data_inicio) VALUES (?, ?, ?)`, clienteID, vendedorID, diaBUG30Integracao)
	require.NoError(t, err)
	for _, p := range []struct {
		dia   string
		valor float64
	}{{diaBUG30Integracao, valorPedidoDiaBUG30}, {diaSeguinteBUG30, valorPedidoSeguinteBUG30}} {
		_, err = db.Exec(`INSERT INTO pedidos (cliente_id, vendedor_id, data_pedido, canal, status, valor_total) VALUES (?, ?, ?, 'App', 'Faturado', ?)`,
			clienteID, vendedorID, p.dia, p.valor)
		require.NoError(t, err)
	}
	return vendedorID
}

func TestIntegracaoBUG30_HojeDoGoNaViradaDoDia(t *testing.T) {
	db := integracao.Abrir(t)
	ctx := context.Background()
	vendedorID := dadosBUG30(t, db)
	doVendedor, err := kernel.EscopoDoVendedor(vendedorID)
	require.NoError(t, err)

	var fusoSessao string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT @@session.time_zone").Scan(&fusoSessao))
	t.Logf("time_zone da sessão: %s", fusoSessao)

	d0, err := time.ParseInLocation("2006-01-02", diaBUG30Integracao, tz.Local)
	require.NoError(t, err)
	relogio := relogioEm(d0.Add(22*time.Hour + 30*time.Minute).UTC()) // 01:30 UTC de D+1

	d := analyticsmysql.NovoDashboard(db, relogio)
	tot, err := d.VendasTotais(ctx, doVendedor, analytics.PeriodoHoje)
	require.NoError(t, err)
	assert.Equal(t, analytics.TotaisVendas{Valor: valorPedidoDiaBUG30, Quantidade: 1}, tot, "só o pedido de D entra em hoje")

	n, err := d.TotalPedidos(ctx, doVendedor, analytics.PeriodoHoje)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	serie, err := d.SerieVendas(ctx, doVendedor, analytics.DiasDe(2))
	require.NoError(t, err)
	require.Len(t, serie, 2)
	assert.Equal(t, analytics.PontoVenda{Dia: diaBUG30Integracao, TotalVendas: valorPedidoDiaBUG30, TotalPedidos: 1}, serie[1],
		"o último ponto é D; D+1 fica fora da sequência")

	novos, err := analyticsmysql.NovoClientes(db, relogio).NovosNoPeriodo(ctx, doVendedor, analytics.PeriodoHoje)
	require.NoError(t, err)
	assert.Equal(t, 1, novos, "cliente cadastrado em D é novo hoje")
}
