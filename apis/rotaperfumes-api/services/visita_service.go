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
	"github.com/rotaperfumes/shared/vlog"
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
	vlog.Printf("visita_service.go", "VisitaService.ListVisitas", "montando literal repositories.VisitaFiltro e declarando repoFiltro")
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
	vlog.Printf("visita_service.go", "VisitaService.GetVisitaByID", "chamando s.repo.GetByID e declarando v, err")
	v, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("visita_service.go", "VisitaService.GetVisitaByID", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("visita_service.go", "VisitaService.GetVisitaByID", "verificando condição errors.Is(err, repositories.ErrNotFound)")
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
	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "chamando strings.TrimSpace e declarando resultado")
	resultado := strings.TrimSpace(input.Resultado)
	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "chamando strings.TrimSpace e declarando dataVisitaStr")
	dataVisitaStr := strings.TrimSpace(input.DataVisita)

	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "verificando condição input.ClienteID <= 0")
	if input.ClienteID <= 0 {
		return nil, ErrVisitaClienteInvalido
	}
	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "verificando condição input.VendedorID <= 0")
	if input.VendedorID <= 0 {
		return nil, ErrVisitaVendedorInvalido
	}
	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "verificando condição resultado == \"\"")
	if resultado == "" {
		return nil, ErrVisitaResultadoInvalido
	}
	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "verificando condição input.DuracaoMin < 0")
	if input.DuracaoMin < 0 {
		return nil, ErrVisitaDuracaoInvalida
	}
	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "verificando condição dataVisitaStr == \"\"")
	if dataVisitaStr == "" {
		return nil, ErrVisitaDataInvalida
	}

	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "chamando s.clienteRepo.ExistsByID e declarando clienteExiste, err")
	clienteExiste, err := s.clienteRepo.ExistsByID(ctx, db, input.ClienteID)
	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "verificando condição !clienteExiste")
	if !clienteExiste {
		return nil, ErrVisitaClienteInvalido
	}

	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "chamando s.vendedorRepo.ExistsByID e declarando vendedorExiste, err")
	vendedorExiste, err := s.vendedorRepo.ExistsByID(ctx, db, input.VendedorID)
	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "verificando condição !vendedorExiste")
	if !vendedorExiste {
		return nil, ErrVisitaVendedorInvalido
	}

	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "chamando time.ParseInLocation e declarando dataVisita, err")
	dataVisita, err := time.ParseInLocation(dataVisitaLayout, dataVisitaStr, time.Local)
	vlog.Printf("visita_service.go", "VisitaService.validarVisitaInput", "verificando condição err != nil")
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
	vlog.Printf("visita_service.go", "VisitaService.CreateVisita", "chamando s.validarVisitaInput e declarando v, err")
	v, err := s.validarVisitaInput(ctx, db, input)
	vlog.Printf("visita_service.go", "VisitaService.CreateVisita", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("visita_service.go", "VisitaService.CreateVisita", "chamando s.repo.Create e declarando err e verificando condição err != nil")
	if err := s.repo.Create(ctx, db, v); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[visitas] criada: id=%d cliente_id=%d vendedor_id=%d resultado=%s", v.VisitaID, v.ClienteID, v.VendedorID, v.Resultado)
	}
	return s.relerVisitaCriada(ctx, db, v), nil
}

// relerVisitaCriada relê do banco a visita recém-gravada (BUG-09), para
// devolver created_at/updated_at preenchidos pelo MySQL. O INSERT já foi
// confirmado: se a releitura falhar, loga e devolve o objeto em memória
// (timestamps zerados) em vez de responder erro para uma gravação que deu
// certo — um retry duplicaria a visita.
func (s *VisitaService) relerVisitaCriada(ctx context.Context, db *sql.DB, v *models.Visita) *models.Visita {
	vlog.Printf("visita_service.go", "VisitaService.relerVisitaCriada", "chamando s.repo.GetByID e declarando gravada, err")
	gravada, err := s.repo.GetByID(ctx, db, v.VisitaID)
	vlog.Printf("visita_service.go", "VisitaService.relerVisitaCriada", "verificando condição err != nil")
	if err != nil {
		log.Printf("[visitas] criada, mas falhou a releitura: id=%d: %v", v.VisitaID, err)
		return v
	}
	return gravada
}

// UpdateVisita atualiza os campos editáveis de uma visita existente.
// Retorna ErrVisitaNaoEncontrada se não existir.
func (s *VisitaService) UpdateVisita(ctx context.Context, db *sql.DB, id int64, input VisitaInput) (*models.Visita, error) {
	vlog.Printf("visita_service.go", "VisitaService.UpdateVisita", "chamando s.validarVisitaInput e declarando v, err")
	v, err := s.validarVisitaInput(ctx, db, input)
	vlog.Printf("visita_service.go", "VisitaService.UpdateVisita", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("visita_service.go", "VisitaService.UpdateVisita", "chamando s.repo.Update e declarando err e verificando condição err != nil")
	if err := s.repo.Update(ctx, db, id, v); err != nil {
		vlog.Printf("visita_service.go", "VisitaService.UpdateVisita", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVisitaNaoEncontrada
		}
		return nil, err
	}

	vlog.Printf("visita_service.go", "VisitaService.UpdateVisita", "chamando s.repo.GetByID e declarando atualizada, err")
	atualizada, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("visita_service.go", "VisitaService.UpdateVisita", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[visitas] atualizada: id=%d resultado=%s", id, v.Resultado)
	}
	return atualizada, nil
}

// DeleteVisita remove uma visita (hard delete). Retorna
// ErrVisitaNaoEncontrada se não existir. O scope check por carteira é
// responsabilidade do handler chamador, feito antes de invocar este método.
func (s *VisitaService) DeleteVisita(ctx context.Context, db *sql.DB, id int64) error {
	vlog.Printf("visita_service.go", "VisitaService.DeleteVisita", "chamando s.repo.Delete e declarando err e verificando condição err != nil")
	if err := s.repo.Delete(ctx, db, id); err != nil {
		vlog.Printf("visita_service.go", "VisitaService.DeleteVisita", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrVisitaNaoEncontrada
		}
		return err
	}

	if s.Cfg.Verbose {
		log.Printf("[visitas] excluída: id=%d", id)
	}
	return nil
}
