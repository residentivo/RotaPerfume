// Package services (da API) orquestra regras de negócio dos endpoints.
package services

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
)

// Erros exportados para uso em handlers.
var (
	ErrVisitaNaoEncontrada     = errors.New("visita não encontrada")
	ErrVisitaClienteInvalido   = errors.New("cliente_id é obrigatório e deve existir")
	ErrVisitaVendedorInvalido  = errors.New("vendedor_id é obrigatório e deve existir")
	ErrVisitaDataInvalida      = errors.New("data_visita inválida (use o formato AAAA-MM-DD)")
	ErrVisitaResultadoInvalido = errors.New("resultado é obrigatório")
	ErrVisitaDuracaoInvalida   = errors.New("duracao_min deve ser maior ou igual a zero")
)

// dataVisitaLayout é o formato aceito para data_visita no payload de
// criação/edição de visitas (mesmo formato de DATE do MySQL).
const dataVisitaLayout = "2006-01-02"

// VisitaFiltro agrupa os filtros opcionais aceitos por ListVisitas.
type VisitaFiltro struct {
	ClienteID     int64
	VendedorID    int64
	Resultado     string
	DataVisitaDe  string
	DataVisitaAte string
	Q             string
	OrderBy       string
	OrderDir      string
}

// VisitaService agrega regras de negócio sobre visitas.
type VisitaService struct {
	repo         *repositories.VisitaRepository
	clienteRepo  *repositories.ClienteRepository
	vendedorRepo *repositories.VendedorRepository
	Cfg          *config.Config
}

// NewVisitaService cria um VisitaService com pool de conexão injetado.
func NewVisitaService(db *sql.DB, cfg *config.Config) *VisitaService {
	return &VisitaService{
		repo:         repositories.NewVisitaRepository(),
		clienteRepo:  repositories.NewClienteRepository(),
		vendedorRepo: repositories.NewVendedorRepository(),
		Cfg:          cfg,
	}
}

// ListVisitas pagina visitas aplicando os filtros informados. page/limit são
// validados (limit max 100) no repositório.
func (s *VisitaService) ListVisitas(ctx context.Context, db *sql.DB, page, limit int, filtro VisitaFiltro) ([]models.Visita, int, error) {
	if s.Cfg.Verbose {
		log.Printf("[visitas] list page=%d limit=%d cliente_id=%d vendedor_id=%d resultado=%q q=%q",
			page, limit, filtro.ClienteID, filtro.VendedorID, filtro.Resultado, filtro.Q)
	}
	repoFiltro := repositories.VisitaFiltro{
		ClienteID:     filtro.ClienteID,
		VendedorID:    filtro.VendedorID,
		Resultado:     filtro.Resultado,
		DataVisitaDe:  filtro.DataVisitaDe,
		DataVisitaAte: filtro.DataVisitaAte,
		Q:             filtro.Q,
		OrderBy:       filtro.OrderBy,
		OrderDir:      filtro.OrderDir,
	}
	return s.repo.List(ctx, db, page, limit, repoFiltro)
}

// GetVisitaByID busca uma visita por id. Retorna ErrVisitaNaoEncontrada se
// não existir.
func (s *VisitaService) GetVisitaByID(ctx context.Context, db *sql.DB, id int64) (*models.Visita, error) {
	v, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVisitaNaoEncontrada
		}
		return nil, err
	}
	return v, nil
}

// VisitaInput agrupa os campos editáveis de uma visita, usados tanto na
// criação quanto na edição.
type VisitaInput struct {
	ClienteID  int64
	VendedorID int64
	DataVisita string // formato AAAA-MM-DD
	Resultado  string
	DuracaoMin int
}

// validarVisitaInput aplica as validações comuns a criação e edição e
// normaliza os campos (trim).
func (s *VisitaService) validarVisitaInput(ctx context.Context, db *sql.DB, input VisitaInput) (*models.Visita, error) {
	resultado := strings.TrimSpace(input.Resultado)
	dataVisitaStr := strings.TrimSpace(input.DataVisita)

	if input.ClienteID <= 0 {
		return nil, ErrVisitaClienteInvalido
	}
	if input.VendedorID <= 0 {
		return nil, ErrVisitaVendedorInvalido
	}
	if resultado == "" {
		return nil, ErrVisitaResultadoInvalido
	}
	if input.DuracaoMin < 0 {
		return nil, ErrVisitaDuracaoInvalida
	}
	if dataVisitaStr == "" {
		return nil, ErrVisitaDataInvalida
	}

	clienteExiste, err := s.clienteRepo.ExistsByID(ctx, db, input.ClienteID)
	if err != nil {
		return nil, err
	}
	if !clienteExiste {
		return nil, ErrVisitaClienteInvalido
	}

	vendedorExiste, err := s.vendedorRepo.ExistsByID(ctx, db, input.VendedorID)
	if err != nil {
		return nil, err
	}
	if !vendedorExiste {
		return nil, ErrVisitaVendedorInvalido
	}

	dataVisita, err := time.Parse(dataVisitaLayout, dataVisitaStr)
	if err != nil {
		return nil, ErrVisitaDataInvalida
	}

	return &models.Visita{
		ClienteID:  input.ClienteID,
		VendedorID: input.VendedorID,
		DataVisita: dataVisita,
		Resultado:  resultado,
		DuracaoMin: input.DuracaoMin,
	}, nil
}

// CreateVisita cria uma nova visita, validando os campos obrigatórios.
func (s *VisitaService) CreateVisita(ctx context.Context, db *sql.DB, input VisitaInput) (*models.Visita, error) {
	v, err := s.validarVisitaInput(ctx, db, input)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, db, v); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[visitas] criada: id=%d cliente_id=%d vendedor_id=%d resultado=%s", v.VisitaID, v.ClienteID, v.VendedorID, v.Resultado)
	}
	return v, nil
}

// UpdateVisita atualiza os campos editáveis de uma visita existente.
// Retorna ErrVisitaNaoEncontrada se não existir.
func (s *VisitaService) UpdateVisita(ctx context.Context, db *sql.DB, id int64, input VisitaInput) (*models.Visita, error) {
	v, err := s.validarVisitaInput(ctx, db, input)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, db, id, v); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVisitaNaoEncontrada
		}
		return nil, err
	}

	atualizada, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[visitas] atualizada: id=%d resultado=%s", id, v.Resultado)
	}
	return atualizada, nil
}
