// Package models_test testava o models.Usuario legado. ARQ-07 (§7.6): o
// pacote models saiu; os testes, com os mesmos nomes, exercitam os
// equivalentes do domínio de Identidade: o VO kernel.Role (EhAdmin, I-05) e
// o agregado identidade.Usuario (EhAdmin e o flag Ativo).
package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
)

// TestUsuarioIsAdmin garante que EhAdmin é verdadeiro apenas para role "admin".
func TestUsuarioIsAdmin(t *testing.T) {
	t.Run("admin retorna true", func(t *testing.T) {
		u := &identidade.Usuario{Role: kernel.RoleAdmin}
		assert.True(t, u.EhAdmin())
	})

	t.Run("normal retorna false", func(t *testing.T) {
		u := &identidade.Usuario{Role: kernel.RoleNormal}
		assert.False(t, u.EhAdmin())
	})

	t.Run("nil retorna false (sem panic)", func(t *testing.T) {
		// O VO não tem ponteiro: o zero value é o "nenhum papel".
		var r kernel.Role
		assert.NotPanics(t, func() {
			assert.False(t, r.EhAdmin())
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
				u := &identidade.Usuario{Role: kernel.RoleDoBanco(c.role)}
				assert.Equal(t, c.expect, u.EhAdmin())
				assert.Equal(t, c.expect, kernel.RoleDoBanco(c.role).EhAdmin())
			})
		}
	})
}

// TestUsuarioIsActive garante que o flag Ativo do agregado é o que veio do banco.
func TestUsuarioIsActive(t *testing.T) {
	t.Run("ativo true retorna true", func(t *testing.T) {
		u := identidade.Usuario{Ativo: true}
		assert.True(t, u.Ativo)
	})

	t.Run("ativo false retorna false", func(t *testing.T) {
		u := identidade.Usuario{Ativo: false}
		assert.False(t, u.Ativo)
	})

	t.Run("nil retorna false (sem panic)", func(t *testing.T) {
		var u identidade.Usuario // zero value: inativo
		assert.NotPanics(t, func() {
			assert.False(t, u.Ativo)
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
				u := identidade.Usuario{Role: kernel.RoleDoBanco(c.role), Ativo: c.ativo}
				assert.Equal(t, c.admin, u.EhAdmin())
				assert.Equal(t, c.active, u.Ativo)
			})
		}
	})
}
