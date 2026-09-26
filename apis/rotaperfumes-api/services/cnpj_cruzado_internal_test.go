package services

// NEG-02 (TestBrain, Lote 5): a camada de serviço da API (fluxo de
// Create/Update: TrimSpace + normalizarCNPJ + cnpjDigitosValidos) concorda
// com a massa comum usada pelo front (frontend/src/lib/cnpj.cruzado.test.ts).
// Também cobre termoBuscaCNPJ (busca ?q= com máscara) com a mesma massa.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type casoCNPJCruzado struct {
	Descricao   string `json:"descricao"`
	Entrada     string `json:"entrada"`
	Valido      bool   `json:"valido"`
	Normalizado string `json:"normalizado"`
}

func carregarCasosCNPJCruzados(t *testing.T) (casos, divergencias []casoCNPJCruzado) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "shared", "cnpj", "testdata", "casos_cruzados.json"))
	require.NoError(t, err)
	var m struct {
		Casos        []casoCNPJCruzado `json:"casos"`
		Divergencias []casoCNPJCruzado `json:"divergencias"`
	}
	require.NoError(t, json.Unmarshal(raw, &m))
	require.NotEmpty(t, m.Casos)
	return m.Casos, m.Divergencias
}

func validarCNPJComoService(raw string) (string, bool) {
	n, ok := normalizarCNPJ(strings.TrimSpace(raw))
	if !ok || !cnpjDigitosValidos(n) {
		return "", false
	}
	return n, true
}

func TestValidacaoCNPJ_MassaCruzada(t *testing.T) {
	casos, divergencias := carregarCasosCNPJCruzados(t)
	for _, tc := range append(casos, divergencias...) {
		t.Run(tc.Descricao, func(t *testing.T) {
			n, ok := validarCNPJComoService(tc.Entrada)
			assert.Equal(t, tc.Valido, ok, "entrada %q", tc.Entrada)
			if tc.Valido {
				assert.Equal(t, tc.Normalizado, n)
			}
		})
	}
}

// TestTermoBuscaCNPJ_MassaCruzada: todo CNPJ válido da massa digitado com
// máscara (e em minúsculas) vira, na busca, exatamente o valor gravado no
// banco; sem máscara, a busca usa o próprio q (a collation ignora caixa).
func TestTermoBuscaCNPJ_MassaCruzada(t *testing.T) {
	casos, _ := carregarCasosCNPJCruzados(t)
	mascarar := func(s string) string {
		return s[0:2] + "." + s[2:5] + "." + s[5:8] + "/" + s[8:12] + "-" + s[12:14]
	}
	n := 0
	for _, tc := range casos {
		if !tc.Valido {
			continue
		}
		n++
		t.Run(tc.Descricao, func(t *testing.T) {
			assert.Equal(t, tc.Normalizado, termoBuscaCNPJ(mascarar(tc.Normalizado)))
			assert.Equal(t, tc.Normalizado, termoBuscaCNPJ(strings.ToLower(mascarar(tc.Normalizado))))
			// Prefixo mascarado (busca parcial enquanto digita).
			assert.Equal(t, tc.Normalizado[:8], termoBuscaCNPJ(strings.ToLower(mascarar(tc.Normalizado)[:10])))
			// Sem máscara: devolve "" e a busca usa q.
			assert.Equal(t, "", termoBuscaCNPJ(tc.Normalizado))
		})
	}
	require.Greater(t, n, 0)
}

func TestTermoBuscaCNPJ_Bordas(t *testing.T) {
	cases := []struct{ nome, q, want string }{
		{"só máscara vira termo vazio", "./-", ""},
		{"razão social com hífen e acento não vira termo", "Perfumaria São-Paulo", ""},
		{"razão social com hífen sem acento vira termo (inofensivo: não casa CNPJ)", "Rota-Perfumes", "ROTAPERFUMES"},
		{"símbolo inválido com máscara", "12.ABC#", ""},
		{"máscara com espaço interno", "11 222.333", "11222333"},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			assert.Equal(t, tc.want, termoBuscaCNPJ(tc.q))
		})
	}
}
