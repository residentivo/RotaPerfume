package analytics_test

// BUG-19 (Lote 25): arredondar2 (não exportada) passa a math.Round
// (metade para longe do zero, simétrico nos negativos), com NaN/±Inf → 0 e
// sem -0 no JSON. Exercitada pela API pública que a usa: o atingimento e o
// ticket médio do ranking (NovaLinhaRanking) e os valores/percentuais do
// funil (NovoFunil).

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/analytics"
)

var bug19Casos = []struct {
	entrada float64
	want    float64
}{
	{1.005, 1.00}, // 1.005 é 1.00499… em float64: arredonda para baixo
	{2.675, 2.68},
	{0.125, 0.13},
	{12.345, 12.35},
	{-1.555, -1.56}, // legado (soma 0,5 e trunca) dava -1.55
	{-0.125, -0.13},
	{-2.675, -2.68},
	{0, 0},
	{-0.004, 0}, // math.Round(-0.4) é -0 → +0
	{-0.001, 0},
	{0.004, 0},
	{0.005, 0.01},
	{33.3333, 33.33},
	{99.995, 100},
	{-99.995, -100},
	{1e6 + 0.125, 1000000.13},
	{math.NaN(), 0},
	{math.Inf(1), 0},
	{math.Inf(-1), 0},
}

func bug19SemMenosZero(t *testing.T, v float64) {
	t.Helper()
	if v == 0 {
		assert.False(t, math.Signbit(v), "zero sempre +0")
	}
	b, err := json.Marshal(v)
	require.NoError(t, err, "NaN/Inf quebrariam o json.Marshal")
	assert.NotEqual(t, "-0", string(b))
}

func TestBUG19_Ranking_AtingimentoArredondado(t *testing.T) {
	for _, c := range bug19Casos {
		t.Run(fmt.Sprintf("%v", c.entrada), func(t *testing.T) {
			l := analytics.NovaLinhaRanking(analytics.DadosRanking{Atingimento: c.entrada})
			assert.Equal(t, c.want, l.AtingimentoMeta)
			bug19SemMenosZero(t, l.AtingimentoMeta)
		})
	}
}

// Ticket médio = total/pedidos, também arredondado (1 pedido → o próprio valor).
func TestBUG19_Ranking_TicketArredondado(t *testing.T) {
	for _, c := range bug19Casos {
		if math.IsNaN(c.entrada) || math.IsInf(c.entrada, 0) {
			continue
		}
		t.Run(fmt.Sprintf("%v", c.entrada), func(t *testing.T) {
			l := analytics.NovaLinhaRanking(analytics.DadosRanking{TotalVendas: c.entrada, TotalPedidos: 1})
			assert.Equal(t, c.want, l.TicketMedio)
			assert.Equal(t, c.entrada, l.TotalVendas, "o total não é arredondado")
			bug19SemMenosZero(t, l.TicketMedio)
		})
	}
	// Divisão que gera dízima: 10/3 = 3.333… → 3.33; 20/3 = 6.666… → 6.67.
	assert.Equal(t, 3.33, analytics.NovaLinhaRanking(analytics.DadosRanking{TotalVendas: 10, TotalPedidos: 3}).TicketMedio)
	assert.Equal(t, 6.67, analytics.NovaLinhaRanking(analytics.DadosRanking{TotalVendas: 20, TotalPedidos: 3}).TicketMedio)
	// Overflow do total (Inf) dividido por pedidos: 0, não Inf no JSON.
	l := analytics.NovaLinhaRanking(analytics.DadosRanking{TotalVendas: math.Inf(1), TotalPedidos: 2})
	assert.Zero(t, l.TicketMedio)
}

// A linha inteira serializa sem erro e sem "-0" mesmo com entradas ruins.
func TestBUG19_Ranking_JSONSemMenosZeroNemNaN(t *testing.T) {
	for _, at := range []float64{-0.004, math.NaN(), math.Inf(-1)} {
		l := analytics.NovaLinhaRanking(analytics.DadosRanking{Atingimento: at, TotalVendas: -0.001, TotalPedidos: 1})
		b, err := json.Marshal(struct{ T, A float64 }{l.TicketMedio, l.AtingimentoMeta})
		require.NoError(t, err)
		assert.JSONEq(t, `{"T":0,"A":0}`, string(b))
		assert.NotContains(t, string(b), "-0")
	}
}

// Funil: valor estimado e percentuais passam pelo mesmo arredondamento.
func TestBUG19_Funil_ValoresEPercentuais(t *testing.T) {
	f := analytics.NovoFunil(analytics.Janela(0), []analytics.ContagemEtapa{
		{Etapa: analytics.EtapaProspeccao, Quantidade: 3, Valor: 2.675},
		{Etapa: analytics.EtapaGanho, Quantidade: 1, Valor: 1.005},
		{Etapa: analytics.EtapaPerdido, Quantidade: 2, Valor: -0.004},
	})
	require.Len(t, f.Etapas, 5)
	assert.Equal(t, 3.68, f.Etapas[0].ValorEstimado, "2.675 + 1.005 = 3.68")
	assert.Equal(t, 1.0, f.Etapas[4].ValorEstimado)
	assert.Equal(t, 25.0, f.Etapas[1].ConversaoTopoPct, "1/4")
	assert.Equal(t, 33.33, f.TaxaGanhoPct, "1/3 = 33.333…")
	assert.Equal(t, 0.0, f.Perdidas.ValorEstimado)
	bug19SemMenosZero(t, f.Perdidas.ValorEstimado)

	// 2/3 = 66.666… → 66.67 (metade para cima, nunca truncado).
	f = analytics.NovoFunil(analytics.Janela(0), []analytics.ContagemEtapa{
		{Etapa: analytics.EtapaGanho, Quantidade: 2}, {Etapa: analytics.EtapaPerdido, Quantidade: 1},
	})
	assert.Equal(t, 66.67, f.TaxaGanhoPct)

	b, err := json.Marshal(f)
	require.NoError(t, err)
	assert.NotContains(t, string(b), ":-0")
}
