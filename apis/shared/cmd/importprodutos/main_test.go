package main

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// parseAtivo
// ---------------------------------------------------------------------------

func TestParseAtivo(t *testing.T) {
	testCases := []struct {
		nome string
		in   string
		want bool
	}{
		{"S maiúsculo", "S", true},
		{"s minúsculo", "s", true},
		{"N maiúsculo", "N", false},
		{"n minúsculo", "n", false},
		// REGRA DE NEGÓCIO: vazio ou valor inválido/ausente vira TRUE (default ativo).
		{"vazio vira TRUE (default ativo)", "", true},
		{"valor inválido vira TRUE (default ativo)", "talvez", true},
		{"valor inválido X vira TRUE", "X", true},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			got := parseAtivo(tc.in)
			if got != tc.want {
				t.Errorf("parseAtivo(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseDataLancamento
// ---------------------------------------------------------------------------

func TestParseDataLancamento(t *testing.T) {
	testCases := []struct {
		nome    string
		in      string
		want    *time.Time
		wantErr bool
	}{
		{
			nome: "vazio retorna nil (NULL)",
			in:   "",
			want: nil,
		},
		{
			nome: "data válida",
			in:   "2024-01-15",
			want: timePtr(time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local)),
		},
		{
			nome:    "formato inválido",
			in:      "15/01/2024",
			wantErr: true,
		},
		{
			nome:    "data inexistente",
			in:      "2024-13-40",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			got, err := parseDataLancamento(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseDataLancamento(%q) esperava erro, obteve nil", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDataLancamento(%q) erro inesperado: %v", tc.in, err)
			}
			if tc.want == nil {
				if got != nil {
					t.Errorf("parseDataLancamento(%q) = %v, want nil", tc.in, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("parseDataLancamento(%q) = nil, want %v", tc.in, tc.want)
			}
			if !got.Equal(*tc.want) {
				t.Errorf("parseDataLancamento(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func timePtr(t time.Time) *time.Time { return &t }

// ---------------------------------------------------------------------------
// parseRow (integra parsing de floats + parseAtivo + parseDataLancamento + validações)
// ---------------------------------------------------------------------------

func TestParseRow(t *testing.T) {
	testCases := []struct {
		nome    string
		record  []string
		wantErr bool
		check   func(t *testing.T, row produtoRow)
	}{
		{
			nome: "linha válida completa",
			record: []string{
				"SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico",
				"99.90", "45.00", "UN", "S", "2024-01-15",
			},
			check: func(t *testing.T, row produtoRow) {
				if row.SKU != "SKU-001" {
					t.Errorf("SKU = %q, want SKU-001", row.SKU)
				}
				if row.PrecoTabela != 99.90 {
					t.Errorf("PrecoTabela = %v, want 99.90", row.PrecoTabela)
				}
				if row.CustoUnitario != 45.00 {
					t.Errorf("CustoUnitario = %v, want 45.00", row.CustoUnitario)
				}
				if !row.Ativo {
					t.Errorf("Ativo = false, want true")
				}
				if row.DataLancamento == nil {
					t.Fatalf("DataLancamento = nil, want não nil")
				}
				want := time.Date(2024, 1, 15, 0, 0, 0, 0, time.Local)
				if !row.DataLancamento.Equal(want) {
					t.Errorf("DataLancamento = %v, want %v", row.DataLancamento, want)
				}
			},
		},
		{
			nome: "sku vazio",
			record: []string{
				"", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico",
				"99.90", "45.00", "UN", "S", "2024-01-15",
			},
			wantErr: true,
		},
		{
			nome: "descricao vazia",
			record: []string{
				"SKU-001", "", "Perfumaria", "Marca X", "Cítrico",
				"99.90", "45.00", "UN", "S", "2024-01-15",
			},
			wantErr: true,
		},
		{
			nome: "categoria vazia",
			record: []string{
				"SKU-001", "Perfume Teste", "", "Marca X", "Cítrico",
				"99.90", "45.00", "UN", "S", "2024-01-15",
			},
			wantErr: true,
		},
		{
			nome: "marca vazia",
			record: []string{
				"SKU-001", "Perfume Teste", "Perfumaria", "", "Cítrico",
				"99.90", "45.00", "UN", "S", "2024-01-15",
			},
			wantErr: true,
		},
		{
			nome: "unidade vazia",
			record: []string{
				"SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico",
				"99.90", "45.00", "", "S", "2024-01-15",
			},
			wantErr: true,
		},
		{
			nome: "preco_tabela inválido",
			record: []string{
				"SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico",
				"abc", "45.00", "UN", "S", "2024-01-15",
			},
			wantErr: true,
		},
		{
			nome: "custo_unitario inválido",
			record: []string{
				"SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico",
				"99.90", "xyz", "UN", "S", "2024-01-15",
			},
			wantErr: true,
		},
		{
			nome: "data_lancamento inválida",
			record: []string{
				"SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico",
				"99.90", "45.00", "UN", "S", "data-invalida",
			},
			wantErr: true,
		},
		{
			nome: "ativo N explícito",
			record: []string{
				"SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico",
				"99.90", "45.00", "UN", "N", "2024-01-15",
			},
			check: func(t *testing.T, row produtoRow) {
				if row.Ativo {
					t.Errorf("Ativo = true, want false (N explícito)")
				}
			},
		},
		{
			nome: "ativo vazio vira TRUE (default) - regra de negócio",
			record: []string{
				"SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico",
				"99.90", "45.00", "UN", "", "2024-01-15",
			},
			check: func(t *testing.T, row produtoRow) {
				if !row.Ativo {
					t.Errorf("Ativo = false, want true (default quando vazio)")
				}
			},
		},
		{
			nome: "data_lancamento vazia -> DataLancamento nil",
			record: []string{
				"SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "Cítrico",
				"99.90", "45.00", "UN", "S", "",
			},
			check: func(t *testing.T, row produtoRow) {
				if row.DataLancamento != nil {
					t.Errorf("DataLancamento = %v, want nil", row.DataLancamento)
				}
			},
		},
		{
			nome: "nota_olfativa vazia é permitida",
			record: []string{
				"SKU-001", "Perfume Teste", "Perfumaria", "Marca X", "",
				"99.90", "45.00", "UN", "S", "2024-01-15",
			},
			check: func(t *testing.T, row produtoRow) {
				if row.NotaOlfativa != "" {
					t.Errorf("NotaOlfativa = %q, want vazio", row.NotaOlfativa)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			record := append([]string{}, tc.record...)
			row, err := parseRow(record)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseRow(%v) esperava erro, obteve nil", tc.record)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRow(%v) erro inesperado: %v", tc.record, err)
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
	got := resolveCSVPath("/caminho/custom/produtos.csv")
	want := "/caminho/custom/produtos.csv"
	if got != want {
		t.Errorf("resolveCSVPath com flag = %q, want %q", got, want)
	}
}

func TestResolveCSVPath_EnvVarQuandoSemFlag(t *testing.T) {
	t.Setenv("PRODUTOS_CSV_PATH", "/env/produtos.csv")
	got := resolveCSVPath("")
	want := "/env/produtos.csv"
	if got != want {
		t.Errorf("resolveCSVPath via env = %q, want %q", got, want)
	}
}
