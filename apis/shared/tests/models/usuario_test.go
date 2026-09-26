// Package models_test contém testes unitários para o modelo Usuario.
package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/models"
)

// TestUsuarioIsAdmin garante que IsAdmin retorna true apenas para role "admin".
func TestUsuarioIsAdmin(t *testing.T) {
	t.Run("admin retorna true", func(t *testing.T) {
		u := &models.Usuario{Role: models.RoleAdmin}
		assert.True(t, u.IsAdmin())
	})

	t.Run("normal retorna false", func(t *testing.T) {
		u := &models.Usuario{Role: models.RoleNormal}
		assert.False(t, u.IsAdmin())
	})

	t.Run("nil retorna false (sem panic)", func(t *testing.T) {
		var u *models.Usuario
		assert.NotPanics(t, func() {
			assert.False(t, u.IsAdmin())
		})
	})

	t.Run("parametrizado: roles diversas", func(t *testing.T) {
		cases := []struct {
			role   string
			expect bool
		}{
			{"admin", true},
			{"normal", false},
			{"ADMIN", false}, // case sensitive
			{"", false},
			{" root", false},
			{"Admin", false},
		}
		for _, c := range cases {
			t.Run(c.role, func(t *testing.T) {
				u := &models.Usuario{Role: c.role}
				assert.Equal(t, c.expect, u.IsAdmin())
			})
		}
	})
}

// TestUsuarioIsActive garante que IsActive reflete corretamente o flag Ativo.
func TestUsuarioIsActive(t *testing.T) {
	t.Run("ativo true retorna true", func(t *testing.T) {
		u := &models.Usuario{Ativo: true}
		assert.True(t, u.IsActive())
	})

	t.Run("ativo false retorna false", func(t *testing.T) {
		u := &models.Usuario{Ativo: false}
		assert.False(t, u.IsActive())
	})

	t.Run("nil retorna false (sem panic)", func(t *testing.T) {
		var u *models.Usuario
		assert.NotPanics(t, func() {
			assert.False(t, u.IsActive())
		})
	})

	t.Run("parametrizado: combina role e ativo", func(t *testing.T) {
		cases := []struct {
			name   string
			role   string
			ativo  bool
			admin  bool
			active bool
		}{
			{"admin ativo", "admin", true, true, true},
			{"admin inativo", "admin", false, true, false},
			{"normal ativo", "normal", true, false, true},
			{"normal inativo", "normal", false, false, false},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				u := &models.Usuario{Role: c.role, Ativo: c.ativo}
				assert.Equal(t, c.admin, u.IsAdmin())
				assert.Equal(t, c.active, u.IsActive())
			})
		}
	})
}
