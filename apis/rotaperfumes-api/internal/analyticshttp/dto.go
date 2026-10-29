package analyticshttp

import "github.com/rotaperfumes/shared/analytics"

// DTOs do contrato do Dashboard. O legado respondia map[string]any (o
// encoder ordena as chaves), então os campos que vinham de map estão em
// ORDEM ALFABÉTICA de chave JSON; os que vinham de struct legada
// (VendedorRanking, MetaVendedor, SegmentoContagem, UFContagem) mantêm a
// ordem dos campos daquela struct. Os tipos numéricos são os mesmos (int
// continua int, float64 continua float64): o JSON sai byte a byte igual.
// Listas são sempre [] (nunca null).

// metricasResposta era o map de metricsResponse (dashboard_service.go).
type metricasResposta struct {
	MetaMes           float64            `json:"meta_mes"`
	MetasVendedores   []metaVendedorJSON `json:"metas_vendedores"`
	Periodo           string             `json:"periodo"`
	TicketMedio       float64            `json:"ticket_medio"`
	TopVendedores     []topVendedorJSON  `json:"top_vendedores"`
	TotalPedidos      int                `json:"total_pedidos"`
	TotalVendas       float64            `json:"total_vendas"`
	TotalVendasQtd    int                `json:"total_vendas_qtd"`
	VendedorDesligado bool               `json:"vendedor_desligado"`
}

// topVendedorJSON era repositories.VendedorRanking (ordem da struct).
type topVendedorJSON struct {
	ID               int64   `json:"id"`
	Nome             string  `json:"nome"`
	TotalVendas      float64 `json:"total_vendas"`
	Meta             float64 `json:"meta"`
	PercentualMeta   float64 `json:"percentual_meta"`
	QuantidadeVendas int     `json:"quantidade_vendas"`
}

// metaVendedorJSON era repositories.MetaVendedor (ordem da struct).
type metaVendedorJSON struct {
	ID               int64   `json:"id"`
	Nome             string  `json:"nome"`
	Regiao           string  `json:"regiao"`
	UF               string  `json:"uf"`
	Meta             float64 `json:"meta"`
	Realizado        float64 `json:"realizado"`
	Percentual       float64 `json:"percentual"`
	QuantidadeVendas int     `json:"quantidade_vendas"`
}

func metricasJSON(m analytics.Metricas) metricasResposta {
	return metricasResposta{
		MetaMes:           m.MetaMes,
		MetasVendedores:   lista(m.MetasVendedores, func(v analytics.MetaVendedor) metaVendedorJSON { return metaVendedorJSON(v) }),
		Periodo:           m.Periodo.String(),
		TicketMedio:       m.TicketMedio,
		TopVendedores:     lista(m.TopVendedores, func(v analytics.VendedorRanking) topVendedorJSON { return topVendedorJSON(v) }),
		TotalPedidos:      m.TotalPedidos,
		TotalVendas:       m.TotalVendas,
		TotalVendasQtd:    m.TotalVendasQtd,
		VendedorDesligado: m.VendedorDesligado,
	}
}

// serieResposta era o map de vendasSeriesResponse.
type serieResposta struct {
	Dias   int         `json:"dias"`
	Pontos []pontoJSON `json:"pontos"`
}

// pontoJSON era o map de cada ponto da série.
type pontoJSON struct {
	Dia          string  `json:"dia"`
	TotalPedidos int     `json:"total_pedidos"`
	TotalVendas  float64 `json:"total_vendas"`
}

func serieJSON(s analytics.SerieVendas) serieResposta {
	return serieResposta{
		Dias: s.Dias.Int(),
		Pontos: lista(s.Pontos, func(p analytics.PontoVenda) pontoJSON {
			return pontoJSON{Dia: p.Dia, TotalPedidos: p.TotalPedidos, TotalVendas: p.TotalVendas}
		}),
	}
}

// linhaRankingResposta era o map de cada linha de GetVendedoresRanking.
type linhaRankingResposta struct {
	AtingimentoMeta float64 `json:"atingimento_meta"`
	Meta            float64 `json:"meta"`
	Regiao          string  `json:"regiao"`
	TicketMedio     float64 `json:"ticket_medio"`
	TotalPedidos    int     `json:"total_pedidos"`
	TotalVendas     float64 `json:"total_vendas"`
	UF              string  `json:"uf"`
	VendedorID      int64   `json:"vendedor_id"`
	VendedorNome    string  `json:"vendedor_nome"`
}

