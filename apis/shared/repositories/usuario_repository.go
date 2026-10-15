// Package repositories contém funções puras de acesso a dados (stateless).
//
// Recebem *sql.DB explicitamente para evitar ciclo de dependência com
// os services da API. Cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rotaperfumes/shared/models"
)

// ARQ-03: o contexto Identidade migrou para identidade/identidademysql. Aqui
// fica só GetByID (notificador de alerta, ARQ-07). ARQ-04: o vínculo do
// usuário (escopo) e a inativação por vendedor (I-09) foram para o
// identidademysql (VendedorDoUsuario, InativarDoVendedor).

// UsuarioRepository agrupa as queries legadas da tabela usuarios.
type UsuarioRepository struct{}

// NewUsuarioRepository cria um repositório stateless.
func NewUsuarioRepository() *UsuarioRepository {
	return &UsuarioRepository{}
}

// usuarioSelectComVendedor é a base do SELECT com LEFT JOIN em vendedores.
const usuarioSelectComVendedor = `
	SELECT u.id, u.nome, u.email, u.password_hash, u.role, u.id_vendedor, u.ativo,
	       u.deve_trocar_senha, u.created_at, u.updated_at, u.ultimo_login_at, v.nome
	FROM usuarios u
	LEFT JOIN vendedores v ON v.id = u.id_vendedor`

// GetByID busca um usuário pelo ID. Retorna ErrNotFound se não existir.
func (r *UsuarioRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Usuario, error) {
	q := usuarioSelectComVendedor + `
		WHERE u.id = ?
		LIMIT 1`
	return scanUsuario(db.QueryRowContext(ctx, q, id))
}

func scanUsuario(s rowScanner) (*models.Usuario, error) {
	var u models.Usuario
	var idVendedor sql.NullInt64
	var ultimoLogin sql.NullTime
	var vendedorNome sql.NullString
	var deveTrocarSenha sql.NullBool
	if err := s.Scan(&u.ID, &u.Nome, &u.Email, &u.PasswordHash, &u.Role, &idVendedor, &u.Ativo,
		&deveTrocarSenha, &u.CreatedAt, &u.UpdatedAt, &ultimoLogin, &vendedorNome); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan usuario: %w", err)
	}
	if idVendedor.Valid {
		v := idVendedor.Int64
		u.IDVendedor = &v
	}
	u.DeveTrocarSenha = deveTrocarSenha.Valid && deveTrocarSenha.Bool
	if ultimoLogin.Valid {
		t := ultimoLogin.Time
		u.UltimoLoginAt = &t
	}
	if vendedorNome.Valid {
		n := vendedorNome.String
		u.VendedorNome = &n
	}
	return &u, nil
}
