package analyticsmysql

import (
	"time"

	"github.com/rotaperfumes/shared/analytics"
	"github.com/rotaperfumes/shared/tz"
)

// BUG-30: as datas das cláusulas de período e de janela são calculadas no
// Go (o "hoje" no fuso fixo -03 do pacote tz) e vão por placeholder; o SQL
// não usa o relógio do servidor (decisão TST-23), que na imagem MariaDB
// roda em UTC e divergia de dia entre 21h e 24h -03.

// layoutDiaSQL é o formato das colunas DATE (data_pedido, data_cadastro,
// data_abertura).
const layoutDiaSQL = "2006-01-02"

// Dias para trás de cada cláusula, contando hoje: semana = últimos 7 dias;
// janelas do funil = últimos 30/90/365 dias.
const (
	diasAtrasSemana = 6
	diasAtras30d    = 29
	diasAtras90d    = 89
	diasAtras365d   = 364
)

// hojeEm devolve a meia-noite de hoje no fuso tz.Local segundo o relógio.
// O .In(tz.Local) é explícito: no CI o time.Local pode ser UTC. O fuso tem
// offset fixo, então AddDate sobre o resultado é seguro.
func hojeEm(relogio func() time.Time) time.Time {
	t := relogio().In(tz.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tz.Local)
}

// diaSQL formata o dia como o literal de uma coluna DATE.
func diaSQL(t time.Time) string {
	return t.Format(layoutDiaSQL)
}

// diasAntes devolve o dia (formato DATE) n dias antes de hoje.
func diasAntes(hoje time.Time, n int) string {
	return diaSQL(hoje.AddDate(0, 0, -n))
}

// argsPeriodo devolve os argumentos dos placeholders de
// clausulaPeriodoPedido/clausulaPeriodoCadastro, com o MESMO switch sobre o
// enum (fora do enum, o mês).
func argsPeriodo(p analytics.Periodo, hoje time.Time) []any {
	switch p {
	case analytics.PeriodoHoje:
		return []any{diaSQL(hoje)}
	case analytics.PeriodoSemana:
		return []any{diasAntes(hoje, diasAtrasSemana)}
	default:
		return argsMes(hoje)
	}
}

// argsMes devolve {ano, mês} para as cláusulas YEAR(x) = ? AND MONTH(x) = ?.
func argsMes(hoje time.Time) []any {
	return []any{hoje.Year(), int(hoje.Month())}
}

// argsJanela devolve os argumentos do placeholder de clausulaJanelaAbertura,
// com o MESMO switch sobre o enum (fora do enum, sem cláusula e sem
// argumento).
func argsJanela(j analytics.Janela, hoje time.Time) []any {
	switch j {
	case analytics.Janela30d:
		return []any{diasAntes(hoje, diasAtras30d)}
	case analytics.Janela90d:
		return []any{diasAntes(hoje, diasAtras90d)}
	case analytics.Janela365d:
		return []any{diasAntes(hoje, diasAtras365d)}
	default:
		return nil
	}
}

// sequenciaDias devolve os n dias da série, de hoje-(n-1) até hoje, em
// ordem crescente (a mesma sequência de analytics.SerieZerada).
func sequenciaDias(n int, hoje time.Time) []string {
	dias := make([]string, 0, max(n, 0))
	for i := n - 1; i >= 0; i-- {
		dias = append(dias, diasAntes(hoje, i))
	}
	return dias
}
