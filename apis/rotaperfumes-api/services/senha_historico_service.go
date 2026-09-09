// Package services (da API) orquestra regras de negócio de histórico de senhas.
package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/rotaperfumes/shared/repositories"
)

const (
	// TipoResetUsuario indica que o usuário alterou própria senha.
	TipoResetUsuario = "usuario"
	// TipoResetAdmin indica que um admin resetou a senha do usuário.
	TipoResetAdmin = "admin"
	// TipoResetPrimeiroAcesso indica primeiro acesso do usuário.
	TipoResetPrimeiroAcesso = "primeiro_acesso"
	// TipoResetEsquecimento indica recuperação por esquecimento.
	TipoResetEsquecimento = "esquecimento"
)

// SenhaHistoricoService gerencia histórico de alterações de senha.
type SenhaHistoricoService struct {
	repo *repositories.SenhaHistoricoRepository
}

// NewSenhaHistoricoService cria um SenhaHistoricoService.
func NewSenhaHistoricoService() *SenhaHistoricoService {
	return &SenhaHistoricoService{
		repo: repositories.NewSenhaHistoricoRepository(),
	}
}

// Registrar insere um registro de auditoria de alteração de senha.
// resetadoPorID é o ID do admin que resetou (nil se foi o próprio usuário).
// tipo é "usuario", "admin", "primeiro_acesso" ou "esquecimento".
func (s *SenhaHistoricoService) Registrar(ctx context.Context, db *sql.DB, usuarioID int64, resetadoPorID *int64, senhaHashAnterior, ipOrigem, userAgent, tipo string) error {
	h := &repositories.SenhaHistorico{
		UsuarioID:         usuarioID,
		SenhaHashAnterior: senhaHashAnterior,
		IPOrigem:          ipOrigem,
		UserAgent:         userAgent,
		TipoReset:         tipo,
	}
	if resetadoPorID != nil {
		h.ResetadoPorID.Valid = true
		h.ResetadoPorID.Int64 = *resetadoPorID
	}

	if err := s.repo.Create(ctx, db, h); err != nil {
		log.Printf("[senha-historico] Registrar: %v", err)
		return fmt.Errorf("registrar histórico de senha: %w", err)
	}

	log.Printf("[senha-historico] registrado: usuario_id=%d tipo=%s por=%v", usuarioID, tipo, resetadoPorID)
	return nil
}

// ListarPorUsuario retorna histórico de senhas de um usuário (paginado).
func (s *SenhaHistoricoService) ListarPorUsuario(ctx context.Context, db *sql.DB, usuarioID int64, page, limit int) ([]repositories.SenhaHistorico, int, error) {
	return s.repo.FindByUsuario(ctx, db, usuarioID, page, limit)
}

// ListarTodos retorna todo histórico de senhas (paginado) — admin only.
func (s *SenhaHistoricoService) ListarTodos(ctx context.Context, db *sql.DB, page, limit int) ([]repositories.SenhaHistorico, int, error) {
	return s.repo.FindAll(ctx, db, page, limit)
}
