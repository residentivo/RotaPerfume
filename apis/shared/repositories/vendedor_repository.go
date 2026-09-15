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

// vendedorColunas lista as colunas usadas em GetByID/Create/Update.
const vendedorColunas = `id, nome, regiao, uf, data_admissao, data_desligamento, meta_mensal, created_at, updated_at`

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

// GetByID busca um vendedor pelo ID (ativo ou não). Retorna ErrNotFound se não existir.
func (r *VendedorRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Vendedor, error) {
	q := "SELECT " + vendedorColunas + " FROM vendedores WHERE id = ? LIMIT 1"
	row := db.QueryRowContext(ctx, q, id)
	return scanVendedor(row)
}

// Create insere um novo vendedor e preenche v.ID com o id gerado.
func (r *VendedorRepository) Create(ctx context.Context, db *sql.DB, v *models.Vendedor) error {
	const q = `
		INSERT INTO vendedores (nome, regiao, uf, data_admissao, data_desligamento, meta_mensal)
		VALUES (?, ?, ?, ?, ?, ?)`
	res, err := db.ExecContext(ctx, q,
		v.Nome,
		v.Regiao,
		v.UF,
		v.DataAdmissao,
		v.DataDesligamento,
		v.MetaMensal,
	)
	if err != nil {
		return fmt.Errorf("repositories: create vendedor: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repositories: create vendedor lastInsertId: %w", err)
	}
	v.ID = id
	return nil
}

// Update atualiza os campos editáveis de um vendedor (data_desligamento não é
// alterado por aqui — ver SetDataDesligamento). Retorna ErrNotFound se não existir.
func (r *VendedorRepository) Update(ctx context.Context, db *sql.DB, id int64, v *models.Vendedor) error {
	const q = `
		UPDATE vendedores
		SET nome = ?, regiao = ?, uf = ?, data_admissao = ?, meta_mensal = ?
		WHERE id = ?`
	res, err := db.ExecContext(ctx, q,
		v.Nome,
		v.Regiao,
		v.UF,
		v.DataAdmissao,
		v.MetaMensal,
		id,
	)
	if err != nil {
		return fmt.Errorf("repositories: update vendedor: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: update vendedor rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetDataDesligamento define (ou limpa, se nil) a data de desligamento de um
// vendedor — soft-delete/reativação. Retorna ErrNotFound se não existir.
func (r *VendedorRepository) SetDataDesligamento(ctx context.Context, db *sql.DB, id int64, dataDesligamento *sql.NullTime) error {
	const q = `UPDATE vendedores SET data_desligamento = ? WHERE id = ?`
	res, err := db.ExecContext(ctx, q, dataDesligamento, id)
	if err != nil {
		return fmt.Errorf("repositories: set data_desligamento vendedor: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repositories: set data_desligamento vendedor rowsAffected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanVendedor(s rowScanner) (*models.Vendedor, error) {
	var v models.Vendedor
	if err := s.Scan(
		&v.ID,
		&v.Nome,
		&v.Regiao,
		&v.UF,
		&v.DataAdmissao,
		&v.DataDesligamento,
		&v.MetaMensal,
		&v.CreatedAt,
		&v.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan vendedor: %w", err)
	}
	return &v, nil
}
