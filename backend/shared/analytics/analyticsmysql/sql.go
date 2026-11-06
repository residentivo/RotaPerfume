// Package analyticsmysql tem os adapters MariaDB (protocolo MySQL) das consultas do Dashboard
// (ARQ-06): o SQL agregador do legado (dashboard_repository.go e as
// métricas de cliente do cliente_repository.go), copiado BYTE A BYTE.
//
// Regras de segurança (docs/arq-00-invariantes-seguranca.md, D-01..D-03):
//   - todo método exportado recebe kernel.Escopo e o traduz SÓ por
//     Escopo.Filtro() (O1): SemAcesso e zero value devolvem o erro do
//     Filtro SEM executar SQL; não existe "0 = global";
//   - fmt.Sprintf monta SQL só com constantes: o formato é constante e cada
//     argumento é o resultado de uma função deste pacote que faz switch
//     sobre o enum analytics.Periodo e devolve constantes (D-02,
//     TestARQ_SprintfSQLSoConstantes);
//   - todo valor (vendedor, ids, datas, limite, página) vai por placeholder;
//   - nenhum SQL usa o relógio do servidor (BUG-30/TST-23): as datas são
//     calculadas no Go (datas.go) no fuso fixo do pacote tz.
package analyticsmysql

// SQL do dashboard_repository.go legado; o BUG-30 trocou só as datas do
// servidor (CURDATE()) por placeholders e tirou a sequência de dias. As
// partes "Inicio"/"Fim" recebem entre si o filtro de vendedor (constante ou
// vazio, decidido pelo Filtro()); os "fmt*" recebem a cláusula de período.
const (
	fmtVendasTotais = `
		SELECT COALESCE(SUM(valor_total), 0), COUNT(*)
		FROM pedidos
		WHERE %s AND status NOT IN ('cancelado', 'devolvido')`
	fmtTotalPedidos = `
		SELECT COUNT(*) FROM pedidos
		WHERE %s AND status NOT IN ('cancelado', 'devolvido')`
	sqlMetaMensalTotal = `
		SELECT COALESCE(SUM(meta_mensal), 0)
		FROM vendedores
		WHERE data_desligamento IS NULL`
	sqlTopVendedoresInicio = `
		SELECT
			v.id,
			v.nome,
			v.meta_mensal AS meta
		FROM vendedores v
		WHERE v.data_desligamento IS NULL`
	sqlTopVendedoresFim = `
		ORDER BY v.meta_mensal DESC
		LIMIT ?`
	sqlMetasVendedoresInicio = `
		SELECT
			v.id,
			v.nome,
			v.regiao,
			v.uf,
			v.meta_mensal AS meta
		FROM vendedores v
		WHERE v.data_desligamento IS NULL`
	sqlMetasVendedoresFim = `
		ORDER BY v.meta_mensal DESC`
	sqlVendasDoMesInicio = `
		SELECT vendedor_id,
			   COALESCE(SUM(valor_total), 0),
			   COUNT(*)
		FROM pedidos
		WHERE vendedor_id IN (`
	sqlVendasDoMesFim = `)
		  AND YEAR(data_pedido) = ?
		  AND MONTH(data_pedido) = ?
		  AND status NOT IN ('cancelado', 'devolvido')
		GROUP BY vendedor_id`
	sqlSerieVendasInicio = `
		SELECT DATE_FORMAT(data_pedido, '%Y-%m-%d') AS data,
			   COALESCE(SUM(valor_total), 0) AS valor,
			   COUNT(*) AS quantidade
		FROM pedidos
		WHERE data_pedido >= ?
		  AND status NOT IN ('cancelado', 'devolvido')`
	// DB-04/DB-06: GROUP BY pelo alias do SELECT (pedidos não tem coluna
	// "data"): vale com e sem ONLY_FULL_GROUP_BY no MariaDB.
	sqlSerieVendasFim = `
		GROUP BY data
		ORDER BY data ASC`
	sqlRankingTotal  = `SELECT COUNT(*) FROM vendedores WHERE data_desligamento IS NULL`
	sqlRankingInicio = `
		SELECT
			v.id, v.nome, v.regiao, v.uf, v.meta_mensal,
			COALESCE(p.total_vendas, 0) AS total_vendas,
			COALESCE(p.total_pedidos, 0) AS total_pedidos,
			CASE WHEN v.meta_mensal > 0 THEN (COALESCE(p.total_vendas, 0) / v.meta_mensal) * 100 ELSE 0 END AS atingimento_meta
		FROM vendedores v
		LEFT JOIN (
			SELECT vendedor_id, SUM(valor_total) AS total_vendas, COUNT(*) AS total_pedidos
			FROM pedidos
			WHERE YEAR(data_pedido) = ? AND MONTH(data_pedido) = ?
			  AND status NOT IN ('cancelado', 'devolvido')
			GROUP BY vendedor_id
		) p ON p.vendedor_id = v.id
		WHERE v.data_desligamento IS NULL`
	sqlRankingFim = `
		ORDER BY v.meta_mensal DESC, atingimento_meta DESC
		LIMIT ? OFFSET ?`
)
