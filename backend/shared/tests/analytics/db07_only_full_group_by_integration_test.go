package analytics_test

// DB-07 (Lote 36): integração (INTEGRATION=1 + DB_USUARIO/DB_SENHA; senão
// t.Skip; SEC-50: integracao.Abrir recusa a produção antes de conectar) das
// consultas com GROUP BY dos adapters analyticsmysql com ONLY_FULL_GROUP_BY
// LIGADO na sessão (o sql_mode do deploy/mariadb-docker.sh desde o DB-07),
// independente do sql_mode global do banco de teste. Um GROUP BY incompleto
// falharia com o erro 1055 (ER_WRONG_FIELD_WITH_GROUP). O enriquecimento
// "vendas do mês" engole erro (legado), então o teste confere o VALOR
// somado de um pedido de teste, não só a ausência de erro.
//
// Dados: vendedor/cliente "ZZ-TEST-DB07-*" e um pedido de hoje, apagados no
// t.Cleanup. BUG-30: o "hoje" é o do Go (fuso tz.Local), o mesmo relógio
// dos três adapters; nenhum SQL usa o relógio do servidor.

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

const valorPedidoDB07 = 123.45

// conexaoFullGroupBy devolve UMA conexão do pool com ONLY_FULL_GROUP_BY na
// sessão (SET SESSION vale só para a conexão; o *sql.Conn a fixa).
func conexaoFullGroupBy(t *testing.T, db *sql.DB) *sql.Conn {
	t.Helper()
	ctx := context.Background()
	c, err := db.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	_, err = c.ExecContext(ctx, "SET SESSION sql_mode = CONCAT_WS(',', NULLIF(@@SESSION.sql_mode, ''), 'ONLY_FULL_GROUP_BY')")
	require.NoError(t, err)
	var modo string
	require.NoError(t, c.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode").Scan(&modo))
	require.Contains(t, strings.Split(modo, ","), "ONLY_FULL_GROUP_BY", modo)
	return c
}

// dadosDB07 cria vendedor, cliente (na carteira ativa do vendedor) e um
// pedido faturado de hoje (dia no formato DATE, calculado no Go).
func dadosDB07(t *testing.T, db *sql.DB, hoje string) (vendedorID int64) {
	t.Helper()
	n := time.Now().UnixNano()
	res, err := db.Exec(`INSERT INTO vendedores (nome, regiao, uf, data_admissao, meta_mensal) VALUES (?, 'Teste', 'PR', '2024-01-01', 1000)`,
		fmt.Sprintf("ZZ-TEST-DB07-%d", n))
	require.NoError(t, err)
	vendedorID, err = res.LastInsertId()
	require.NoError(t, err)
	res, err = db.Exec(`INSERT INTO clientes (cnpj, razao_social, segmento, cidade, uf, data_cadastro) VALUES (?, ?, 'ZZ-TEST-DB07', 'Curitiba', 'PR', ?)`,
		fmt.Sprintf("%014d", n%1e14), fmt.Sprintf("ZZ-TEST-DB07-%d", n), hoje)
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
		_, err := db.Exec("DELETE FROM clientes WHERE cliente_id_origem = ? AND razao_social LIKE 'ZZ-TEST-DB07-%'", clienteID)
		assert.NoError(t, err)
		_, err = db.Exec("DELETE FROM vendedores WHERE id = ? AND nome LIKE 'ZZ-TEST-DB07-%'", vendedorID)
		assert.NoError(t, err)
	})
	_, err = db.Exec(`INSERT INTO carteiras (cliente_id, vendedor_id, data_inicio) VALUES (?, ?, ?)`, clienteID, vendedorID, hoje)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO pedidos (cliente_id, vendedor_id, data_pedido, canal, status, valor_total) VALUES (?, ?, ?, 'App', 'Faturado', ?)`,
		clienteID, vendedorID, hoje, valorPedidoDB07)
	require.NoError(t, err)
	return vendedorID
}

func TestIntegracaoDB07_GroupByComOnlyFullGroupBy(t *testing.T) {
	db := integracao.Abrir(t)
	ctx := context.Background()
	agora := time.Now()
	relogio := func() time.Time { return agora }
	vendedorID := dadosDB07(t, db, agora.In(tz.Local).Format("2006-01-02"))
	c := conexaoFullGroupBy(t, db)
	doVendedor, err := kernel.EscopoDoVendedor(vendedorID)
	require.NoError(t, err)

	for _, e := range []struct {
		nome   string
		escopo kernel.Escopo
	}{{"irrestrito", irrestrito(t)}, {"vendedor", doVendedor}} {
		t.Run(e.nome, func(t *testing.T) {
			_, err := analyticsmysql.NovoOportunidades(c, relogio).Funil(ctx, e.escopo, analytics.JanelaTudo)
			require.NoError(t, err, "funil (GROUP BY etapa)")

			cl := analyticsmysql.NovoClientes(c, relogio)
			seg, err := cl.PorSegmento(ctx, e.escopo)
			require.NoError(t, err, "clientes por segmento")
			assert.Contains(t, seg, analytics.ContagemSegmento{Segmento: "ZZ-TEST-DB07", Total: 1})
			_, err = cl.PorUF(ctx, e.escopo)
			require.NoError(t, err, "clientes por UF")

			d := analyticsmysql.NovoDashboard(c, relogio)
			serie, err := d.SerieVendas(ctx, e.escopo, analytics.DiasDe(7))
			require.NoError(t, err, "série de vendas (GROUP BY data)")
			require.NotEmpty(t, serie)
			assert.GreaterOrEqual(t, serie[len(serie)-1].TotalVendas, valorPedidoDB07, "o pedido de hoje entra no último ponto")
			_, err = d.TopVendedores(ctx, e.escopo, 1000)
			require.NoError(t, err, "top vendedores")

			metas, err := d.MetasVendedores(ctx, e.escopo)
			require.NoError(t, err, "metas (vendas do mês: GROUP BY vendedor_id)")
			var achou bool
			for _, m := range metas {
				if m.ID == vendedorID {
					achou = true
					assert.InDelta(t, valorPedidoDB07, m.Realizado, 0.001, "vendas do mês somadas (a falha seria silenciosa)")
					assert.Equal(t, 1, m.QuantidadeVendas)
				}
			}
			assert.True(t, achou, "vendedor de teste nas metas")

			r, err := d.Ranking(ctx, e.escopo, kernel.NovaPagina(1, 1000))
			require.NoError(t, err, "ranking (subconsulta GROUP BY vendedor_id)")
			achou = false
			for _, l := range r.Itens {
				if l.VendedorID == vendedorID {
					achou = true
					assert.InDelta(t, valorPedidoDB07, l.TotalVendas, 0.001)
					assert.Equal(t, 1, l.TotalPedidos)
				}
			}
			if e.nome == "vendedor" { // irrestrito: a página pode não chegar até ele
				assert.True(t, achou, "vendedor de teste no ranking")
			}
		})
	}
}