func linhaRankingJSON(l analytics.LinhaRanking) linhaRankingResposta {
	return linhaRankingResposta{
		AtingimentoMeta: l.AtingimentoMeta,
		Meta:            l.Meta,
		Regiao:          l.Regiao,
		TicketMedio:     l.TicketMedio,
		TotalPedidos:    l.TotalPedidos,
		TotalVendas:     l.TotalVendas,
		UF:              l.UF,
		VendedorID:      l.VendedorID,
		VendedorNome:    l.VendedorNome,
	}
}

// metricasClientesResposta era o map de clienteMetricsResponse.
type metricasClientesResposta struct {
	NovosNoPeriodo int            `json:"novos_no_periodo"`
	Periodo        string         `json:"periodo"`
	PorSegmento    []segmentoJSON `json:"por_segmento"`
	PorUF          []ufJSON       `json:"por_uf"`
	TotalAtivos    int            `json:"total_ativos"`
	TotalClientes  int            `json:"total_clientes"`
	TotalInativos  int            `json:"total_inativos"`
}

// segmentoJSON era repositories.SegmentoContagem.
type segmentoJSON struct {
	Segmento string `json:"segmento"`
	Total    int    `json:"total"`
}

// ufJSON era repositories.UFContagem.
type ufJSON struct {
	UF    string `json:"uf"`
	Total int    `json:"total"`
}

func metricasClientesJSON(m analytics.MetricasClientes) metricasClientesResposta {
	return metricasClientesResposta{
		NovosNoPeriodo: m.NovosNoPeriodo,
		Periodo:        m.Periodo.String(),
		PorSegmento:    lista(m.PorSegmento, func(c analytics.ContagemSegmento) segmentoJSON { return segmentoJSON(c) }),
		PorUF:          lista(m.PorUF, func(c analytics.ContagemUF) ufJSON { return ufJSON(c) }),
		TotalAtivos:    m.TotalAtivos,
		TotalClientes:  m.TotalClientes,
		TotalInativos:  m.TotalInativos,
	}
}

// funilResposta é o contrato de /funil (DASH-01, endpoint novo, sem
// legado): campos na ordem do contrato.
type funilResposta struct {
	Janela             string           `json:"janela"`
	TotalOportunidades int              `json:"total_oportunidades"`
	Etapas             []etapaFunilJSON `json:"etapas"`
	Perdidas           totalEtapaJSON   `json:"perdidas"`
	TaxaGanhoPct       float64          `json:"taxa_ganho_pct"`
	VendedorDesligado  bool             `json:"vendedor_desligado"`
}

// etapaFunilJSON é uma etapa do funil cumulativo.
type etapaFunilJSON struct {
	Etapa                string  `json:"etapa"`
	Quantidade           int     `json:"quantidade"`
	ValorEstimado        float64 `json:"valor_estimado"`
	ConversaoAnteriorPct float64 `json:"conversao_anterior_pct"`
	ConversaoTopoPct     float64 `json:"conversao_topo_pct"`
}

// totalEtapaJSON são as perdidas (quantidade e valor estimado).
type totalEtapaJSON struct {
	Quantidade    int     `json:"quantidade"`
	ValorEstimado float64 `json:"valor_estimado"`
}

func funilJSON(f analytics.Funil) funilResposta {
	return funilResposta{
		Janela:             f.Janela.String(),
		TotalOportunidades: f.TotalOportunidades,
		Etapas:             lista(f.Etapas, func(e analytics.EtapaFunil) etapaFunilJSON { return etapaFunilJSON(e) }),
		Perdidas:           totalEtapaJSON(f.Perdidas),
		TaxaGanhoPct:       f.TaxaGanhoPct,
		VendedorDesligado:  f.VendedorDesligado,
	}
}

// lista converte os itens; o resultado nunca é nil (o contrato é []).
func lista[T, D any](itens []T, conv func(T) D) []D {
	out := make([]D, 0, len(itens))
	for _, v := range itens {
		out = append(out, conv(v))
	}
	return out
}
