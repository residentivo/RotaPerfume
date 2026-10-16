package analytics

// MetricasClientes é o read model de GET /api/dashboard/clientes: totais,
// novos cadastros no período e as distribuições por segmento e por UF.
type MetricasClientes struct {
	Periodo        Periodo
	TotalClientes  int
	TotalAtivos    int
	TotalInativos  int
	NovosNoPeriodo int
	PorSegmento    []ContagemSegmento
	PorUF          []ContagemUF
}

// ContagemSegmento é o total de clientes de um segmento.
type ContagemSegmento struct {
	Segmento string
	Total    int
}

// ContagemUF é o total de clientes de uma UF.
type ContagemUF struct {
	UF    string
	Total int
}

// NovasMetricasClientes monta o read model; listas nil viram [].
func NovasMetricasClientes(m MetricasClientes) MetricasClientes {
	m.PorSegmento = naoNulo(m.PorSegmento)
	m.PorUF = naoNulo(m.PorUF)
	return m
}

// MetricasClientesZeradas é a resposta de quem não tem acesso (D-01).
func MetricasClientesZeradas(p Periodo) MetricasClientes {
	return NovasMetricasClientes(MetricasClientes{Periodo: p})
}
