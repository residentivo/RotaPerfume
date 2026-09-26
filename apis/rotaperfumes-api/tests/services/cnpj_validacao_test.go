package services_test

// NEG-02: validação de CNPJ numérico e alfanumérico na camada de serviço,
// exercitada pela API pública do ClienteService:
//   - CreateCliente (TrimSpace + normalização + DV) grava o CNPJ sem máscara
//     e em maiúsculas, ou recusa com ErrCNPJInvalido antes de tocar o banco;
//   - ListClientes (?q=) converte um CNPJ digitado com máscara no termo de
//     busca gravado no banco (cnpj LIKE %termo%).
//
// A massa comum com o front (shared/cnpj/testdata/casos_cruzados.json) é a
// mesma usada por frontend/src/lib/cnpj.cruzado.test.ts.

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
)

// cnpjCriarCliente chama CreateCliente com o CNPJ informado e devolve o CNPJ
// que chegou ao INSERT ("" se o INSERT não foi executado).
func cnpjCriarCliente(t *testing.T, raw string) (gravado string, err error) {
	t.Helper()
	db, mock := dlNewDB(t)
	mock.ExpectExec(`INSERT INTO clientes`).
		WithArgs(dlCapturaTexto{&gravado}, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), true).
		WillReturnResult(sqlmock.NewResult(1, 1))
	// A releitura (BUG-08) não tem expectativa: o service devolve o objeto em memória.
	c, err := services.NewClienteService(db, &config.Config{}).CreateCliente(context.Background(), db, services.ClienteInput{
		CNPJ: raw, RazaoSocial: "Cliente", Segmento: "Varejo", Cidade: "Curitiba", UF: "PR",
	})
	if err == nil {
		require.NoError(t, mock.ExpectationsWereMet())
		assert.Equal(t, gravado, c.CNPJ, "objeto devolvido = valor gravado")
	}
	return gravado, err
}

