// Package repositories contém funções puras de acesso a dados (stateless).
// Dashboard queries - cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/vlog"
)

// DashboardRepository agrupa queries do dashboard.
type DashboardRepository struct{}

// NewDashboardRepository cria um repositório stateless.
func NewDashboardRepository() *DashboardRepository {
	return &DashboardRepository{}
}

// periodoWhereClause monta a clausula WHERE de filtro de data para data_pedido
// de acordo com o periodo solicitado.
// periodo: "today" = dia atual, "week" = ultimos 7 dias (incluindo hoje),
// qualquer outro valor (inclusive "month") = mes atual.
func periodoWhereClause(periodo string) string {
	vlog.Printf("dashboard_repository.go", "periodoWhereClause", "avaliando switch sobre periodo")
	switch periodo {
	case "today":
		return "DATE(data_pedido) = CURDATE()"
	case "week":
		return "data_pedido >= DATE_SUB(CURDATE(), INTERVAL 6 DAY)"
	default:
		// month: primeiro ao ultimo dia do mes atual.
		return "YEAR(data_pedido) = YEAR(CURDATE()) AND MONTH(data_pedido) = MONTH(CURDATE())"
	}
}

// vendedorFilter devolve o trecho SQL "AND <coluna> = ?" e o argumento
// correspondente quando vendedorID > 0 (usuário normal, escopo restrito).
// Para vendedorID <= 0 (admin, sem filtro) devolve string vazia e nenhum
// argumento. A coluna é sempre uma constante do código (nunca entrada do
// usuário); o valor vai exclusivamente por placeholder.
func vendedorFilter(coluna string, vendedorID int64) (string, []any) {
	vlog.Printf("dashboard_repository.go", "vendedorFilter", "verificando se vendedorID <= 0")
	if vendedorID <= 0 {
		return "", nil
	}
	return " AND " + coluna + " = ?", []any{vendedorID}
}

// GetVendasTotais retorna (valor_total, quantidade) de vendas no periodo.
// Se a tabela pedidos não existir, retorna (0, 0).
// periodo: "today" = dia atual, "week" = ultimos 7 dias, "month" = mes atual.
// vendedorID > 0 restringe aos pedidos do vendedor (pedidos.vendedor_id);
// vendedorID = 0 (admin) considera todos os pedidos.
func (r *DashboardRepository) GetVendasTotais(ctx context.Context, db *sql.DB, periodo string, vendedorID int64) (float64, int, error) {
	// Tenta buscar da tabela pedidos (se existir).
	// A query abaixo usa um filtro de periodo baseado na coluna data_pedido (se existir).
	// Se a tabela nao existir, a query falha e retornamos 0,0.
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "declarando variável valorTotal")
	var valorTotal sql.NullFloat64
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "declarando variável quantidade")
	var quantidade sql.NullInt64

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "definindo whereClause com resultado de chamada a periodoWhereClause")
	whereClause := periodoWhereClause(periodo)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "definindo filtroVendedor, args com resultado de chamada a vendedorFilter")
	filtroVendedor, args := vendedorFilter("vendedor_id", vendedorID)

	// Query genérica que só funciona se a tabela pedidos existir.
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "definindo q com resultado de chamada a fmt.Sprintf")
	q := fmt.Sprintf(`
		SELECT COALESCE(SUM(valor_total), 0), COUNT(*)
		FROM pedidos
		WHERE %s AND status NOT IN ('cancelado', 'devolvido')%s`, whereClause, filtroVendedor)

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query q, args omitidos) com leitura do resultado")
	err := db.QueryRowContext(ctx, q, args...).Scan(&valorTotal, &quantidade)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "verificando se err != nil")
	if err != nil {
		vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "verificando se isTableNotFound(err)")
		if isTableNotFound(err) {
			// Tabela pedidos ainda não existe - retorna zeros.
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("GetVendasTotais: %w", err)
	}

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "definindo v com resultado de chamada a float64")
	v := float64(0)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "definindo qtd com resultado de chamada a int")
	qtd := int(0)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "verificando se valorTotal.Valid")
	if valorTotal.Valid {
		vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "atribuindo v = valorTotal.Float64")
		v = valorTotal.Float64
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "verificando se quantidade.Valid")
	if quantidade.Valid {
		vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasTotais", "atribuindo qtd com resultado de chamada a int")
		qtd = int(quantidade.Int64)
	}
	return v, qtd, nil
}

