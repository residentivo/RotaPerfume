package services_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/services"
)

var (
	temMinuscula = regexp.MustCompile(`[a-z]`)
	temMaiuscula = regexp.MustCompile(`[A-Z]`)
	temDigito    = regexp.MustCompile(`[0-9]`)
	temSimbolo   = regexp.MustCompile(`[!@#$%&*+\-=?]`)
)

// TestGerarSenhaAleatoria_TamanhoMinimo cobre o piso de 12 caracteres,
// parametrizado com tamanhos solicitados abaixo, igual e acima do mínimo.
func TestGerarSenhaAleatoria_TamanhoMinimo(t *testing.T) {
	testCases := []struct {
		nome       string
		tamanho    int
		wantLength int
	}{
		{"tamanho 0 é ajustado para 12", 0, 12},
		{"tamanho negativo é ajustado para 12", -5, 12},
		{"tamanho 5 (abaixo do mínimo) é ajustado para 12", 5, 12},
		{"tamanho 12 (exatamente o mínimo) mantém 12", 12, 12},
		{"tamanho 16 (acima do mínimo) mantém 16", 16, 16},
		{"tamanho 32 mantém 32", 32, 32},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			senha, err := services.GerarSenhaAleatoria(tc.tamanho)
			require.NoError(t, err)
			assert.Len(t, senha, tc.wantLength)
		})
	}
}

// TestGerarSenhaAleatoria_PoliticaDeSenha garante que toda senha gerada
// contém ao menos 1 minúscula, 1 maiúscula, 1 dígito e 1 símbolo.
func TestGerarSenhaAleatoria_PoliticaDeSenha(t *testing.T) {
	// Gera várias senhas para reduzir a chance de falso-positivo por sorte.
	for i := 0; i < 50; i++ {
		senha, err := services.GerarSenhaAleatoria(16)
		require.NoError(t, err)
		assert.True(t, temMinuscula.MatchString(senha), "senha deve conter minúscula: %s", senha)
		assert.True(t, temMaiuscula.MatchString(senha), "senha deve conter maiúscula: %s", senha)
		assert.True(t, temDigito.MatchString(senha), "senha deve conter dígito: %s", senha)
		assert.True(t, temSimbolo.MatchString(senha), "senha deve conter símbolo: %s", senha)
	}
}

// TestGerarSenhaAleatoria_Aleatoriedade garante que chamadas sucessivas
// produzem senhas diferentes (crypto/rand não deve ser determinístico).
func TestGerarSenhaAleatoria_Aleatoriedade(t *testing.T) {
	senha1, err := services.GerarSenhaAleatoria(16)
	require.NoError(t, err)
	senha2, err := services.GerarSenhaAleatoria(16)
	require.NoError(t, err)
	assert.NotEqual(t, senha1, senha2, "duas gerações sucessivas não devem coincidir")
}
