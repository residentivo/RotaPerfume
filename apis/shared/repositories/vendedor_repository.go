// Package repositories contém funções puras de acesso a dados (stateless).
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rotaperfumes/shared/models"
)

// VendedorRepository agrupa queries da tabela vendedores.
type VendedorRepository struct{}

// NewVendedorRepository cria um repositório stateless.
func NewVendedorRepository() *VendedorRepository {
	return &VendedorRepository{}
}

// List retorna os vendedores ativos (data_desligamento IS NULL), ordenados por nome.
//
// Não pagina: é usado para popular listas de seleção (ex: combobox no admin de usuários).
func (r *VendedorRepository) List(ctx context.Context, db *sql.DB) ([]models.Vendedor, error) {
	const q = `
		SELECT id, nome, regiao, uf
		FROM vendedores
		WHERE data_desligamento IS NULL
		ORDER BY nome ASC`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("repositories: list vendedores: %w", err)
	}
	defer rows.Close()

	var out []models.Vendedor
	for rows.Next() {
		var v models.Vendedor
		if err := rows.Scan(&v.ID, &v.Nome, &v.Regiao, &v.UF); err != nil {
			return nil, fmt.Errorf("repositories: scan vendedor: %w", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repositories: list vendedores iteração: %w", err)
	}
	return out, nil
}

// ExistsByID verifica se existe um vendedor com o id informado (ativo ou não).
func (r *VendedorRepository) ExistsByID(ctx context.Context, db *sql.DB, id int64) (bool, error) {
	const q = `SELECT 1 FROM vendedores WHERE id = ? LIMIT 1`
	var one int
	err := db.QueryRowContext(ctx, q, id).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("repositories: exists vendedor: %w", err)
	}
	return true, nil
}
