package services_test

// SEC-10: gerador de senhas sem viés de módulo (indiceAleatorio com
// crypto/rand.Int). Testes de distribuição com margens largas o bastante para
// nunca oscilar com um gerador uniforme, mas que pegam o viés do antigo
// byte%n (com n=62, os 8 primeiros símbolos saíam com 5/256 contra 4/256).

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/services"
)

const (
	sec10Minusculas  = "abcdefghijklmnopqrstuvwxyz"
	sec10Maiusculas  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	sec10Digitos     = "0123456789"
	sec10Simbolos    = "!@#$%&*+-=?"
	sec10Alfanumeric = sec10Minusculas + sec10Maiusculas + sec10Digitos
)

func TestSEC10_SenhaAlfanumerica_TamanhoEAlfabeto(t *testing.T) {
	casos := []struct {
		nome string
		n    int
		want int
	}{
		{"negativo devolve vazio", -5, 0},
		{"zero devolve vazio", 0, 0},
		{"1", 1, 1},
		{"16", 16, 16},
		{"62", 62, 62},
		{"256", 256, 256},
		{"1000", 1000, 1000},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			s, err := services.SenhaAlfanumerica(c.n)
			require.NoError(t, err)
			assert.Len(t, s, c.want)
			for _, r := range s {
				assert.True(t, strings.ContainsRune(sec10Alfanumeric, r), "caractere fora de [a-zA-Z0-9]: %q", r)
			}
		})
	}
	t.Run("duas chamadas não se repetem", func(t *testing.T) {
		a, err := services.SenhaAlfanumerica(32)
		require.NoError(t, err)
		b, err := services.SenhaAlfanumerica(32)
		require.NoError(t, err)
		assert.NotEqual(t, a, b)
	})
}

// TestSEC10_SenhaAlfanumerica_Distribuicao: 62.000 caracteres (média 1000
// por símbolo; desvio-padrão ~31). ±40% = 12 desvios: nunca falha por acaso.
// O qui-quadrado (61 g.l., média 61, dp ~11) com limite 150 tem p < 1e-8 sob
// uniformidade; o antigo byte%62 daria ~400 em média.
func TestSEC10_SenhaAlfanumerica_Distribuicao(t *testing.T) {
	const total = 62000
	s, err := services.SenhaAlfanumerica(total)
	require.NoError(t, err)
	require.Len(t, s, total)

	cont := map[byte]int{}
	for i := 0; i < len(s); i++ {
		cont[s[i]]++
	}
	require.Len(t, cont, len(sec10Alfanumeric), "todos os 62 símbolos aparecem")

	media := float64(total) / float64(len(sec10Alfanumeric))
	qui := 0.0
	for i := 0; i < len(sec10Alfanumeric); i++ {
		c := sec10Alfanumeric[i]
		obs := float64(cont[c])
		assert.InDelta(t, media, obs, media*0.40, "símbolo %q fora de ±40%% da média", c)
		qui += (obs - media) * (obs - media) / media
	}
	assert.Less(t, qui, 150.0, "qui-quadrado alto: indício de viés na distribuição")
}

func TestSEC10_GerarSenhaAleatoria_TamanhosGrandes(t *testing.T) {
	casos := []struct {
		n, want int
	}{
		{0, 12}, {11, 12}, {12, 12}, {255, 255}, {256, 256}, {257, 257}, {300, 300}, {1024, 1024},
	}
	for _, c := range casos {
		t.Run("", func(t *testing.T) {
			var s string
			var err error
			require.NotPanics(t, func() { s, err = services.GerarSenhaAleatoria(c.n) })
			require.NoError(t, err)
			assert.Len(t, s, c.want)
			assert.True(t, strings.ContainsAny(s, sec10Minusculas), "sem minúscula")
			assert.True(t, strings.ContainsAny(s, sec10Maiusculas), "sem maiúscula")
			assert.True(t, strings.ContainsAny(s, sec10Digitos), "sem dígito")
			assert.True(t, strings.ContainsAny(s, sec10Simbolos), "sem símbolo")
			for _, r := range s {
				assert.True(t, strings.ContainsRune(sec10Alfanumeric+sec10Simbolos, r), "caractere fora do alfabeto: %q", r)
			}
		})
	}
}

// TestSEC10_GerarSenhaAleatoria_ObrigatoriosEmbaralhados: os 4 caracteres
// obrigatórios são gravados nas posições 0..3 e depois embaralhados. Em 2000
// senhas de 12, a posição 0 deve ser minúscula em ~ (1/12 + 11/12*26/73)
// ≈ 41% dos casos; sem embaralhar seria 100%. Limite 60% (folga enorme).
func TestSEC10_GerarSenhaAleatoria_ObrigatoriosEmbaralhados(t *testing.T) {
	const rodadas = 2000
	pos0Minuscula, pos3Simbolo := 0, 0
	for i := 0; i < rodadas; i++ {
		s, err := services.GerarSenhaAleatoria(12)
		require.NoError(t, err)
		if strings.ContainsRune(sec10Minusculas, rune(s[0])) {
			pos0Minuscula++
		}
		if strings.ContainsRune(sec10Simbolos, rune(s[3])) {
			pos3Simbolo++
		}
	}
	assert.Less(t, pos0Minuscula, rodadas*60/100, "posição 0 quase sempre minúscula: embaralhamento não ocorre")
	assert.Less(t, pos3Simbolo, rodadas*50/100, "posição 3 quase sempre símbolo: embaralhamento não ocorre")
}
