package kernel

import (
	"strings"
	"time"
)

// LayoutData é o formato de data aceito na API (mesmo do DATE do MySQL).
const LayoutData = "2006-01-02"

// Data é uma data de calendário (AAAA-MM-DD), sem hora. O zero value é a
// "data ausente" (IsZero).
type Data struct {
	t time.Time
}

// ParseData lê AAAA-MM-DD (espaços nas pontas são ignorados). Em qualquer
// falha, inclusive vazio, devolve errCampo: o sentinela do campo, com a
// mensagem pública do contexto (ex.: ErrDataLancamentoInvalida).
// errCampo nil é erro de programação (engoliria a data inválida): panic.
func ParseData(s string, errCampo error) (Data, error) {
	if errCampo == nil {
		panic("kernel: ParseData exige errCampo não nulo")
	}
	t, err := time.Parse(LayoutData, strings.TrimSpace(s))
	if err != nil {
		return Data{}, errCampo
	}
	return Data{t: t}, nil
}

// ParseDataOuHoje é ParseData com default: vazio vira a data de hoje
// (no fuso de hoje).
func ParseDataOuHoje(s string, hoje time.Time, errCampo error) (Data, error) {
	if errCampo == nil {
		panic("kernel: ParseDataOuHoje exige errCampo não nulo")
	}
	if strings.TrimSpace(s) == "" {
		return DataDe(hoje), nil
	}
	return ParseData(s, errCampo)
}

// DataDe extrai a data de calendário de t (no fuso de t).
func DataDe(t time.Time) Data {
	a, m, d := t.Date()
	return Data{t: time.Date(a, m, d, 0, 0, 0, 0, time.UTC)}
}

// IsZero informa se a data está ausente.
func (d Data) IsZero() bool { return d.t.IsZero() }

// Time devolve a data à meia-noite UTC.
func (d Data) Time() time.Time { return d.t }

// MeiaNoiteEm devolve a meia-noite da data no fuso loc (zero para data
// ausente). É o instante que vai ao banco e ao JSON: com o DSN loc=Local, a
// data AAAA-MM-DD precisa ser a meia-noite em time.Local, nunca a meia-noite
// UTC, senão o driver grava o dia anterior (BUG-01). Mesmo resultado de
// time.ParseInLocation(LayoutData, s, loc).
func (d Data) MeiaNoiteEm(loc *time.Location) time.Time {
	if d.IsZero() {
		return time.Time{}
	}
	a, m, dia := d.t.Date()
	return time.Date(a, m, dia, 0, 0, 0, 0, loc)
}

// String devolve AAAA-MM-DD ("" para data ausente).
func (d Data) String() string {
	if d.IsZero() {
		return ""
	}
	return d.t.Format(LayoutData)
}

// ParseDataLocal é ParseData devolvendo a meia-noite da data em time.Local:
// o instante que vai ao banco (DSN loc=Local) e ao JSON (BUG-01). Mesmo
// resultado de time.ParseInLocation(LayoutData, strings.TrimSpace(s),
// time.Local). Em qualquer falha devolve errCampo (nil: panic, como
// ParseData).
func ParseDataLocal(s string, errCampo error) (time.Time, error) {
	d, err := ParseData(s, errCampo)
	if err != nil {
		return time.Time{}, err
	}
	return d.MeiaNoiteEm(time.Local), nil
}
