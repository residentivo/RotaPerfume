package kernel_test

// ARQ-07 / N4: a rota no contexto é só leitura para os logs da aplicação.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/kernel"
)

func TestARQ07_ComRotaERotaDe(t *testing.T) {
	assert.Equal(t, "", kernel.RotaDe(context.Background()), "fora de requisição (CLI): vazio")
	ctx := kernel.ComRota(context.Background(), "/api/clientes/1")
	assert.Equal(t, "/api/clientes/1", kernel.RotaDe(ctx))
}

// ARQ-07 NB-C: o path chega decodificado (%0A vira "\n"); ComRota grava a
// forma escapada para não injetar linhas falsas no log.
func TestARQ07_NBC_ComRotaSaneiaControle(t *testing.T) {
	sepLinha := string(rune(0x2028)) // U+2028 (LINE SEPARATOR)
	ctx := kernel.ComRota(context.Background(), "/api/x\n[escopo] falso\r\t\x00"+sepLinha)
	esperado := `/api/x\n[escopo] falso\r\t\x00\` + "u2028"
	assert.Equal(t, esperado, kernel.RotaDe(ctx))
	assert.Equal(t, "/api/clientes/ação", kernel.SanearRota("/api/clientes/ação"), "Unicode comum fica")
}
