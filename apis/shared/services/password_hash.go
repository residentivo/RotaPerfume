// Package services (shared) — hash de senha Argon2id com pepper (SEC-13).
//
// Formato gravado em usuarios.password_hash (PHC string):
//
//	$argon2id$v=19$m=<KiB>,t=<iterações>,p=<paralelismo>$<salt b64>$<hash b64>
//
// A entrada do Argon2id não é a senha pura: é HMAC-SHA256(pepper, senha).
// O pepper (PASSWORD_PEPPER) fica só no ambiente, fora do banco — quem vazar
// apenas o banco não consegue testar senhas offline. Usar o pepper como chave
// de HMAC equivale a acrescentá-lo à senha, sem a ambiguidade de concatenação.
//
// Hashes bcrypt legados ($2a$/$2b$/$2y$) continuam aceitos por VerificarSenha
// (sem pepper, como foram gerados) e são marcados por PrecisaRehash para
// serem trocados por Argon2id no próximo login.
package services

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"

	"github.com/rotaperfumes/shared/config"
)

const (
	argon2SaltBytes  = 16
	argon2ChaveBytes = 32
	prefixoArgon2id  = "$argon2id$"
)

// ErrPepperNaoConfigurado indica HashSenha sem pepper (Config montada à mão
// sem passar por config.Load). Nunca geramos hash sem pepper.
var ErrPepperNaoConfigurado = errors.New("auth: pepper de senha não configurado")

// hashArgon2 é um hash Argon2id decodificado do formato PHC.
type hashArgon2 struct {
	memoria, iteracoes uint32
	paralelismo        uint8
	salt, chave        []byte
}

// GerarHashSenha gera o hash Argon2id (formato PHC) de senha com o pepper e
// os parâmetros de p.
func GerarHashSenha(p config.HashSenha, senha string) (string, error) {
	if p.Pepper == "" {
		return "", ErrPepperNaoConfigurado
	}
	if p.MemoriaKiB == 0 || p.Iteracoes == 0 || p.Paralelismo == 0 {
		return "", fmt.Errorf("auth: parâmetros Argon2id inválidos (m=%d t=%d p=%d)", p.MemoriaKiB, p.Iteracoes, p.Paralelismo)
	}
	salt := make([]byte, argon2SaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: gerar salt: %w", err)
	}
	chave := argon2.IDKey(aplicarPepper(p.Pepper, senha), salt, p.Iteracoes, p.MemoriaKiB, p.Paralelismo, argon2ChaveBytes)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("%sv=%d$m=%d,t=%d,p=%d$%s$%s",
		prefixoArgon2id, argon2.Version, p.MemoriaKiB, p.Iteracoes, p.Paralelismo,
		b64.EncodeToString(salt), b64.EncodeToString(chave)), nil
}

// VerificarSenha confere senha contra hash: Argon2id com pepper ou bcrypt
// legado (sem pepper). Hash vazio, malformado ou desconhecido devolve false.
func VerificarSenha(p config.HashSenha, hash, senha string) bool {
	if ehBcrypt(hash) {
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(senha)) == nil
	}
	if p.Pepper == "" {
		return false
	}
	h, err := decodificarArgon2(hash)
	if err != nil {
		return false
	}
	calc := argon2.IDKey(aplicarPepper(p.Pepper, senha), h.salt, h.iteracoes, h.memoria, h.paralelismo, uint32(len(h.chave)))
	return subtle.ConstantTimeCompare(calc, h.chave) == 1
}

// PrecisaRehash indica se hash deve ser regravado com os parâmetros atuais:
// bcrypt legado ou Argon2id com parâmetros diferentes de p.
func PrecisaRehash(p config.HashSenha, hash string) bool {
	if ehBcrypt(hash) {
		return true
	}
	h, err := decodificarArgon2(hash)
	if err != nil {
		return false // placeholder/desconhecido: não é caso de re-hash
	}
	return h.memoria != p.MemoriaKiB || h.iteracoes != p.Iteracoes || h.paralelismo != p.Paralelismo
}

// aplicarPepper devolve HMAC-SHA256(pepper, senha), a entrada do Argon2id.
func aplicarPepper(pepper, senha string) []byte {
	mac := hmac.New(sha256.New, []byte(pepper))
	mac.Write([]byte(senha))
	return mac.Sum(nil)
}

func ehBcrypt(hash string) bool {
	return strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$") || strings.HasPrefix(hash, "$2y$")
}

// decodificarArgon2 interpreta um hash PHC $argon2id$v=19$m=..,t=..,p=..$salt$chave.
func decodificarArgon2(hash string) (*hashArgon2, error) {
	partes := strings.Split(hash, "$")
	if len(partes) != 6 || partes[1] != "argon2id" {
		return nil, errors.New("auth: hash não é argon2id")
	}
	var versao int
	if _, err := fmt.Sscanf(partes[2], "v=%d", &versao); err != nil || versao != argon2.Version {
		return nil, errors.New("auth: versão argon2 incompatível")
	}
	h := &hashArgon2{}
	if _, err := fmt.Sscanf(partes[3], "m=%d,t=%d,p=%d", &h.memoria, &h.iteracoes, &h.paralelismo); err != nil {
		return nil, fmt.Errorf("auth: parâmetros argon2 malformados: %w", err)
	}
	if h.memoria == 0 || h.iteracoes == 0 || h.paralelismo == 0 {
		return nil, errors.New("auth: parâmetros argon2 zerados")
	}
	var err error
	b64 := base64.RawStdEncoding
	if h.salt, err = b64.DecodeString(partes[4]); err != nil || len(h.salt) == 0 {
		return nil, errors.New("auth: salt argon2 malformado")
	}
	if h.chave, err = b64.DecodeString(partes[5]); err != nil || len(h.chave) == 0 {
		return nil, errors.New("auth: hash argon2 malformado")
	}
	return h, nil
}