// GetTotalPedidos retorna o total de pedidos no periodo.
// vendedorID > 0 restringe aos pedidos do vendedor; 0 (admin) = todos.
func (r *DashboardRepository) GetTotalPedidos(ctx context.Context, db *sql.DB, periodo string, vendedorID int64) (int, error) {
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTotalPedidos", "definindo whereClause com resultado de chamada a periodoWhereClause")
	whereClause := periodoWhereClause(periodo)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTotalPedidos", "definindo filtroVendedor, args com resultado de chamada a vendedorFilter")
	filtroVendedor, args := vendedorFilter("vendedor_id", vendedorID)

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTotalPedidos", "definindo q com resultado de chamada a fmt.Sprintf")
	q := fmt.Sprintf(`
		SELECT COUNT(*) FROM pedidos
		WHERE %s AND status NOT IN ('cancelado', 'devolvido')%s`, whereClause, filtroVendedor)

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTotalPedidos", "declarando variável total")
	var total sql.NullInt64
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTotalPedidos", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query q, args omitidos) com leitura do resultado")
	err := db.QueryRowContext(ctx, q, args...).Scan(&total)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTotalPedidos", "verificando se err != nil")
	if err != nil {
		vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTotalPedidos", "verificando se isTableNotFound(err)")
		if isTableNotFound(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("GetTotalPedidos: %w", err)
	}

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTotalPedidos", "verificando se total.Valid")
	if total.Valid {
		return int(total.Int64), nil
	}
	return 0, nil
}

// GetMetaMensalTotal retorna a soma de meta_mensal de todos os vendedores
// ativos (sem data_desligamento). Se a tabela vendedores nao existir,
// retorna 0. vendedorID > 0 considera apenas a meta_mensal do próprio
// vendedor; 0 (admin) soma todos os vendedores ativos.
func (r *DashboardRepository) GetMetaMensalTotal(ctx context.Context, db *sql.DB, vendedorID int64) (float64, error) {
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetaMensalTotal", "declarando variável metaTotal")
	var metaTotal sql.NullFloat64

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetaMensalTotal", "definindo filtroVendedor, args com resultado de chamada a vendedorFilter")
	filtroVendedor, args := vendedorFilter("id", vendedorID)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetaMensalTotal", "montando texto da query SQL SELECT em vendedores em q")
	q := `
		SELECT COALESCE(SUM(meta_mensal), 0)
		FROM vendedores
		WHERE data_desligamento IS NULL` + filtroVendedor

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetaMensalTotal", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query q, args omitidos) com leitura do resultado")
	err := db.QueryRowContext(ctx, q, args...).Scan(&metaTotal)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetaMensalTotal", "verificando se err != nil")
	if err != nil {
		vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetaMensalTotal", "verificando se isTableNotFound(err)")
		if isTableNotFound(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("GetMetaMensalTotal: %w", err)
	}

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetaMensalTotal", "verificando se metaTotal.Valid")
	if metaTotal.Valid {
		return metaTotal.Float64, nil
	}
	return 0, nil
}

// VendedorRanking representa um vendedor no ranking de vendas.
type VendedorRanking struct {
	ID               int64   `json:"id"`
	Nome             string  `json:"nome"`
	TotalVendas      float64 `json:"total_vendas"`
	Meta             float64 `json:"meta"`
	PercentualMeta   float64 `json:"percentual_meta"`
	QuantidadeVendas int     `json:"quantidade_vendas"`
}

// GetTopVendedores retorna o ranking dos top N vendedores por valor de vendas no mes atual.
// vendedorID > 0 restringe à linha do próprio vendedor; 0 (admin) = todos.
func (r *DashboardRepository) GetTopVendedores(ctx context.Context, db *sql.DB, limit int, vendedorID int64) ([]VendedorRanking, error) {
	// Join entre vendedores e pedidos (se existir).
	// Para funcionar sem pedidos, retornamos vendedores ativos ordenados por meta.
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTopVendedores", "definindo filtroVendedor, args com resultado de chamada a vendedorFilter")
	filtroVendedor, args := vendedorFilter("v.id", vendedorID)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTopVendedores", "montando texto da query SQL SELECT em vendedores em q")
	q := `
		SELECT
			v.id,
			v.nome,
			v.meta_mensal AS meta
		FROM vendedores v
		WHERE v.data_desligamento IS NULL` + filtroVendedor + `
		ORDER BY v.meta_mensal DESC
		LIMIT ?`

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTopVendedores", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, append(args, limit)...)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTopVendedores", "verificando se err != nil")
	if err != nil {
		vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTopVendedores", "verificando se isTableNotFound(err)")
		if isTableNotFound(err) {
			return []VendedorRanking{}, nil
		}
		return nil, fmt.Errorf("GetTopVendedores: %w", err)
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTopVendedores", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTopVendedores", "definindo result com resultado de chamada a make")
	result := make([]VendedorRanking, 0)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTopVendedores", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		var vr VendedorRanking
		if err := rows.Scan(&vr.ID, &vr.Nome, &vr.Meta); err != nil {
			return nil, fmt.Errorf("GetTopVendedores scan: %w", err)
		}
		vr.TotalVendas = 0
		vr.QuantidadeVendas = 0
		vr.PercentualMeta = 0
		result = append(result, vr)
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTopVendedores", "loop concluído; itens acumulados em result: %d", len(result))

	// Se a tabela pedidos existir, enriquecemos com dados de vendas.
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTopVendedores", "verificando se len(result) > 0")
	if len(result) > 0 {
		vlog.Printf("dashboard_repository.go", "DashboardRepository.GetTopVendedores", "executando chamada a r.enrichWithVendas")
		r.enrichWithVendas(ctx, db, result)
	}

	return result, nil
}

