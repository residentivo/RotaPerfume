// Testes do hash de senha Argon2id com pepper (SEC-13).
package services_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/services"
)

var hsTeste = config.HashSenha{Pepper: "pepper-de-teste-com-pelo-menos-32-bytes", MemoriaKiB: 64, Iteracoes: 1, Paralelismo: 1}

func TestGerarHashSenha_FormatoPHC(t *testing.T) {
	hash, err := services.GerarHashSenha(hsTeste, "Senha@123")
	require.NoError(t, err)

	partes := strings.Split(hash, "$")
	require.Len(t, partes, 6, hash)
	assert.Equal(t, "argon2id", partes[1])
	assert.Equal(t, "v=19", partes[2])
	assert.Equal(t, "m=64,t=1,p=1", partes[3])
	assert.Len(t, partes[4], 22, "salt de 16 bytes em base64 sem padding")
	assert.Len(t, partes[5], 43, "chave de 32 bytes em base64 sem padding")
	assert.NotContains(t, hash, hsTeste.Pepper, "o pepper nunca vai para o hash")
	assert.LessOrEqual(t, len(hash), 255, "cabe em usuarios.password_hash VARCHAR(255)")
}

func TestGerarHashSenha_ErroSemPepper(t *testing.T) {
	hs := hsTeste
	hs.Pepper = ""
	hash, err := services.GerarHashSenha(hs, "Senha@123")
	assert.ErrorIs(t, err, services.ErrPepperNaoConfigurado)
	assert.Empty(t, hash)
}

// O pepper entra no hash: outro pepper (ou nenhum) não valida a mesma senha.
func TestVerificarSenha_Pepper(t *testing.T) {
	hash, err := services.GerarHashSenha(hsTeste, "Senha@123")
	require.NoError(t, err)

	outro := hsTeste
	outro.Pepper = "outro-pepper-tambem-com-32-bytes-ou-mais"
	semPepper := hsTeste
	semPepper.Pepper = ""

	assert.True(t, services.VerificarSenha(hsTeste, hash, "Senha@123"))
	assert.False(t, services.VerificarSenha(outro, hash, "Senha@123"), "pepper diferente")
	assert.False(t, services.VerificarSenha(semPepper, hash, "Senha@123"), "sem pepper")
	assert.False(t, services.VerificarSenha(hsTeste, hash, "Senha@123"+hsTeste.Pepper), "pepper não é texto da senha")
}

// O hash guarda os próprios parâmetros: mudar ARGON2_* não quebra hashes antigos.
func TestVerificarSenha_ParametrosDoHash(t *testing.T) {
	antigo, err := services.GerarHashSenha(hsTeste, "Senha@123")
	require.NoError(t, err)
	novo := hsTeste
	novo.MemoriaKiB, novo.Iteracoes, novo.Paralelismo = 128, 2, 2

	assert.True(t, services.VerificarSenha(novo, antigo, "Senha@123"))
	assert.True(t, services.PrecisaRehash(novo, antigo))
	assert.False(t, services.PrecisaRehash(hsTeste, antigo))
}

func TestVerificarSenha_BcryptLegado(t *testing.T) {
	legado, err := bcrypt.GenerateFromPassword([]byte("Senha@123"), bcrypt.MinCost)
	require.NoError(t, err)

	assert.True(t, services.VerificarSenha(hsTeste, string(legado), "Senha@123"), "bcrypt é verificado sem pepper")
	assert.False(t, services.VerificarSenha(hsTeste, string(legado), "errada"))
	assert.True(t, services.PrecisaRehash(hsTeste, string(legado)))
}

func TestVerificarSenha_HashesMalformados(t *testing.T) {
	valido, err := services.GerarHashSenha(hsTeste, "Senha@123")
	require.NoError(t, err)
	p := strings.Split(valido, "$")

	casos := map[string]string{
		"vazio":              "",
		"placeholder":        "PLACEHOLDER_ADMIN",
		"argon2i":            "$argon2i$v=19$m=64,t=1,p=1$" + p[4] + "$" + p[5],
		"versão 16":          "$argon2id$v=16$m=64,t=1,p=1$" + p[4] + "$" + p[5],
		"partes faltando":    "$argon2id$v=19$m=64,t=1,p=1$" + p[4],
		"parâmetros ruins":   "$argon2id$v=19$m=x,t=1,p=1$" + p[4] + "$" + p[5],
		"parâmetros zerados": "$argon2id$v=19$m=64,t=0,p=1$" + p[4] + "$" + p[5],
		"salt inválido":      "$argon2id$v=19$m=64,t=1,p=1$!!!$" + p[5],
		"salt vazio":         "$argon2id$v=19$m=64,t=1,p=1$$" + p[5],
		"chave inválida":     "$argon2id$v=19$m=64,t=1,p=1$" + p[4] + "$!!!",
		"chave adulterada":   "$argon2id$v=19$m=64,t=1,p=1$" + p[4] + "$" + strings.Repeat("A", 43),
	}
	for nome, hash := range casos {
		t.Run(nome, func(t *testing.T) {
			assert.NotPanics(t, func() {
				assert.False(t, services.VerificarSenha(hsTeste, hash, "Senha@123"))
			})
			assert.False(t, services.PrecisaRehash(hsTeste, hash), "malformado não é caso de re-hash")
		})
	}
}
