package identidadecrypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/rotaperfumes/shared/identidade"
)

// Alfabetos dos geradores. O conjunto de símbolos é o da política de senha
// (identidade.SimbolosSenha).
const (
	minusculas    = "abcdefghijklmnopqrstuvwxyz"
	maiusculas    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitos       = "0123456789"
	alfabetoSenha = minusculas + maiusculas + digitos + identidade.SimbolosSenha
	alfanumerico  = minusculas + maiusculas + digitos

	// SenhaAleatoriaMinLen é o mínimo de GerarSenhaAleatoria.
	SenhaAleatoriaMinLen = 12
	// TamanhoSenhaGerada é a senha de criação de usuário e de reset (admin
	// e em massa).
	TamanhoSenhaGerada = 16
	// BytesRefresh é o tamanho do refresh token (hex no cookie: 64 chars).
	BytesRefresh = 32
)

// GerarSenhaAleatoria gera uma senha de n caracteres (mínimo 12) com
// crypto/rand, contendo minúscula, maiúscula, dígito e símbolo, embaralhada
// (Fisher-Yates), sem viés de módulo (SEC-10).
func GerarSenhaAleatoria(n int) (string, error) {
	if n < SenhaAleatoriaMinLen {
		n = SenhaAleatoriaMinLen
	}
	senha := make([]byte, n)
	obrigatorios := []string{minusculas, maiusculas, digitos, identidade.SimbolosSenha}
	for i := range senha {
		conjunto := alfabetoSenha
		if i < len(obrigatorios) {
			conjunto = obrigatorios[i]
		}
		c, err := caractereAleatorio(conjunto)
		if err != nil {
			return "", err
		}
		senha[i] = c
	}
	if err := embaralhar(senha); err != nil {
		return "", err
	}
	return string(senha), nil
}

// SenhaAlfanumerica gera n caracteres uniformes de [a-zA-Z0-9] (CLIs de
// seed/reset). n <= 0 devolve "".
func SenhaAlfanumerica(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}
	senha := make([]byte, n)
	for i := range senha {
		c, err := caractereAleatorio(alfanumerico)
		if err != nil {
			return "", err
		}
		senha[i] = c
	}
	return string(senha), nil
}

// GeradorSenha é a porta identidadeapp.GeradorSenha: senha de 16 caracteres.
type GeradorSenha struct{}

// Gerar devolve uma senha aleatória forte.
func (GeradorSenha) Gerar() (identidade.Senha, error) {
	s, err := GerarSenhaAleatoria(TamanhoSenhaGerada)
	if err != nil {
		return identidade.Senha{}, err
	}
	return identidade.NovaSenha(s), nil
}

// GeradorRefresh é a porta identidadeapp.GeradorRefresh: token opaco de 32
// bytes aleatórios (hex) e o hash SHA-256 que vai para o banco (I-06).
type GeradorRefresh struct{}

// Novo gera um refresh token.
func (GeradorRefresh) Novo() identidade.TokenOpaco {
	b := make([]byte, BytesRefresh)
	_, _ = rand.Read(b) // crypto/rand.Read nunca falha (Go 1.24+)
	return identidade.NovoTokenOpaco(hex.EncodeToString(b))
}

// Hash devolve o SHA-256 (hex) do token, a chave de busca no banco.
func (GeradorRefresh) Hash(t identidade.TokenOpaco) string {
	sum := sha256.Sum256([]byte(t.Revelar()))
	return hex.EncodeToString(sum[:])
}

func caractereAleatorio(conjunto string) (byte, error) {
	i, err := indiceAleatorio(len(conjunto))
	if err != nil {
		return 0, err
	}
	return conjunto[i], nil
}

// indiceAleatorio é uniforme em [0, n) via crypto/rand.Int (sem viés).
func indiceAleatorio(n int) (int, error) {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, fmt.Errorf("identidadecrypto: aleatório: %w", err)
	}
	return int(v.Int64()), nil
}

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
