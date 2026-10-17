// Package analytics_test testa o contexto Analytics (ARQ-06): domínio,
// consultas (com fakes de porta) e adapters MySQL (com sqlmock).
package analytics_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/analytics"
)

func TestAnalytics_ParsePeriodo(t *testing.T) {
	casos := []struct {
		entrada string
		want    analytics.Periodo
		texto   string
		erro    bool
	}{
		{"", analytics.PeriodoMes, "month", false},
		{"month", analytics.PeriodoMes, "month", false},
		{"today", analytics.PeriodoHoje, "today", false},
		{"week", analytics.PeriodoSemana, "week", false},
		{"MONTH", analytics.PeriodoMes, "month", true},
		{"ano", analytics.PeriodoMes, "month", true},
		{"today ", analytics.PeriodoMes, "month", true},
	}
	for _, c := range casos {
		p, err := analytics.ParsePeriodo(c.entrada)
		assert.Equal(t, c.want, p, c.entrada)
		assert.Equal(t, c.texto, p.String(), c.entrada)
		if c.erro {
			assert.ErrorIs(t, err, analytics.ErrPeriodoInvalido, c.entrada)
			assert.Equal(t, "periodo deve ser 'today', 'week' ou 'month'", err.Error())
		} else {
			assert.NoError(t, err, c.entrada)
		}
	}
	assert.Equal(t, "month", analytics.Periodo(99).String(), "fora do enum cai no mês, como a cláusula SQL")
}

// D-03: a janela é sempre 1..365; o resto (vazio, sinal, espaço, dígito não
// ASCII, 0, > 365) é 30, como o parseIntDefault legado. O acumulador sem
// checagem de overflow é o do legado (documentado, não corrigido).
func TestAnalytics_Dias(t *testing.T) {
	casos := map[string]int{
		"": 30, "0": 30, "1": 1, "7": 7, "365": 365, "366": 30, "999999": 30,
		"abc": 30, "-5": 30, "+5": 30, " 7": 30, "7 ": 30, "7.0": 30, "１": 30,
		"18446744073709551623": 7, // 2^64 + 7: o int do legado dá a volta
	}
	for entrada, want := range casos {
		assert.Equal(t, want, analytics.ParseDias(entrada).Int(), "%q", entrada)
	}
	assert.Equal(t, analytics.DiasPadrao, analytics.Dias{}.Int(), "zero value = 30, nunca 0 no SQL")
	assert.Equal(t, 30, analytics.DiasDe(-1).Int())
	assert.Equal(t, 30, analytics.DiasDe(analytics.DiasMaximo+1).Int())
	assert.Equal(t, analytics.DiasMaximo, analytics.DiasDe(analytics.DiasMaximo).Int())
}

func TestAnalytics_Series(t *testing.T) {
	hoje := time.Date(2026, 3, 1, 15, 0, 0, 0, time.UTC)
	pontos := analytics.SerieZerada(3, hoje)
	assert.Equal(t, []analytics.PontoVenda{{Dia: "2026-02-27"}, {Dia: "2026-02-28"}, {Dia: "2026-03-01"}}, pontos)
	for _, n := range []int{0, -2} {
		vazia := analytics.SerieZerada(n, hoje)
		require.NotNil(t, vazia, "lista vazia, nunca nil")
		assert.Empty(t, vazia)
	}

	completa := analytics.SerieCompleta([]string{"2026-02-28", "2026-03-01"},
		map[string]analytics.PontoVenda{"2026-03-01": {Dia: "2026-03-01", TotalVendas: 9.5, TotalPedidos: 2}, "2026-01-01": {TotalVendas: 1}})
	assert.Equal(t, []analytics.PontoVenda{{Dia: "2026-02-28"}, {Dia: "2026-03-01", TotalVendas: 9.5, TotalPedidos: 2}}, completa)
	require.NotNil(t, analytics.SerieCompleta(nil, nil))
}

func TestAnalytics_Metricas(t *testing.T) {
	top := []analytics.VendedorRanking{{ID: 1, Meta: 200}, {ID: 2, Meta: 0, PercentualMeta: 0}}
	top[0].AplicarVendas(analytics.VendasDoMes{Total: 99.99, Quantidade: 1})
	top[1].AplicarVendas(analytics.VendasDoMes{Total: 50, Quantidade: 3})
	assert.InDelta(t, 49.995, top[0].PercentualMeta, 1e-9)
	assert.Equal(t, analytics.VendedorRanking{ID: 2, TotalVendas: 50, QuantidadeVendas: 3}, top[1], "sem meta: percentual 0")

	metas := []analytics.MetaVendedor{{ID: 1, Meta: 0}, {ID: 2, Meta: 1000}}
	metas[0].AplicarVendas(analytics.VendasDoMes{Total: 10, Quantidade: 1})
	metas[1].AplicarVendas(analytics.VendasDoMes{Total: 250, Quantidade: 2})
	assert.Zero(t, metas[0].Percentual)
	assert.Equal(t, 25.0, metas[1].Percentual)

	m := analytics.NovasMetricas(analytics.PeriodoSemana, analytics.TotaisVendas{Valor: 100, Quantidade: 3}, 4, nil, nil, 500)
	assert.InDelta(t, 33.333333, m.TicketMedio, 1e-6)
	assert.NotNil(t, m.TopVendedores)
	assert.NotNil(t, m.MetasVendedores)
	assert.False(t, m.VendedorDesligado)
	assert.Zero(t, analytics.NovasMetricas(analytics.PeriodoMes, analytics.TotaisVendas{Valor: 10}, 0, nil, nil, 0).TicketMedio)

	z := analytics.MetricasZeradas(analytics.PeriodoHoje, true)
	assert.True(t, z.VendedorDesligado)
	assert.Equal(t, analytics.PeriodoHoje, z.Periodo)
	assert.Equal(t, []analytics.VendedorRanking{}, z.TopVendedores)
	assert.Equal(t, []analytics.MetaVendedor{}, z.MetasVendedores)

	c := analytics.MetricasClientesZeradas(analytics.PeriodoSemana)
	assert.Equal(t, analytics.MetricasClientes{Periodo: analytics.PeriodoSemana, PorSegmento: []analytics.ContagemSegmento{}, PorUF: []analytics.ContagemUF{}}, c)
}

// O arredondamento e o ticket são os do legado (mathRound, 2 casas).
func TestAnalytics_LinhaRanking(t *testing.T) {
	l := analytics.NovaLinhaRanking(analytics.DadosRanking{VendedorID: 7, VendedorNome: "A", Regiao: "Sul", UF: "PR",
		Meta: 1000, TotalVendas: 333.333, TotalPedidos: 3, Atingimento: 33.3333})
	assert.Equal(t, analytics.LinhaRanking{VendedorID: 7, VendedorNome: "A", Regiao: "Sul", UF: "PR", TotalVendas: 333.333,
		TotalPedidos: 3, TicketMedio: 111.11, Meta: 1000, AtingimentoMeta: 33.33}, l)

	semPedidos := analytics.NovaLinhaRanking(analytics.DadosRanking{TotalVendas: 50, Atingimento: 12.345})
	assert.Zero(t, semPedidos.TicketMedio)
	assert.Equal(t, 12.35, semPedidos.AtingimentoMeta)
	assert.Equal(t, -0.0, analytics.NovaLinhaRanking(analytics.DadosRanking{Atingimento: -0.004}).AtingimentoMeta)
}
