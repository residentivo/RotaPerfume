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
	"strings"
	"time"

	"github.com/rotaperfumes/shared/models"
)

// ErrNotFound é retornado quando um registro não é encontrado.
var ErrNotFound = errors.New("repositories: registro não encontrado")

// UsuarioRepository agrupa queries da tabela usuarios.
type UsuarioRepository struct{}

// NewUsuarioRepository cria um repositório stateless.
func NewUsuarioRepository() *UsuarioRepository {
	return &UsuarioRepository{}
}

// usuarioSelectComVendedor é a base do SELECT com LEFT JOIN em vendedores,
// usada por GetByEmail, GetByID e List para evitar N+1 queries ao expor o
// nome do vendedor vinculado.
const usuarioSelectComVendedor = `
	SELECT u.id, u.nome, u.email, u.password_hash, u.role, u.id_vendedor, u.ativo,
	       u.deve_trocar_senha, u.created_at, u.updated_at, u.ultimo_login_at, v.nome
	FROM usuarios u
	LEFT JOIN vendedores v ON v.id = u.id_vendedor`

// GetByEmail busca um usuário pelo email. Retorna ErrNotFound se não existir.
func (r *UsuarioRepository) GetByEmail(ctx context.Context, db *sql.DB, email string) (*models.Usuario, error) {
	q := usuarioSelectComVendedor + `
		WHERE u.email = ?
		LIMIT 1`
	row := db.QueryRowContext(ctx, q, email)
	return scanUsuario(row)
}

// GetByID busca um usuário pelo ID. Retorna ErrNotFound se não existir.
func (r *UsuarioRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Usuario, error) {
	q := usuarioSelectComVendedor + `
		WHERE u.id = ?
		LIMIT 1`
	row := db.QueryRowContext(ctx, q, id)
	return scanUsuario(row)
}

// List retorna usuários paginados, mais o total para meta-dados de paginação.
func (r *UsuarioRepository) List(ctx context.Context, db *sql.DB, page, limit int) ([]models.Usuario, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usuarios`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count falhou: %w", err)
	}

	q := usuarioSelectComVendedor + `
		ORDER BY u.id ASC
		LIMIT ? OFFSET ?`
	rows, err := db.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list falhou: %w", err)
	}
	defer rows.Close()

	var out []models.Usuario
	for rows.Next() {
		u, err := scanUsuario(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list iteração: %w", err)
	}
	return out, total, nil
}

// UpdatePasswordHash atualiza o password_hash e a flag deve_trocar_senha de
// um usuário na mesma query, evitando estado inconsistente entre as duas
// colunas caso uma segunda escrita separada falhe.
func (r *UsuarioRepository) UpdatePasswordHash(ctx context.Context, db *sql.DB, id int64, newHash string, deveTrocarSenha bool) error {
	const q = `UPDATE usuarios SET password_hash = ?, deve_trocar_senha = ? WHERE id = ?`
	res, err := db.ExecContext(ctx, q, newHash, deveTrocarSenha, id)
	if err != nil {
		return fmt.Errorf("repositories: update password: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateUltimoLogin marca o timestamp de último login.
func (r *UsuarioRepository) UpdateUltimoLogin(ctx context.Context, db *sql.DB, id int64, t time.Time) error {
	const q = `UPDATE usuarios SET ultimo_login_at = ? WHERE id = ?`
	_, err := db.ExecContext(ctx, q, t, id)
	if err != nil {
		return fmt.Errorf("repositories: update ultimo_login: %w", err)
	}
	return nil
}

// ErrEmailDuplicado é retornado quando o email já existe (UNIQUE constraint).
var ErrEmailDuplicado = errors.New("repositories: email já cadastrado")

// Create insere um novo usuário e retorna o registro populado (id, created_at, updated_at).
func (r *UsuarioRepository) Create(ctx context.Context, db *sql.DB, u *models.Usuario) error {
	const q = `
		INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := db.ExecContext(ctx, q, u.Nome, u.Email, u.PasswordHash, u.Role, u.IDVendedor, u.Ativo, u.DeveTrocarSenha)
	if err != nil {
		// MySQL duplicate-key error code = 1062.
		if strings.Contains(err.Error(), "Error 1062") || strings.Contains(err.Error(), "Duplicate entry") {
			return ErrEmailDuplicado
		}
		return fmt.Errorf("repositories: insert usuario: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("repositories: last insert id: %w", err)
	}
	created, err := r.GetByID(ctx, db, id)
	if err != nil {
		return err
	}
	*u = *created
	return nil
}

// Update atualiza nome, role e o vendedor vinculado de um usuário.
// idVendedor nil grava NULL na coluna id_vendedor.
// Retorna ErrNotFound se não existir.
func (r *UsuarioRepository) Update(ctx context.Context, db *sql.DB, id int64, nome, role string, idVendedor *int64) error {
	const q = `UPDATE usuarios SET nome = ?, role = ?, id_vendedor = ? WHERE id = ?`
	res, err := db.ExecContext(ctx, q, nome, role, idVendedor, id)
	if err != nil {
		return fmt.Errorf("repositories: update usuario: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetAtivo ativa/inativa um usuário (toggle). Retorna ErrNotFound se não existir.
func (r *UsuarioRepository) SetAtivo(ctx context.Context, db *sql.DB, id int64, ativo bool) error {
	const q = `UPDATE usuarios SET ativo = ? WHERE id = ?`
	res, err := db.ExecContext(ctx, q, ativo, id)
	if err != nil {
		return fmt.Errorf("repositories: set ativo: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetDeveTrocarSenha marca/desmarca a flag de primeiro acesso (troca de
// senha obrigatória). Retorna ErrNotFound se não existir.
func (r *UsuarioRepository) SetDeveTrocarSenha(ctx context.Context, db *sql.DB, id int64, valor bool) error {
	const q = `UPDATE usuarios SET deve_trocar_senha = ? WHERE id = ?`
	res, err := db.ExecContext(ctx, q, valor, id)
	if err != nil {
		return fmt.Errorf("repositories: set deve_trocar_senha: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// rowScanner é a interface comum entre *sql.Row e *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanUsuario(s rowScanner) (*models.Usuario, error) {
	var u models.Usuario
	var idVendedor sql.NullInt64
	var ultimoLogin sql.NullTime
	var vendedorNome sql.NullString
	var deveTrocarSenha sql.NullBool
	if err := s.Scan(
		&u.ID,
		&u.Nome,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&idVendedor,
		&u.Ativo,
		&deveTrocarSenha,
		&u.CreatedAt,
		&u.UpdatedAt,
		&ultimoLogin,
		&vendedorNome,
	); err != nil {
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
