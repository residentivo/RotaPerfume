package analytics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/analytics/analyticsapp"
	"github.com/rotaperfumes/shared/kernel"
)

var errInfra = errors.New("infra")

// vendasFake falha na chamada de número `falhaEm` (1-based; 0 = nunca) e
// conta as chamadas (o caso de uso não pode chegar aqui sem acesso).
type vendasFake struct {
	falhaEm, chamadas int
}

func (f *vendasFake) passo() error {
	f.chamadas++
	if f.chamadas == f.falhaEm {
		return errInfra
	}
	return nil
}

func (f *vendasFake) VendasTotais(context.Context, kernel.Escopo, analytics.Periodo) (analytics.TotaisVendas, error) {
	return analytics.TotaisVendas{Valor: 90, Quantidade: 3}, f.passo()
}
func (f *vendasFake) TotalPedidos(context.Context, kernel.Escopo, analytics.Periodo) (int, error) {
	return 3, f.passo()
}
func (f *vendasFake) TopVendedores(_ context.Context, _ kernel.Escopo, limite int) ([]analytics.VendedorRanking, error) {
	return []analytics.VendedorRanking{{ID: int64(limite)}}, f.passo()
}
func (f *vendasFake) MetasVendedores(context.Context, kernel.Escopo) ([]analytics.MetaVendedor, error) {
	return nil, f.passo()
}
func (f *vendasFake) MetaMensalTotal(context.Context, kernel.Escopo) (float64, error) {
	return 1000, f.passo()
}
func (f *vendasFake) SerieVendas(_ context.Context, _ kernel.Escopo, d analytics.Dias) ([]analytics.PontoVenda, error) {
	return []analytics.PontoVenda{{Dia: "x", TotalPedidos: d.Int()}}, f.passo()
}
func (f *vendasFake) Ranking(_ context.Context, _ kernel.Escopo, p kernel.Pagina) (kernel.Resultado[analytics.LinhaRanking], error) {
	return kernel.NovoResultado([]analytics.LinhaRanking{{VendedorID: 1}}, p, 1), f.passo()
}

// Funil faz o vendasFake satisfazer também IndicadoresFunil (DASH-01).
func (f *vendasFake) Funil(context.Context, kernel.Escopo, analytics.Janela) ([]analytics.ContagemEtapa, error) {
	return nil, f.passo()
}

type clientesFake struct{ vendasFake }

func (f *clientesFake) Total(context.Context, kernel.Escopo) (int, error) { return 10, f.passo() }
func (f *clientesFake) PorAtivo(_ context.Context, _ kernel.Escopo, ativo bool) (int, error) {
	if ativo {
		return 8, f.passo()
	}
	return 2, f.passo()
}
func (f *clientesFake) NovosNoPeriodo(context.Context, kernel.Escopo, analytics.Periodo) (int, error) {
	return 1, f.passo()
}
func (f *clientesFake) PorSegmento(context.Context, kernel.Escopo) ([]analytics.ContagemSegmento, error) {
	return nil, f.passo()
}
func (f *clientesFake) PorUF(context.Context, kernel.Escopo) ([]analytics.ContagemUF, error) {
	return []analytics.ContagemUF{{UF: "PR", Total: 10}}, f.passo()
}

type escopoFake struct {
	e         kernel.Escopo
	desligado bool
	err       error
}

func (f escopoFake) ResolverParaAnalytics(context.Context, kernel.Principal) (kernel.Escopo, bool, error) {
	return f.e, f.desligado, f.err
}

var hojeFixo = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)

func novasConsultas(v *vendasFake, c *clientesFake, e escopoFake) *analyticsapp.Consultas {
	return analyticsapp.Novo(analyticsapp.Deps{Vendas: v, Clientes: c, Funil: v, Escopo: e, Relogio: func() time.Time { return hojeFixo }})
}

func irrestrito(t *testing.T) kernel.Escopo {
	t.Helper()
	admin, err := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	require.NoError(t, err)
	e, err := kernel.EscopoIrrestrito(admin)
	require.NoError(t, err)
	return e
}

func TestAnalyticsapp_Novo_ExigeDependencias(t *testing.T) {
	v, c, e := &vendasFake{}, &clientesFake{}, escopoFake{}
	relogio := func() time.Time { return hojeFixo }
	for nome, d := range map[string]analyticsapp.Deps{
		"vendas":   {Clientes: c, Funil: v, Escopo: e, Relogio: relogio},
		"clientes": {Vendas: v, Funil: v, Escopo: e, Relogio: relogio},
		"funil":    {Vendas: v, Clientes: c, Escopo: e, Relogio: relogio},
		"escopo":   {Vendas: v, Clientes: c, Funil: v, Relogio: relogio},
		"relógio":  {Vendas: v, Clientes: c, Funil: v, Escopo: e},
	} {
		assert.Panics(t, func() { analyticsapp.Novo(d) }, nome)
	}
}

