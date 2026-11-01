package analytics_test

// DASH-01 (domínio): enum Janela e o read model Funil (cumulativo, perdidas
// fora do funil, taxa de ganho e conversões arredondadas).

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/kernel"
)

func TestDASH01_ParseJanela(t *testing.T) {
	casos := []struct {
		entrada string
		want    analytics.Janela
		texto   string
		erro    bool
	}{
		{"", analytics.JanelaTudo, "all", false},
		{"all", analytics.JanelaTudo, "all", false},
		{"30d", analytics.Janela30d, "30d", false},
		{"90d", analytics.Janela90d, "90d", false},
		{"365d", analytics.Janela365d, "365d", false},
		{"ALL", analytics.JanelaTudo, "all", true},
		{"30D", analytics.JanelaTudo, "all", true},
		{"90D", analytics.JanelaTudo, "all", true},
		{"365D", analytics.JanelaTudo, "all", true},
		{"30", analytics.JanelaTudo, "all", true},
		{"7d", analytics.JanelaTudo, "all", true},
		{" 30d", analytics.JanelaTudo, "all", true},
		{"30d ", analytics.JanelaTudo, "all", true},
		{"month", analytics.JanelaTudo, "all", true},
		{"30d;DROP TABLE oportunidades", analytics.JanelaTudo, "all", true},
		{"lixo", analytics.JanelaTudo, "all", true},
	}
	for _, c := range casos {
		t.Run(c.entrada, func(t *testing.T) {
			j, err := analytics.ParseJanela(c.entrada)
			assert.Equal(t, c.want, j)
			assert.Equal(t, c.texto, j.String())
			if !c.erro {
				assert.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, analytics.ErrJanelaInvalida)
			assert.Equal(t, "janela deve ser '30d', '90d', '365d' ou 'all'", err.Error())
			var ke *kernel.Erro
			require.ErrorAs(t, err, &ke)
			assert.Equal(t, kernel.KindInvalido, ke.Kind(), "inválido → 400 no mapper")
		})
	}
}

func TestDASH01_JanelaString(t *testing.T) {
	casos := map[analytics.Janela]string{
		analytics.JanelaTudo: "all", analytics.Janela30d: "30d", analytics.Janela90d: "90d", analytics.Janela365d: "365d",
		analytics.Janela(4): "all", analytics.Janela(99): "all", analytics.Janela(255): "all",
	}
	for j, want := range casos {
		assert.Equal(t, want, j.String(), "Janela(%d)", uint8(j))
	}
	assert.Equal(t, analytics.JanelaTudo, analytics.Janela(0), "zero value é o default all")
}

// etapas monta as 5 etapas esperadas na ordem do funil a partir de
// (quantidade, valor, conv. anterior, conv. topo).
func etapas(v [5][4]float64) []analytics.EtapaFunil {
	nomes := []string{analytics.EtapaProspeccao, analytics.EtapaQualificacao, analytics.EtapaProposta, analytics.EtapaNegociacao, analytics.EtapaGanho}
	out := make([]analytics.EtapaFunil, 5)
	for i, n := range nomes {
		out[i] = analytics.EtapaFunil{Etapa: n, Quantidade: int(v[i][0]), ValorEstimado: v[i][1], ConversaoAnteriorPct: v[i][2], ConversaoTopoPct: v[i][3]}
	}
	return out
}

