package catalogo_test

// CHORE-10 (Lote 31): ErrImagemMuitoGrande é KindMuitoGrande (413 pelo
// mapper de erros), com a mesma mensagem; o caso especial do handler saiu.

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/kernel"
)

func TestCHORE10_ErrImagemMuitoGrande_KindMuitoGrande(t *testing.T) {
	var e *kernel.Erro
	require.True(t, errors.As(catalogo.ErrImagemMuitoGrande, &e))
	assert.Equal(t, kernel.KindMuitoGrande, e.Kind())
	assert.Equal(t, "imagem excede 1 MB", e.Error())
}
