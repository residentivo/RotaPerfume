package main

// BUG-01: parseData deve validar AAAA-MM-DD em time.Local e devolver
// exatamente a mesma data (sem deslocamento de fuso). Não usa t.Parallel
// (altera time.Local).

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseData_SemDeslocamento(t *testing.T) {
	fusos := []*time.Location{time.FixedZone("UTC-3", -3*3600), time.FixedZone("UTC+9", 9*3600), time.UTC}
	casos := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{"2024-01-01", "2024-01-01", false},
		{"2024-02-29", "2024-02-29", false},
		{"2023-03-01", "2023-03-01", false},
		{"01/01/2024", "", true},
		{"2024-02-30", "", true},
		{"", "", true},
	}
	for _, loc := range fusos {
		for _, c := range casos {
			t.Run(loc.String()+"/"+c.raw, func(t *testing.T) {
				old := time.Local
				time.Local = loc
				defer func() { time.Local = old }()

				got, err := parseData(c.raw)
				if c.wantErr {
					assert.Error(t, err)
					return
				}
				assert.NoError(t, err)
				assert.Equal(t, c.want, got)
			})
		}
	}
}