// vendasVendedor agrega as vendas do mes de um vendedor. total permanece
// float64 (centavos preservados) para que percentuais sejam calculados sem
// truncamento.
type vendasVendedor struct {
	total float64
	qtd   int
}

// enrichWithVendas atualiza TotalVendas e PercentualMeta a partir de pedidos.
func (r *DashboardRepository) enrichWithVendas(ctx context.Context, db *sql.DB, ranking []VendedorRanking) {
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "verificando se len(ranking) == 0")
	if len(ranking) == 0 {
		return
	}

	// Monta placeholders (?) para IN.
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "definindo placeholders = \"\"")
	placeholders := ""
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "definindo args com resultado de chamada a make")
	args := make([]any, len(ranking))
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "iniciando loop range sobre ranking (sem log por iteração)")
	for i, v := range ranking {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = v.ID
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "loop concluído; itens acumulados em args: %d", len(args))

	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "definindo q com resultado de chamada a fmt.Sprintf")
	q := fmt.Sprintf(`
		SELECT vendedor_id,
			   COALESCE(SUM(valor_total), 0),
			   COUNT(*)
		FROM pedidos
		WHERE vendedor_id IN (%s)
		  AND YEAR(data_pedido) = YEAR(CURDATE())
		  AND MONTH(data_pedido) = MONTH(CURDATE())
		  AND status NOT IN ('cancelado', 'devolvido')
		GROUP BY vendedor_id`, placeholders)

	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, args...)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "verificando se err != nil")
	if err != nil {
		return // silent fail - ranking ja tem dados
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "definindo salesMap com resultado de chamada a make")
	salesMap := make(map[int64]vendasVendedor)

	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		var vid int64
		var total sql.NullFloat64
		var qtd sql.NullInt64
		if err := rows.Scan(&vid, &total, &qtd); err != nil {
			continue
		}
		salesMap[vid] = vendasVendedor{total: total.Float64, qtd: int(qtd.Int64)}
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "loop concluído; itens acumulados em salesMap: %d", len(salesMap))

	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "iniciando loop range sobre ranking (sem log por iteração)")
	for i := range ranking {
		if sale, ok := salesMap[ranking[i].ID]; ok {
			ranking[i].TotalVendas = sale.total
			ranking[i].QuantidadeVendas = sale.qtd
			if ranking[i].Meta > 0 {
				ranking[i].PercentualMeta = (ranking[i].TotalVendas / ranking[i].Meta) * 100
			}
		}
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichWithVendas", "loop concluído")
}

// MetaVendedor representa a meta de um vendedor comparada com realizacao.
type MetaVendedor struct {
	ID               int64   `json:"id"`
	Nome             string  `json:"nome"`
	Regiao           string  `json:"regiao"`
	UF               string  `json:"uf"`
	Meta             float64 `json:"meta"`
	Realizado        float64 `json:"realizado"`
	Percentual       float64 `json:"percentual"`
	QuantidadeVendas int     `json:"quantidade_vendas"`
}

// GetMetasVendedores retorna todas as metas dos vendedores ativos com comparativo.
// vendedorID > 0 restringe à linha do próprio vendedor; 0 (admin) = todos.
func (r *DashboardRepository) GetMetasVendedores(ctx context.Context, db *sql.DB, vendedorID int64) ([]MetaVendedor, error) {
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetasVendedores", "definindo filtroVendedor, args com resultado de chamada a vendedorFilter")
	filtroVendedor, args := vendedorFilter("v.id", vendedorID)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetasVendedores", "montando texto da query SQL SELECT em vendedores em q")
	q := `
		SELECT
			v.id,
			v.nome,
			v.regiao,
			v.uf,
			v.meta_mensal AS meta
		FROM vendedores v
		WHERE v.data_desligamento IS NULL` + filtroVendedor + `
		ORDER BY v.meta_mensal DESC`

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetasVendedores", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, args...)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetasVendedores", "verificando se err != nil")
	if err != nil {
		return nil, fmt.Errorf("GetMetasVendedores: %w", err)
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetasVendedores", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetasVendedores", "definindo result com resultado de chamada a make")
	result := make([]MetaVendedor, 0)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetasVendedores", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		var mv MetaVendedor
		if err := rows.Scan(&mv.ID, &mv.Nome, &mv.Regiao, &mv.UF, &mv.Meta); err != nil {
			return nil, fmt.Errorf("GetMetasVendedores scan: %w", err)
		}
		mv.Realizado = 0
		mv.Percentual = 0
		mv.QuantidadeVendas = 0
		result = append(result, mv)
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetasVendedores", "loop concluído; itens acumulados em result: %d", len(result))

	// Enriquecer com vendas do mes atual.
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetMetasVendedores", "executando chamada a r.enrichMetasWithVendas")
	r.enrichMetasWithVendas(ctx, db, result)

	return result, nil
}

