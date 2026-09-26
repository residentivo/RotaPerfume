package visitas_test

// BUG-01: parseData deve devolver meia-noite em time.Local (DSN loc=Local),
// nunca meia-noite UTC, para ISO e BR. Não usa t.Parallel (altera time.Local).

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/importers/visitas"
)

func TestParseData_MeiaNoiteLocal(t *testing.T) {
	fusos := []*time.Location{time.FixedZone("UTC-3", -3*3600), time.FixedZone("UTC+9", 9*3600), time.UTC}
	casos := []struct {
		raw     string
		y       int
		m       time.Month
		d       int
		wantErr bool
	}{
		{"2024-01-01", 2024, time.January, 1, false},
		{"01/01/2024", 2024, time.January, 1, false},
		{"2024-02-29", 2024, time.February, 29, false},
		{"29/02/2024", 2024, time.February, 29, false},
		{"2024-13-01", 0, 0, 0, true},
		{"abc", 0, 0, 0, true},
	}
	for _, loc := range fusos {
		for _, c := range casos {
			t.Run(loc.String()+"/"+c.raw, func(t *testing.T) {
				old := time.Local
				time.Local = loc
				defer func() { time.Local = old }()

				got, err := visitas.ParseData(c.raw)
				if c.wantErr {
					assert.Error(t, err)
					return
				}
				require.NoError(t, err)
				want := time.Date(c.y, c.m, c.d, 0, 0, 0, 0, time.Local)
				assert.True(t, got.Equal(want), "got %s want %s", got, want)
				assert.Equal(t, c.d, got.Day())
				assert.Equal(t, 0, got.Hour())
				_, off := got.Zone()
				_, wantOff := want.Zone()
				assert.Equal(t, wantOff, off)
			})
		}
	}
}
