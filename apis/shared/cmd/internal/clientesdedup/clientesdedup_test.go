package clientesdedup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalcular(t *testing.T) {
	cases := []struct {
		nome string
		regs []Registro
		want Unificacao
	}{
		{"sem duplicados", []Registro{{1, "11222333000181"}, {2, "11444777000161"}}, Unificacao{}},
		{
			"mantém a 1ª ocorrência (máscara não importa)",
			[]Registro{{5, "11.222.333/0001-81"}, {3, "11222333000181"}, {9, " 11222333000181 "}},
			Unificacao{3: 5, 9: 5},
		},
		{"CNPJ vazio é ignorado", []Registro{{1, ""}, {2, "--"}}, Unificacao{}},
		{"mesmo id repetido não vira cópia de si mesmo", []Registro{{1, "11222333000181"}, {1, "11222333000181"}}, Unificacao{}},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			assert.Equal(t, tc.want, Calcular(tc.regs))
		})
	}
}

func TestCanonicoEAplicarAoLookup(t *testing.T) {
	u := Unificacao{3001: 10, 3002: 20}
	assert.Equal(t, int64(10), u.Canonico(3001))
	assert.Equal(t, int64(7), u.Canonico(7))

	// 20 não existe no banco → a cópia 3002 não é redirecionada.
	lookup := map[int64]int64{10: 10, 7: 7}
	n := u.AplicarAoLookup(lookup)
	assert.Equal(t, 1, n)
	assert.Equal(t, int64(10), lookup[3001])
	_, ok := lookup[3002]
	assert.False(t, ok)
}

func TestLerRegistros(t *testing.T) {
	csv := strings.Join([]string{
		"cliente_id,cnpj,razao_social",
		"1, 11222333000181 ,A",
		"2,11444777000161,B",
		"x,11222333000181,id inválido",
		"4,123,cnpj curto",
		"3001,11.222.333/0001-81,CÓPIA DE A",
		"3002",
	}, "\n")
	u, err := lerRegistros(strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, Unificacao{3001: 1}, u)

	_, err = lerRegistros(strings.NewReader(""))
	assert.Error(t, err, "sem cabeçalho")
}

func TestCarregarDoCSVERedirecionar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clientes.csv")
	require.NoError(t, os.WriteFile(path, []byte("cliente_id,cnpj\n1,11222333000181\n3001,11222333000181\n"), 0o600))

	t.Setenv(EnvCSVPath, path)
	assert.Equal(t, path, CaminhoCSV("/qualquer"))

	lookup := map[int64]int64{1: 1}
	u := Redirecionar("teste", "/qualquer", lookup)
	assert.Equal(t, Unificacao{3001: 1}, u)
	assert.Equal(t, int64(1), lookup[3001])

	t.Setenv(EnvCSVPath, filepath.Join(dir, "inexistente.csv"))
	lookup = map[int64]int64{1: 1}
	u = Redirecionar("teste", "/qualquer", lookup)
	assert.Empty(t, u, "CSV ausente: segue sem unificação")
	assert.Len(t, lookup, 1)

	t.Setenv(EnvCSVPath, "")
	assert.Equal(t, filepath.Join("raiz", "dados", "crm", "clientes.csv"), CaminhoCSV("raiz"))
}
