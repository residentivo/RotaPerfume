package analyticsapp

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqConsultas = "consultas.go"

// limiteTopVendedores é o tamanho do top de vendedores de /metrics.
const limiteTopVendedores = 10

// Deps são as dependências das consultas, todas obrigatórias.
type Deps struct {
	Vendas   IndicadoresVendas
	Clientes IndicadoresClientes
	Funil    IndicadoresFunil
	Escopo   EscopoAnalytics
	// Relogio é a fonte de "hoje" da série zerada.
	Relogio func() time.Time
}

// Consultas agrupa as consultas do Dashboard (uma por endpoint). Todas
// recebem o Principal e o passam ao autorizador: Principal inválido (rota
// sem usuário) é SemAcesso, o escopo mais restritivo, sem consulta
// (contrato legado; registro ARQ-04, divergência b).
type Consultas struct{ d Deps }

// Novo monta as consultas. Dependência nil é erro de montagem: panic.
// Ressalva N-7 (ARQ-07, só documentada): uma interface não nula guardando
// um ponteiro nil tipado passa por esta checagem, como no legado; a proteção
// é o bootstrap montar sempre os adapters reais (analyticsmysql/comercialapp).
func Novo(d Deps) *Consultas {
	for _, dep := range []any{d.Vendas, d.Clientes, d.Funil, d.Escopo} {
		if dep == nil {
			panic("analyticsapp: Novo exige todas as dependências")
		}
	}
	if d.Relogio == nil {
		panic("analyticsapp: Novo exige o relógio")
	}
	return &Consultas{d: d}
}

// Query objects (§5.14): a entrada tipada de cada consulta. Periodo e Dias
// chegam já validados (D-02, D-03); nenhum texto livre chega ao SQL.
type (
	ConsultaMetricas struct {
		Principal kernel.Principal
		Periodo   analytics.Periodo
	}
	ConsultaSerieVendas struct {
		Principal kernel.Principal
		Dias      analytics.Dias
	}
	ConsultaRanking struct {
		Principal kernel.Principal
		Pagina    kernel.Pagina
	}
	ConsultaMetricasClientes struct {
		Principal kernel.Principal
		Periodo   analytics.Periodo
	}
	ConsultaFunil struct {
		Principal kernel.Principal
		Janela    analytics.Janela
	}
)

// Metricas responde /metrics: os números do escopo, ou tudo zerado com o
// aviso vendedor_desligado para quem não tem acesso (D-01).
func (c *Consultas) Metricas(ctx context.Context, q ConsultaMetricas) (m analytics.Metricas, err error) {
	defer vlog.Trace(arqConsultas, "Analytics.Metricas", "periodo=%s", q.Periodo.String())(&err)
	a, err := c.acessar(ctx, q.Principal, "Metricas")
	if err != nil {
		return m, err
	}
	if !a.liberado {
		return analytics.MetricasZeradas(q.Periodo, a.desligado), nil
	}
	return c.metricasDoEscopo(ctx, a.escopo, q.Periodo)
}

// metricasDoEscopo faz as cinco consultas na ordem do legado; a primeira
// falha interrompe (sem resposta parcial).
func (c *Consultas) metricasDoEscopo(ctx context.Context, e kernel.Escopo, p analytics.Periodo) (analytics.Metricas, error) {
	v := c.d.Vendas
	totais, err := v.VendasTotais(ctx, e, p)
	if err != nil {
		return analytics.Metricas{}, err
	}
	pedidos, err := v.TotalPedidos(ctx, e, p)
	if err != nil {
		return analytics.Metricas{}, err
	}
	top, err := v.TopVendedores(ctx, e, limiteTopVendedores)
	if err != nil {
		return analytics.Metricas{}, err
	}
	metas, err := v.MetasVendedores(ctx, e)
	if err != nil {
		return analytics.Metricas{}, err
	}
	metaMes, err := v.MetaMensalTotal(ctx, e)
	if err != nil {
		return analytics.Metricas{}, err
	}
	return analytics.NovasMetricas(p, totais, pedidos, top, metas, metaMes), nil
}

// SerieVendas responde /vendas: um ponto por dia da janela; sem acesso,
// todos os dias zerados (sem SQL).
func (c *Consultas) SerieVendas(ctx context.Context, q ConsultaSerieVendas) (s analytics.SerieVendas, err error) {
	defer vlog.Trace(arqConsultas, "Analytics.SerieVendas", "dias=%d", q.Dias.Int())(&err)
	a, err := c.acessar(ctx, q.Principal, "SerieVendas")
	if err != nil {
		return s, err
	}
	s.Dias = q.Dias
	if !a.liberado {
		s.Pontos = analytics.SerieZerada(q.Dias.Int(), c.d.Relogio())
		return s, nil
	}
	s.Pontos, err = c.d.Vendas.SerieVendas(ctx, a.escopo, q.Dias)
	return s, err
}

