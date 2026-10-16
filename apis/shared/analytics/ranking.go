package analytics

// LinhaRanking é uma linha de GET /api/dashboard/vendedores: vendas do mês,
// meta e atingimento de um vendedor ativo. A ordenação (meta desc,
// atingimento desc) é do SQL, antes do LIMIT/OFFSET.
type LinhaRanking struct {
	VendedorID      int64
	VendedorNome    string
	Regiao          string
	UF              string
	TotalVendas     float64
	TotalPedidos    int
	TicketMedio     float64
	Meta            float64
	AtingimentoMeta float64
}

// DadosRanking é a linha crua lida do banco.
type DadosRanking struct {
	VendedorID   int64
	VendedorNome string
	Regiao       string
	UF           string
	Meta         float64
	TotalVendas  float64
	TotalPedidos int
	Atingimento  float64
}

// NovaLinhaRanking deriva o ticket médio (0 sem pedidos) e arredonda ticket
// e atingimento em 2 casas, como o legado.
func NovaLinhaRanking(d DadosRanking) LinhaRanking {
	ticket := 0.0
	if d.TotalPedidos > 0 {
		ticket = d.TotalVendas / float64(d.TotalPedidos)
	}
	return LinhaRanking{
		VendedorID:      d.VendedorID,
		VendedorNome:    d.VendedorNome,
		Regiao:          d.Regiao,
		UF:              d.UF,
		TotalVendas:     d.TotalVendas,
		TotalPedidos:    d.TotalPedidos,
		TicketMedio:     arredondar2(ticket),
		Meta:            d.Meta,
		AtingimentoMeta: arredondar2(d.Atingimento),
	}
}

// arredondar2 é o arredondamento legado (mathRound(val, 2)): soma 0,5 na
// segunda casa e trunca para inteiro. Mantido igual para nenhum número do
// ranking mudar na migração (inclusive com valores negativos).
func arredondar2(val float64) float64 {
	const mult = 100.0
	return float64(int(val*mult+0.5)) / mult
}
