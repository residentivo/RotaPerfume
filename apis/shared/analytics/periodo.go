// Package analytics é o domínio do contexto Analytics (ARQ-06): o lado de
// CONSULTA (CQRS leve, §5.14) do Dashboard. Não há agregados nem escrita:
// só os read models tipados (no lugar do map[string]any legado), os VOs de
// entrada das consultas (Periodo e Dias) e as regras puras de montagem
// (ticket médio, percentual de meta, série completa, arredondamento).
//
// O pacote é puro (docs/arquitetura-ddd.md §4.1): importa só a stdlib e o
// kernel. Não loga, não conhece SQL nem HTTP e não tem tags JSON (o DTO do
// analyticshttp serializa com as chaves do contrato).
package analytics

import "github.com/rotaperfumes/shared/kernel"

// Periodo é a janela de /metrics e /clientes (D-02): um enum TIPADO antes
// de qualquer SQL. O adapter só traduz Periodo em cláusula por switch sobre
// este enum, devolvendo constantes; o texto da query string nunca chega ao
// SQL.
type Periodo uint8

const (
	// PeriodoMes é o mês atual. É o zero value e o default (query sem
	// ?periodo=), como no legado.
	PeriodoMes Periodo = iota
	// PeriodoHoje é o dia atual.
	PeriodoHoje
	// PeriodoSemana são os últimos 7 dias, incluindo hoje.
	PeriodoSemana
)

// ErrPeriodoInvalido é o 400 de ?periodo= fora do enum (mensagem do contrato).
var ErrPeriodoInvalido = kernel.NovoErro(kernel.KindInvalido, "periodo deve ser 'today', 'week' ou 'month'")

// ParsePeriodo lê o ?periodo= do contrato: vazio → mês; "today", "week" e
// "month" exatos (maiúsculas são inválidas, como no legado); qualquer outro
// valor → ErrPeriodoInvalido (com PeriodoMes, o zero value).
func ParsePeriodo(s string) (Periodo, error) {
	switch s {
	case "", "month":
		return PeriodoMes, nil
	case "today":
		return PeriodoHoje, nil
	case "week":
		return PeriodoSemana, nil
	default:
		return PeriodoMes, ErrPeriodoInvalido
	}
}

// String devolve o valor do contrato ("today", "week" ou "month"), ecoado no
// campo "periodo" das respostas. Valor fora do enum cai em "month", o mesmo
// default da cláusula SQL.
func (p Periodo) String() string {
	switch p {
	case PeriodoHoje:
		return "today"
	case PeriodoSemana:
		return "week"
	default:
		return "month"
	}
}
