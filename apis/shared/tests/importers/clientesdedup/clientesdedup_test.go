package clientesdedup_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/importers/clientesdedup"
)

func TestCalcular(t *testing.T) {
	cases := []struct {
		nome string
		regs []clientesdedup.Registro
		want clientesdedup.Unificacao
	}{
		{"sem duplicados", []clientesdedup.Registro{{ClienteID: 1, CNPJ: "11222333000181"}, {ClienteID: 2, CNPJ: "11444777000161"}}, clientesdedup.Unificacao{}},
		{
			"mantém a 1ª ocorrência (máscara não importa)",
			[]clientesdedup.Registro{{ClienteID: 5, CNPJ: "11.222.333/0001-81"}, {ClienteID: 3, CNPJ: "11222333000181"}, {ClienteID: 9, CNPJ: " 11222333000181 "}},
			clientesdedup.Unificacao{3: 5, 9: 5},
		},
		{"CNPJ vazio é ignorado", []clientesdedup.Registro{{ClienteID: 1, CNPJ: ""}, {ClienteID: 2, CNPJ: "--"}}, clientesdedup.Unificacao{}},
		{
			"NEG-02: alfanumérico unifica ignorando máscara e caixa",
			[]clientesdedup.Registro{{ClienteID: 1, CNPJ: "12ABC34501DE35"}, {ClienteID: 2, CNPJ: "12.abc.345/01de-35"}},
			clientesdedup.Unificacao{2: 1},
		},
		{"CNPJ fora do formato é ignorado", []clientesdedup.Registro{{ClienteID: 1, CNPJ: "12ABC34501DEA5"}, {ClienteID: 2, CNPJ: "12ABC34501DEA5"}}, clientesdedup.Unificacao{}},
		{"mesmo id repetido não vira cópia de si mesmo", []clientesdedup.Registro{{ClienteID: 1, CNPJ: "11222333000181"}, {ClienteID: 1, CNPJ: "11222333000181"}}, clientesdedup.Unificacao{}},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			assert.Equal(t, tc.want, clientesdedup.Calcular(tc.regs))
		})
	}
}

func TestCanonicoEAplicarAoLookup(t *testing.T) {
	u := clientesdedup.Unificacao{3001: 10, 3002: 20}
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
		"5,12.ABC.345/01DE-35,alfanumérico",
		"3005,12abc34501de35,CÓPIA DE 5 (minúsculas)",
		"6,12ABC34501DE3#,símbolo inválido",
	}, "\n")
	u, err := clientesdedup.LerRegistros(strings.NewReader(csv))
	require.NoError(t, err)
	assert.Equal(t, clientesdedup.Unificacao{3001: 1, 3005: 5}, u)

	_, err = clientesdedup.LerRegistros(strings.NewReader(""))
	assert.Error(t, err, "sem cabeçalho")
}

func TestCarregarDoCSVERedirecionar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clientes.csv")
	require.NoError(t, os.WriteFile(path, []byte("cliente_id,cnpj\n1,11222333000181\n3001,11222333000181\n"), 0o600))

	t.Setenv(clientesdedup.EnvCSVPath, path)
	assert.Equal(t, path, clientesdedup.CaminhoCSV("/qualquer"))

	lookup := map[int64]int64{1: 1}
	u := clientesdedup.Redirecionar("teste", "/qualquer", lookup)
	assert.Equal(t, clientesdedup.Unificacao{3001: 1}, u)
	assert.Equal(t, int64(1), lookup[3001])

	t.Setenv(clientesdedup.EnvCSVPath, filepath.Join(dir, "inexistente.csv"))
	lookup = map[int64]int64{1: 1}
	u = clientesdedup.Redirecionar("teste", "/qualquer", lookup)
	assert.Empty(t, u, "CSV ausente: segue sem unificação")
	assert.Len(t, lookup, 1)

	t.Setenv(clientesdedup.EnvCSVPath, "")
	assert.Equal(t, filepath.Join("raiz", "dados", "crm", "clientes.csv"), clientesdedup.CaminhoCSV("raiz"))
}
