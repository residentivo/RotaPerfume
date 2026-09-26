package clientes_test

import (
	"testing"
	"time"

	"github.com/rotaperfumes/shared/importers/clientes"
)

// ---------------------------------------------------------------------------
// CNPJ no parseRow (normalização central: clientesdedup.NormalizarCNPJ →
// shared/cnpj, NEG-02). O DV não é validado no importador.
// ---------------------------------------------------------------------------

func TestParseRowCNPJ(t *testing.T) {
	testCases := []struct {
		nome    string
		in      string
		want    string
		wantErr bool
	}{
		{nome: "já normalizado", in: "12345678000199", want: "12345678000199"},
		{nome: "com máscara padrão", in: "12.345.678/0001-99", want: "12345678000199"},
		{nome: "com espaços em volta", in: "  12345678000199  ", want: "12345678000199"},
		{nome: "alfanumérico", in: "12ABC34501DE35", want: "12ABC34501DE35"},
		{nome: "alfanumérico minúsculo com máscara", in: "12.abc.345/01de-35", want: "12ABC34501DE35"},
		{nome: "DV inválido é aceito (dados fictícios)", in: "12ABC34501DE99", want: "12ABC34501DE99"},
		{nome: "vazio", in: "", wantErr: true},
		{nome: "apenas letras", in: "abc", wantErr: true},
		{nome: "letra na posição do DV", in: "12ABC34501DEA5", wantErr: true},
		{nome: "letras excedentes não são descartadas", in: "12a34b56c78d000199", wantErr: true},
		{nome: "símbolo fora da máscara", in: "12345678000199#", wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			record := []string{"1", tc.in, "Empresa", "varejo", "SP", "SP", "Centro", "2023-05-10", "S"}
			row, err := clientes.ParseRow(record)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseRow com cnpj %q esperava erro, obteve CNPJ=%q", tc.in, row.CNPJ)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRow com cnpj %q erro inesperado: %v", tc.in, err)
			}
			if row.CNPJ != tc.want {
				t.Errorf("CNPJ = %q, want %q", row.CNPJ, tc.want)
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
		{"formato ISO (YYYY-MM-DD)", "2023-05-10", time.Date(2023, 5, 10, 0, 0, 0, 0, time.Local), false},
		{"formato BR (DD/MM/YYYY)", "10/05/2023", time.Date(2023, 5, 10, 0, 0, 0, 0, time.Local), false},
		{"formato inválido", "10-05-2023", time.Time{}, true},
		{"string vazia", "", time.Time{}, true},
		{"data inexistente", "2023-13-40", time.Time{}, true},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			got, err := clientes.ParseData(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("clientes.ParseData(%q) esperava erro, obteve nil", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("clientes.ParseData(%q) erro inesperado: %v", tc.in, err)
			}
			if !got.Equal(tc.want) {
				t.Errorf("clientes.ParseData(%q) = %v, want %v", tc.in, got, tc.want)
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
			got, err := clientes.ParseAtivo(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("clientes.ParseAtivo(%q) esperava erro, obteve nil", tc.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("clientes.ParseAtivo(%q) erro inesperado: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("clientes.ParseAtivo(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseRow (integra NormalizarCNPJ + parseData + parseAtivo + validações)
// ---------------------------------------------------------------------------

func TestParseRow(t *testing.T) {
	testCases := []struct {
		nome    string
		record  []string
		wantErr bool
		check   func(t *testing.T, row clientes.Row)
	}{
		{
			nome:   "linha válida completa",
			record: []string{"100", "12.345.678/0001-99", "Empresa Teste", "varejo", "São Paulo", "sp", "Centro", "2023-05-10", "S"},
			check: func(t *testing.T, row clientes.Row) {
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
			check: func(t *testing.T, row clientes.Row) {
				if row.Ativo {
					t.Errorf("Ativo = true, want false")
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			row, err := clientes.ParseRow(tc.record)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("clientes.ParseRow(%v) esperava erro, obteve nil", tc.record)
				}
				return
			}
			if err != nil {
				t.Fatalf("clientes.ParseRow(%v) erro inesperado: %v", tc.record, err)
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
	got, err := clientes.ResolveCSVPath("/caminho/custom/clientes.csv")
	if err != nil {
		t.Fatalf("ResolveCSVPath: %v", err)
	}
	want := "/caminho/custom/clientes.csv"
	if got != want {
		t.Errorf("resolveCSVPath com flag = %q, want %q", got, want)
	}
}

func TestResolveCSVPath_EnvVarQuandoSemFlag(t *testing.T) {
	t.Setenv("CLIENTES_CSV_PATH", "/env/clientes.csv")
	got, err := clientes.ResolveCSVPath("")
	if err != nil {
		t.Fatalf("ResolveCSVPath: %v", err)
	}
	want := "/env/clientes.csv"
	if got != want {
		t.Errorf("resolveCSVPath via env = %q, want %q", got, want)
	}
}
