package handlers_test

// BUG-30 (Lote 38): as datas do analytics são calculadas no Go (fuso
// tz.Local) a partir do relógio do bootstrap (time.Now) e vão por
// placeholder. Os testes que sobem o servidor/bootstrap real não fixam o
// relógio; estes matchers comparam o argumento com o dia calculado NO
// MOMENTO DO MATCH (a consulta roda logo após o cálculo do hoje), então não
// dependem da hora em que o teste roda.

import (
	"database/sql/driver"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/rotaperfumes/shared/tz"
)

// hojeBUG30 é a meia-noite de hoje no fuso tz.Local.
func hojeBUG30() time.Time {
	t := time.Now().In(tz.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, tz.Local)
}

// diaBUG30 é o dia (formato DATE) n dias antes de hoje.
func diaBUG30(diasAtras int) string {
	return hojeBUG30().AddDate(0, 0, -diasAtras).Format("2006-01-02")
}

// argDia casa com o dia (string DATE) n dias antes de hoje.
type argDia int

func (a argDia) Match(v driver.Value) bool {
	s, ok := v.(string)
	return ok && s == diaBUG30(int(a))
}

// argAnoMes casa com o ano (false) ou o mês (true) de hoje.
type argAnoMes bool

func (a argAnoMes) Match(v driver.Value) bool {
	n, ok := v.(int64)
	if !ok {
		return false
	}
	hoje := hojeBUG30()
	if a {
		return n == int64(hoje.Month())
	}
	return n == int64(hoje.Year())
}

var (
	_ sqlmock.Argument = argDia(0)
	_ sqlmock.Argument = argAnoMes(false)
)

// argsMesAtual são os args {ano, mês} das cláusulas do mês.
func argsMesAtual(resto ...driver.Value) []driver.Value {
	return append([]driver.Value{argAnoMes(false), argAnoMes(true)}, resto...)
}

// idsComMesAtual são os args das vendas do mês: os ids do IN e depois
// {ano, mês}.
func idsComMesAtual(ids ...driver.Value) []driver.Value {
	return append(ids, argAnoMes(false), argAnoMes(true))
}

// argsPeriodoAtual são os args da cláusula de período (today|week|month,
// inválido = month), seguidos de resto.
func argsPeriodoAtual(periodo string, resto ...driver.Value) []driver.Value {
	switch periodo {
	case "today":
		return append([]driver.Value{argDia(0)}, resto...)
	case "week":
		return append([]driver.Value{argDia(6)}, resto...)
	default:
		return argsMesAtual(resto...)
	}
}
