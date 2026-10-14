package kernel_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

var errDataTeste = kernel.NovoErro(kernel.KindInvalido, "data_teste inválida (use o formato AAAA-MM-DD)")

func TestParseData(t *testing.T) {
	casos := []struct {
		in   string
		want string
		ok   bool
	}{
		{"2026-10-13", "2026-10-13", true},
		{" 2026-01-02 ", "2026-01-02", true},
		{"", "", false},
		{"13/10/2026", "", false},
		{"2026-02-30", "", false},
		{"2026-10-13T10:00:00Z", "", false},
	}
	for _, c := range casos {
		t.Run(c.in, func(t *testing.T) {
			d, err := kernel.ParseData(c.in, errDataTeste)
			if !c.ok {
				assert.ErrorIs(t, err, errDataTeste, "devolve o erro do campo")
				assert.True(t, d.IsZero())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, d.String())
			assert.Equal(t, time.UTC, d.Time().Location())
		})
	}
}

func TestParseDataOuHoje(t *testing.T) {
	sp := time.FixedZone("BRT", -3*3600)
	hoje := time.Date(2026, 10, 13, 23, 30, 0, 0, sp) // já é dia 14 em UTC

	d, err := kernel.ParseDataOuHoje("  ", hoje, errDataTeste)
	require.NoError(t, err)
	assert.Equal(t, "2026-10-13", d.String(), "usa a data no fuso de hoje")

	d, err = kernel.ParseDataOuHoje("2025-01-31", hoje, errDataTeste)
	require.NoError(t, err)
	assert.Equal(t, "2025-01-31", d.String())

	_, err = kernel.ParseDataOuHoje("ontem", hoje, errDataTeste)
	assert.ErrorIs(t, err, errDataTeste)
}

func TestData_ZeroValue(t *testing.T) {
	var d kernel.Data
	assert.True(t, d.IsZero())
	assert.Equal(t, "", d.String())
}

func TestNovoEmail(t *testing.T) {
	casos := []struct {
		in   string
		want string
		ok   bool
	}{
		{"Ana.Silva@Empresa.com", "ana.silva@empresa.com", true},
		{"  a@b.io  ", "a@b.io", true},
		{"a@b.com\r\nBcc: x@y.com", "", false}, // M-01
		{"a@b.com\n", "", false},
		{"a@b.com\r", "", false},
		{"", "", false},
		{"sem-arroba.com", "", false},
		{"@b.com", "", false},
		{"a@", "", false},
		{"a@b", "", false},
		{"a@.com", "", false},
		{"a@b.", "", false},
		{"a@@b.com", "", false},
		{"a b@c.com", "", false},
		{"a@b\tc.com", "", false},
	}
	for _, c := range casos {
		t.Run(c.in, func(t *testing.T) {
			e, err := kernel.NovoEmail(c.in)
			if !c.ok {
				assert.ErrorIs(t, err, kernel.ErrEmailInvalido)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, e.String())
		})
	}
	assert.Equal(t, kernel.KindInvalido, kernel.ErrEmailInvalido.Kind())
}

// Mascarado tem o mesmo resultado de vlog.MaskEmail (o kernel não importa vlog).
func TestEmail_Mascarado_IgualAoVlog(t *testing.T) {
	for _, in := range []string{"ana.silva@empresa.com", "x@y.io", "Bob@Corp.COM"} {
		e, err := kernel.NovoEmail(in)
		require.NoError(t, err)
		assert.Equal(t, vlog.MaskEmail(e.String()), e.Mascarado(), "in=%q", in)
		assert.NotEqual(t, e.String(), e.Mascarado())
	}
	var zero kernel.Email
	assert.Equal(t, "***", zero.Mascarado())
}

func TestNovoDinheiro(t *testing.T) {
	casos := []struct {
		nome string
		v    float64
		ok   bool
	}{
		{"zero", 0, true},
		{"positivo", 199.9, true},
		{"negativo", -0.01, false},
		{"NaN", math.NaN(), false},
		{"+Inf", math.Inf(1), false},
		{"-Inf", math.Inf(-1), false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			d, err := kernel.NovoDinheiro(c.v)
			if !c.ok {
				assert.ErrorIs(t, err, kernel.ErrDinheiroInvalido)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.v, d.Float64())
		})
	}
	a, _ := kernel.NovoDinheiro(10.5)
	b, _ := kernel.NovoDinheiro(0.25)
	assert.Equal(t, 10.75, a.Somar(b).Float64())
}

// N-2: errCampo nil engoliria a data inválida; é erro de programação.
func TestParseData_ErrCampoNilPanica(t *testing.T) {
	hoje := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for _, s := range []string{"", "2026-10-13", "invalida"} {
		assert.Panics(t, func() { _, _ = kernel.ParseData(s, nil) }, "s=%q", s)
		assert.Panics(t, func() { _, _ = kernel.ParseDataOuHoje(s, hoje, nil) }, "s=%q", s)
	}
}
