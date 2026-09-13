// Package services (da API) orquestra regras de negócio dos endpoints.
package services

import (
	"context"
	"database/sql"
	"errors"
	"log"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
)

// ErrClienteNaoEncontrado é retornado quando o cliente não existe.
var ErrClienteNaoEncontrado = errors.New("cliente não encontrado")

// ClienteFiltro agrupa os filtros opcionais aceitos por ListClientes.
type ClienteFiltro struct {
	UF       string
	Segmento string
	Ativo    *bool
	Q        string
}

// ClienteService agrega regras de negócio sobre clientes.
type ClienteService struct {
	repo *repositories.ClienteRepository
	Cfg  *config.Config
}

// NewClienteService cria um ClienteService com pool de conexão injetado.
func NewClienteService(db *sql.DB, cfg *config.Config) *ClienteService {
	return &ClienteService{
		repo: repositories.NewClienteRepository(),
		Cfg:  cfg,
	}
}

// ListClientes pagina clientes aplicando os filtros informados.
// page/limit são validados (limit max 100) no repositório.
func (s *ClienteService) ListClientes(ctx context.Context, db *sql.DB, page, limit int, filtro ClienteFiltro) ([]models.Cliente, int, error) {
	if s.Cfg.Verbose {
		log.Printf("[clientes] list page=%d limit=%d uf=%q segmento=%q ativo=%v q=%q",
			page, limit, filtro.UF, filtro.Segmento, filtro.Ativo, filtro.Q)
	}
	repoFiltro := repositories.ClienteFiltro{
		UF:       filtro.UF,
		Segmento: filtro.Segmento,
		Ativo:    filtro.Ativo,
		Q:        filtro.Q,
	}
	return s.repo.List(ctx, db, page, limit, repoFiltro)
}

// GetClienteByID busca um cliente por id. Retorna ErrClienteNaoEncontrado se não existir.
func (s *ClienteService) GetClienteByID(ctx context.Context, db *sql.DB, id int64) (*models.Cliente, error) {
	c, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrClienteNaoEncontrado
		}
		return nil, err
	}
	return c, nil
}

// ToggleAtivoCliente ativa/inativa um cliente. Se ativo for nil, inverte o
// status atual (toggle). Retorna ErrClienteNaoEncontrado se não existir.
func (s *ClienteService) ToggleAtivoCliente(ctx context.Context, db *sql.DB, id int64, ativo *bool) (*models.Cliente, error) {
	c, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrClienteNaoEncontrado
		}
		return nil, err
	}
	newAtivo := !c.Ativo
	if ativo != nil {
		newAtivo = *ativo
	}
	if err := s.repo.SetAtivo(ctx, db, id, newAtivo); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrClienteNaoEncontrado
		}
		return nil, err
	}
	c.Ativo = newAtivo
	if s.Cfg.Verbose {
		log.Printf("[clientes] ativo toggle: id=%d ativo=%t", id, newAtivo)
	}
	return c, nil
}