// Ranking responde /vendedores (paginado); sem acesso, lista vazia com
// total 0 (sem SQL). O restrito vê só a própria linha.
func (c *Consultas) Ranking(ctx context.Context, q ConsultaRanking) (r kernel.Resultado[analytics.LinhaRanking], err error) {
	defer vlog.Trace(arqConsultas, "Analytics.Ranking", "page=%d limit=%d", q.Pagina.Numero(), q.Pagina.Limite())(&err)
	a, err := c.acessar(ctx, q.Principal, "Ranking")
	if err != nil {
		return r, err
	}
	if !a.liberado {
		return kernel.NovoResultado(make([]analytics.LinhaRanking, 0), q.Pagina, 0), nil
	}
	return c.d.Vendas.Ranking(ctx, a.escopo, q.Pagina)
}

// MetricasClientes responde /clientes: as contagens da carteira do escopo;
// sem acesso, tudo zerado e listas vazias (sem SQL).
func (c *Consultas) MetricasClientes(ctx context.Context, q ConsultaMetricasClientes) (m analytics.MetricasClientes, err error) {
	defer vlog.Trace(arqConsultas, "Analytics.MetricasClientes", "periodo=%s", q.Periodo.String())(&err)
	a, err := c.acessar(ctx, q.Principal, "MetricasClientes")
	if err != nil {
		return m, err
	}
	if !a.liberado {
		return analytics.MetricasClientesZeradas(q.Periodo), nil
	}
	return c.clientesDoEscopo(ctx, a.escopo, q.Periodo)
}

// clientesDoEscopo faz as seis contagens na ordem do legado; a primeira
// falha interrompe (sem resposta parcial).
func (c *Consultas) clientesDoEscopo(ctx context.Context, e kernel.Escopo, p analytics.Periodo) (analytics.MetricasClientes, error) {
	cl := c.d.Clientes
	m := analytics.MetricasClientes{Periodo: p}
	var err error
	if m.TotalClientes, err = cl.Total(ctx, e); err != nil {
		return analytics.MetricasClientes{}, err
	}
	if m.TotalAtivos, err = cl.PorAtivo(ctx, e, true); err != nil {
		return analytics.MetricasClientes{}, err
	}
	if m.TotalInativos, err = cl.PorAtivo(ctx, e, false); err != nil {
		return analytics.MetricasClientes{}, err
	}
	if m.NovosNoPeriodo, err = cl.NovosNoPeriodo(ctx, e, p); err != nil {
		return analytics.MetricasClientes{}, err
	}
	if m.PorSegmento, err = cl.PorSegmento(ctx, e); err != nil {
		return analytics.MetricasClientes{}, err
	}
	if m.PorUF, err = cl.PorUF(ctx, e); err != nil {
		return analytics.MetricasClientes{}, err
	}
	return analytics.NovasMetricasClientes(m), nil
}

// Funil responde /funil (DASH-01): o funil cumulativo do escopo na janela;
// sem acesso, as 5 etapas zeradas com o aviso vendedor_desligado (D-01,
// sem SQL).
func (c *Consultas) Funil(ctx context.Context, q ConsultaFunil) (f analytics.Funil, err error) {
	defer vlog.Trace(arqConsultas, "Analytics.Funil", "janela=%s", q.Janela.String())(&err)
	a, err := c.acessar(ctx, q.Principal, "Funil")
	if err != nil {
		return f, err
	}
	if !a.liberado {
		return analytics.FunilZerado(q.Janela, a.desligado), nil
	}
	contagens, err := c.d.Funil.Funil(ctx, a.escopo, q.Janela)
	if err != nil {
		return f, err
	}
	return analytics.NovoFunil(q.Janela, contagens), nil
}

// acesso é o escopo resolvido de uma consulta. liberado=false responde
// zerado ANTES de qualquer repositório (D-01).
type acesso struct {
	escopo    kernel.Escopo
	liberado  bool
	desligado bool
}

// acessar resolve o escopo pelo autorizador e decide SÓ pelo Filtro() (O1):
//   - desligado → sem acesso, com o aviso (D-01: nunca "zera o vendedor e
//     segue");
//   - SemAcesso (sem vínculo, órfão, Principal inválido) → sem acesso;
//   - zero value/inválido → sem acesso (fail-closed sem dado algum) e log
//     operacional [escopo] invalido com tag fixa (R-e);
//   - Irrestrito ou Vendedor → liberado com o escopo;
//   - erro do autorizador → erro (500), nunca "sem filtro" (R3).
func (c *Consultas) acessar(ctx context.Context, p kernel.Principal, consulta string) (acesso, error) {
	e, desligado, err := c.d.Escopo.ResolverParaAnalytics(ctx, p)
	if err != nil {
		return acesso{}, err
	}
	vendedorID, filtra, ferr := e.Filtro()
	switch {
	case desligado:
		vlog.Printf(arqConsultas, "Analytics."+consulta, "decisão sem_acesso motivo=vendedor_desligado")
		return acesso{desligado: true}, nil
	case errors.Is(ferr, kernel.ErrEscopoSemAcesso):
		vlog.Printf(arqConsultas, "Analytics."+consulta, "decisão sem_acesso motivo=sem_vendedor")
		return acesso{}, nil
	case ferr != nil:
		log.Printf("[escopo] invalido [dashboard] %s", consulta)
		return acesso{}, nil
	}
	vlog.Printf(arqConsultas, "Analytics."+consulta, "decisão escopo restrito=%t vendedor_id=%d", filtra, vendedorID)
	return acesso{escopo: e, liberado: true}, nil
}
