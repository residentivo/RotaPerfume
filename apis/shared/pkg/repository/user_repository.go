// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"database/sql"
	"fmt"

	"rotaperfumes/shared/pkg/entities"
)

// UserRepository defines the interface for user data access.
type UserRepository interface {
	FindByLogin(ctx context.Context, login string) (*entities.User, error)
	FindByID(ctx context.Context, id int64) (*entities.User, error)
}

// MySQLUserRepository implements UserRepository using MySQL.
type MySQLUserRepository struct {
	db *sql.DB
}

// NewMySQLUserRepository creates a new MySQL user repository.
func NewMySQLUserRepository(db *sql.DB) *MySQLUserRepository {
	return &MySQLUserRepository{db: db}
}

// FindByLogin retrieves a user by their login name.
func (r *MySQLUserRepository) FindByLogin(ctx context.Context, login string) (*entities.User, error) {
	query := `
		SELECT id, login, email, password_hash, role, ativo, gerente_id, data_criacao, data_atualizacao
		FROM usuarios
		WHERE login = ?
	`

	var user entities.User
	var email, passwordHash, role string
	var gerenteID sql.NullInt64
	var dataCriacao, dataAtualizacao sql.NullTime

	err := r.db.QueryRowContext(ctx, query, login).Scan(
		&user.ID,
		&user.Login,
		&email,
		&passwordHash,
		&role,
		&user.Ativo,
		&gerenteID,
		&dataCriacao,
		&dataAtualizacao,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find user by login: %w", err)
	}

	user.Email = email
	user.PasswordHash = passwordHash
	user.Role = entities.Role(role)
	if gerenteID.Valid {
		user.GerenteID = &gerenteID.Int64
	}
	if dataCriacao.Valid {
		user.DataCriacao = dataCriacao.Time
	}
	if dataAtualizacao.Valid {
		user.DataAtualizacao = dataAtualizacao.Time
	}

	return &user, nil
}

// FindByID retrieves a user by their ID.
func (r *MySQLUserRepository) FindByID(ctx context.Context, id int64) (*entities.User, error) {
	query := `
		SELECT id, login, email, password_hash, role, ativo, gerente_id, data_criacao, data_atualizacao
		FROM usuarios
		WHERE id = ?
	`

	var user entities.User
	var email, passwordHash, role string
	var gerenteID sql.NullInt64
	var dataCriacao, dataAtualizacao sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Login,
		&email,
		&passwordHash,
		&role,
		&user.Ativo,
		&gerenteID,
		&dataCriacao,
		&dataAtualizacao,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find user by id: %w", err)
	}

	user.Email = email
	user.PasswordHash = passwordHash
	user.Role = entities.Role(role)
	if gerenteID.Valid {
		user.GerenteID = &gerenteID.Int64
	}
	if dataCriacao.Valid {
		user.DataCriacao = dataCriacao.Time
	}
	if dataAtualizacao.Valid {
		user.DataAtualizacao = dataAtualizacao.Time
	}

	return &user, nil
}