// cnpjTermoBusca chama ListClientes com ?q= e devolve o termo usado em
// "cnpj LIKE ?" (sem os curingas %).
func cnpjTermoBusca(t *testing.T, q string) string {
	t.Helper()
	db, mock := dlNewDB(t)
	var razao, termo string
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM clientes WHERE \(razao_social LIKE \? OR cnpj LIKE \?\)`).
		WithArgs(dlCapturaTexto{&razao}, dlCapturaTexto{&termo}).
		WillReturnError(errDLParar)
	_, _, err := services.NewClienteService(db, &config.Config{}).ListClientes(context.Background(), db, 1, 10, services.ClienteFiltro{Q: q})
	require.ErrorIs(t, err, errDLParar)
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, "%"+q+"%", razao, "razão social busca sempre o q original")
	require.True(t, strings.HasPrefix(termo, "%") && strings.HasSuffix(termo, "%"), "termo %q", termo)
	return strings.TrimSuffix(strings.TrimPrefix(termo, "%"), "%")
}

func TestValidacaoCNPJ_CreateCliente(t *testing.T) {
	cases := []struct {
		nome, in, normalizado string
		valido                bool
	}{
		{"numérico válido", "11222333000181", "11222333000181", true},
		{"numérico válido (Banco do Brasil)", "00000000000191", "00000000000191", true},
		{"numérico com máscara", "11.222.333/0001-81", "11222333000181", true},
		{"numérico com espaços", " 11 222 333 0001 81 ", "11222333000181", true},
		{"alfanumérico válido (exemplo oficial da Receita)", "12ABC34501DE35", "12ABC34501DE35", true},
		{"alfanumérico com máscara", "12.ABC.345/01DE-35", "12ABC34501DE35", true},
		{"minúsculas normalizadas", "12abc34501de35", "12ABC34501DE35", true},
		{"minúsculas com máscara", "12.abc.345/01de-35", "12ABC34501DE35", true},
		{"numérico com DV errado", "11222333000182", "", false},
		{"alfanumérico com 1º DV errado", "12ABC34501DE45", "", false},
		{"alfanumérico com 2º DV errado", "12ABC34501DE36", "", false},
		{"fictício do seed (DV errado)", "12345678000199", "", false},
		{"letra na posição do 1º DV", "12ABC34501DEA5", "", false},
		{"letra na posição do 2º DV", "11.222.333/0001-8A", "", false},
		{"13 caracteres", "12ABC34501DE3", "", false},
		{"15 caracteres", "112223330001810", "", false},
		{"só máscara", "../-", "", false},
		{"símbolo inválido", "11222333000181#", "", false},
		{"todos iguais (zeros)", "00000000000000", "", false},
		{"todos iguais com máscara", "11.111.111/1111-11", "", false},
		{"todos iguais letras", "AAAAAAAAAAAAAA", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			gravado, err := cnpjCriarCliente(t, tc.in)
			if tc.valido {
				require.NoError(t, err)
				assert.Equal(t, tc.normalizado, gravado)
				return
			}
			assert.ErrorIs(t, err, services.ErrCNPJInvalido)
			assert.Empty(t, gravado, "CNPJ inválido não pode chegar ao INSERT")
		})
	}
}

func TestValidacaoCNPJ_VazioEhObrigatorio(t *testing.T) {
	for _, in := range []string{"", "   "} {
		t.Run("["+in+"]", func(t *testing.T) {
			gravado, err := cnpjCriarCliente(t, in)
			assert.ErrorIs(t, err, services.ErrCNPJObrigatorio)
			assert.Empty(t, gravado)
		})
	}
}

func TestTermoBuscaCNPJ_ListClientes(t *testing.T) {
	cases := []struct {
		nome, q, want string // want "" = busca por cnpj usa o próprio q
	}{
		{"sem máscara usa o próprio q", "Perfumaria", ""},
		{"dígitos sem máscara usa o próprio q", "11222333", ""},
		{"máscara numérica", "11.222.333/0001-81", "11222333000181"},
		{"prefixo mascarado", "11.222", "11222"},
		{"alfanumérico minúsculo com máscara", "12.abc.345", "12ABC345"},
		{"razão social com barra e espaço", "Perfumes S/A", "PERFUMESSA"},
		{"caractere que não é de CNPJ", "Ótica S/A", ""},
		{"só máscara", "./-", ""},
		{"razão social com hífen e acento não vira termo", "Perfumaria São-Paulo", ""},
		{"razão social com hífen sem acento vira termo (inofensivo: não casa CNPJ)", "Rota-Perfumes", "ROTAPERFUMES"},
		{"símbolo inválido com máscara", "12.ABC#", ""},
		{"máscara com espaço interno", "11 222.333", "11222333"},
	}
	for _, tc := range cases {
		t.Run(tc.nome, func(t *testing.T) {
			want := tc.want
			if want == "" {
				want = tc.q
			}
			assert.Equal(t, want, cnpjTermoBusca(t, tc.q))
		})
	}
}

type casoCNPJCruzado struct {
	Descricao   string `json:"descricao"`
	Entrada     string `json:"entrada"`
	Valido      bool   `json:"valido"`
	Normalizado string `json:"normalizado"`
}

func carregarCasosCNPJCruzados(t *testing.T) (casos, divergencias []casoCNPJCruzado) {
	t.Helper()
	// tests/services -> apis/shared/cnpj/testdata
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "shared", "cnpj", "testdata", "casos_cruzados.json"))
	require.NoError(t, err)
	var m struct {
		Casos        []casoCNPJCruzado `json:"casos"`
		Divergencias []casoCNPJCruzado `json:"divergencias"`
	}
	require.NoError(t, json.Unmarshal(raw, &m))
	require.NotEmpty(t, m.Casos)
	return m.Casos, m.Divergencias
}

// TestValidacaoCNPJ_MassaCruzada: a API concorda com o front na massa comum.
func TestValidacaoCNPJ_MassaCruzada(t *testing.T) {
	casos, divergencias := carregarCasosCNPJCruzados(t)
	for _, tc := range append(casos, divergencias...) {
		t.Run(tc.Descricao, func(t *testing.T) {
			gravado, err := cnpjCriarCliente(t, tc.Entrada)
			if tc.Valido {
				require.NoError(t, err, "entrada %q", tc.Entrada)
				assert.Equal(t, tc.Normalizado, gravado)
				return
			}
			require.Error(t, err, "entrada %q", tc.Entrada)
			assert.True(t, errorsIsAny(err, services.ErrCNPJInvalido, services.ErrCNPJObrigatorio), "entrada %q: %v", tc.Entrada, err)
			assert.Empty(t, gravado)
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
			assert.Equal(t, tc.Normalizado, cnpjTermoBusca(t, mascarar(tc.Normalizado)))
			assert.Equal(t, tc.Normalizado, cnpjTermoBusca(t, strings.ToLower(mascarar(tc.Normalizado))))
			// Prefixo mascarado (busca parcial enquanto digita).
			assert.Equal(t, tc.Normalizado[:8], cnpjTermoBusca(t, strings.ToLower(mascarar(tc.Normalizado)[:10])))
			// Sem máscara: a busca usa o próprio q.
			assert.Equal(t, tc.Normalizado, cnpjTermoBusca(t, tc.Normalizado))
		})
	}
	require.Greater(t, n, 0)
}

func errorsIsAny(err error, alvos ...error) bool {
	for _, a := range alvos {
		if errors.Is(err, a) {
			return true
		}
	}
	return false
}