// enrichMetasWithVendas atualiza Realizado e Percentual a partir de pedidos.
func (r *DashboardRepository) enrichMetasWithVendas(ctx context.Context, db *sql.DB, metas []MetaVendedor) {
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "verificando se len(metas) == 0")
	if len(metas) == 0 {
		return
	}

	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "definindo placeholders = \"\"")
	placeholders := ""
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "definindo args com resultado de chamada a make")
	args := make([]any, len(metas))
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "iniciando loop range sobre metas (sem log por iteração)")
	for i, v := range metas {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args[i] = v.ID
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "loop concluído; itens acumulados em args: %d", len(args))

	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "definindo q com resultado de chamada a fmt.Sprintf")
	q := fmt.Sprintf(`
		SELECT vendedor_id,
			   COALESCE(SUM(valor_total), 0),
			   COUNT(*)
		FROM pedidos
		WHERE vendedor_id IN (%s)
		  AND YEAR(data_pedido) = YEAR(CURDATE())
		  AND MONTH(data_pedido) = MONTH(CURDATE())
		  AND status NOT IN ('cancelado', 'devolvido')
		GROUP BY vendedor_id`, placeholders)

	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, args...)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "verificando se err != nil")
	if err != nil {
		return
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "definindo salesMap com resultado de chamada a make")
	salesMap := make(map[int64]vendasVendedor)

	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		var vid int64
		var total sql.NullFloat64
		var qtd sql.NullInt64
		if err := rows.Scan(&vid, &total, &qtd); err != nil {
			continue
		}
		salesMap[vid] = vendasVendedor{total: total.Float64, qtd: int(qtd.Int64)}
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "loop concluído; itens acumulados em salesMap: %d", len(salesMap))

	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "iniciando loop range sobre metas (sem log por iteração)")
	for i := range metas {
		if sale, ok := salesMap[metas[i].ID]; ok {
			metas[i].Realizado = sale.total
			metas[i].QuantidadeVendas = sale.qtd
			if metas[i].Meta > 0 {
				metas[i].Percentual = (metas[i].Realizado / metas[i].Meta) * 100
			}
		}
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.enrichMetasWithVendas", "loop concluído")
}

// GetVendasSeries retorna serie temporal (data -> valor, quantidade) dos ultimos N dias.
// vendedorID > 0 restringe aos pedidos do vendedor; 0 (admin) = todos.
func (r *DashboardRepository) GetVendasSeries(ctx context.Context, db *sql.DB, dias int, vendedorID int64) ([]map[string]any, error) {
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasSeries", "definindo filtroVendedor, filtroArgs com resultado de chamada a vendedorFilter")
	filtroVendedor, filtroArgs := vendedorFilter("vendedor_id", vendedorID)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasSeries", "montando texto da query SQL SELECT em pedidos em q")
	q := `
		SELECT DATE_FORMAT(data_pedido, '%Y-%m-%d') AS data,
			   COALESCE(SUM(valor_total), 0) AS valor,
			   COUNT(*) AS quantidade
		FROM pedidos
		WHERE data_pedido >= DATE_SUB(CURDATE(), INTERVAL ? DAY)
		  AND status NOT IN ('cancelado', 'devolvido')` + filtroVendedor + `
		GROUP BY DATE_FORMAT(data_pedido, '%Y-%m-%d')
		ORDER BY data ASC`

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasSeries", "adicionando 1 parâmetro(s) de placeholder em args (valores omitidos)")
	args := append([]any{dias}, filtroArgs...)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasSeries", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, args...)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasSeries", "verificando se err != nil")
	if err != nil {
		vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasSeries", "verificando se isTableNotFound(err)")
		if isTableNotFound(err) {
			// Gera serie vazia com dias zeros.
			return r.emptySeries(dias), nil
		}
		return nil, fmt.Errorf("GetVendasSeries: %w", err)
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasSeries", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasSeries", "definindo seriesMap com resultado de chamada a make")
	seriesMap := make(map[string]struct {
		valor      float64
		quantidade int
	})

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasSeries", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		var data string
		var valor sql.NullFloat64
		var qtd sql.NullInt64
		if err := rows.Scan(&data, &valor, &qtd); err != nil {
			continue
		}
		v := 0.0
		q := 0
		if valor.Valid {
			v = valor.Float64
		}
		if qtd.Valid {
			q = int(qtd.Int64)
		}
		seriesMap[data] = struct {
			valor      float64
			quantidade int
		}{valor: v, quantidade: q}
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasSeries", "loop concluído; itens acumulados em seriesMap: %d", len(seriesMap))

	// Se nao teve dados, retorna serie vazia.
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasSeries", "verificando se len(seriesMap) == 0")
	if len(seriesMap) == 0 {
		return r.emptySeries(dias), nil
	}

	// Monta serie completa (todos os dias do range, mesmo sem vendas).
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendasSeries", "definindo series com resultado de chamada a r.buildFullSeries")
	series := r.buildFullSeries(ctx, db, dias, seriesMap)
	return series, nil
}

