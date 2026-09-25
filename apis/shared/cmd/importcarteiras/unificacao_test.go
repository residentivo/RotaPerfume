package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/cmd/internal/clientesdedup"
)

// NEG-01: vínculos de carteira de clientes unificados por CNPJ duplicado.

func vinculo(carteiraID, clienteID, vendedorID int64, inicio string) carteiraRow {
	d, _ := time.ParseInLocation("2006-01-02", inicio, time.Local)
	return carteiraRow{CarteiraIDOrigem: carteiraID, ClienteIDOrigem: clienteID, VendedorID: vendedorID, DataInicio: d}
}

func idsCarteira(rows []carteiraRow) []int64 {
	out := make([]int64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.CarteiraIDOrigem)
	}
	return out
}

func TestDescartarVinculosEquivalentes(t *testing.T) {
	u := clientesdedup.Unificacao{3001: 1, 3002: 2}
	cases := []struct {
		nome          string
		rows          []carteiraRow
		wantIDs       []int64
		wantDescartes int
	}{
		{
			nome: "vínculo da cópia igual ao do sobrevivente é descartado",
			rows: []carteiraRow{
				vinculo(10, 1, 5, "2024-09-01"),
				vinculo(20, 3001, 5, "2024-09-01"),
			},
			wantIDs: []int64{10}, wantDescartes: 1,
		},
		{
			nome: "prefere o sobrevivente mesmo com carteira_id maior",
			rows: []carteiraRow{
				vinculo(5, 3001, 5, "2024-09-01"),
				vinculo(10, 1, 5, "2024-09-01"),
			},
			wantIDs: []int64{10}, wantDescartes: 1,
		},
		{
			nome: "outro vendedor ou outra data_inicio é mantido (histórico)",
			rows: []carteiraRow{
				vinculo(10, 1, 5, "2024-09-01"),
				vinculo(20, 3001, 6, "2024-09-01"),
				vinculo(21, 3001, 5, "2025-01-01"),
			},
			wantIDs: []int64{10, 20, 21},
		},
		{
			nome: "sem vínculo do sobrevivente, fica a cópia de menor carteira_id",
			rows: []carteiraRow{
				vinculo(40, 3002, 7, "2024-09-01"),
				vinculo(30, 3002, 7, "2024-09-01"),
			},
			wantIDs: []int64{30}, wantDescartes: 1,
		},
		{
			nome:    "clientes não unificados não são afetados",
			rows:    []carteiraRow{vinculo(1, 50, 5, "2024-09-01"), vinculo(2, 50, 5, "2024-09-01")},
			wantIDs: []int64{1, 2},
		},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			mantidas, descartadas := descartarVinculosEquivalentes(tc.rows, u)
			assert.Equal(t, tc.wantIDs, idsCarteira(mantidas))
			assert.Equal(t, tc.wantDescartes, descartadas)
		})
	}

	t.Run("sem unificação devolve as linhas intactas", func(t *testing.T) {
		rows := []carteiraRow{vinculo(1, 1, 5, "2024-09-01")}
		mantidas, n := descartarVinculosEquivalentes(rows, clientesdedup.Unificacao{})
		assert.Equal(t, rows, mantidas)
		assert.Zero(t, n)
	})
}
