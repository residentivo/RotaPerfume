package analytics

import "github.com/rotaperfumes/shared/kernel"

// Janela é o recorte de GET /api/dashboard/funil (DASH-01) por data de
// abertura da oportunidade: um enum TIPADO antes de qualquer SQL, como o
// Periodo (D-02). Não reusa Periodo de propósito: os dados de oportunidades
// vão até 2026-08-31 e today/week/month ficariam vazios; um enum próprio
// também não muda o contrato dos endpoints existentes.
type Janela uint8

const (
	// JanelaTudo é sem recorte de data. É o zero value e o default (query
	// sem ?janela=).
	JanelaTudo Janela = iota
	// Janela30d são os últimos 30 dias, incluindo hoje.
	Janela30d
	// Janela90d são os últimos 90 dias, incluindo hoje.
	Janela90d
	// Janela365d são os últimos 365 dias, incluindo hoje.
	Janela365d
)

// ErrJanelaInvalida é o 400 de ?janela= fora do enum (mensagem do contrato).
var ErrJanelaInvalida = kernel.NovoErro(kernel.KindInvalido, "janela deve ser '30d', '90d', '365d' ou 'all'")

// ParseJanela lê o ?janela= do contrato: vazio ou "all" → tudo; "30d",
// "90d" e "365d" exatos (maiúsculas são inválidas); qualquer outro valor →
// ErrJanelaInvalida (com JanelaTudo, o zero value).
func ParseJanela(s string) (Janela, error) {
	switch s {
	case "", "all":
		return JanelaTudo, nil
	case "30d":
		return Janela30d, nil
	case "90d":
		return Janela90d, nil
	case "365d":
		return Janela365d, nil
	default:
		return JanelaTudo, ErrJanelaInvalida
	}
}

// String devolve o valor do contrato ("30d", "90d", "365d" ou "all"),
// ecoado no campo "janela" da resposta. Valor fora do enum cai em "all", o
// mesmo default da cláusula SQL.
func (j Janela) String() string {
	switch j {
	case Janela30d:
		return "30d"
	case Janela90d:
		return "90d"
	case Janela365d:
		return "365d"
	default:
		return "all"
	}
}
