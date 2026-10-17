// Package identidadecrypto implementa as portas criptográficas de
// identidadeapp (ARQ-03): hash de senha (Strategy Argon2id+pepper / bcrypt
// legado, SEC-13), JWT HS256 (I-01), refresh token opaco (I-06) e geradores
// de senha aleatória (SEC-10). Satisfaz as interfaces implicitamente, sem
// importar identidadeapp (§4.1).
package identidadecrypto

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
	"github.com/rotaperfumes/shared/identidade"
)

// Formato gravado em usuarios.password_hash (PHC string):
//
//	$argon2id$v=19$m=<KiB>,t=<iterações>,p=<paralelismo>$<salt b64>$<hash b64>
//
// A entrada do Argon2id é HMAC-SHA256(pepper, senha) (I-11): quem vazar só o
// banco não testa senhas offline. Hashes bcrypt legados ($2a$/$2b$/$2y$) são
// verificados sem pepper e marcados para rehash.
const (
	argon2SaltBytes  = 16
	argon2ChaveBytes = 32
	prefixoArgon2id  = "$argon2id$"
)

// ErrPepperNaoConfigurado: nunca se gera hash sem pepper (Config montada à
// mão sem config.Load).
var ErrPepperNaoConfigurado = errors.New("identidadecrypto: pepper de senha não configurado")

// estrategia é uma forma de hash reconhecida (Strategy, §5.6).
type estrategia interface {
	verificar(hash, senha string) bool
	precisaRehash(hash string) bool
}

// Hasher gera hashes Argon2id com os parâmetros atuais e verifica Argon2id
// e bcrypt legado. Só o Argon2id gera hash.
type Hasher struct {
	argon2 argon2idPepper
}

// NovoHasher monta o hasher com o pepper e os parâmetros da config.
func NovoHasher(p config.HashSenha) *Hasher {
	a := argon2idPepper{p: p}
	return &Hasher{argon2: a}
}

// Gerar devolve o hash Argon2id (PHC) da senha.
func (h *Hasher) Gerar(s identidade.Senha) (identidade.HashSenha, error) {
	v, err := h.argon2.gerar(s.Revelar())
	if err != nil {
		return identidade.HashSenha{}, err
	}
	return identidade.NovoHashSenha(v), nil
}

// Verificar confere a senha contra o hash, em tempo constante no Argon2id.
// Hash vazio, malformado ou desconhecido devolve false.
func (h *Hasher) Verificar(hash identidade.HashSenha, s identidade.Senha) bool {
	e := h.estrategiaDe(hash.Revelar())
	return e.verificar(hash.Revelar(), s.Revelar())
}

// PrecisaRehash: bcrypt legado ou Argon2id com parâmetros diferentes dos
// atuais. Hash malformado não é caso de rehash.
func (h *Hasher) PrecisaRehash(hash identidade.HashSenha) bool {
	return h.estrategiaDe(hash.Revelar()).precisaRehash(hash.Revelar())
}

// ExecutarDescartavel roda um Argon2id com os parâmetros atuais e descarta o
// resultado: o login de e-mail inexistente custa o mesmo que o de um
// existente (I-16).
func (h *Hasher) ExecutarDescartavel(s identidade.Senha) {
	_, _ = h.argon2.gerar(s.Revelar())
}

// estrategiaDe escolhe a Strategy pelo formato: bcrypt pelo prefixo; todo o
// resto é tratado como Argon2id (o decode falha fechado para lixo).
func (h *Hasher) estrategiaDe(hash string) estrategia {
	if (bcryptLegado{}).reconhece(hash) {
		return bcryptLegado{}
	}
	return h.argon2
}

// bcryptLegado só verifica (sem pepper, como foi gerado) e sempre pede rehash.
type bcryptLegado struct{}

func (bcryptLegado) reconhece(hash string) bool {
	return strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$") || strings.HasPrefix(hash, "$2y$")
}

func (bcryptLegado) verificar(hash, senha string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(senha)) == nil
}

