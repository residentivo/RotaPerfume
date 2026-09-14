// Package services (da API) orquestra regras de negócio dos endpoints.
//
// É uma camada fina que delega ao package shared/services (auth_service) e
// shared/repositories — sem manter estado compartilhado entre requests.
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

// tamanhoSenhaGerada é o tamanho da senha aleatória gerada para novos
// usuários e resets administrativos.
const tamanhoSenhaGerada = 16

// Erros exportados para uso em handlers.
var (
	ErrUsuarioNaoEncontrado  = errors.New("usuário não encontrado")
	ErrEmailDuplicado        = errors.New("email já cadastrado")
	ErrRoleInvalido          = errors.New("role inválido (admin|normal)")
	ErrEmailInvalido         = errors.New("email inválido")
	ErrNomeObrigatorio       = errors.New("nome é obrigatório")
	ErrVendedorNaoEncontrado = errors.New("vendedor não encontrado")
)

// UsuarioService agrega regras de negócio sobre usuários.
type UsuarioService struct {
	repo         *repositories.UsuarioRepository
	vendedorRepo *repositories.VendedorRepository
	auth         *sharedsvc.AuthService
	email        sharedsvc.EmailService
	Cfg          *config.Config // exportado para handlers acessarem o DSN
}

// NewUsuarioService cria um UsuarioService com pool de conexão injetado.
// emailSvc é obrigatório (injete sharedsvc.NewNoopEmailService() quando SMTP
// não estiver configurado — nunca deixe nil).
func NewUsuarioService(db *sql.DB, cfg *config.Config, emailSvc sharedsvc.EmailService) *UsuarioService {
	return &UsuarioService{
		repo:         repositories.NewUsuarioRepository(),
		vendedorRepo: repositories.NewVendedorRepository(),
		auth:         sharedsvc.NewAuthService(),
		email:        emailSvc,
		Cfg:          cfg,
	}
}

// gerarSenhaEHash gera uma senha aleatória e retorna a senha em texto claro
// (para envio por email logo em seguida) junto com seu hash bcrypt (para
// persistência). O chamador é responsável por nunca logar/retornar a senha
// em texto claro — apenas usá-la imediatamente para envio de email.
func (s *UsuarioService) gerarSenhaEHash() (senha, hash string, err error) {
	senha, err = sharedsvc.GerarSenhaAleatoria(tamanhoSenhaGerada)
	if err != nil {
		return "", "", fmt.Errorf("gerar senha aleatória: %w", err)
	}
	hash, err = s.auth.HashPassword(s.Cfg, senha)
	if err != nil {
		return "", "", err
	}
	return senha, hash, nil
}

// enviarSenhaInicial dispara o envio da senha por email (best-effort — falha
// de envio nunca é fatal para o fluxo que a chamou, apenas é logada e
// refletida no retorno emailEnviado).
func (s *UsuarioService) enviarSenhaInicial(ctx context.Context, destinatario, nomeUsuario, senha string) (emailEnviado bool) {
	if err := s.email.EnviarSenhaInicial(ctx, destinatario, nomeUsuario, senha); err != nil {
		log.Printf("[usuarios] falha ao enviar email de senha inicial para %s: %v", destinatario, err)
		return false
	}
	return true
}

// validarVendedor confere se idVendedor (quando informado) existe na base.
// idVendedor nil é válido (usuário sem vendedor vinculado).
func (s *UsuarioService) validarVendedor(ctx context.Context, db *sql.DB, idVendedor *int64) error {
	if idVendedor == nil {
		return nil
	}
	existe, err := s.vendedorRepo.ExistsByID(ctx, db, *idVendedor)
	if err != nil {
		return err
	}
	if !existe {
		return ErrVendedorNaoEncontrado
	}
	return nil
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
// orderBy/orderDir controlam a ordenação (whitelist validada no
// repositório); default "id asc".
func (s *UsuarioService) ListUsuarios(ctx context.Context, db *sql.DB, page, limit int, orderBy, orderDir string) ([]models.Usuario, int, error) {
	if s.Cfg.Verbose {
		log.Printf("[usuarios] list page=%d limit=%d order_by=%q order_dir=%q", page, limit, orderBy, orderDir)
	}
	return s.repo.List(ctx, db, page, limit, orderBy, orderDir)
}

// ResetSenha redefine a senha de um usuário para um valor explícito (uso
// interno/testes). Retorna ErrUsuarioNaoEncontrado se não existir.
func (s *UsuarioService) ResetSenha(ctx context.Context, db *sql.DB, id int64, novaSenha string) error {
	hash, err := s.auth.HashPassword(s.Cfg, novaSenha)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePasswordHash(ctx, db, id, hash, false); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrUsuarioNaoEncontrado
		}
		return err
	}
	log.Printf("[usuarios] senha redefinida: id=%d", id)
	return nil
}

