// Package services (da API) orquestra regras de negócio dos endpoints.
package services

import (
	"context"
	"database/sql"
	"log"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
)

// VendedorService agrega regras de negócio sobre vendedores.
type VendedorService struct {
	repo *repositories.VendedorRepository
	Cfg  *config.Config
}

// NewVendedorService cria um VendedorService com pool de conexão injetado.
func NewVendedorService(db *sql.DB, cfg *config.Config) *VendedorService {
	return &VendedorService{
		repo: repositories.NewVendedorRepository(),
		Cfg:  cfg,
	}
}

// ListVendedores retorna os vendedores ativos, ordenados por nome.
func (s *VendedorService) ListVendedores(ctx context.Context, db *sql.DB) ([]models.Vendedor, error) {
	vendedores, err := s.repo.List(ctx, db)
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[vendedores] list: total=%d", len(vendedores))
	}
	return vendedores, nil
}