func (bcryptLegado) precisaRehash(string) bool { return true }

// argon2idPepper é a estratégia atual: gera e verifica.
type argon2idPepper struct{ p config.HashSenha }

func (a argon2idPepper) gerar(senha string) (string, error) {
	if a.p.Pepper == "" {
		return "", ErrPepperNaoConfigurado
	}
	if a.p.MemoriaKiB == 0 || a.p.Iteracoes == 0 || a.p.Paralelismo == 0 {
		return "", fmt.Errorf("identidadecrypto: parâmetros Argon2id inválidos (m=%d t=%d p=%d)", a.p.MemoriaKiB, a.p.Iteracoes, a.p.Paralelismo)
	}
	salt := make([]byte, argon2SaltBytes)
	_, _ = rand.Read(salt) // crypto/rand.Read nunca falha (Go 1.24+)
	chave := argon2.IDKey(a.comPepper(senha), salt, a.p.Iteracoes, a.p.MemoriaKiB, a.p.Paralelismo, argon2ChaveBytes)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("%sv=%d$m=%d,t=%d,p=%d$%s$%s",
		prefixoArgon2id, argon2.Version, a.p.MemoriaKiB, a.p.Iteracoes, a.p.Paralelismo,
		b64.EncodeToString(salt), b64.EncodeToString(chave)), nil
}

func (a argon2idPepper) verificar(hash, senha string) bool {
	if a.p.Pepper == "" {
		return false
	}
	h, err := decodificarArgon2(hash)
	if err != nil {
		return false
	}
	calc := argon2.IDKey(a.comPepper(senha), h.salt, h.iteracoes, h.memoria, h.paralelismo, uint32(len(h.chave)))
	return subtle.ConstantTimeCompare(calc, h.chave) == 1
}

func (a argon2idPepper) precisaRehash(hash string) bool {
	h, err := decodificarArgon2(hash)
	if err != nil {
		return false
	}
	return h.memoria != a.p.MemoriaKiB || h.iteracoes != a.p.Iteracoes || h.paralelismo != a.p.Paralelismo
}

// comPepper devolve HMAC-SHA256(pepper, senha), a entrada do Argon2id.
func (a argon2idPepper) comPepper(senha string) []byte {
	mac := hmac.New(sha256.New, []byte(a.p.Pepper))
	mac.Write([]byte(senha))
	return mac.Sum(nil)
}

// hashArgon2 é um hash Argon2id decodificado do formato PHC.
type hashArgon2 struct {
	memoria, iteracoes uint32
	paralelismo        uint8
	salt, chave        []byte
}

var errHashArgon2 = errors.New("identidadecrypto: hash argon2id malformado")

// decodificarArgon2 interpreta $argon2id$v=19$m=..,t=..,p=..$salt$chave.
func decodificarArgon2(hash string) (*hashArgon2, error) {
	partes := strings.Split(hash, "$")
	if len(partes) != 6 || partes[1] != "argon2id" {
		return nil, errHashArgon2
	}
	var versao int
	if _, err := fmt.Sscanf(partes[2], "v=%d", &versao); err != nil || versao != argon2.Version {
		return nil, errHashArgon2
	}
	h := &hashArgon2{}
	if _, err := fmt.Sscanf(partes[3], "m=%d,t=%d,p=%d", &h.memoria, &h.iteracoes, &h.paralelismo); err != nil {
		return nil, errHashArgon2
	}
	if h.memoria == 0 || h.iteracoes == 0 || h.paralelismo == 0 {
		return nil, errHashArgon2
	}
	var err error
	b64 := base64.RawStdEncoding
	if h.salt, err = b64.DecodeString(partes[4]); err != nil || len(h.salt) == 0 {
		return nil, errHashArgon2
	}
	if h.chave, err = b64.DecodeString(partes[5]); err != nil || len(h.chave) == 0 {
		return nil, errHashArgon2
	}
	return h, nil
}
