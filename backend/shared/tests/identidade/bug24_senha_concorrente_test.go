package identidade_test

// BUG-24 (Lote 31): a gravação da senha é condicional ao hash lido no início
// do fluxo (UPDATE ... WHERE id = ? AND password_hash = ?). Se outra troca
// venceu no meio, o adapter devolve ErrSenhaAlteradaConcorrentemente: 409
// "a senha foi alterada por outra operação; tente novamente", sem retry
// automático, com rollback do histórico e da revogação na mesma tx. O
// histórico só grava o HashAnterior que de fato foi substituído.

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/kernel"
)

func TestBUG24_ErroDeConflito_409ComMensagemFixa(t *testing.T) {
	var ke *kernel.Erro
	require.True(t, errors.As(identidade.ErrSenhaAlteradaConcorrentemente, &ke))
	assert.Equal(t, kernel.KindConflito, ke.Kind())
	assert.Equal(t, "a senha foi alterada por outra operação; tente novamente", ke.Error())
}

func TestBUG24_TrocarSenha_CondicionadaAoHashLido(t *testing.T) {
	u := usuarioAtivo(2, "a@b.c", "Atual#123", kernel.RoleNormal)
	a := novoAmbiente(u)

	require.NoError(t, a.casos().TrocarSenha(ctx, principal(2, kernel.RoleNormal), bug15CmdTroca()))
	require.Len(t, a.usuarios.anteriores, 1)
	assert.Equal(t, u.Hash, a.usuarios.anteriores[0], "o UPDATE é condicionado ao hash lido no início")
	assert.Equal(t, u.Hash, a.historico.ultimo.HashAnterior, "o histórico grava o mesmo hash da condição")
}

func TestBUG24_TrocarSenha_PerdeuACorrida_409ERollback(t *testing.T) {
	out := bug15CapturarLog(t)
	a := novoAmbiente(usuarioAtivo(2, "a@b.c", "Atual#123", kernel.RoleNormal))
	a.usuarios.errSenha = identidade.ErrSenhaAlteradaConcorrentemente

	err := a.casos().TrocarSenha(ctx, principal(2, kernel.RoleNormal), bug15CmdTroca())

	assert.Same(t, identidade.ErrSenhaAlteradaConcorrentemente, err)
	bug15NadaGravado(t, a)
	assert.Len(t, a.usuarios.anteriores, 1, "uma tentativa só, sem retry automático")
	assert.Equal(t, 1, a.txSenha.chamadas)
	assert.Contains(t, out.String(), "senha alterada por outra operação (rollback): user_id=2")
}

func TestBUG24_AdminResetarSenha_PerdeuACorrida_409SemEmail(t *testing.T) {
	alvo := usuarioAtivo(5, "a@b.c", "Velha#123", kernel.RoleNormal)
	a := novoAmbiente(alvo)
	a.usuarios.errSenha = identidade.ErrSenhaAlteradaConcorrentemente

	enviado, err := a.casos().AdminResetarSenha(ctx, principal(1, kernel.RoleAdmin), identidadeapp.ComandoAdminResetarSenha{UsuarioID: 5})

	assert.False(t, enviado)
	assert.Same(t, identidade.ErrSenhaAlteradaConcorrentemente, err)
	bug15NadaGravado(t, a)
	assert.Equal(t, []string{"rollback"}, a.linhaDoTempo, "nenhum e-mail com a senha que não foi gravada")
	require.Len(t, a.usuarios.anteriores, 1)
	assert.Equal(t, alvo.Hash, a.usuarios.anteriores[0])
}

// Inexistente no UPDATE continua com o mapeamento de antes: 500 na troca
// pelo próprio usuário (sem %w) e 404 no reset administrativo.
func TestBUG24_InexistenteNoUpdate_MantemMapeamento(t *testing.T) {
	a := novoAmbiente(usuarioAtivo(2, "a@b.c", "Atual#123", kernel.RoleNormal))
	a.usuarios.errSenha = identidade.ErrUsuarioNaoEncontrado
	err := a.casos().TrocarSenha(ctx, principal(2, kernel.RoleNormal), bug15CmdTroca())
	bug15Erro500(t, err)
}
