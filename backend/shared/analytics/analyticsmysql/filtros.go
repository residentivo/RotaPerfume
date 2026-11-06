package analyticsmysql

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/kernel"
)

// Filtros de vendedor do legado (vendedorFilter), um por coluna: sempre
// constantes; o vendedor vai só por placeholder.
const (
	filtroPedidoDoVendedor = " AND vendedor_id = ?" // pedidos.vendedor_id
	filtroVendedorPorID    = " AND id = ?"          // vendedores.id
	filtroVendedorAlias    = " AND v.id = ?"        // vendedores v
	// filtroOportunidadeDoVendedor é o dono da oportunidade (DASH-01), o
	// mesmo de /api/oportunidades; não a carteira.
	filtroOportunidadeDoVendedor = " AND vendedor_id = ?" // oportunidades.vendedor_id
)

// filtroVendedor traduz o escopo (O1, só por Filtro()): Irrestrito → sem
// filtro nem argumento; Vendedor → a condição constante e o id; SemAcesso
// ou zero value → o erro do Filtro, e o chamador NÃO executa SQL.
func filtroVendedor(e kernel.Escopo, condicao string) (string, []any, error) {
	vendedorID, filtra, err := e.Filtro()
	if err != nil || !filtra {
		return "", nil, err
	}
	return condicao, []any{vendedorID}, nil
}

// clienteNaCarteira restringe clientes à carteira ATIVA (data_fim nula) do
// vendedor do escopo.
const clienteNaCarteira = "cliente_id_origem IN (SELECT cliente_id FROM carteiras WHERE vendedor_id = ? AND data_fim IS NULL)"

// whereCarteira monta " WHERE ..." (ou "") com as condições constantes e,
// no escopo de vendedor, a carteira ativa (legado carteiraWhere). Os args
// das condições vêm antes do vendedor. SemAcesso/zero value → erro sem SQL.
func whereCarteira(e kernel.Escopo, conds ...string) (string, []any, error) {
	vendedorID, filtra, err := e.Filtro()
	if err != nil {
		return "", nil, err
	}
	var args []any
	if filtra {
		conds = append(conds, clienteNaCarteira)
		args = append(args, vendedorID)
	}
	if len(conds) == 0 {
		return "", nil, nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args, nil
}

// Cláusulas de período do legado (periodoWhereClause e o switch de
// CountNovosNoPeriodo). As datas vêm de argsPeriodo (BUG-30: calculadas no
// Go, sem o relógio do servidor), na ordem dos placeholders.
const (
	periodoPedidoHoje     = "DATE(data_pedido) = ?"
	periodoPedidoSemana   = "data_pedido >= ?"
	periodoPedidoMes      = "YEAR(data_pedido) = ? AND MONTH(data_pedido) = ?"
	periodoCadastroHoje   = "data_cadastro = ?"
	periodoCadastroSemana = "data_cadastro >= ?"
	periodoCadastroMes    = "YEAR(data_cadastro) = ? AND MONTH(data_cadastro) = ?"
)

// clausulaPeriodoPedido traduz o enum em constante (D-02); fora do enum, o
// mês (o default do legado).
func clausulaPeriodoPedido(p analytics.Periodo) string {
	switch p {
	case analytics.PeriodoHoje:
		return periodoPedidoHoje
	case analytics.PeriodoSemana:
		return periodoPedidoSemana
	default:
		return periodoPedidoMes
	}
}

// clausulaPeriodoCadastro é clausulaPeriodoPedido sobre clientes.data_cadastro.
func clausulaPeriodoCadastro(p analytics.Periodo) string {
	switch p {
	case analytics.PeriodoHoje:
		return periodoCadastroHoje
	case analytics.PeriodoSemana:
		return periodoCadastroSemana
	default:
		return periodoCadastroMes
	}
}

// Cláusulas da janela do funil (DASH-01) sobre oportunidades.data_abertura,
// incluindo hoje (29 dias atrás + hoje = 30 dias). "all" não tem cláusula.
// A data vem de argsJanela (BUG-30).
// As três janelas têm o mesmo texto; muda só a data (argsJanela).
const janelaAberturaDesde = " AND data_abertura >= ?"

// clausulaJanelaAbertura traduz o enum em constante (D-02); fora do enum,
// sem cláusula (o default "all").
func clausulaJanelaAbertura(j analytics.Janela) string {
	switch j {
	case analytics.Janela30d, analytics.Janela90d, analytics.Janela365d:
		return janelaAberturaDesde
	default:
		return ""
	}
}

// codigoTabelaInexistente é o ER_NO_SUCH_TABLE do MariaDB/MySQL.
const codigoTabelaInexistente = 1146

// tabelaInexistente reconhece "tabela não existe" SÓ pelo código 1146 do
// driver (BUG-18): as consultas de pedidos degradam para zero nesse caso
// (instalação sem a tabela). A comparação por texto do legado ("not found"
// etc.) mascarava outros erros e saiu.
func tabelaInexistente(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == codigoTabelaInexistente
}

// marcadores devolve "?,?,…" com n placeholders (a lista do IN).
func marcadores(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
