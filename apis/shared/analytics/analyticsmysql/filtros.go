package analyticsmysql

import (
	"strings"

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
// CountNovosNoPeriodo).
const (
	periodoPedidoHoje     = "DATE(data_pedido) = CURDATE()"
	periodoPedidoSemana   = "data_pedido >= DATE_SUB(CURDATE(), INTERVAL 6 DAY)"
	periodoPedidoMes      = "YEAR(data_pedido) = YEAR(CURDATE()) AND MONTH(data_pedido) = MONTH(CURDATE())"
	periodoCadastroHoje   = "data_cadastro = CURDATE()"
	periodoCadastroSemana = "data_cadastro >= DATE_SUB(CURDATE(), INTERVAL 6 DAY)"
	periodoCadastroMes    = "YEAR(data_cadastro) = YEAR(CURDATE()) AND MONTH(data_cadastro) = MONTH(CURDATE())"
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
const (
	janelaAbertura30d  = " AND data_abertura >= DATE_SUB(CURDATE(), INTERVAL 29 DAY)"
	janelaAbertura90d  = " AND data_abertura >= DATE_SUB(CURDATE(), INTERVAL 89 DAY)"
	janelaAbertura365d = " AND data_abertura >= DATE_SUB(CURDATE(), INTERVAL 364 DAY)"
)

// clausulaJanelaAbertura traduz o enum em constante (D-02); fora do enum,
// sem cláusula (o default "all").
func clausulaJanelaAbertura(j analytics.Janela) string {
	switch j {
	case analytics.Janela30d:
		return janelaAbertura30d
	case analytics.Janela90d:
		return janelaAbertura90d
	case analytics.Janela365d:
		return janelaAbertura365d
	default:
		return ""
	}
}

// tabelaInexistente reconhece "tabela não existe" pelo texto do erro, como
// o isTableNotFound legado: as consultas de pedidos degradam para zero
// nesse caso (instalação sem a tabela). É a única comparação de texto de
// erro do pacote, mantida para não mudar o comportamento.
func tabelaInexistente(err error) bool {
	msg := err.Error()
	for _, trecho := range []string{"doesn't exist", "not found", "Error 1146", "no such table"} {
		if strings.Contains(msg, trecho) {
			return true
		}
	}
	return false
}

// marcadores devolve "?,?,…" com n placeholders (a lista do IN).
func marcadores(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
