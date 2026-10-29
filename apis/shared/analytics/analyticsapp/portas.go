// Package analyticsapp tem as consultas do Dashboard (ARQ-06, lado de
// consulta do CQRS leve, §5.14): um Query object por endpoint, todas
// recebendo o kernel.Principal e resolvendo o kernel.Escopo pela porta
// EscopoAnalytics (o AutorizadorEscopo do Comercial a satisfaz, ligada no
// bootstrap; este pacote não importa outro contexto, §3.1).
//
// As portas são declaradas aqui, pelo consumidor; analyticsmysql as
// satisfaz implicitamente. Toda consulta de repositório recebe o Escopo e o
// traduz só por Escopo.Filtro() (O1): não existe "0 = global".
package analyticsapp

import (
	"context"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/kernel"
)

// IndicadoresVendas são as consultas agregadoras de pedidos e vendedores.
// SemAcesso/zero value → erro SEM SQL (O1); o caso de uso nem chega aqui
// nesses casos (D-01).
type IndicadoresVendas interface {
	VendasTotais(ctx context.Context, e kernel.Escopo, p analytics.Periodo) (analytics.TotaisVendas, error)
	TotalPedidos(ctx context.Context, e kernel.Escopo, p analytics.Periodo) (int, error)
	// TopVendedores: os `limite` vendedores ativos de maior meta, com as
	// vendas do mês (lista vazia, nunca nil).
	TopVendedores(ctx context.Context, e kernel.Escopo, limite int) ([]analytics.VendedorRanking, error)
	MetasVendedores(ctx context.Context, e kernel.Escopo) ([]analytics.MetaVendedor, error)
	MetaMensalTotal(ctx context.Context, e kernel.Escopo) (float64, error)
	// SerieVendas: um ponto por dia da janela (nunca nil).
	SerieVendas(ctx context.Context, e kernel.Escopo, d analytics.Dias) ([]analytics.PontoVenda, error)
	// Ranking pagina o ranking de vendedores (Itens nunca nil).
	Ranking(ctx context.Context, e kernel.Escopo, p kernel.Pagina) (kernel.Resultado[analytics.LinhaRanking], error)
}

// IndicadoresClientes são as contagens da base de clientes. O escopo de
// vendedor restringe à carteira ATIVA do vendedor.
type IndicadoresClientes interface {
	Total(ctx context.Context, e kernel.Escopo) (int, error)
	PorAtivo(ctx context.Context, e kernel.Escopo, ativo bool) (int, error)
	NovosNoPeriodo(ctx context.Context, e kernel.Escopo, p analytics.Periodo) (int, error)
	PorSegmento(ctx context.Context, e kernel.Escopo) ([]analytics.ContagemSegmento, error)
	PorUF(ctx context.Context, e kernel.Escopo) ([]analytics.ContagemUF, error)
}

// IndicadoresFunil é a contagem de oportunidades por etapa do funil
// (DASH-01). O escopo de vendedor restringe a oportunidades.vendedor_id
// (o dono da oportunidade, como /api/oportunidades), não à carteira.
// SemAcesso/zero value → erro SEM SQL (O1).
type IndicadoresFunil interface {
	// Funil: uma linha por etapa presente na janela (lista vazia, nunca nil).
	Funil(ctx context.Context, e kernel.Escopo, j analytics.Janela) ([]analytics.ContagemEtapa, error)
}

// EscopoAnalytics é a porta para Comercial (comercialapp.AutorizadorEscopo
// a satisfaz): o escopo do Dashboard resolvido A CADA chamada, sem cache
// (R4). Principal inválido → SemAcesso; admin → Irrestrito; restrito → o
// vendedor do usuário; sem vínculo ou órfão → SemAcesso; desligado →
// SemAcesso com desligado=true (D-01); erro de banco → erro (500).
type EscopoAnalytics interface {
	ResolverParaAnalytics(ctx context.Context, p kernel.Principal) (e kernel.Escopo, desligado bool, err error)
}
