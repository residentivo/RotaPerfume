package analytics

import "time"

// layoutDia é o formato YYYY-MM-DD do contrato (o mesmo do DATE_FORMAT).
const layoutDia = "2006-01-02"

// SerieVendas é o read model de GET /api/dashboard/vendas.
type SerieVendas struct {
	Dias   Dias
	Pontos []PontoVenda
}

// PontoVenda é um dia da série.
type PontoVenda struct {
	Dia          string
	TotalVendas  float64
	TotalPedidos int
}

// SerieZerada é a série com todos os n dias zerados, do mais antigo até
// hoje (datas YYYY-MM-DD no fuso de hoje, equivalentes a
// DATE_SUB(CURDATE(), INTERVAL i DAY)). n ≤ 0 devolve lista vazia, nunca nil.
func SerieZerada(n int, hoje time.Time) []PontoVenda {
	pontos := make([]PontoVenda, 0, max(n, 0))
	for i := n - 1; i >= 0; i-- {
		pontos = append(pontos, PontoVenda{Dia: hoje.AddDate(0, 0, -i).Format(layoutDia)})
	}
	return pontos
}

// SerieCompleta preenche cada dia da sequência com as vendas do dia (ou
// zero, se não houve venda), na ordem da sequência. A lista nunca é nil.
func SerieCompleta(dias []string, vendas map[string]PontoVenda) []PontoVenda {
	pontos := make([]PontoVenda, 0, len(dias))
	for _, dia := range dias {
		p := vendas[dia]
		p.Dia = dia
		pontos = append(pontos, p)
	}
	return pontos
}
