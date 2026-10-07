package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
	"github.com/rotaperfumes/shared/vlog"
)

// ErrEmailResetFalhou indica que o e-mail com a nova senha de um usuário não
// saiu. O reset desse usuário já foi gravado; o lote para ali (OPS-01) para
// não trancar os demais se o SMTP tiver caído.
var ErrEmailResetFalhou = errors.New("reset em massa: e-mail com a nova senha não foi enviado")

// Origem gravada em senha_historico pelos resets em massa.
const (
	ResetMassaIPOrigem  = "127.0.0.1"
	ResetMassaUserAgent = "cmd/resetsenhas (OPS-01)"
)

// ItemResetMassa é o resultado de um usuário no reset em massa.
type ItemResetMassa struct {
	ID    int64
	Email string
	Role  string
	// Status: "simulado", "resetado" ou "falha_email".
	Status string
}

// ResetSenhasAtivosService redefine a senha de todos os usuários ativos com o
// mesmo fluxo do POST /api/admin/reset-password (OPS-01): senha aleatória,
// hash Argon2id com pepper, deve_trocar_senha, e-mail, histórico e revogação
// dos refresh tokens. A senha nunca é logada nem devolvida.
type ResetSenhasAtivosService struct {
	repo     *repositories.UsuarioRepository
	usuarios *UsuarioService
	senhas   *SenhaHistoricoService
	refresh  *RefreshTokenService
}

// NewResetSenhasAtivosService monta o serviço; usuarios deve ter um
// EmailService real (o chamador garante que o SMTP está configurado).
func NewResetSenhasAtivosService(usuarios *UsuarioService) *ResetSenhasAtivosService {
	return &ResetSenhasAtivosService{
		repo:     repositories.NewUsuarioRepository(),
		usuarios: usuarios,
		senhas:   NewSenhaHistoricoService(),
		refresh:  NewRefreshTokenService(),
	}
}

// ListarAtivos devolve os usuários que o reset em massa vai atingir.
func (s *ResetSenhasAtivosService) ListarAtivos(ctx context.Context, db *sql.DB) ([]models.Usuario, error) {
	return s.repo.ListAtivos(ctx, db)
}

// Executar redefine a senha de cada usuário ativo, em ordem de id. Com
// simular=true nada é gravado nem enviado. Para no primeiro e-mail que
// falhar, devolvendo ErrEmailResetFalhou junto com os itens processados até
// ali (o último com Status "falha_email").
func (s *ResetSenhasAtivosService) Executar(ctx context.Context, db *sql.DB, simular bool) ([]ItemResetMassa, error) {
	vlog.Printf("reset_senhas_ativos_service.go", "ResetSenhasAtivosService.Executar", "chamando s.ListarAtivos e declarando ativos, err")
	ativos, err := s.ListarAtivos(ctx, db)
	vlog.Printf("reset_senhas_ativos_service.go", "ResetSenhasAtivosService.Executar", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("reset_senhas_ativos_service.go", "ResetSenhasAtivosService.Executar", "chamando make e declarando itens")
	itens := make([]ItemResetMassa, 0, len(ativos))
	vlog.Printf("reset_senhas_ativos_service.go", "ResetSenhasAtivosService.Executar", "iniciando loop range sobre ativos")
	for _, u := range ativos {
		item := ItemResetMassa{ID: u.ID, Email: u.Email, Role: u.Role, Status: "simulado"}
		if simular {
			itens = append(itens, item)
			continue
		}

		emailEnviado, err := s.usuarios.AdminResetPassword(ctx, db, u.ID, u.Email, u.Nome)
		if err != nil {
			return itens, fmt.Errorf("reset em massa: usuario_id=%d: %w", u.ID, err)
		}
		// Mesmo pós-processamento do handler de reset administrativo; falhas
		// aqui só geram log (a senha nova já está gravada).
		if err := s.senhas.Registrar(ctx, db, u.ID, nil, u.PasswordHash, ResetMassaIPOrigem, ResetMassaUserAgent, "admin"); err != nil {
			log.Printf("[reset-massa] falha ao registrar histórico do usuario_id=%d: %v", u.ID, err)
		}
		if err := s.refresh.RevokeAllUserTokens(ctx, db, u.ID, repositories.RevokeReasonSenha); err != nil {
			log.Printf("[reset-massa] falha ao revogar refresh tokens do usuario_id=%d: %v", u.ID, err)
		}

		if !emailEnviado {
			item.Status = "falha_email"
			itens = append(itens, item)
			return itens, fmt.Errorf("%w: usuario_id=%d", ErrEmailResetFalhou, u.ID)
		}
		item.Status = "resetado"
		itens = append(itens, item)
	}
	vlog.Printf("reset_senhas_ativos_service.go", "ResetSenhasAtivosService.Executar", "loop range concluído sobre ativos: %d itens", len(ativos))
	return itens, nil
}
