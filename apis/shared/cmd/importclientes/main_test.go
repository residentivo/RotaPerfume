package main

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// normalizeCNPJ
// ---------------------------------------------------------------------------

func TestNormalizeCNPJ(t *testing.T) {
	testCases := []struct {
		nome string
		in   string
		want string
	}{
		{"já normalizado", "12345678000199", "12345678000199"},
		{"com máscara padrão", "12.345.678/0001-99", "12345678000199"},
		{"com espaços em volta", "  12345678000199  ", "12345678000199"},
		{"vazio", "", ""},
		{"apenas letras", "abc", ""},
		{"letras misturadas com dígitos", "12a34b56c78d000199", "12345678000199"},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			got := normalizeCNPJ(tc.in)
			if got != tc.want {
				t.Errorf("normalizeCNPJ(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

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
		{"formato ISO (YYYY-MM-DD)", "2023-05-10", time.Date(2023, 5, 10, 0, 0, 0, 0, time.UTC), false},
		{"formato BR (DD/MM/YYYY)", "10/05/2023", time.Date(2023, 5, 10, 0, 0, 0, 0, time.UTC), false},
		{"formato inválido", "10-05-2023", time.Time{}, true},
		{"string vazia", "", time.Time{}, true},
		{"data inexistente", "2023-13-40", time.Time{}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			got, err := parseData(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseData(%q) esperava erro, obteve nil", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseData(%q) erro inesperado: %v", tc.in, err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("parseData(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseAtivo
// ---------------------------------------------------------------------------

func TestParseAtivo(t *testing.T) {
	testCases := []struct {
		nome    string
		in      string
		want    bool
		wantErr bool
	}{
		{"S maiúsculo", "S", true, false},
		{"s minúsculo", "s", true, false},
		{"N maiúsculo", "N", false, false},
		{"n minúsculo", "n", false, false},
		{"valor inválido", "X", false, true},
		{"vazio", "", false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			got, err := parseAtivo(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseAtivo(%q) esperava erro, obteve nil", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAtivo(%q) erro inesperado: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("parseAtivo(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseRow (integra normalizeCNPJ + parseData + parseAtivo + validações)
// ---------------------------------------------------------------------------

func TestParseRow(t *testing.T) {
	testCases := []struct {
		nome    string
		record  []string
		wantErr bool
		check   func(t *testing.T, row clienteRow)
	}{
		{
			nome:   "linha válida completa",
			record: []string{"100", "12.345.678/0001-99", "Empresa Teste", "varejo", "São Paulo", "sp", "Centro", "2023-05-10", "S"},
			check: func(t *testing.T, row clienteRow) {
				if row.ClienteIDOrigem != 100 {
					t.Errorf("ClienteIDOrigem = %d, want 100", row.ClienteIDOrigem)
				}
				if row.CNPJ != "12345678000199" {
					t.Errorf("CNPJ = %q, want normalizado", row.CNPJ)
				}
				if row.UF != "SP" {
					t.Errorf("UF = %q, want SP (uppercase)", row.UF)
				}
				if !row.Ativo {
					t.Errorf("Ativo = false, want true")
				}
			},
		},
		{
			nome:    "cliente_id inválido",
			record:  []string{"abc", "12345678000199", "Empresa", "varejo", "SP", "SP", "Centro", "2023-05-10", "S"},
			wantErr: true,
		},
		{
			nome:    "cnpj com menos de 14 dígitos",
			record:  []string{"1", "123", "Empresa", "varejo", "SP", "SP", "Centro", "2023-05-10", "S"},
			wantErr: true,
		},
		{
			nome:    "data inválida",
			record:  []string{"1", "12345678000199", "Empresa", "varejo", "SP", "SP", "Centro", "data-invalida", "S"},
			wantErr: true,
		},
		{
			nome:    "ativo inválido",
			record:  []string{"1", "12345678000199", "Empresa", "varejo", "SP", "SP", "Centro", "2023-05-10", "talvez"},
			wantErr: true,
		},
		{
			nome:    "razao_social vazia",
			record:  []string{"1", "12345678000199", "", "varejo", "SP", "SP", "Centro", "2023-05-10", "S"},
			wantErr: true,
		},
		{
			nome:    "uf vazia",
			record:  []string{"1", "12345678000199", "Empresa", "varejo", "SP", "", "Centro", "2023-05-10", "S"},
			wantErr: true,
		},
		{
			nome:   "data no formato BR",
			record: []string{"1", "12345678000199", "Empresa", "varejo", "SP", "SP", "Centro", "10/05/2023", "N"},
			check: func(t *testing.T, row clienteRow) {
				if row.Ativo {
					t.Errorf("Ativo = true, want false")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			row, err := parseRow(tc.record)
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
	got := resolveCSVPath("/caminho/custom/clientes.csv")
	want := "/caminho/custom/clientes.csv"
	if got != want {
		t.Errorf("resolveCSVPath com flag = %q, want %q", got, want)
	}
}

func TestResolveCSVPath_EnvVarQuandoSemFlag(t *testing.T) {
	t.Setenv("CLIENTES_CSV_PATH", "/env/clientes.csv")
	got := resolveCSVPath("")
	want := "/env/clientes.csv"
	if got != want {
		t.Errorf("resolveCSVPath via env = %q, want %q", got, want)
	}
}
