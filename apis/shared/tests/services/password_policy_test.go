package services_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/rotaperfumes/shared/services"
)

// TestValidarForcaSenha cobre a política de senha forte (services.ValidarForcaSenha)
// via casos de tabela: tamanho mínimo/máximo e número mínimo de classes de
// caractere (minúscula, maiúscula, dígito, símbolo — mesmo conjunto de
// símbolos usado por GerarSenhaAleatoria).
func TestValidarForcaSenha(t *testing.T) {
	testCases := []struct {
		nome    string
		senha   string
		wantErr error
	}{
		{
			nome:    "senha válida com 3 classes (minúscula+maiúscula+dígito) passa",
			senha:   "Senha1234",
			wantErr: nil,
		},
		{
			nome:    "senha válida com as 4 classes passa",
			senha:   "Senha123!",
			wantErr: nil,
		},
		{
			nome:    "senha com exatamente 8 caracteres e 3 classes passa (limite mínimo)",
			senha:   "Abcdefg1",
			wantErr: nil,
		},
		{
			nome:    "senha curta (menos de 8 caracteres) é rejeitada",
			senha:   "Abc123!",
			wantErr: services.ErrSenhaCurta,
		},
		{
			nome:    "senha vazia é rejeitada como curta",
			senha:   "",
			wantErr: services.ErrSenhaCurta,
		},
		{
			nome:    "senha com 8 runes multi-byte (mais de 8 bytes) não é penalizada por ser contada por rune",
			senha:   "áéíóú1A!", // 8 runes (>8 bytes, já que cada acentuada ocupa 2 bytes)
			wantErr: nil,
		},
		{
			nome:    "senha curta com caracteres multi-byte (menos de 8 runes) é rejeitada",
			senha:   "áéí1A!", // 6 runes (mais de 8 bytes por conta do multi-byte, mas menos de 8 runes)
			wantErr: services.ErrSenhaCurta,
		},
		{
			nome:    "senha muito longa (>72 bytes) é rejeitada",
			senha:   strings.Repeat("Aa1!", 20), // 80 bytes
			wantErr: services.ErrSenhaMuitoLonga,
		},
		{
			nome:    "senha no limite exato de 72 bytes com 3 classes passa",
			senha:   strings.Repeat("a", 69) + "A1!", // 72 bytes
			wantErr: nil,
		},
		{
			nome:    "senha com apenas 1 classe (só minúscula) é fraca",
			senha:   "abcdefghij",
			wantErr: services.ErrSenhaFraca,
		},
		{
			nome:    "senha com apenas 1 classe (só dígitos) é fraca",
			senha:   "12345678",
			wantErr: services.ErrSenhaFraca,
		},
		{
			nome:    "senha com exatamente 2 classes (minúscula+dígito) é fraca",
			senha:   "abcdefgh123",
			wantErr: services.ErrSenhaFraca,
		},
		{
			nome:    "senha com exatamente 2 classes (maiúscula+símbolo) é fraca",
			senha:   "ABCDEFGH!!!!",
			wantErr: services.ErrSenhaFraca,
		},
		{
			nome:    "senha com exatamente 3 classes (minúscula+maiúscula+símbolo, sem dígito) passa",
			senha:   "SenhaForte!",
			wantErr: nil,
		},
		{
			nome:    "símbolo fora do conjunto reconhecido não conta como classe símbolo",
			senha:   "abcdefgh123_", // '_' não está em simbolos ("!@#$%&*+-=?")
			wantErr: services.ErrSenhaFraca,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.nome, func(t *testing.T) {
			err := services.ValidarForcaSenha(tc.senha)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("esperava nil, obteve: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("esperava erro %v, obteve: %v", tc.wantErr, err)
			}
		})
	}
}
