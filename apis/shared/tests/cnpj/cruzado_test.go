package cnpj_test

// NEG-02 (TestBrain, Lote 5): verificação cruzada back x front. A massa
// apis/shared/cnpj/testdata/casos_cruzados.json (não movida: também é lida
// pela rotaperfumes-api e pelo front) é a MESMA lida por
// frontend/tests/lib/cnpj.cruzado.test.ts; os dois lados precisam dar o mesmo
// resultado. Ela foi gerada por uma 3ª implementação de referência do DV
// (independente das duas), então nenhum dos lados "confere a si mesmo".

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/cnpj"
)

type casoCruzado struct {
	Descricao   string `json:"descricao"`
	Entrada     string `json:"entrada"`
	Valido      bool   `json:"valido"`
	Normalizado string `json:"normalizado"`
}

type massaCruzada struct {
	Casos        []casoCruzado `json:"casos"`
	Divergencias []casoCruzado `json:"divergencias"`
}

func carregarMassaCruzada(t *testing.T) massaCruzada {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "cnpj", "testdata", "casos_cruzados.json"))
	require.NoError(t, err)
	var m massaCruzada
	require.NoError(t, json.Unmarshal(raw, &m))
	require.NotEmpty(t, m.Casos)
	return m
}

// validarComoAPI reproduz o fluxo da API (validarClienteInput): TrimSpace,
// Normalizar, Valido.
func validarComoAPI(raw string) (string, bool) {
	n, ok := cnpj.Normalizar(strings.TrimSpace(raw))
	if !ok || !cnpj.Valido(n) {
		return "", false
	}
	return n, true
}

func TestCruzado_MassaComum(t *testing.T) {
	m := carregarMassaCruzada(t)
	validos := 0
	for _, tc := range m.Casos {
		t.Run(tc.Descricao, func(t *testing.T) {
			n, ok := validarComoAPI(tc.Entrada)
			assert.Equal(t, tc.Valido, ok, "entrada %q", tc.Entrada)
			if tc.Valido {
				assert.Equal(t, tc.Normalizado, n)
				// Os DVs calculados batem com a referência.
				d1, d2 := cnpj.DigitosVerificadores(n[:12])
				assert.Equal(t, n[12:], string(rune('0'+d1))+string(rune('0'+d2)))
			}
		})
		if tc.Valido {
			validos++
		}
	}
	assert.GreaterOrEqual(t, validos, 20, "a massa precisa de CNPJs válidos suficientes")
}

// TestCruzado_Divergencias: o backend recusa espaços não ASCII/tabulação/
// quebra de linha NO MEIO do valor e letras fora de [A-Za-z] (ex.: "ı"). O
// front divergia nesses casos até a correção NEG-02-A/B (Lote 5); hoje segue
// o mesmo contrato e a massa fica como regressão nos dois lados (ver
// cnpj.cruzado.test.ts, "ex-divergencias").
func TestCruzado_Divergencias(t *testing.T) {
	m := carregarMassaCruzada(t)
	require.NotEmpty(t, m.Divergencias)
	for _, tc := range m.Divergencias {
		t.Run(tc.Descricao, func(t *testing.T) {
			_, ok := validarComoAPI(tc.Entrada)
			assert.Equal(t, tc.Valido, ok, "entrada %q", tc.Entrada)
		})
	}
}
