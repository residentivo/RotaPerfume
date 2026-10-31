//go:build linux

package catalogofs_test

// IMG-01 (Lote 26, TestBrain): falha de escrita no meio do arquivo
// (disco cheio/quota), simulada com RLIMIT_FSIZE: Gravar devolve erro e o
// temporário é apagado (nada sobra no diretório).

import (
	"context"
	"os/signal"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIMG01_FS_Gravar_FalhaDeEscrita_ApagaTemporario(t *testing.T) {
	a, dir := novo(t)
	signal.Ignore(syscall.SIGXFSZ)
	defer signal.Reset(syscall.SIGXFSZ)
	var antes syscall.Rlimit
	require.NoError(t, syscall.Getrlimit(syscall.RLIMIT_FSIZE, &antes))
	limite := syscall.Rlimit{Cur: 4, Max: antes.Max}
	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_FSIZE, &limite))
	nome, err := a.Gravar(context.Background(), ".png", make([]byte, 64<<10))
	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_FSIZE, &antes))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalogofs: gravar temporário")
	assert.Empty(t, nome)
	assert.Empty(t, listar(t, dir), "o temporário tem de ser removido")
}
