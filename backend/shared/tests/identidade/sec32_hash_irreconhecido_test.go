package identidade_test

// SEC-32 (Lote 31): hash vazio, malformado, com parâmetros acima do teto ou
// sem pepper roda o hash descartável antes de devolver false, e o login
// responde exatamente como senha errada. O log de decisão é
// "hash_irreconhecido motivo=<enum>", sem o hash nem o e-mail.

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/kernel"
)

var hsSEC32 = config.HashSenha{Pepper: "pepper-de-teste-com-pelo-menos-32-bytes", MemoriaKiB: 64, Iteracoes: 1, Paralelismo: 1}

func capturarLogSEC32(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	anterior := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(anterior) })
	return &buf
}

func hashValidoSEC32(t *testing.T) []string {
	t.Helper()
	h, err := identidadecrypto.NovoHasher(hsSEC32).Gerar(identidade.NovaSenha("Senha@123"))
	require.NoError(t, err)
	return strings.Split(h.Revelar(), "$")
}

func TestSEC32_HashIrreconhecido_RodaDescartavelELogaSoOMotivo(t *testing.T) {
	p := hashValidoSEC32(t)
	sufixo := "$" + p[4] + "$" + p[5]
	casos := []struct {
		nome, hash, motivo string
		hs                 config.HashSenha
	}{
		{"vazio", "", "vazio", hsSEC32},
		{"placeholder", "PLACEHOLDER_ADMIN", "malformado", hsSEC32},
		{"partes faltando", "$argon2id$v=19$m=64,t=1,p=1$" + p[4], "malformado", hsSEC32},
		{"memória acima de 4x", "$argon2id$v=19$m=257,t=1,p=1" + sufixo, "parametros_acima_do_teto", hsSEC32},
		{"iterações acima de 10", "$argon2id$v=19$m=64,t=11,p=1" + sufixo, "parametros_acima_do_teto", hsSEC32},
		{"paralelismo acima de 16", "$argon2id$v=19$m=64,t=1,p=17" + sufixo, "parametros_acima_do_teto", hsSEC32},
		{"pepper vazio", "$argon2id$v=19$m=64,t=1,p=1" + sufixo, "pepper_vazio", config.HashSenha{MemoriaKiB: 64, Iteracoes: 1, Paralelismo: 1}},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			buf := capturarLogSEC32(t)
			h := identidadecrypto.NovoHasher(c.hs)
			ok := h.Verificar(identidade.NovoHashSenha(c.hash), identidade.NovaSenha("Senha@123"))
			assert.False(t, ok)
			assert.Equal(t, uint64(1), h.ExecucoesDescartaveis(), "o descartável roda uma vez")
			saida := buf.String()
			assert.Contains(t, saida, "hash_irreconhecido: motivo="+c.motivo)
			if c.hash != "" {
				assert.NotContains(t, saida, c.hash, "o hash nunca vai para o log")
			}
			assert.NotContains(t, saida, p[5])
			assert.NotContains(t, saida, "Senha@123")
		})
	}
}

func TestSEC32_NoTetoAindaVerifica(t *testing.T) {
	// m = 4x a memória atual, t = 10 e p = 16 estão no limite (aceitos).
	maior := config.HashSenha{Pepper: hsSEC32.Pepper, MemoriaKiB: 256, Iteracoes: 2, Paralelismo: 2}
	hash, err := identidadecrypto.NovoHasher(maior).Gerar(identidade.NovaSenha("Senha@123"))
	require.NoError(t, err)
	h := identidadecrypto.NovoHasher(hsSEC32)
	assert.True(t, h.Verificar(hash, identidade.NovaSenha("Senha@123")))
	assert.Zero(t, h.ExecucoesDescartaveis())
}

func TestSEC32_CaminhosReconhecidosNaoRodamDescartavel(t *testing.T) {
	h := identidadecrypto.NovoHasher(hsSEC32)
	valido, err := h.Gerar(identidade.NovaSenha("Senha@123"))
	require.NoError(t, err)
	legado, err := bcrypt.GenerateFromPassword([]byte("Senha@123"), bcrypt.MinCost)
	require.NoError(t, err)

	buf := capturarLogSEC32(t)
	assert.False(t, h.Verificar(valido, identidade.NovaSenha("errada")), "argon2id com senha errada")
	assert.True(t, h.Verificar(valido, identidade.NovaSenha("Senha@123")))
	assert.False(t, h.Verificar(identidade.NovoHashSenha(string(legado)), identidade.NovaSenha("errada")), "bcrypt com senha errada")
	assert.Zero(t, h.ExecucoesDescartaveis(), "hash reconhecido roda o Argon2id real, não o descartável")
	assert.NotContains(t, buf.String(), "hash_irreconhecido")
}

func TestSEC32_ExecutarDescartavel_SemPepperTambemConta(t *testing.T) {
	h := identidadecrypto.NovoHasher(config.HashSenha{MemoriaKiB: 64, Iteracoes: 1, Paralelismo: 1})
	assert.NotPanics(t, func() { h.ExecutarDescartavel(identidade.NovaSenha("x")) })
	assert.Equal(t, uint64(1), h.ExecucoesDescartaveis())

	zerado := identidadecrypto.NovoHasher(config.HashSenha{})
	assert.NotPanics(t, func() { zerado.ExecutarDescartavel(identidade.NovaSenha("x")) }, "parâmetros zero não entram em pânico")
}

// No login, hash irreconhecido devolve o MESMO erro de senha errada e conta
// no rate limit igual.
func TestSEC32_Login_HashIrreconhecidoIgualASenhaErrada(t *testing.T) {
	deOutraSenha, err := identidadecrypto.NovoHasher(hsSEC32).Gerar(identidade.NovaSenha("Outra#123"))
	require.NoError(t, err)

	login := func(hash identidade.HashSenha) (*identidadecrypto.Hasher, error) {
		u := &identidade.Usuario{ID: 9, Nome: "H", Email: "hash@b.c", Role: kernel.RoleNormal, Ativo: true, Hash: hash}
		a := novoAmbiente(u)
		real := identidadecrypto.NovoHasher(hsSEC32)
		d := a.deps()
		d.Hasher = real
		_, err := identidadeapp.Novo(d, identidadeapp.Opcoes{}).Login(ctx, identidadeapp.ComandoLogin{
			Email: "hash@b.c", Senha: identidade.NovaSenha("Senha@123"), Captcha: "ok", Origem: identidadeapp.Origem{IP: "1.2.3.4"}})
		return real, err
	}

	hErrada, errErrada := login(deOutraSenha)
	hVazio, errVazio := login(identidade.NovoHashSenha(""))
	hLixo, errLixo := login(identidade.NovoHashSenha("PLACEHOLDER_ADMIN"))

	require.Error(t, errErrada)
	assert.Equal(t, identidade.ErrCredenciaisInvalidas, errErrada)
	assert.Equal(t, errErrada, errVazio, "hash vazio responde como senha errada")
	assert.Equal(t, errErrada, errLixo, "hash malformado responde como senha errada")
	assert.Zero(t, hErrada.ExecucoesDescartaveis())
	assert.Equal(t, uint64(1), hVazio.ExecucoesDescartaveis())
	assert.Equal(t, uint64(1), hLixo.ExecucoesDescartaveis())
}
