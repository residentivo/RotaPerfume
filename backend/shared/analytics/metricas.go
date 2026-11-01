package analytics

// Metricas é o read model de GET /api/dashboard/metrics: os números de
// vendas do período, o top 10 de vendedores, as metas do mês e o aviso de
// vendedor desligado (D-01). As listas nunca são nil (o contrato é [],
// nunca null).
type Metricas struct {
	Periodo           Periodo
	TotalVendas       float64
	TotalVendasQtd    int
	TotalPedidos      int
	TicketMedio       float64
	TopVendedores     []VendedorRanking
	MetasVendedores   []MetaVendedor
	MetaMes           float64
	VendedorDesligado bool
}

// TotaisVendas são os agregados de vendas do período (pedidos que não
// estão cancelados nem devolvidos).
type TotaisVendas struct {
	Valor      float64
	Quantidade int
}

// NovasMetricas monta as métricas do escopo. O ticket médio é derivado dos
// totais já filtrados pelo escopo (0 sem vendas). Nunca sinaliza
// desligamento: quem tem dados não está bloqueado.
func NovasMetricas(p Periodo, t TotaisVendas, totalPedidos int, top []VendedorRanking, metas []MetaVendedor, metaMes float64) Metricas {
	var ticket float64
	if t.Quantidade > 0 {
		ticket = t.Valor / float64(t.Quantidade)
	}
	return Metricas{
		Periodo:         p,
		TotalVendas:     t.Valor,
		TotalVendasQtd:  t.Quantidade,
		TotalPedidos:    totalPedidos,
		TicketMedio:     ticket,
		TopVendedores:   naoNulo(top),
		MetasVendedores: naoNulo(metas),
		MetaMes:         metaMes,
	}
}

// MetricasZeradas é a resposta de quem não tem acesso (D-01): o mesmo
// formato, tudo zerado, listas vazias e o aviso de desligamento.
func MetricasZeradas(p Periodo, vendedorDesligado bool) Metricas {
	m := NovasMetricas(p, TotaisVendas{}, 0, nil, nil, 0)
	m.VendedorDesligado = vendedorDesligado
	return m
}

// VendasDoMes são as vendas do mês corrente de um vendedor. O total fica em
// float64 (centavos preservados) para o percentual não truncar.
type VendasDoMes struct {
	Total      float64
	Quantidade int
}

// VendedorRanking é uma linha do top de vendedores (por meta).
type VendedorRanking struct {
	ID               int64
	Nome             string
	TotalVendas      float64
	Meta             float64
	PercentualMeta   float64
	QuantidadeVendas int
}

// AplicarVendas registra as vendas do mês e o percentual da meta (sem meta,
// o percentual fica 0).
func (v *VendedorRanking) AplicarVendas(vm VendasDoMes) {
	v.TotalVendas = vm.Total
	v.QuantidadeVendas = vm.Quantidade
	v.PercentualMeta = percentual(v.TotalVendas, v.Meta, v.PercentualMeta)
}

// MetaVendedor é a meta de um vendedor ativo comparada com o realizado.
type MetaVendedor struct {
	ID               int64
	Nome             string
	Regiao           string
	UF               string
	Meta             float64
	Realizado        float64
	Percentual       float64
	QuantidadeVendas int
}

// AplicarVendas registra o realizado do mês e o percentual da meta (sem
// meta, o percentual fica 0).
func (m *MetaVendedor) AplicarVendas(vm VendasDoMes) {
	m.Realizado = vm.Total
	m.QuantidadeVendas = vm.Quantidade
	m.Percentual = percentual(m.Realizado, m.Meta, m.Percentual)
}

// percentual é (valor/meta)*100 quando há meta; senão mantém o atual.
func percentual(valor, meta, atual float64) float64 {
	if meta > 0 {
		return (valor / meta) * 100
	}
	return atual
}

// naoNulo troca nil por uma lista vazia (o contrato serializa []).
func naoNulo[T any](v []T) []T {
	if v == nil {
		return make([]T, 0)
	}
	return v
}
