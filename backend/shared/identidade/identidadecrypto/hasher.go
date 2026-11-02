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
	"log"
	"strings"
	"sync/atomic"

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

	// Teto dos parâmetros aceitos num hash gravado (SEC-32): acima disso o
	// hash é recusado como malformado, para um registro adulterado não
	// transformar o login num DoS de CPU/memória. A memória é relativa à
	// configuração atual (4x).
	fatorTetoMemoria = 4
	tetoIteracoes    = 10
	tetoParalelismo  = 16
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
	// descartaveis conta as execuções do hash descartável (observabilidade
	// e testes do SEC-32/I-16).
	descartaveis atomic.Uint64
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
// Hash vazio, malformado, com parâmetros acima do teto ou sem pepper
// configurado devolve false DEPOIS de rodar o hash descartável: o tempo de
// resposta é o de uma senha errada (SEC-32, I-16).
func (h *Hasher) Verificar(hash identidade.HashSenha, s identidade.Senha) bool {
	v := hash.Revelar()
	if (bcryptLegado{}).reconhece(v) {
		return bcryptLegado{}.verificar(v, s.Revelar())
	}
	decodificado, motivo := h.argon2.reconhecer(v)
	if motivo != "" {
		log.Printf("[auth] hash_irreconhecido: motivo=%s", motivo)
		h.ExecutarDescartavel(s)
		return false
	}
	return h.argon2.conferir(decodificado, s.Revelar())
}

// PrecisaRehash: bcrypt legado ou Argon2id com parâmetros diferentes dos
// atuais. Hash malformado não é caso de rehash.
func (h *Hasher) PrecisaRehash(hash identidade.HashSenha) bool {
	return h.estrategiaDe(hash.Revelar()).precisaRehash(hash.Revelar())
}

// ExecutarDescartavel roda um Argon2id com os parâmetros atuais e descarta o
// resultado: o login de e-mail inexistente custa o mesmo que o de um
// existente (I-16).
//
// Roda mesmo sem pepper (o HMAC com chave vazia custa o mesmo), para o
// caminho de configuração quebrada não ficar mais rápido; só pula se os
// parâmetros forem zero (o argon2 entraria em pânico).
func (h *Hasher) ExecutarDescartavel(s identidade.Senha) {
	h.descartaveis.Add(1)
	p := h.argon2.p
	if p.MemoriaKiB == 0 || p.Iteracoes == 0 || p.Paralelismo == 0 {
		return
	}
	salt := make([]byte, argon2SaltBytes)
	_, _ = rand.Read(salt)
	_ = argon2.IDKey(h.argon2.comPepper(s.Revelar()), salt, p.Iteracoes, p.MemoriaKiB, p.Paralelismo, argon2ChaveBytes)
}

// ExecucoesDescartaveis devolve quantas vezes o hash descartável rodou
// neste Hasher (contador monotônico; SEC-32).
func (h *Hasher) ExecucoesDescartaveis() uint64 {
	return h.descartaveis.Load()
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
	h, motivo := a.reconhecer(hash)
	if motivo != "" {
		return false
	}
	return a.conferir(h, senha)
}

// Motivos (enum fixo) do log hash_irreconhecido. Nunca o hash.
const (
	motivoHashVazio        = "vazio"
	motivoPepperVazio      = "pepper_vazio"
	motivoHashMalformado   = "malformado"
	motivoParametrosNoTeto = "parametros_acima_do_teto"
)

// reconhecer decodifica o hash e aplica o teto de parâmetros. motivo vazio
// = hash utilizável.
func (a argon2idPepper) reconhecer(hash string) (*hashArgon2, string) {
	if hash == "" {
		return nil, motivoHashVazio
	}
	if a.p.Pepper == "" {
		return nil, motivoPepperVazio
	}
	h, err := decodificarArgon2(hash)
	if err != nil {
		return nil, motivoHashMalformado
	}
	if a.acimaDoTeto(h) {
		return nil, motivoParametrosNoTeto
	}
	return h, ""
}

// acimaDoTeto: m > 4x a memória atual, t > 10 ou p > 16 (SEC-32).
func (a argon2idPepper) acimaDoTeto(h *hashArgon2) bool {
	return uint64(h.memoria) > fatorTetoMemoria*uint64(a.p.MemoriaKiB) ||
		h.iteracoes > tetoIteracoes || h.paralelismo > tetoParalelismo
}

// conferir roda o Argon2id com os parâmetros do hash e compara em tempo
// constante.
func (a argon2idPepper) conferir(h *hashArgon2, senha string) bool {
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
