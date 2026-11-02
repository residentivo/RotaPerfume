package handlers_test

// TST-06 (Lote 31): asserção de tipo checada nos testes de handler. Uma
// resposta em formato inesperado reprova o teste com mensagem clara, em vez
// de panic (que abortava o pacote inteiro e escondia as falhas seguintes).

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// como devolve v como T ou reprova o teste (require: para o teste atual,
// não o pacote).
func como[T any](t testing.TB, v any) T {
	t.Helper()
	out, ok := v.(T)
	require.Truef(t, ok, "tipo inesperado: esperado %T, veio %T (%v)", out, v, v)
	return out
}

// TestTST06_Como_ReprovaSemPanic: o helper falha o teste (Fatal) em vez de
// panic quando o tipo não confere.
func TestTST06_Como_ReprovaSemPanic(t *testing.T) {
	assert := require.New(t)
	assert.Equal("x", como[string](t, any("x")))
	assert.Equal(map[string]any{"a": 1.0}, como[map[string]any](t, any(map[string]any{"a": 1.0})))

	falso := &testing.T{}
	reprovou := make(chan bool, 1)
	go func() {
		defer func() { reprovou <- falso.Failed() }()
		defer func() { _ = recover() }()
		_ = como[map[string]any](falso, nil)
	}()
	assert.True(<-reprovou, "nil não é map: o teste reprova")
}
