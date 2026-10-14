package vlog_test

// vlog.Trace (nível 2, ARQ-01): "início" + "fim ok"/"fim erro=<kind>", com o
// erro classificado só pelo Kind (nunca err.Error()).

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

var errTraceNaoEncontrado = kernel.NovoErro(kernel.KindNaoEncontrado, "produto não encontrado")

func TestTrace(t *testing.T) {
	const segredo = "texto-secreto-do-driver"
	casos := []struct {
		nome    string
		err     error
		wantFim string
		semErr  bool
	}{
		{"sucesso", nil, `fim ok dur=\S+`, false},
		{"erro de domínio pelo Kind", errTraceNaoEncontrado, `fim erro=nao_encontrado dur=\S+`, false},
		{"erro de domínio embrulhado", fmt.Errorf("catalogomysql: obter: %w", errTraceNaoEncontrado), `fim erro=nao_encontrado dur=\S+`, false},
		{"bloqueio usa o Kind do sentinela", &kernel.Bloqueio{
			Sentinela: kernel.NovoErro(kernel.KindBloqueado, "muitas tentativas"), RetryAfter: time.Second,
		}, `fim erro=bloqueado dur=\S+`, false},
		{"erro de infra vira interno sem a mensagem", errors.New(segredo), `fim erro=interno dur=\S+`, false},
		{"errp nil é tratado como sucesso", nil, `fim ok dur=\S+`, true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			out := capturar(t, true, func() {
				fim := vlog.Trace("produtos.go", "Catalogo.Obter", "id=%d", 7)
				if c.semErr {
					fim(nil)
					return
				}
				err := c.err
				fim(&err)
			})
			assert.Regexp(t, regexp.MustCompile(`^\[produtos\.go\] \[Catalogo\.Obter\] início id=7\n`), out)
			assert.Regexp(t, regexp.MustCompile(`\[produtos\.go\] \[Catalogo\.Obter\] `+c.wantFim+`\n$`), out)
			assert.NotContains(t, out, segredo)
			assert.NotContains(t, out, "produto não encontrado", "a mensagem do erro nunca vai ao log")
		})
	}
}

func TestTrace_Desligado(t *testing.T) {
	out := capturar(t, false, func() {
		err := errors.New("x")
		vlog.Trace("a.go", "F", "page=%d", 1)(&err)
	})
	assert.Empty(t, out)
}
