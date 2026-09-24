package main

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// ---------------------------------------------------------------------------
// parseRuptura
// ---------------------------------------------------------------------------

func TestParseRuptura(t *testing.T) {
	testCases := []struct {
		nome string
		in   string
		want bool
	}{
		{"S maiúsculo", "S", true},
		{"s minúsculo", "s", true},
		{"N maiúsculo", "N", false},
		{"n minúsculo", "n", false},
		// REGRA DE NEGÓCIO: vazio ou valor inválido/ausente vira FALSE (default sem ruptura).
		{"vazio vira FALSE (default sem ruptura)", "", false},
		{"valor inválido vira FALSE (default sem ruptura)", "talvez", false},
		{"valor inválido X vira FALSE", "X", false},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			got := parseRuptura(tc.in)
			if got != tc.want {
				t.Errorf("parseRuptura(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseRow (integra trim + parsing de data + saldo + parseRuptura + validações)
// ---------------------------------------------------------------------------

func TestParseRow(t *testing.T) {
	testCases := []struct {
		nome    string
		record  []string
		wantErr bool
		check   func(t *testing.T, row estoqueRow)
	}{
		{
			nome:   "linha válida completa",
			record: []string{"2024-06-01", "SKU-001", "10", "N"},
			check: func(t *testing.T, row estoqueRow) {
				if row.SKU != "SKU-001" {
					t.Errorf("SKU = %q, want SKU-001", row.SKU)
				}
				if row.Saldo != 10 {
					t.Errorf("Saldo = %v, want 10", row.Saldo)
				}
				if row.Ruptura {
					t.Errorf("Ruptura = true, want false")
				}
				want := time.Date(2024, 6, 1, 0, 0, 0, 0, time.Local)
				if !row.DataSnapshot.Equal(want) {
					t.Errorf("DataSnapshot = %v, want %v", row.DataSnapshot, want)
				}
			},
		},
		{
			nome:   "ruptura S explícito",
			record: []string{"2024-06-01", "SKU-001", "0", "S"},
			check: func(t *testing.T, row estoqueRow) {
				if !row.Ruptura {
					t.Errorf("Ruptura = false, want true (S explícito)")
				}
			},
		},
		{
			nome:   "ruptura vazia vira FALSE (default) - regra de negócio",
			record: []string{"2024-06-01", "SKU-001", "10", ""},
			check: func(t *testing.T, row estoqueRow) {
				if row.Ruptura {
					t.Errorf("Ruptura = true, want false (default quando vazio)")
				}
			},
		},
		{
			nome:    "sku vazio",
			record:  []string{"2024-06-01", "", "10", "N"},
			wantErr: true,
		},
		{
			nome:    "data_snapshot vazia",
			record:  []string{"", "SKU-001", "10", "N"},
			wantErr: true,
		},
		{
			nome:    "data_snapshot em formato inválido",
			record:  []string{"01/06/2024", "SKU-001", "10", "N"},
			wantErr: true,
		},
		{
			nome:    "saldo inválido (não numérico)",
			record:  []string{"2024-06-01", "SKU-001", "abc", "N"},
			wantErr: true,
		},
		{
			nome:   "saldo negativo é aceito pelo parsing (validação de negócio fica no service)",
			record: []string{"2024-06-01", "SKU-001", "-5", "S"},
			check: func(t *testing.T, row estoqueRow) {
				if row.Saldo != -5 {
					t.Errorf("Saldo = %v, want -5", row.Saldo)
				}
			},
		},
		{
			nome:   "campos com espaços são normalizados via TRIM",
			record: []string{"  2024-06-01  ", "  SKU-001  ", " 10 ", " s "},
			check: func(t *testing.T, row estoqueRow) {
				if row.SKU != "SKU-001" {
					t.Errorf("SKU = %q, want SKU-001 (trim)", row.SKU)
				}
				if !row.Ruptura {
					t.Errorf("Ruptura = false, want true ('s' com espaços)")
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
	got := resolveCSVPath("/caminho/custom/estoque.csv")
	want := "/caminho/custom/estoque.csv"
	if got != want {
		t.Errorf("resolveCSVPath com flag = %q, want %q", got, want)
	}
}

func TestResolveCSVPath_EnvVarQuandoSemFlag(t *testing.T) {
	t.Setenv("ESTOQUE_CSV_PATH", "/env/estoque.csv")
	got := resolveCSVPath("")
	want := "/env/estoque.csv"
	if got != want {
		t.Errorf("resolveCSVPath via env = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// upsertAll (idempotência do upsert: insert novo vs update existente vs erro)
// ---------------------------------------------------------------------------

func TestUpsertAll_InsertNovoAtualizaExistenteEContaErros(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	rows := []estoqueRow{
		{DataSnapshot: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), SKU: "SKU-NOVO", Saldo: 10, Ruptura: false},
		{DataSnapshot: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), SKU: "SKU-EXISTENTE", Saldo: 5, Ruptura: true},
		{DataSnapshot: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), SKU: "SKU-FK-INVALIDA", Saldo: 3, Ruptura: false},
	}

	mock.ExpectPrepare("INSERT INTO estoque")
	// Linha 1: sku novo -> INSERT (affected=1).
	mock.ExpectExec("INSERT INTO estoque").
		WithArgs("2024-06-01", "SKU-NOVO", 10, false).
		WillReturnResult(sqlmock.NewResult(1, 1))
	// Linha 2: (data_snapshot, sku) já existe -> UPDATE via ON DUPLICATE KEY (affected=2).
	mock.ExpectExec("INSERT INTO estoque").
		WithArgs("2024-06-01", "SKU-EXISTENTE", 5, true).
		WillReturnResult(sqlmock.NewResult(0, 2))
	// Linha 3: sku sem produto correspondente -> erro de FK, contado como falha
	// (não aborta a importação inteira).
	mock.ExpectExec("INSERT INTO estoque").
		WithArgs("2024-06-01", "SKU-FK-INVALIDA", 3, false).
		WillReturnError(sql.ErrConnDone)

	inserted, updated, failed := upsertAll(db, rows)

	if inserted != 1 {
		t.Errorf("inserted = %d, want 1", inserted)
	}
	if updated != 1 {
		t.Errorf("updated = %d, want 1", updated)
	}
	if failed != 1 {
		t.Errorf("failed = %d, want 1", failed)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}

func TestUpsertAll_ReenvioMesmaLinhaSemMudanca_ContaComoUpdate(t *testing.T) {
	// affected=0 (MySQL: linha já existia e nenhum valor mudou) ainda é
	// contado como "updated" — o upsert é idempotente, reenviar o mesmo CSV
	// não deve ser tratado como erro nem como novo insert.
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	rows := []estoqueRow{
		{DataSnapshot: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), SKU: "SKU-INALTERADO", Saldo: 10, Ruptura: false},
	}

	mock.ExpectPrepare("INSERT INTO estoque")
	mock.ExpectExec("INSERT INTO estoque").
		WithArgs("2024-06-01", "SKU-INALTERADO", 10, false).
		WillReturnResult(sqlmock.NewResult(0, 0))

	inserted, updated, failed := upsertAll(db, rows)

	if inserted != 0 {
		t.Errorf("inserted = %d, want 0", inserted)
	}
	if updated != 1 {
		t.Errorf("updated = %d, want 1", updated)
	}
	if failed != 0 {
		t.Errorf("failed = %d, want 0", failed)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}