// EmptySeries retorna a serie de vendas zerada para os ultimos N dias, no
// mesmo formato de GetVendasSeries ({dia, total_vendas, total_pedidos}).
// Usada quando o escopo do usuario nao permite ver nenhum pedido.
func (r *DashboardRepository) EmptySeries(dias int) []map[string]any {
	return r.emptySeries(dias)
}

// emptySeries retorna uma serie com zeros para os ultimos N dias.
// As datas sao geradas em Go no formato YYYY-MM-DD, equivalente a
// DATE_SUB(CURDATE(), INTERVAL i DAY), para bater com o contrato do frontend.
func (r *DashboardRepository) emptySeries(dias int) []map[string]any {
	vlog.Printf("dashboard_repository.go", "DashboardRepository.emptySeries", "definindo hoje com resultado de chamada a time.Now")
	hoje := time.Now()
	vlog.Printf("dashboard_repository.go", "DashboardRepository.emptySeries", "definindo series com resultado de chamada a make")
	series := make([]map[string]any, 0, dias)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.emptySeries", "iniciando loop enquanto i >= 0 (sem log por iteração)")
	for i := dias - 1; i >= 0; i-- {
		dia := hoje.AddDate(0, 0, -i).Format("2006-01-02")
		series = append(series, map[string]any{
			"dia":           dia,
			"total_vendas":  0.0,
			"total_pedidos": 0,
		})
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.emptySeries", "loop concluído; itens acumulados em series: %d", len(series))
	return series
}

// buildFullSeries gera entrada para cada dia do range.
func (r *DashboardRepository) buildFullSeries(ctx context.Context, db *sql.DB, dias int, data map[string]struct {
	valor      float64
	quantidade int
}) []map[string]any {
	vlog.Printf("dashboard_repository.go", "DashboardRepository.buildFullSeries", "definindo series com resultado de chamada a make")
	series := make([]map[string]any, 0, dias)

	// Pega datas do banco para ter a sequencia correta.
	vlog.Printf("dashboard_repository.go", "DashboardRepository.buildFullSeries", "montando texto da query SQL SELECT em q")
	q := `SELECT DATE_FORMAT(DATE_SUB(CURDATE(), INTERVAL n DAY), '%Y-%m-%d') AS dia
	      FROM (
	          SELECT 0 AS n UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4
	          UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9
	          UNION SELECT 10 UNION SELECT 11 UNION SELECT 12 UNION SELECT 13 UNION SELECT 14
	          UNION SELECT 15 UNION SELECT 16 UNION SELECT 17 UNION SELECT 18 UNION SELECT 19
	          UNION SELECT 20 UNION SELECT 21 UNION SELECT 22 UNION SELECT 23 UNION SELECT 24
	          UNION SELECT 25 UNION SELECT 26 UNION SELECT 27 UNION SELECT 28 UNION SELECT 29
	          UNION SELECT 30 UNION SELECT 31 UNION SELECT 32 UNION SELECT 33 UNION SELECT 34
	          UNION SELECT 35 UNION SELECT 36 UNION SELECT 37 UNION SELECT 38 UNION SELECT 39
	          UNION SELECT 40 UNION SELECT 41 UNION SELECT 42 UNION SELECT 43 UNION SELECT 44
	          UNION SELECT 45 UNION SELECT 46 UNION SELECT 47 UNION SELECT 48 UNION SELECT 49
	          UNION SELECT 50 UNION SELECT 51 UNION SELECT 52 UNION SELECT 53 UNION SELECT 54
	          UNION SELECT 55 UNION SELECT 56 UNION SELECT 57 UNION SELECT 58 UNION SELECT 59
	          UNION SELECT 60 UNION SELECT 61 UNION SELECT 62 UNION SELECT 63 UNION SELECT 64
	          UNION SELECT 65 UNION SELECT 66 UNION SELECT 67 UNION SELECT 68 UNION SELECT 69
	          UNION SELECT 70 UNION SELECT 71 UNION SELECT 72 UNION SELECT 73 UNION SELECT 74
	          UNION SELECT 75 UNION SELECT 76 UNION SELECT 77 UNION SELECT 78 UNION SELECT 79
	          UNION SELECT 80 UNION SELECT 81 UNION SELECT 82 UNION SELECT 83 UNION SELECT 84
	          UNION SELECT 85 UNION SELECT 86 UNION SELECT 87 UNION SELECT 88 UNION SELECT 89
	          UNION SELECT 90 UNION SELECT 91 UNION SELECT 92 UNION SELECT 93 UNION SELECT 94
	          UNION SELECT 95 UNION SELECT 96 UNION SELECT 97 UNION SELECT 98 UNION SELECT 99
	          UNION SELECT 100 UNION SELECT 101 UNION SELECT 102 UNION SELECT 103 UNION SELECT 104
	          UNION SELECT 105 UNION SELECT 106 UNION SELECT 107 UNION SELECT 108 UNION SELECT 109
	          UNION SELECT 110 UNION SELECT 111 UNION SELECT 112 UNION SELECT 113 UNION SELECT 114
	          UNION SELECT 115 UNION SELECT 116 UNION SELECT 117 UNION SELECT 118 UNION SELECT 119
	          UNION SELECT 120 UNION SELECT 121 UNION SELECT 122 UNION SELECT 123 UNION SELECT 124
	          UNION SELECT 125 UNION SELECT 126 UNION SELECT 127 UNION SELECT 128 UNION SELECT 129
	          UNION SELECT 130 UNION SELECT 131 UNION SELECT 132 UNION SELECT 133 UNION SELECT 134
	          UNION SELECT 135 UNION SELECT 136 UNION SELECT 137 UNION SELECT 138 UNION SELECT 139
	          UNION SELECT 140 UNION SELECT 141 UNION SELECT 142 UNION SELECT 143 UNION SELECT 144
	          UNION SELECT 145 UNION SELECT 146 UNION SELECT 147 UNION SELECT 148 UNION SELECT 149
	          UNION SELECT 150 UNION SELECT 151 UNION SELECT 152 UNION SELECT 153 UNION SELECT 154
	          UNION SELECT 155 UNION SELECT 156 UNION SELECT 157 UNION SELECT 158 UNION SELECT 159
	          UNION SELECT 160 UNION SELECT 161 UNION SELECT 162 UNION SELECT 163 UNION SELECT 164
	          UNION SELECT 165 UNION SELECT 166 UNION SELECT 167 UNION SELECT 168 UNION SELECT 169
	          UNION SELECT 170 UNION SELECT 171 UNION SELECT 172 UNION SELECT 173 UNION SELECT 174
	          UNION SELECT 175 UNION SELECT 176 UNION SELECT 177 UNION SELECT 178 UNION SELECT 179
	          UNION SELECT 180 UNION SELECT 181 UNION SELECT 182 UNION SELECT 183 UNION SELECT 184
	          UNION SELECT 185 UNION SELECT 186 UNION SELECT 187 UNION SELECT 188 UNION SELECT 189
	          UNION SELECT 190 UNION SELECT 191 UNION SELECT 192 UNION SELECT 193 UNION SELECT 194
	          UNION SELECT 195 UNION SELECT 196 UNION SELECT 197 UNION SELECT 198 UNION SELECT 199
	          UNION SELECT 200 UNION SELECT 201 UNION SELECT 202 UNION SELECT 203 UNION SELECT 204
	          UNION SELECT 205 UNION SELECT 206 UNION SELECT 207 UNION SELECT 208 UNION SELECT 209
	          UNION SELECT 210 UNION SELECT 211 UNION SELECT 212 UNION SELECT 213 UNION SELECT 214
	          UNION SELECT 215 UNION SELECT 216 UNION SELECT 217 UNION SELECT 218 UNION SELECT 219
	          UNION SELECT 220 UNION SELECT 221 UNION SELECT 222 UNION SELECT 223 UNION SELECT 224
	          UNION SELECT 225 UNION SELECT 226 UNION SELECT 227 UNION SELECT 228 UNION SELECT 229
	          UNION SELECT 230 UNION SELECT 231 UNION SELECT 232 UNION SELECT 233 UNION SELECT 234
	          UNION SELECT 235 UNION SELECT 236 UNION SELECT 237 UNION SELECT 238 UNION SELECT 239
	          UNION SELECT 240 UNION SELECT 241 UNION SELECT 242 UNION SELECT 243 UNION SELECT 244
	          UNION SELECT 245 UNION SELECT 246 UNION SELECT 247 UNION SELECT 248 UNION SELECT 249
	          UNION SELECT 250 UNION SELECT 251 UNION SELECT 252 UNION SELECT 253 UNION SELECT 254
	          UNION SELECT 255 UNION SELECT 256 UNION SELECT 257 UNION SELECT 258 UNION SELECT 259
	          UNION SELECT 260 UNION SELECT 261 UNION SELECT 262 UNION SELECT 263 UNION SELECT 264
	          UNION SELECT 265 UNION SELECT 266 UNION SELECT 267 UNION SELECT 268 UNION SELECT 269
	          UNION SELECT 270 UNION SELECT 271 UNION SELECT 272 UNION SELECT 273 UNION SELECT 274
	          UNION SELECT 275 UNION SELECT 276 UNION SELECT 277 UNION SELECT 278 UNION SELECT 279
	          UNION SELECT 280 UNION SELECT 281 UNION SELECT 282 UNION SELECT 283 UNION SELECT 284
	          UNION SELECT 285 UNION SELECT 286 UNION SELECT 287 UNION SELECT 288 UNION SELECT 289
	          UNION SELECT 290 UNION SELECT 291 UNION SELECT 292 UNION SELECT 293 UNION SELECT 294
	          UNION SELECT 295 UNION SELECT 296 UNION SELECT 297 UNION SELECT 298 UNION SELECT 299
	          UNION SELECT 300 UNION SELECT 301 UNION SELECT 302 UNION SELECT 303 UNION SELECT 304
	          UNION SELECT 305 UNION SELECT 306 UNION SELECT 307 UNION SELECT 308 UNION SELECT 309
	          UNION SELECT 310 UNION SELECT 311 UNION SELECT 312 UNION SELECT 313 UNION SELECT 314
	          UNION SELECT 315 UNION SELECT 316 UNION SELECT 317 UNION SELECT 318 UNION SELECT 319
	          UNION SELECT 320 UNION SELECT 321 UNION SELECT 322 UNION SELECT 323 UNION SELECT 324
	          UNION SELECT 325 UNION SELECT 326 UNION SELECT 327 UNION SELECT 328 UNION SELECT 329
	          UNION SELECT 330 UNION SELECT 331 UNION SELECT 332 UNION SELECT 333 UNION SELECT 334
	          UNION SELECT 335 UNION SELECT 336 UNION SELECT 337 UNION SELECT 338 UNION SELECT 339
	          UNION SELECT 340 UNION SELECT 341 UNION SELECT 342 UNION SELECT 343 UNION SELECT 344
	          UNION SELECT 345 UNION SELECT 346 UNION SELECT 347 UNION SELECT 348 UNION SELECT 349
	          UNION SELECT 350 UNION SELECT 351 UNION SELECT 352 UNION SELECT 353 UNION SELECT 354
	          UNION SELECT 355 UNION SELECT 356 UNION SELECT 357 UNION SELECT 358 UNION SELECT 359
	          UNION SELECT 360 UNION SELECT 361 UNION SELECT 362 UNION SELECT 363 UNION SELECT 364
	      ) nums
	      WHERE n < ?
	      ORDER BY dia ASC`

	vlog.Printf("dashboard_repository.go", "DashboardRepository.buildFullSeries", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, dias)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.buildFullSeries", "verificando se err != nil")
	if err != nil {
		return r.emptySeries(dias)
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.buildFullSeries", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("dashboard_repository.go", "DashboardRepository.buildFullSeries", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
	if err := rows.Err(); err != nil {
		return r.emptySeries(dias)
	}

	vlog.Printf("dashboard_repository.go", "DashboardRepository.buildFullSeries", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		var dia string
		if err := rows.Scan(&dia); err != nil {
			continue
		}
		if d, ok := data[dia]; ok {
			series = append(series, map[string]any{
				"dia":           dia,
				"total_vendas":  d.valor,
				"total_pedidos": d.quantidade,
			})
		} else {
			series = append(series, map[string]any{
				"dia":           dia,
				"total_vendas":  0.0,
				"total_pedidos": 0,
			})
		}
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.buildFullSeries", "loop concluído; itens acumulados em series: %d", len(series))

	return series
}

// GetVendedoresRanking retorna ranking paginado de vendedores com vendas, meta e percentual.
// vendedorID > 0 restringe à linha do próprio vendedor (total coerente: 0
// ou 1); 0 (admin) = todos os vendedores ativos.
func (r *DashboardRepository) GetVendedoresRanking(ctx context.Context, db *sql.DB, page, limit int, vendedorID int64) ([]map[string]any, int, error) {
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "verificando se page < 1")
	if page < 1 {
		vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "atribuindo page = 1")
		page = 1
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "verificando se limit < 1")
	if limit < 1 {
		vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "atribuindo limit = 20")
		limit = 20
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "verificando se limit > 100")
	if limit > 100 {
		vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "atribuindo limit = 100")
		limit = 100
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "definindo offset = (page - 1) * limit")
	offset := (page - 1) * limit

	// Total de vendedores ativos (no escopo).
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "definindo filtroCount, countArgs com resultado de chamada a vendedorFilter")
	filtroCount, countArgs := vendedorFilter("id", vendedorID)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "declarando variável total")
	var total int
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query dinâmica, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM vendedores WHERE data_desligamento IS NULL`+filtroCount,
		countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("GetVendedoresRanking count: %w", err)
	}

	// Lista de vendedores com vendas do mes, ordenados por meta e, em caso de
	// empate, por atingimento da meta — ordenacao feita no banco, antes do
	// LIMIT/OFFSET, para que a paginacao seja consistente entre paginas.
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "definindo filtroLista, listaArgs com resultado de chamada a vendedorFilter")
	filtroLista, listaArgs := vendedorFilter("v.id", vendedorID)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "montando texto da query SQL SELECT em vendedores em q")
	q := `
		SELECT
			v.id, v.nome, v.regiao, v.uf, v.meta_mensal,
			COALESCE(p.total_vendas, 0) AS total_vendas,
			COALESCE(p.total_pedidos, 0) AS total_pedidos,
			CASE WHEN v.meta_mensal > 0 THEN (COALESCE(p.total_vendas, 0) / v.meta_mensal) * 100 ELSE 0 END AS atingimento_meta
		FROM vendedores v
		LEFT JOIN (
			SELECT vendedor_id, SUM(valor_total) AS total_vendas, COUNT(*) AS total_pedidos
			FROM pedidos
			WHERE YEAR(data_pedido) = YEAR(CURDATE()) AND MONTH(data_pedido) = MONTH(CURDATE())
			  AND status NOT IN ('cancelado', 'devolvido')
			GROUP BY vendedor_id
		) p ON p.vendedor_id = v.id
		WHERE v.data_desligamento IS NULL` + filtroLista + `
		ORDER BY v.meta_mensal DESC, atingimento_meta DESC
		LIMIT ? OFFSET ?`

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, append(listaArgs, limit, offset)...)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "verificando se err != nil")
	if err != nil {
		return nil, 0, fmt.Errorf("GetVendedoresRanking: %w", err)
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "definindo result com resultado de chamada a make")
	result := make([]map[string]any, 0, limit)
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		var (
			id             int64
			nome           string
			regiao         string
			uf             string
			meta           float64
			totalVendas    float64
			totalPedidos   int
			atingimentoRaw float64
		)
		if err := rows.Scan(&id, &nome, &regiao, &uf, &meta, &totalVendas, &totalPedidos, &atingimentoRaw); err != nil {
			return nil, 0, fmt.Errorf("GetVendedoresRanking scan: %w", err)
		}

		ticketMedio := 0.0
		if totalPedidos > 0 {
			ticketMedio = totalVendas / float64(totalPedidos)
		}

		result = append(result, map[string]any{
			"vendedor_id":      id,
			"vendedor_nome":    nome,
			"regiao":           regiao,
			"uf":               uf,
			"total_vendas":     totalVendas,
			"total_pedidos":    totalPedidos,
			"ticket_medio":     mathRound(ticketMedio, 2),
			"meta":             meta,
			"atingimento_meta": mathRound(atingimentoRaw, 2),
		})
	}
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "loop concluído; itens acumulados em result: %d", len(result))
	vlog.Printf("dashboard_repository.go", "DashboardRepository.GetVendedoresRanking", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("GetVendedoresRanking rows: %w", err)
	}

	return result, total, nil
}

// isTableNotFound retorna true se o erro indica que a tabela nao existe.
func isTableNotFound(err error) bool {
	vlog.Printf("dashboard_repository.go", "isTableNotFound", "verificando se err == nil")
	if err == nil {
		return false
	}
	vlog.Printf("dashboard_repository.go", "isTableNotFound", "definindo errStr com resultado de chamada a err.Error")
	errStr := err.Error()
	return contains(errStr, "doesn't exist") || contains(errStr, "not found") ||
		contains(errStr, "Error 1146") || contains(errStr, "no such table")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	vlog.Printf("dashboard_repository.go", "containsSubstr", "iniciando loop enquanto i <= len(s)-len(substr) (sem log por iteração)")
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	vlog.Printf("dashboard_repository.go", "containsSubstr", "loop concluído")
	return false
}

// mathRound arredonda um float para N casas decimais.
func mathRound(val float64, prec int) float64 {
	vlog.Printf("dashboard_repository.go", "mathRound", "definindo mult = 1.0")
	mult := 1.0
	vlog.Printf("dashboard_repository.go", "mathRound", "iniciando loop enquanto i < prec (sem log por iteração)")
	for i := 0; i < prec; i++ {
		mult *= 10
	}
	vlog.Printf("dashboard_repository.go", "mathRound", "loop concluído")
	return float64(int(val*mult+0.5)) / mult
}
