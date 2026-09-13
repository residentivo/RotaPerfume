// Package services (shared) expõe utilitários reutilizáveis tanto pela API
// quanto por CLIs (ex: cmd/seedusers, cmd/resetpassword).
package services

import (
	"crypto/rand"
	"fmt"
)

// Conjuntos de caracteres usados na geração de senha aleatória. Cada conjunto
// contribui com pelo menos 1 caractere garantido, para satisfazer políticas
// de senha que exigem maiúscula + minúscula + dígito + símbolo.
const (
	minusculas = "abcdefghijklmnopqrstuvwxyz"
	maiusculas = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitos    = "0123456789"
	simbolos   = "!@#$%&*+-=?"

	// senhaAleatoriaMinLen é o tamanho mínimo aceito por GerarSenhaAleatoria,
	// alinhado com a política de senha do sistema.
	senhaAleatoriaMinLen = 12
)

var alfabetoCompleto = minusculas + maiusculas + digitos + simbolos

// GerarSenhaAleatoria gera uma senha aleatória criptograficamente segura
// (crypto/rand — nunca math/rand) com pelo menos 12 caracteres, contendo
// obrigatoriamente letra minúscula, letra maiúscula, dígito e símbolo.
//
// n é o tamanho desejado; se menor que 12, é ajustado para 12.
func GerarSenhaAleatoria(n int) (string, error) {
	if n < senhaAleatoriaMinLen {
		n = senhaAleatoriaMinLen
	}

	senha := make([]byte, n)

	// Garante 1 caractere de cada conjunto obrigatório nas primeiras posições.
	obrigatorios := []string{minusculas, maiusculas, digitos, simbolos}
	for i, conjunto := range obrigatorios {
		c, err := charAleatorio(conjunto)
		if err != nil {
			return "", fmt.Errorf("services: gerar senha aleatória: %w", err)
		}
		senha[i] = c
	}

	// Preenche o restante com o alfabeto completo.
	for i := len(obrigatorios); i < n; i++ {
		c, err := charAleatorio(alfabetoCompleto)
		if err != nil {
			return "", fmt.Errorf("services: gerar senha aleatória: %w", err)
		}
		senha[i] = c
	}

	// Embaralha para que os caracteres obrigatórios não fiquem sempre no início.
	if err := embaralhar(senha); err != nil {
		return "", fmt.Errorf("services: gerar senha aleatória: %w", err)
	}

	return string(senha), nil
}

// charAleatorio escolhe um caractere aleatório de um conjunto usando crypto/rand.
func charAleatorio(conjunto string) (byte, error) {
	b := make([]byte, 1)
	if _, err := rand.Read(b); err != nil {
		return 0, err
	}
	return conjunto[int(b[0])%len(conjunto)], nil
}

// embaralhar aplica um Fisher-Yates shuffle usando crypto/rand como fonte
// de aleatoriedade (evita viés previsível de math/rand).
func embaralhar(b []byte) error {
	for i := len(b) - 1; i > 0; i-- {
		jBytes := make([]byte, 1)
		if _, err := rand.Read(jBytes); err != nil {
			return err
		}
		j := int(jBytes[0]) % (i + 1)
		b[i], b[j] = b[j], b[i]
	}
	return nil
}
