package main

// NEG-02 (TestBrain, Lote 5): o importador usa a mesma normalização da API e
// do front. Com a massa comum (apis/shared/cnpj/testdata/casos_cruzados.json):
//   - todo CNPJ válido é importado com o mesmo valor que a API grava;
//   - um CNPJ com formato válido e DV errado é importado (dados fictícios do
//     CSV; o importador não valida DV, por decisão do NEG-01/NEG-02);
//   - formato inválido e as divergências conhecidas do front são recusados;
//   - variantes de máscara/caixa do mesmo CNPJ unificam no clientesdedup.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotaperfumes/shared/cmd/internal/clientesdedup"
	"github.com/rotaperfumes/shared/cnpj"
)

type casoCruzadoImport struct {
	Descricao   string `json:"descricao"`
	Entrada     string `json:"entrada"`
	Valido      bool   `json:"valido"`
	Normalizado string `json:"normalizado"`
}

func carregarMassaImport(t *testing.T) (casos, divergencias []casoCruzadoImport) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "cnpj", "testdata", "casos_cruzados.json"))
	if err != nil {
		t.Fatalf("massa cruzada: %v", err)
	}
	var m struct {
		Casos        []casoCruzadoImport `json:"casos"`
		Divergencias []casoCruzadoImport `json:"divergencias"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("massa cruzada: %v", err)
	}
	return m.Casos, m.Divergencias
}

func linhaCSV(id int, doc string) []string {
	return []string{fmt.Sprint(id), doc, "Empresa", "varejo", "Curitiba", "PR", "Centro", "2023-05-10", "S"}
}

func TestParseRowCNPJ_MassaCruzada(t *testing.T) {
	casos, divergencias := carregarMassaImport(t)
	for i, tc := range casos {
		t.Run(tc.Descricao, func(t *testing.T) {
			row, err := parseRow(linhaCSV(i+1, tc.Entrada))
			n, okNorm := cnpj.Normalizar(strings.TrimSpace(tc.Entrada))
			formatoOK := okNorm && cnpj.FormatoValido(n)
			switch {
			case tc.Valido:
				if err != nil {
					t.Fatalf("CNPJ válido %q recusado: %v", tc.Entrada, err)
				}
				if row.CNPJ != tc.Normalizado {
					t.Errorf("CNPJ = %q, want %q (mesmo valor gravado pela API)", row.CNPJ, tc.Normalizado)
				}
			case formatoOK:
				// Formato certo, DV errado: o importador aceita (não valida DV).
				if err != nil || row.CNPJ != n {
					t.Errorf("formato válido com DV errado deveria importar %q: row=%q err=%v", n, row.CNPJ, err)
				}
			default:
				if err == nil {
					t.Errorf("formato inválido %q deveria ser recusado, importou %q", tc.Entrada, row.CNPJ)
				}
			}
		})
	}
	for i, tc := range divergencias {
		t.Run("divergência: "+tc.Descricao, func(t *testing.T) {
			if row, err := parseRow(linhaCSV(1000+i, tc.Entrada)); err == nil {
				t.Errorf("%q deveria ser recusado (contrato do back), importou %q", tc.Entrada, row.CNPJ)
			}
		})
	}
}

func TestClientesdedup_VariantesUnificam_MassaCruzada(t *testing.T) {
	casos, _ := carregarMassaImport(t)
	var regs []clientesdedup.Registro
	want := clientesdedup.Unificacao{}
	id := int64(1)
	visto := map[string]bool{}
	for _, tc := range casos {
		if !tc.Valido || visto[tc.Normalizado] {
			continue
		}
		visto[tc.Normalizado] = true
		d := tc.Normalizado
		original := id
		mascarado := d[0:2] + "." + d[2:5] + "." + d[5:8] + "/" + d[8:12] + "-" + d[12:14]
		for _, v := range []string{d, strings.ToLower(mascarado), " " + mascarado + " "} {
			regs = append(regs, clientesdedup.Registro{ClienteID: id, CNPJ: v})
			if id != original {
				want[id] = original
			}
			id++
		}
	}
	got := clientesdedup.Calcular(regs)
	if len(got) != len(want) {
		t.Fatalf("unificações = %d, want %d", len(got), len(want))
	}
	for copia, sobrevivente := range want {
		if got[copia] != sobrevivente {
			t.Errorf("cópia %d -> %d, want %d", copia, got[copia], sobrevivente)
		}
	}
}
