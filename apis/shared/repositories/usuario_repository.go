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
	"time"

	"github.com/rotaperfumes/shared/models"
)

// ARQ-03: o contexto Identidade migrou para identidade/identidademysql. Aqui
// ficam só as consultas de usuarios que o legado de outros contextos ainda
// usa: GetByID (notificador de alerta, ARQ-07), GetIDVendedorByUsuarioID
// (escopo de carteira, ARQ-04) e InativarByVendedorID (desligamento de
// vendedor na mesma tx, I-09, ARQ-04).

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

// GetIDVendedorByUsuarioID retorna o id_vendedor vinculado ao usuário (nil
// sem vínculo). Retorna ErrNotFound se o usuário não existir.
func (r *UsuarioRepository) GetIDVendedorByUsuarioID(ctx context.Context, db *sql.DB, usuarioID int64) (*int64, error) {
	const q = `SELECT id_vendedor FROM usuarios WHERE id = ? LIMIT 1`
	var idVendedor sql.NullInt64
	if err := db.QueryRowContext(ctx, q, usuarioID).Scan(&idVendedor); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: get id_vendedor usuario: %w", err)
	}
	if !idVendedor.Valid {
		return nil, nil
	}
	v := idVendedor.Int64
	return &v, nil
}

// corteDeSessaoAgora é o instante gravado em tokens_validos_desde (SEC-08):
// relógio do Go, truncado em segundos (precisão do iat do JWT).
func corteDeSessaoAgora() time.Time {
	return time.Now().Truncate(time.Second)
}

// InativarByVendedorID inativa os usuários ativos do vendedor e devolve
// quantos foram afetados (0 não é erro). Aceita *sql.DB ou *sql.Tx; grava
// também o corte de sessão (SEC-08).
func (r *UsuarioRepository) InativarByVendedorID(ctx context.Context, db Execer, vendedorID int64) (int64, error) {
	const q = `UPDATE usuarios SET ativo = 0, tokens_validos_desde = ? WHERE id_vendedor = ? AND ativo = 1`
	res, err := db.ExecContext(ctx, q, corteDeSessaoAgora(), vendedorID)
	if err != nil {
		return 0, fmt.Errorf("repositories: inativar usuarios do vendedor: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("repositories: inativar usuarios do vendedor rowsAffected: %w", err)
	}
	return n, nil
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
