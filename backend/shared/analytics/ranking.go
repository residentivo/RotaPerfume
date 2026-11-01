package analytics

import "math"

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

// arredondar2 arredonda para 2 casas, metade para longe do zero
// (math.Round, BUG-19; nunca RoundToEven): negativos passam a arredondar
// simetricamente (-1.555 → -1.56; o legado somava 0,5 e truncava, -1.55).
// NaN e ±Inf viram 0 (o json.Marshal falharia) e o resultado zero é sempre
// +0 (math.Round(-0.4) é -0, que o JSON serializaria como "-0"). Usada
// também pelo funil.
func arredondar2(val float64) float64 {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	r := math.Round(val*100) / 100
	if r == 0 {
		return 0 // sem -0 no JSON
	}
	return r
}
