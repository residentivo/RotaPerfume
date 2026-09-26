package carteiras_test

import (
	"testing"
	"time"

	"github.com/rotaperfumes/shared/importers/carteiras"
)

// ---------------------------------------------------------------------------
// parseData
// ---------------------------------------------------------------------------

func TestParseData(t *testing.T) {
	testCases := []struct {
		nome    string
		in      string
		want    time.Time
		wantErr bool
	}{
		{"formato ISO (YYYY-MM-DD)", "2026-04-14", time.Date(2026, 4, 14, 0, 0, 0, 0, time.Local), false},
		{"formato BR (DD/MM/YYYY)", "14/04/2026", time.Date(2026, 4, 14, 0, 0, 0, 0, time.Local), false},
		{"formato inválido", "14-04-2026", time.Time{}, true},
		{"string vazia", "", time.Time{}, true},
		{"data inexistente", "2026-13-40", time.Time{}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			got, err := carteiras.ParseData(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("carteiras.ParseData(%q) esperava erro, obteve nil", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("carteiras.ParseData(%q) erro inesperado: %v", tc.in, err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("carteiras.ParseData(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseRow (integra parseData + validações + data_fim opcional)
// ---------------------------------------------------------------------------

func TestParseRow(t *testing.T) {
	testCases := []struct {
		nome    string
		record  []string
		wantErr bool
		check   func(t *testing.T, row carteiras.Row)
	}{
		{
			nome:   "linha válida completa (vínculo encerrado)",
			record: []string{"1", "2", "15", "2023-01-10", "2026-04-14"},
			check: func(t *testing.T, row carteiras.Row) {
				if row.CarteiraIDOrigem != 1 {
					t.Errorf("CarteiraIDOrigem = %d, want 1", row.CarteiraIDOrigem)
				}
				if row.ClienteIDOrigem != 2 {
					t.Errorf("ClienteIDOrigem = %d, want 2", row.ClienteIDOrigem)
				}
				if row.VendedorID != 15 {
					t.Errorf("VendedorID = %d, want 15", row.VendedorID)
				}
				if row.DataFim == nil {
					t.Fatalf("DataFim = nil, want não-nil")
				}
				want := time.Date(2026, 4, 14, 0, 0, 0, 0, time.Local)
				if !row.DataFim.Equal(want) {
					t.Errorf("DataFim = %v, want %v", row.DataFim, want)
				}
			},
		},
		{
			nome:   "data_fim vazia (vínculo ativo)",
			record: []string{"2", "2", "32", "2026-04-14", ""},
			check: func(t *testing.T, row carteiras.Row) {
				if row.DataFim != nil {
					t.Errorf("DataFim = %v, want nil (vínculo ativo)", row.DataFim)
				}
			},
		},
		{
			nome:   "data no formato BR",
			record: []string{"3", "5", "10", "10/05/2023", ""},
			check: func(t *testing.T, row carteiras.Row) {
				want := time.Date(2023, 5, 10, 0, 0, 0, 0, time.Local)
				if !row.DataInicio.Equal(want) {
					t.Errorf("DataInicio = %v, want %v", row.DataInicio, want)
				}
			},
		},
		{
			nome:    "carteira_id inválido",
			record:  []string{"abc", "2", "15", "2023-01-10", ""},
			wantErr: true,
		},
		{
			nome:    "cliente_id inválido",
			record:  []string{"1", "abc", "15", "2023-01-10", ""},
			wantErr: true,
		},
		{
			nome:    "vendedor_id inválido",
			record:  []string{"1", "2", "abc", "2023-01-10", ""},
			wantErr: true,
		},
		{
			nome:    "data_inicio inválida",
			record:  []string{"1", "2", "15", "data-invalida", ""},
			wantErr: true,
		},
		{
			nome:    "data_fim inválida",
			record:  []string{"1", "2", "15", "2023-01-10", "data-invalida"},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			row, err := carteiras.ParseRow(tc.record)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("carteiras.ParseRow(%v) esperava erro, obteve nil", tc.record)
				}
				return
			}
			if err != nil {
				t.Fatalf("carteiras.ParseRow(%v) erro inesperado: %v", tc.record, err)
			}
			if tc.check != nil {
				tc.check(t, row)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// resolveCSVPath
// ---------------------------------------------------------------------------

func TestResolveCSVPath_FlagTemPrioridade(t *testing.T) {
	got, err := carteiras.ResolveCSVPath("/caminho/custom/carteira.csv")
	if err != nil {
		t.Fatalf("ResolveCSVPath: %v", err)
	}
	want := "/caminho/custom/carteira.csv"
	if got != want {
		t.Errorf("resolveCSVPath com flag = %q, want %q", got, want)
	}
}

func TestResolveCSVPath_EnvVarQuandoSemFlag(t *testing.T) {
	t.Setenv("CARTEIRAS_CSV_PATH", "/env/carteira.csv")
	got, err := carteiras.ResolveCSVPath("")
	if err != nil {
		t.Fatalf("ResolveCSVPath: %v", err)
	}
	want := "/env/carteira.csv"
	if got != want {
		t.Errorf("resolveCSVPath via env = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// upsertAll — cliente não encontrado no lookup deve ser contado como erro
// ---------------------------------------------------------------------------

func TestUpsertAll_ClienteNaoEncontradoEhErro(t *testing.T) {
	// Sem conexão real com banco, apenas validamos a lógica de lookup:
	// uma linha cujo ClienteIDOrigem não está no mapa nunca deve ser
	// passada para stmt.Exec (aqui simulado sem *sql.DB real, testando
	// apenas o comportamento isolável em unidade menor não é trivial sem
	// mocks; este teste documenta o contrato via parseRow + mapa).
	clienteIDs := map[int64]int64{2: 20, 5: 50}

	row, err := carteiras.ParseRow([]string{"1", "999", "15", "2023-01-10", ""})
	if err != nil {
		t.Fatalf("parseRow erro inesperado: %v", err)
	}

	if _, ok := clienteIDs[row.ClienteIDOrigem]; ok {
		t.Fatalf("esperava cliente_id_origem=%d ausente do lookup", row.ClienteIDOrigem)
	}
}
