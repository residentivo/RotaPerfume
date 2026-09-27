// Package services (shared) expõe utilitários reutilizáveis tanto pela API
// quanto por CLIs (ex: cmd/seedusers, cmd/resetpassword).
package services

import (
	"crypto/rand"
	"fmt"
	"math/big"
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

// alfanumerico é o alfabeto de SenhaAlfanumerica ([a-zA-Z0-9]).
const alfanumerico = minusculas + maiusculas + digitos

// SenhaAlfanumerica gera uma senha de n caracteres do alfabeto [a-zA-Z0-9]
// com distribuição uniforme (crypto/rand, sem viés de módulo). Usada pelas
// CLIs de seed/reset (tools/seedusers, tools/resetpassword) quando nenhuma
// senha é informada. n <= 0 devolve string vazia.
func SenhaAlfanumerica(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	senha := make([]byte, n)
	for i := range senha {
		c, err := charAleatorio(alfanumerico)
		if err != nil {
			return "", fmt.Errorf("services: gerar senha alfanumérica: %w", err)
		}
		senha[i] = c
	}
	return string(senha), nil
}

// indiceAleatorio devolve um inteiro uniforme em [0, n) via crypto/rand.Int
// (SEC-10). Substitui o antigo byte%n, que tinha viés de módulo: com 256
// valores de byte e n que não divide 256 (ex.: 62), os primeiros 256%n
// índices saíam com probabilidade maior. n deve ser > 0.
func indiceAleatorio(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("services: indiceAleatorio: n inválido (%d)", n)
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

// charAleatorio escolhe um caractere uniforme de um conjunto usando crypto/rand.
func charAleatorio(conjunto string) (byte, error) {
	i, err := indiceAleatorio(len(conjunto))
	if err != nil {
		return 0, err
	}
	return conjunto[i], nil
}

// embaralhar aplica um Fisher-Yates shuffle usando crypto/rand como fonte
// de aleatoriedade (evita viés previsível de math/rand e, via
// indiceAleatorio, o viés de módulo).
func embaralhar(b []byte) error {
	for i := len(b) - 1; i > 0; i-- {
		j, err := indiceAleatorio(i + 1)
		if err != nil {
			return err
		}
		b[i], b[j] = b[j], b[i]
	}
	return nil
}
