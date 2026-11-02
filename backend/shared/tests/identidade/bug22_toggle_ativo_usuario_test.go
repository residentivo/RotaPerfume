package identidade_test

// BUG-22 (Lote 31): o PATCH /inativar de usuário inverte no próprio UPDATE
// (toggle atômico), relê na mesma tx e, se o usuário ficou inativo, revoga
// os refresh tokens e grava o corte de sessão na mesma tx. Falha em
// qualquer etapa → rollback (500). A regra AdminOnly (I-05) continua.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
)

func TestBUG22_Usuario_ToggleUsaUpdateAtomico(t *testing.T) {
	a := novoAmbiente(usuarioAtivo(9, "a@b.c", "s", kernel.RoleNormal))
	admin := principal(1, kernel.RoleAdmin)

	u, err := a.casos().AlternarAtivoUsuario(ctx, admin, 9, nil)
	require.NoError(t, err)
	assert.False(t, u.Ativo, "a resposta vem da releitura")
	assert.Equal(t, 1, a.usuarios.alternados, "toggle = UPDATE ... SET ativo = NOT ativo")
	assert.Equal(t, []identidade.MotivoRevogacao{identidade.MotivoInativacao}, a.tokens.revogados)
	assert.Equal(t, 1, a.usuarios.invalidou, "corte de sessão na mesma tx")
	assert.Equal(t, 1, a.txAtivo.commits)

	// Dois toggles em sequência se compõem (não leem-invertem-gravam).
	u, err = a.casos().AlternarAtivoUsuario(ctx, admin, 9, nil)
	require.NoError(t, err)
	assert.True(t, u.Ativo)
	assert.Equal(t, 2, a.usuarios.alternados)
	assert.Len(t, a.tokens.revogados, 1, "reativar não revoga nada")
	assert.Equal(t, 1, a.usuarios.invalidou, "reativar não grava corte")
}

func TestBUG22_Usuario_ValorExplicitoNaoUsaToggle(t *testing.T) {
	for _, alvo := range []bool{true, false} {
		a := novoAmbiente(usuarioAtivo(9, "a@b.c", "s", kernel.RoleNormal))
		u, err := a.casos().AlternarAtivoUsuario(ctx, principal(1, kernel.RoleAdmin), 9, &alvo)
		require.NoError(t, err)
		assert.Equal(t, alvo, u.Ativo)
		assert.Zero(t, a.usuarios.alternados, "body com ativo usa DefinirAtivo")
		if alvo {
			assert.Empty(t, a.tokens.revogados)
		} else {
			assert.Equal(t, []identidade.MotivoRevogacao{identidade.MotivoInativacao}, a.tokens.revogados)
			assert.Equal(t, 1, a.usuarios.invalidou)
		}
	}
}

func TestBUG22_Usuario_FalhaNoCorteDesfazTudo(t *testing.T) {
	a := novoAmbiente(usuarioAtivo(9, "a@b.c", "s", kernel.RoleNormal))
	a.usuarios.errInvalid = errInfra

	_, err := a.casos().AlternarAtivoUsuario(ctx, principal(1, kernel.RoleAdmin), 9, nil)

	assert.ErrorIs(t, err, errInfra)
	assert.True(t, a.usuarios.porID[9].Ativo, "rollback: continua ativo")
	assert.Empty(t, a.tokens.revogados, "rollback: revogação desfeita")
	assert.Equal(t, 1, a.txAtivo.rollbacks)
	assert.Zero(t, a.txAtivo.commits)
}

func TestBUG22_Usuario_FalhaNaReleitura500(t *testing.T) {
	a := novoAmbiente(usuarioAtivo(9, "a@b.c", "s", kernel.RoleNormal))
	a.usuarios.errPorID = []error{identidade.ErrUsuarioNaoEncontrado}

	_, err := a.casos().AlternarAtivoUsuario(ctx, principal(1, kernel.RoleAdmin), 9, nil)

	require.Error(t, err)
	assert.NotErrorIs(t, err, identidade.ErrUsuarioNaoEncontrado, "sem %%w: releitura que falha é 500")
	assert.Equal(t, 1, a.txAtivo.rollbacks)
}

func TestBUG22_Usuario_NaoAdminNemAbreTx(t *testing.T) {
	a := novoAmbiente(usuarioAtivo(9, "a@b.c", "s", kernel.RoleNormal))
	_, err := a.casos().AlternarAtivoUsuario(ctx, principal(2, kernel.RoleNormal), 9, nil)
	assert.Equal(t, identidade.ErrAcessoRestritoAdmin, err)
	assert.Zero(t, a.txAtivo.commits+a.txAtivo.rollbacks)
}
