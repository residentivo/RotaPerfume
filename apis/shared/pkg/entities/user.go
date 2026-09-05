// Package entities defines the core domain entities.
package entities

import "time"

// Role represents user access levels.
type Role string

const (
	RoleVendedor Role = "VENDEDOR"
	RoleGerente  Role = "GERENTE"
	RoleRH       Role = "RH"
)

// User represents a user in the system.
type User struct {
	ID            int64     `json:"id"`
	Login         string    `json:"login"`
	Email         string    `json:"email"`
	PasswordHash  string    `json:"-"`
	Role          Role      `json:"role"`
	Ativo         bool      `json:"ativo"`
	GerenteID     *int64    `json:"gerente_id,omitempty"`
	DataCriacao   time.Time `json:"data_criacao"`
	DataAtualizacao time.Time `json:"data_atualizacao"`
}

// IsValid checks if the user has a valid role.
func (u *User) IsValid() bool {
	switch u.Role {
	case RoleVendedor, RoleGerente, RoleRH:
		return true
	default:
		return false
	}
}

// IsActive checks if the user account is active.
func (u *User) IsActive() bool {
	return u.Ativo
}

// HasManager checks if the user has a manager assigned.
func (u *User) HasManager() bool {
	return u.GerenteID != nil
}
