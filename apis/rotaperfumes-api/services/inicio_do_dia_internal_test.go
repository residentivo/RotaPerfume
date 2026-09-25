package services

// SEC-01/RISCO-01: inicioDoDia (data_inicio da carteira criada junto com o
// cliente) deve devolver a meia-noite em time.Local do dia do instante
// informado — convertendo antes de truncar, para que um instante UTC já no
// dia seguinte continue contando como "hoje" no fuso do sistema (-03).

import (
	"testing"
	"time"
)

func TestInicioDoDia(t *testing.T) {
	old := time.Local
	time.Local = time.FixedZone("-03", -3*3600)
	t.Cleanup(func() { time.Local = old })

	casos := []struct {
		nome    string
		entrada time.Time
		want    string
	}{
		{"meio do dia local", time.Date(2026, 9, 24, 15, 30, 0, 0, time.Local), "2026-09-24T00:00:00-03:00"},
		{"meia-noite local ja truncada", time.Date(2026, 9, 24, 0, 0, 0, 0, time.Local), "2026-09-24T00:00:00-03:00"},
		{"ultimo instante do dia local", time.Date(2026, 9, 24, 23, 59, 59, 999, time.Local), "2026-09-24T00:00:00-03:00"},
		{"UTC ja no dia seguinte, local ainda hoje", time.Date(2026, 9, 25, 1, 0, 0, 0, time.UTC), "2026-09-24T00:00:00-03:00"},
		{"UTC+9 no dia seguinte", time.Date(2026, 9, 25, 8, 0, 0, 0, time.FixedZone("+09", 9*3600)), "2026-09-24T00:00:00-03:00"},
		{"antigo inicio de horario de verao", time.Date(2018, 11, 4, 12, 0, 0, 0, time.Local), "2018-11-04T00:00:00-03:00"},
	}
	for _, tc := range casos {
		t.Run(tc.nome, func(t *testing.T) {
			got := inicioDoDia(tc.entrada)
			if got.Format(time.RFC3339) != tc.want {
				t.Fatalf("inicioDoDia(%v) = %v, want %s", tc.entrada, got.Format(time.RFC3339), tc.want)
			}
			if got.Location() != time.Local {
				t.Fatalf("location = %v, want time.Local", got.Location())
			}
		})
	}
}
