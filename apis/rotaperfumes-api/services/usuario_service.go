// Package services (da API) orquestra regras de negócio dos endpoints.
//
// É uma camada fina que delega ao package shared/services (auth_service) e
// shared/repositories — sem manter estado compartilhado entre requests.
package services

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

// DefaultPassword é a senha padrão atribuída a novos usuários.
const DefaultPassword = "Mudar@123"

// Erros exportados para uso em handlers.
var (
	ErrUsuarioNaoEncontrado = errors.New("usuário não encontrado")
	ErrEmailDuplicado       = errors.New("email já cadastrado")
	ErrRoleInvalido         = errors.New("role inválido (admin|normal)")
	ErrEmailInvalido        = errors.New("email inválido")
	ErrNomeObrigatorio      = errors.New("nome é obrigatório")
)

// UsuarioService agrega regras de negócio sobre usuários.
type UsuarioService struct {
	repo *repositories.UsuarioRepository
	auth *sharedsvc.AuthService
	Cfg  *config.Config // exportado para handlers acessarem o DSN
}

// NewUsuarioService cria um UsuarioService com pool de conexão injetado.
func NewUsuarioService(db *sql.DB, cfg *config.Config) *UsuarioService {
	return &UsuarioService{
		repo: repositories.NewUsuarioRepository(),
		auth: sharedsvc.NewAuthService(),
		Cfg:  cfg,
	}
}

// GetUsuarioByEmail busca por email. Retorna ErrUsuarioNaoEncontrado se não existir.
func (s *UsuarioService) GetUsuarioByEmail(ctx context.Context, db *sql.DB, email string) (*models.Usuario, error) {
	u, err := s.repo.GetByEmail(ctx, db, email)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrUsuarioNaoEncontrado
		}
		return nil, err
	}
	return u, nil
}

// GetUsuarioByID busca por id.
func (s *UsuarioService) GetUsuarioByID(ctx context.Context, db *sql.DB, id int64) (*models.Usuario, error) {
	u, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrUsuarioNaoEncontrado
		}
		return nil, err
	}
	return u, nil
}

// ListUsuarios pagina usuários. page/limit são validados (limit max 100).
func (s *UsuarioService) ListUsuarios(ctx context.Context, db *sql.DB, page, limit int) ([]models.Usuario, int, error) {
	if s.Cfg.Verbose {
		log.Printf("[usuarios] list page=%d limit=%d", page, limit)
	}
	return s.repo.List(ctx, db, page, limit)
}

// ResetSenha redefine a senha de um usuário. Retorna ErrUsuarioNaoEncontrado se não existir.
func (s *UsuarioService) ResetSenha(ctx context.Context, db *sql.DB, id int64, novaSenha string) error {
	hash, err := s.auth.HashPassword(s.Cfg, novaSenha)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePasswordHash(ctx, db, id, hash); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrUsuarioNaoEncontrado
		}
		return err
	}
	log.Printf("[usuarios] senha redefinida: id=%d", id)
	return nil
}

// CreateUsuario cria um novo usuário validando role e email duplicado.
func (s *UsuarioService) CreateUsuario(ctx context.Context, db *sql.DB, input struct {
	Nome  string
	Email string
	Role  string
}) (*models.Usuario, error) {
	if input.Nome == "" {
		return nil, ErrNomeObrigatorio
	}
	if input.Email == "" {
		return nil, ErrEmailInvalido
	}
	if input.Role != models.RoleAdmin && input.Role != models.RoleNormal {
		return nil, ErrRoleInvalido
	}
	// Gera hash da senha padrão.
	hash, err := s.auth.HashPassword(s.Cfg, DefaultPassword)
	if err != nil {
		return nil, err
	}
	u := &models.Usuario{
		Nome:         input.Nome,
		Email:        strings.TrimSpace(strings.ToLower(input.Email)),
		PasswordHash: hash,
		Role:         input.Role,
		Ativo:        true,
	}
	if err := s.repo.Create(ctx, db, u); err != nil {
		if errors.Is(err, repositories.ErrEmailDuplicado) {
			return nil, ErrEmailDuplicado
		}
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[usuarios] criado: id=%d email=%s role=%s", u.ID, u.Email, u.Role)
	}
	return u, nil
}

// UpdateUsuario atualiza nome e role. Retorna ErrUsuarioNaoEncontrado se não existir.
func (s *UsuarioService) UpdateUsuario(ctx context.Context, db *sql.DB, id int64, nome, role string) (*models.Usuario, error) {
	if nome == "" {
		return nil, ErrNomeObrigatorio
	}
	if role != models.RoleAdmin && role != models.RoleNormal {
		return nil, ErrRoleInvalido
	}
	if err := s.repo.Update(ctx, db, id, nome, role); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrUsuarioNaoEncontrado
		}
		return nil, err
	}
	u, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[usuarios] atualizado: id=%d nome=%s role=%s", id, nome, role)
	}
	return u, nil
}

// ToggleAtivoUsuario ativa/inativa um usuário. Retorna ErrUsuarioNaoEncontrado se não existir.
func (s *UsuarioService) ToggleAtivoUsuario(ctx context.Context, db *sql.DB, id int64, ativo *bool) (*models.Usuario, error) {
	// Se ativo é nil, inverte o status atual (toggle).
	// Busca usuário para inverter.
	u, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrUsuarioNaoEncontrado
		}
		return nil, err
	}
	newAtivo := !u.Ativo
	if ativo != nil {
		newAtivo = *ativo
	}
	if err := s.repo.SetAtivo(ctx, db, id, newAtivo); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrUsuarioNaoEncontrado
		}
		return nil, err
	}
	u.Ativo = newAtivo
	if s.Cfg.Verbose {
		log.Printf("[usuarios] ativo toggle: id=%d ativo=%t", id, newAtivo)
	}
	return u, nil
}

// ParsePagination lê ?page= e ?limit= da query string, com defaults e validação.
func ParsePagination(pageStr, limitStr string) (page, limit int) {
	page, _ = strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit, _ = strconv.Atoi(limitStr)
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return
}