// AdminResetPassword gera uma senha aleatória para o usuário identificado por
// id (email e nome já resolvidos pelo chamador — o handler já fez sua própria
// consulta para capturar o hash anterior com fins de auditoria, então evitamos
// uma segunda leitura redundante aqui), atualiza o hash e envia a nova senha
// por email ao endereço cadastrado. A senha nunca é retornada nem logada em
// texto claro.
//
// Retorna emailEnviado=false (sem erro) quando o envio de email falha — o
// reset de senha já foi persistido com sucesso e não deve ser desfeito;
// cabe ao handler avisar o admin que o email não chegou.
// Retorna ErrUsuarioNaoEncontrado se o usuário não existir.
func (s *UsuarioService) AdminResetPassword(ctx context.Context, db *sql.DB, id int64, email, nome string) (emailEnviado bool, err error) {
	senha, hash, err := s.gerarSenhaEHash()
	if err != nil {
		return false, err
	}

	// Senha gerada pelo sistema (não escolhida pelo admin ou pelo usuário) —
	// força a troca no próximo login.
	if err := s.repo.UpdatePasswordHash(ctx, db, id, hash, true); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return false, ErrUsuarioNaoEncontrado
		}
		return false, err
	}

	// Senha já foi persistida com sucesso — falha no envio de email não desfaz
	// o reset, apenas é refletida em emailEnviado para o handler avisar o admin.
	emailEnviado = s.enviarSenhaInicial(ctx, email, nome, senha)

	log.Printf("[usuarios] admin reset: senha redefinida: id=%d email_enviado=%t", id, emailEnviado)
	return emailEnviado, nil
}

// CreateUsuario cria um novo usuário validando role, email duplicado e
// vendedor vinculado. A senha inicial é gerada aleatoriamente e enviada por
// email ao endereço cadastrado — nunca é retornada nem logada em texto claro.
//
// emailEnviado indica se o email com a senha inicial foi entregue com
// sucesso. Se o envio falhar, o usuário já foi criado com sucesso mesmo
// assim (a criação não é desfeita) — o handler deve avisar o admin.
func (s *UsuarioService) CreateUsuario(ctx context.Context, db *sql.DB, input struct {
	Nome       string
	Email      string
	Role       string
	IDVendedor *int64
}) (usuario *models.Usuario, emailEnviado bool, err error) {
	if input.Nome == "" {
		return nil, false, ErrNomeObrigatorio
	}
	if input.Email == "" {
		return nil, false, ErrEmailInvalido
	}
	if input.Role != models.RoleAdmin && input.Role != models.RoleNormal {
		return nil, false, ErrRoleInvalido
	}
	if err := s.validarVendedor(ctx, db, input.IDVendedor); err != nil {
		return nil, false, err
	}

	emailNormalizado := strings.TrimSpace(strings.ToLower(input.Email))

	senha, hash, err := s.gerarSenhaEHash()
	if err != nil {
		return nil, false, err
	}

	u := &models.Usuario{
		Nome:            input.Nome,
		Email:           emailNormalizado,
		PasswordHash:    hash,
		Role:            input.Role,
		IDVendedor:      input.IDVendedor,
		Ativo:           true,
		DeveTrocarSenha: true, // senha aleatória gerada pelo sistema — força troca no primeiro acesso
	}
	if err := s.repo.Create(ctx, db, u); err != nil {
		if errors.Is(err, repositories.ErrEmailDuplicado) {
			return nil, false, ErrEmailDuplicado
		}
		return nil, false, err
	}

	// Usuário já foi criado com sucesso — falha no envio de email não desfaz
	// a criação, apenas é refletida em emailEnviado para o handler avisar o admin.
	emailEnviado = s.enviarSenhaInicial(ctx, u.Email, u.Nome, senha)

	if s.Cfg.Verbose {
		log.Printf("[usuarios] criado: id=%d email=%s role=%s email_enviado=%t", u.ID, u.Email, u.Role, emailEnviado)
	}
	return u, emailEnviado, nil
}

// UpdateUsuario atualiza nome, role e vendedor vinculado.
// Retorna ErrUsuarioNaoEncontrado se não existir.
func (s *UsuarioService) UpdateUsuario(ctx context.Context, db *sql.DB, id int64, nome, role string, idVendedor *int64) (*models.Usuario, error) {
	if nome == "" {
		return nil, ErrNomeObrigatorio
	}
	if role != models.RoleAdmin && role != models.RoleNormal {
		return nil, ErrRoleInvalido
	}
	if err := s.validarVendedor(ctx, db, idVendedor); err != nil {
		return nil, err
	}
	if err := s.repo.Update(ctx, db, id, nome, role, idVendedor); err != nil {
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
