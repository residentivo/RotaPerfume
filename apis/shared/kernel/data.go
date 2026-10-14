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

// String devolve AAAA-MM-DD ("" para data ausente).
func (d Data) String() string {
	if d.IsZero() {
		return ""
	}
	return d.t.Format(LayoutData)
}
