package kernel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rotaperfumes/shared/kernel"
)

func TestRole(t *testing.T) {
	casos := []struct {
		bruto     string
		ehAdmin   bool
		conhecido bool
	}{
		{"admin", true, true},
		{"normal", false, true},
		{"Admin", false, false}, // R-a / I-05: só "admin" exato
		{" admin", false, false},
		{"gerente", false, false},
		{"", false, false},
	}
	for _, c := range casos {
		t.Run("role="+c.bruto, func(t *testing.T) {
			r := kernel.RoleDoBanco(c.bruto)
			assert.Equal(t, c.bruto, r.String(), "guarda o valor bruto do banco")
			assert.Equal(t, c.ehAdmin, r.EhAdmin())
			assert.Equal(t, c.conhecido, r.Conhecido())
		})
	}
}

func TestNovoPrincipal(t *testing.T) {
	casos := []struct {
		nome    string
		id      int64
		role    kernel.Role
		valido  bool
		ehAdmin bool
	}{
		{"admin válido", 1, kernel.RoleAdmin, true, true},
		{"normal válido", 2, kernel.RoleNormal, true, false},
		{"role desconhecido é válido e restrito (R-a)", 3, kernel.RoleDoBanco("gerente"), true, false},
		{"id zero", 0, kernel.RoleAdmin, false, false},
		{"id negativo", -1, kernel.RoleAdmin, false, false},
		{"role vazio", 5, kernel.RoleDoBanco(""), false, false},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p, err := kernel.NovoPrincipal(c.id, c.role)
			if !c.valido {
				require.ErrorIs(t, err, kernel.ErrPrincipalInvalido)
				assert.False(t, p.Valido())
				assert.False(t, p.EhAdmin())
				return
			}
			require.NoError(t, err)
			assert.True(t, p.Valido())
			assert.Equal(t, c.ehAdmin, p.EhAdmin())
			assert.Equal(t, c.id, p.UserID())
			assert.Equal(t, c.role, p.Role())
		})
	}
}

func TestPrincipal_ZeroValueInvalido(t *testing.T) {
	var p kernel.Principal
	assert.False(t, p.Valido())
	assert.False(t, p.EhAdmin())
	assert.Zero(t, p.UserID())
	assert.Equal(t, kernel.KindInterno, kernel.ErrPrincipalInvalido.Kind())
	assert.Equal(t, "erro interno", kernel.ErrPrincipalInvalido.Error())
}