func TestAnalyticsapp_ComEscopo(t *testing.T) {
	ctx := context.Background()
	v, c := &vendasFake{}, &clientesFake{}
	q := novasConsultas(v, c, escopoFake{e: irrestrito(t)})

	m, err := q.Metricas(ctx, analyticsapp.ConsultaMetricas{Periodo: analytics.PeriodoSemana})
	require.NoError(t, err)
	assert.Equal(t, 30.0, m.TicketMedio)
	assert.Equal(t, int64(10), m.TopVendedores[0].ID, "top 10")
	assert.Equal(t, []analytics.MetaVendedor{}, m.MetasVendedores, "nil vira []")

	s, err := q.SerieVendas(ctx, analyticsapp.ConsultaSerieVendas{Dias: analytics.DiasDe(7)})
	require.NoError(t, err)
	assert.Equal(t, 7, s.Dias.Int())
	assert.Equal(t, 7, s.Pontos[0].TotalPedidos)

	r, err := q.Ranking(ctx, analyticsapp.ConsultaRanking{Pagina: kernel.NovaPagina(2, 5)})
	require.NoError(t, err)
	assert.Equal(t, 1, r.Total)

	mc, err := q.MetricasClientes(ctx, analyticsapp.ConsultaMetricasClientes{Periodo: analytics.PeriodoHoje})
	require.NoError(t, err)
	assert.Equal(t, analytics.MetricasClientes{Periodo: analytics.PeriodoHoje, TotalClientes: 10, TotalAtivos: 8, TotalInativos: 2,
		NovosNoPeriodo: 1, PorSegmento: []analytics.ContagemSegmento{}, PorUF: []analytics.ContagemUF{{UF: "PR", Total: 10}}}, mc)
}

// D-01/O1: sem acesso (SemAcesso, zero value, desligado) responde zerado
// SEM chamar repositório algum.
func TestAnalyticsapp_SemAcesso_ZeradoSemRepositorio(t *testing.T) {
	ctx := context.Background()
	for nome, e := range map[string]escopoFake{
		"sem acesso": {e: kernel.EscopoSemAcesso()},
		"zero value": {e: kernel.Escopo{}},
		"desligado":  {e: kernel.EscopoSemAcesso(), desligado: true},
		// desligado manda mesmo se o escopo vier com vendedor (defesa).
		"desligado com escopo": {e: irrestrito(t), desligado: true},
	} {
		v, c := &vendasFake{}, &clientesFake{}
		q := novasConsultas(v, c, e)
		m, err := q.Metricas(ctx, analyticsapp.ConsultaMetricas{Periodo: analytics.PeriodoHoje})
		require.NoError(t, err, nome)
		assert.Equal(t, analytics.MetricasZeradas(analytics.PeriodoHoje, e.desligado), m, nome)
		s, err := q.SerieVendas(ctx, analyticsapp.ConsultaSerieVendas{Dias: analytics.DiasDe(2)})
		require.NoError(t, err, nome)
		assert.Equal(t, analytics.SerieZerada(2, hojeFixo), s.Pontos, nome)
		r, err := q.Ranking(ctx, analyticsapp.ConsultaRanking{Pagina: kernel.NovaPagina(3, 5)})
		require.NoError(t, err, nome)
		assert.Equal(t, kernel.NovoResultado([]analytics.LinhaRanking{}, kernel.NovaPagina(3, 5), 0), r, nome)
		mc, err := q.MetricasClientes(ctx, analyticsapp.ConsultaMetricasClientes{Periodo: analytics.PeriodoSemana})
		require.NoError(t, err, nome)
		assert.Equal(t, analytics.MetricasClientesZeradas(analytics.PeriodoSemana), mc, nome)
		assert.Zero(t, v.chamadas+c.chamadas, nome)
	}
}

// R3: erro do autorizador é erro (500), nunca "sem filtro".
func TestAnalyticsapp_ErroDoEscopo(t *testing.T) {
	ctx := context.Background()
	v, c := &vendasFake{}, &clientesFake{}
	q := novasConsultas(v, c, escopoFake{err: errInfra})
	_, err := q.Metricas(ctx, analyticsapp.ConsultaMetricas{})
	assert.ErrorIs(t, err, errInfra)
	_, err = q.SerieVendas(ctx, analyticsapp.ConsultaSerieVendas{})
	assert.ErrorIs(t, err, errInfra)
	_, err = q.Ranking(ctx, analyticsapp.ConsultaRanking{})
	assert.ErrorIs(t, err, errInfra)
	_, err = q.MetricasClientes(ctx, analyticsapp.ConsultaMetricasClientes{})
	assert.ErrorIs(t, err, errInfra)
	assert.Zero(t, v.chamadas+c.chamadas)
}

// A primeira falha interrompe (sem resposta parcial), em cada etapa.
func TestAnalyticsapp_ErroEmCadaEtapa(t *testing.T) {
	ctx := context.Background()
	for etapa := 1; etapa <= 5; etapa++ {
		v := &vendasFake{falhaEm: etapa}
		m, err := novasConsultas(v, &clientesFake{}, escopoFake{e: irrestrito(t)}).Metricas(ctx, analyticsapp.ConsultaMetricas{})
		assert.ErrorIs(t, err, errInfra, "métricas etapa %d", etapa)
		assert.Equal(t, analytics.Metricas{}, m)
		assert.Equal(t, etapa, v.chamadas)
	}
	for etapa := 1; etapa <= 6; etapa++ {
		c := &clientesFake{vendasFake{falhaEm: etapa}}
		m, err := novasConsultas(&vendasFake{}, c, escopoFake{e: irrestrito(t)}).MetricasClientes(ctx, analyticsapp.ConsultaMetricasClientes{})
		assert.ErrorIs(t, err, errInfra, "clientes etapa %d", etapa)
		assert.Equal(t, analytics.MetricasClientes{}, m)
		assert.Equal(t, etapa, c.chamadas)
	}
	_, err := novasConsultas(&vendasFake{falhaEm: 1}, &clientesFake{}, escopoFake{e: irrestrito(t)}).SerieVendas(ctx, analyticsapp.ConsultaSerieVendas{})
	assert.ErrorIs(t, err, errInfra)
}
