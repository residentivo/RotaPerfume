package analytics

// Etapas de oportunidades.etapa (texto do CSV importado, VARCHAR livre; a
// comparação é exata).
const (
	EtapaProspeccao   = "Prospecção"
	EtapaQualificacao = "Qualificação"
	EtapaProposta     = "Proposta enviada"
	EtapaNegociacao   = "Negociação"
	EtapaGanho        = "Fechado ganho"
	EtapaPerdido      = "Fechado perdido"
)

// etapasDoFunil é a ordem fixa do funil (topo → fundo). Fechado perdido
// fica fora: é saída do funil, não etapa.
var etapasDoFunil = [...]string{EtapaProspeccao, EtapaQualificacao, EtapaProposta, EtapaNegociacao, EtapaGanho}

// Funil é o read model de GET /api/dashboard/funil (DASH-01): o funil
// cumulativo das oportunidades do escopo na janela, as perdidas, a taxa de
// ganho e o aviso de vendedor desligado (D-01). Etapas tem sempre as 5
// etapas, na ordem do funil (nunca nil).
type Funil struct {
	Janela             Janela
	TotalOportunidades int
	Etapas             []EtapaFunil
	Perdidas           TotalEtapa
	TaxaGanhoPct       float64
	VendedorDesligado  bool
}

// EtapaFunil é uma etapa do funil cumulativo: as oportunidades cuja etapa
// atual é esta ou alguma adiante (sem as perdidas) e as conversões em
// relação à etapa anterior e ao topo.
type EtapaFunil struct {
	Etapa                string
	Quantidade           int
	ValorEstimado        float64
	ConversaoAnteriorPct float64
	ConversaoTopoPct     float64
}

// TotalEtapa é quantidade e valor estimado somados de um grupo de
// oportunidades.
type TotalEtapa struct {
	Quantidade    int
	ValorEstimado float64
}

// ContagemEtapa é a linha crua lida do banco: oportunidades de uma etapa
// (GROUP BY etapa).
type ContagemEtapa struct {
	Etapa      string
	Quantidade int
	Valor      float64
}

// NovoFunil monta o funil a partir das contagens por etapa:
//   - total: todas as linhas, inclusive perdidas e etapas desconhecidas;
//   - cada etapa soma ela e as seguintes (cumulativo); desconhecidas e
//     Fechado perdido não entram em etapa alguma;
//   - conversão = quantidade / quantidade de referência * 100 (anterior ou
//     topo; o topo é relativo a si mesmo, 100); referência 0 → 0;
//   - taxa de ganho = ganhas / (ganhas + perdidas) * 100; sem fechadas, 0.
//
// Percentuais e valores saem com o arredondamento do pacote (arredondar2).
// Nunca sinaliza desligamento: quem tem dados não está bloqueado.
func NovoFunil(j Janela, contagens []ContagemEtapa) Funil {
	porEtapa, total := somarPorEtapa(contagens)
	f := Funil{
		Janela:             j,
		TotalOportunidades: total,
		Etapas:             etapasCumulativas(porEtapa),
		Perdidas:           arredondarTotal(porEtapa[EtapaPerdido]),
	}
	ganhas, perdidas := porEtapa[EtapaGanho].Quantidade, porEtapa[EtapaPerdido].Quantidade
	f.TaxaGanhoPct = conversao(ganhas, ganhas+perdidas)
	return f
}

// FunilZerado é a resposta de quem não tem acesso (D-01): as 5 etapas com
// zeros, total e perdidas zerados e o aviso de desligamento.
func FunilZerado(j Janela, vendedorDesligado bool) Funil {
	f := NovoFunil(j, nil)
	f.VendedorDesligado = vendedorDesligado
	return f
}

// somarPorEtapa agrupa as contagens pelo texto exato da etapa (linhas
// repetidas somam) e devolve o total geral.
func somarPorEtapa(contagens []ContagemEtapa) (map[string]TotalEtapa, int) {
	porEtapa := make(map[string]TotalEtapa, len(contagens))
	total := 0
	for _, c := range contagens {
		t := porEtapa[c.Etapa]
		t.Quantidade += c.Quantidade
		t.ValorEstimado += c.Valor
		porEtapa[c.Etapa] = t
		total += c.Quantidade
	}
	return porEtapa, total
}

// etapasCumulativas acumula do fundo para o topo e depois calcula as
// conversões do topo para o fundo.
func etapasCumulativas(porEtapa map[string]TotalEtapa) []EtapaFunil {
	etapas := make([]EtapaFunil, len(etapasDoFunil))
	var acumulado TotalEtapa
	for i := len(etapasDoFunil) - 1; i >= 0; i-- {
		t := porEtapa[etapasDoFunil[i]]
		acumulado.Quantidade += t.Quantidade
		acumulado.ValorEstimado += t.ValorEstimado
		etapas[i] = EtapaFunil{Etapa: etapasDoFunil[i], Quantidade: acumulado.Quantidade, ValorEstimado: arredondar2(acumulado.ValorEstimado)}
	}
	topo := etapas[0].Quantidade
	anterior := topo
	for i := range etapas {
		etapas[i].ConversaoAnteriorPct = conversao(etapas[i].Quantidade, anterior)
		etapas[i].ConversaoTopoPct = conversao(etapas[i].Quantidade, topo)
		anterior = etapas[i].Quantidade
	}
	return etapas
}

// conversao é parte/base*100 arredondado; base 0 → 0.
func conversao(parte, base int) float64 {
	if base == 0 {
		return 0
	}
	return arredondar2(float64(parte) / float64(base) * 100)
}

// arredondarTotal arredonda o valor estimado de um total.
func arredondarTotal(t TotalEtapa) TotalEtapa {
	t.ValorEstimado = arredondar2(t.ValorEstimado)
	return t
}
