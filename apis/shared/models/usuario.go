// Package models contém as estruturas de domínio compartilhadas pela API.
package models

import "time"

// RoleAdmin e RoleNormal refletem o ENUM('admin','normal') da tabela usuarios.
const (
	RoleAdmin  = "admin"
	RoleNormal = "normal"
)

// Usuario representa um usuário do sistema (admin ou vendedor).
type Usuario struct {
	ID            int64      `json:"id"`
	Nome          string     `json:"nome,omitempty"`
	IDVendedor    *int64     `json:"id_vendedor,omitempty"`
	Email         string     `json:"email"`
	Role          string     `json:"role"`
	PasswordHash  string     `json:"-"` // nunca serializar
	Ativo         bool       `json:"ativo"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	UltimoLoginAt *time.Time `json:"ultimo_login_at,omitempty"`
}

// IsAdmin retorna true se o usuário tem papel de administrador.
func (u *Usuario) IsAdmin() bool {
	return u != nil && u.Role == RoleAdmin
}

// IsActive retorna true se o usuário está ativo.
func (u *Usuario) IsActive() bool {
	return u != nil && u.Ativo
}