func TestDASH01_NovoFunil(t *testing.T) {
	zeros := etapas([5][4]float64{})
	casos := []struct {
		nome      string
		janela    analytics.Janela
		contagens []analytics.ContagemEtapa
		want      analytics.Funil
	}{
		{
			nome: "nil: 5 etapas zeradas, taxa 0 (sem fechadas)", janela: analytics.Janela30d,
			want: analytics.Funil{Janela: analytics.Janela30d, Etapas: zeros},
		},
		{
			nome: "vazio (não nil) idem", janela: analytics.JanelaTudo, contagens: []analytics.ContagemEtapa{},
			want: analytics.Funil{Etapas: zeros},
		},
		{
			nome: "cumulativo completo com perdidas e etapa desconhecida", janela: analytics.Janela90d,
			contagens: []analytics.ContagemEtapa{
				{Etapa: analytics.EtapaGanho, Quantidade: 4, Valor: 40},
				{Etapa: analytics.EtapaProspeccao, Quantidade: 10, Valor: 100},
				{Etapa: "Desconhecida", Quantidade: 7, Valor: 70},
				{Etapa: analytics.EtapaQualificacao, Quantidade: 5, Valor: 50},
				{Etapa: analytics.EtapaPerdido, Quantidade: 6, Valor: 60},
				{Etapa: analytics.EtapaProposta, Quantidade: 3, Valor: 30},
				{Etapa: analytics.EtapaNegociacao, Quantidade: 2, Valor: 20},
			},
			want: analytics.Funil{
				Janela: analytics.Janela90d, TotalOportunidades: 37,
				Etapas: etapas([5][4]float64{
					{24, 240, 100, 100},
					{14, 140, 58.33, 58.33}, // 14/24
					{9, 90, 64.29, 37.5},    // 9/14, 9/24
					{6, 60, 66.67, 25},      // 6/9, 6/24
					{4, 40, 66.67, 16.67},   // 4/6, 4/24
				}),
				Perdidas:     analytics.TotalEtapa{Quantidade: 6, ValorEstimado: 60},
				TaxaGanhoPct: 40, // 4/(4+6)
			},
		},
		{
			nome: "perdidas e desconhecidas só no total; anterior 0 → conversão 0", janela: analytics.Janela365d,
			contagens: []analytics.ContagemEtapa{
				{Etapa: analytics.EtapaPerdido, Quantidade: 3, Valor: 30.556},
				{Etapa: "prospecção", Quantidade: 2, Valor: 1}, // caixa diferente: comparação exata
				{Etapa: "", Quantidade: 1, Valor: 1},
			},
			want: analytics.Funil{
				Janela: analytics.Janela365d, TotalOportunidades: 6, Etapas: zeros,
				Perdidas:     analytics.TotalEtapa{Quantidade: 3, ValorEstimado: 30.56},
				TaxaGanhoPct: 0, // 0/(0+3)
			},
		},
		{
			nome: "etapas duplicadas somadas e valores arredondados",
			contagens: []analytics.ContagemEtapa{
				{Etapa: analytics.EtapaProspeccao, Quantidade: 1, Valor: 1.111},
				{Etapa: analytics.EtapaProspeccao, Quantidade: 2, Valor: 2.222},
				{Etapa: analytics.EtapaGanho, Quantidade: 1, Valor: 10.004},
				{Etapa: analytics.EtapaGanho, Quantidade: 1, Valor: 0.002},
			},
			want: analytics.Funil{
				TotalOportunidades: 5,
				Etapas: etapas([5][4]float64{
					{5, 13.34, 100, 100}, // 3.333 + 10.006 = 13.339
					{2, 10.01, 40, 40},
					{2, 10.01, 100, 40},
					{2, 10.01, 100, 40},
					{2, 10.01, 100, 40},
				}),
				TaxaGanhoPct: 100, // ganhas sem perdidas
			},
		},
		{
			nome:      "só topo: conversões seguintes 0 e taxa de ganho com denominador 0",
			contagens: []analytics.ContagemEtapa{{Etapa: analytics.EtapaProspeccao, Quantidade: 5, Valor: 9.999}},
			want: analytics.Funil{
				TotalOportunidades: 5,
				Etapas:             etapas([5][4]float64{{5, 10, 100, 100}}),
			},
		},
		{
			nome: "arredondamento de terços (33.33 / 66.67)",
			contagens: []analytics.ContagemEtapa{
				{Etapa: analytics.EtapaProspeccao, Quantidade: 1},
				{Etapa: analytics.EtapaProposta, Quantidade: 1},
				{Etapa: analytics.EtapaGanho, Quantidade: 1},
				{Etapa: analytics.EtapaPerdido, Quantidade: 2},
			},
			want: analytics.Funil{
				TotalOportunidades: 5,
				Etapas: etapas([5][4]float64{
					{3, 0, 100, 100},
					{2, 0, 66.67, 66.67},
					{2, 0, 100, 66.67},
					{1, 0, 50, 33.33},
					{1, 0, 100, 33.33},
				}),
				Perdidas:     analytics.TotalEtapa{Quantidade: 2},
				TaxaGanhoPct: 33.33, // 1/3
			},
		},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			f := analytics.NovoFunil(c.janela, c.contagens)
			assert.Equal(t, c.want, f)
			assert.NotNil(t, f.Etapas)
			assert.False(t, f.VendedorDesligado, "quem tem dados nunca está desligado")
		})
	}
}

// O funil é monotônico: cada etapa ≤ a anterior (cumulativo do fundo).
func TestDASH01_NovoFunil_Monotonico(t *testing.T) {
	f := analytics.NovoFunil(analytics.JanelaTudo, []analytics.ContagemEtapa{
		{Etapa: analytics.EtapaNegociacao, Quantidade: 50}, {Etapa: analytics.EtapaProspeccao, Quantidade: 1},
		{Etapa: analytics.EtapaGanho, Quantidade: 7}, {Etapa: analytics.EtapaQualificacao, Quantidade: 3},
	})
	for i := 1; i < len(f.Etapas); i++ {
		assert.LessOrEqual(t, f.Etapas[i].Quantidade, f.Etapas[i-1].Quantidade)
		assert.LessOrEqual(t, f.Etapas[i].ConversaoAnteriorPct, 100.0)
	}
}

func TestDASH01_FunilZerado(t *testing.T) {
	for _, j := range []analytics.Janela{analytics.JanelaTudo, analytics.Janela30d, analytics.Janela90d, analytics.Janela365d} {
		for _, desligado := range []bool{false, true} {
			f := analytics.FunilZerado(j, desligado)
			require.Len(t, f.Etapas, 5)
			assert.Equal(t, analytics.Funil{Janela: j, Etapas: etapas([5][4]float64{}), VendedorDesligado: desligado}, f)
			assert.Equal(t, []string{"Prospecção", "Qualificação", "Proposta enviada", "Negociação", "Fechado ganho"},
				[]string{f.Etapas[0].Etapa, f.Etapas[1].Etapa, f.Etapas[2].Etapa, f.Etapas[3].Etapa, f.Etapas[4].Etapa})
		}
	}
}
