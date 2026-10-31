package kernel_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/kernel"
)

var (
	errTesteNaoEncontrado = kernel.NovoErro(kernel.KindNaoEncontrado, "coisa não encontrada")
	errTesteBloqueado     = kernel.NovoErro(kernel.KindBloqueado, "muitas tentativas — tente novamente mais tarde")
)

func TestKind_String(t *testing.T) {
	casos := map[kernel.Kind]string{
		kernel.KindInterno:         "interno",
		kernel.KindInvalido:        "invalido",
		kernel.KindNaoEncontrado:   "nao_encontrado",
		kernel.KindConflito:        "conflito",
		kernel.KindProibido:        "proibido",
		kernel.KindNaoAutenticado:  "nao_autenticado",
		kernel.KindBloqueado:       "bloqueado",
		kernel.KindMuitoGrande:     "muito_grande",
		kernel.KindMuitoGrande + 1: "desconhecido",
		kernel.Kind(255):           "desconhecido",
	}
	for k, want := range casos {
		assert.Equal(t, want, k.String(), "kind=%d", k)
	}
	assert.Equal(t, kernel.Kind(0), kernel.KindInterno, "zero value é interno (fail-closed)")
}

func TestErro_MensagemFixaEWrap(t *testing.T) {
	assert.Equal(t, "coisa não encontrada", errTesteNaoEncontrado.Error())
	assert.Equal(t, kernel.KindNaoEncontrado, errTesteNaoEncontrado.Kind())
	assert.Equal(t, "nao_encontrado", errTesteNaoEncontrado.KindNome())

	embrulhado := fmt.Errorf("catalogomysql: obter produto: %w", errTesteNaoEncontrado)
	assert.ErrorIs(t, embrulhado, errTesteNaoEncontrado)
	var de *kernel.Erro
	assert.True(t, errors.As(embrulhado, &de))
	assert.Equal(t, "coisa não encontrada", de.Error(), "a mensagem pública é a do sentinela, não a do wrap")

	assert.Equal(t, "erro interno", kernel.ErrInterno.Error())
	assert.Equal(t, kernel.KindInterno, kernel.ErrInterno.Kind())
}

func TestBloqueio(t *testing.T) {
	b := &kernel.Bloqueio{Sentinela: errTesteBloqueado, RetryAfter: 42 * time.Second}
	assert.Equal(t, errTesteBloqueado.Error(), b.Error())
	assert.ErrorIs(t, b, errTesteBloqueado)
	var de *kernel.Erro
	assert.True(t, errors.As(fmt.Errorf("op: %w", b), &de))
	assert.Equal(t, kernel.KindBloqueado, de.Kind())

	vazio := &kernel.Bloqueio{}
	assert.Equal(t, "erro interno", vazio.Error())
	assert.Nil(t, vazio.Unwrap())
}
