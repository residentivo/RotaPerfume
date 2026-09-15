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
	ErrOportunidadeNaoEncontrada          = errors.New("oportunidade não encontrada")
	ErrOportunidadeOrigemInvalida         = errors.New("origem é obrigatória")
	ErrOportunidadeEtapaInvalida          = errors.New("etapa é obrigatória")
	ErrOportunidadeClienteInvalido        = errors.New("cliente_id é obrigatório e deve existir")
	ErrOportunidadeVendedorInvalido       = errors.New("vendedor_id é obrigatório e deve existir")
	ErrOportunidadeProbabilidadeInvalida  = errors.New("probabilidade_pct deve estar entre 0 e 100")
	ErrOportunidadeValorEstimadoInvalido  = errors.New("valor_estimado deve ser maior ou igual a zero")
	ErrOportunidadeDataAberturaInvalida   = errors.New("data_abertura inválida (use o formato AAAA-MM-DD)")
	ErrOportunidadeDataFechamentoInvalida = errors.New("data_fechamento inválida (use o formato AAAA-MM-DD)")
	ErrOportunidadeMotivoPerdaObrigatorio = errors.New("motivo_perda é obrigatório quando etapa = \"Fechado perdido\"")
)

// etapaFechadoPerdido é o valor de etapa que exige motivo_perda preenchido.
const etapaFechadoPerdido = "Fechado perdido"

// dataOportunidadeLayout é o formato aceito para data_abertura/data_fechamento
// no payload de criação/edição de oportunidades (mesmo formato de DATE do MySQL).
const dataOportunidadeLayout = "2006-01-02"

// OportunidadeFiltro agrupa os filtros opcionais aceitos por ListOportunidades.
type OportunidadeFiltro struct {
	ClienteID       int64
	VendedorID      int64
	Etapa           string
	Origem          string
	DataAberturaDe  string
	DataAberturaAte string
	Q               string
	OrderBy         string
	OrderDir        string
}

// OportunidadeService agrega regras de negócio sobre oportunidades.
type OportunidadeService struct {
	repo         *repositories.OportunidadeRepository
	clienteRepo  *repositories.ClienteRepository
	vendedorRepo *repositories.VendedorRepository
	Cfg          *config.Config
}

// NewOportunidadeService cria um OportunidadeService com pool de conexão injetado.
func NewOportunidadeService(db *sql.DB, cfg *config.Config) *OportunidadeService {
	return &OportunidadeService{
		repo:         repositories.NewOportunidadeRepository(),
		clienteRepo:  repositories.NewClienteRepository(),
		vendedorRepo: repositories.NewVendedorRepository(),
		Cfg:          cfg,
	}
}

// ListOportunidades pagina oportunidades aplicando os filtros informados.
// page/limit são validados (limit max 100) no repositório.
func (s *OportunidadeService) ListOportunidades(ctx context.Context, db *sql.DB, page, limit int, filtro OportunidadeFiltro) ([]models.Oportunidade, int, error) {
	if s.Cfg.Verbose {
		log.Printf("[oportunidades] list page=%d limit=%d cliente_id=%d vendedor_id=%d etapa=%q origem=%q q=%q",
			page, limit, filtro.ClienteID, filtro.VendedorID, filtro.Etapa, filtro.Origem, filtro.Q)
	}
	repoFiltro := repositories.OportunidadeFiltro{
		ClienteID:       filtro.ClienteID,
		VendedorID:      filtro.VendedorID,
		Etapa:           filtro.Etapa,
		Origem:          filtro.Origem,
		DataAberturaDe:  filtro.DataAberturaDe,
		DataAberturaAte: filtro.DataAberturaAte,
		Q:               filtro.Q,
		OrderBy:         filtro.OrderBy,
		OrderDir:        filtro.OrderDir,
	}
	return s.repo.List(ctx, db, page, limit, repoFiltro)
}

// GetOportunidadeByID busca uma oportunidade por id. Retorna
// ErrOportunidadeNaoEncontrada se não existir.
func (s *OportunidadeService) GetOportunidadeByID(ctx context.Context, db *sql.DB, id int64) (*models.Oportunidade, error) {
	o, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrOportunidadeNaoEncontrada
		}
		return nil, err
	}
	return o, nil
}

// OportunidadeInput agrupa os campos editáveis de uma oportunidade, usados
// tanto na criação quanto na edição.
type OportunidadeInput struct {
	ClienteID        int64
	VendedorID       int64
	Origem           string
	DataAbertura     string // formato AAAA-MM-DD; vazio = default (hoje, apenas na criação)
	Etapa            string
	ProbabilidadePct float64
	ValorEstimado    float64
	DataFechamento   string // formato AAAA-MM-DD; opcional
	CicloDias        *int
	MotivoPerda      string // opcional; obrigatório se etapa = "Fechado perdido"
}

// validarOportunidadeInput aplica as validações comuns a criação e edição,
// normaliza os campos (trim) e resolve datas. defaultHoje controla se
// data_abertura vazio vira a data atual (criação) ou é considerado erro
// (edição, onde o campo já deveria existir).
func (s *OportunidadeService) validarOportunidadeInput(ctx context.Context, db *sql.DB, input OportunidadeInput, defaultHoje bool) (*models.Oportunidade, error) {
	origem := strings.TrimSpace(input.Origem)
	etapa := strings.TrimSpace(input.Etapa)
	motivoPerda := strings.TrimSpace(input.MotivoPerda)
	dataAberturaStr := strings.TrimSpace(input.DataAbertura)
	dataFechamentoStr := strings.TrimSpace(input.DataFechamento)

	if input.ClienteID <= 0 {
		return nil, ErrOportunidadeClienteInvalido
	}
	if input.VendedorID <= 0 {
		return nil, ErrOportunidadeVendedorInvalido
	}
	if origem == "" {
		return nil, ErrOportunidadeOrigemInvalida
	}
	if etapa == "" {
		return nil, ErrOportunidadeEtapaInvalida
	}
	if input.ProbabilidadePct < 0 || input.ProbabilidadePct > 100 {
		return nil, ErrOportunidadeProbabilidadeInvalida
	}
	if input.ValorEstimado < 0 {
		return nil, ErrOportunidadeValorEstimadoInvalido
	}
	if etapa == etapaFechadoPerdido && motivoPerda == "" {
		return nil, ErrOportunidadeMotivoPerdaObrigatorio
	}

	clienteExiste, err := s.clienteRepo.ExistsByID(ctx, db, input.ClienteID)
	if err != nil {
		return nil, err
	}
	if !clienteExiste {
		return nil, ErrOportunidadeClienteInvalido
	}

	vendedorExiste, err := s.vendedorRepo.ExistsByID(ctx, db, input.VendedorID)
	if err != nil {
		return nil, err
	}
	if !vendedorExiste {
		return nil, ErrOportunidadeVendedorInvalido
	}

	var dataAbertura time.Time
	if dataAberturaStr == "" {
		if !defaultHoje {
			return nil, ErrOportunidadeDataAberturaInvalida
		}
		dataAbertura = time.Now()
	} else {
		dataAbertura, err = time.Parse(dataOportunidadeLayout, dataAberturaStr)
		if err != nil {
			return nil, ErrOportunidadeDataAberturaInvalida
		}
	}

	var dataFechamento *time.Time
	if dataFechamentoStr != "" {
		parsed, err := time.Parse(dataOportunidadeLayout, dataFechamentoStr)
		if err != nil {
			return nil, ErrOportunidadeDataFechamentoInvalida
		}
		dataFechamento = &parsed
	}

	var motivoPerdaPtr *string
	if motivoPerda != "" {
		motivoPerdaPtr = &motivoPerda
	}

	return &models.Oportunidade{
		ClienteID:        input.ClienteID,
		VendedorID:       input.VendedorID,
		Origem:           origem,
		DataAbertura:     dataAbertura,
		Etapa:            etapa,
		ProbabilidadePct: input.ProbabilidadePct,
		ValorEstimado:    input.ValorEstimado,
		DataFechamento:   dataFechamento,
		CicloDias:        input.CicloDias,
		MotivoPerda:      motivoPerdaPtr,
	}, nil
}

// CreateOportunidade cria uma nova oportunidade, validando os campos obrigatórios.
func (s *OportunidadeService) CreateOportunidade(ctx context.Context, db *sql.DB, input OportunidadeInput) (*models.Oportunidade, error) {
	o, err := s.validarOportunidadeInput(ctx, db, input, true)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, db, o); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[oportunidades] criada: id=%d cliente_id=%d vendedor_id=%d etapa=%s", o.OportunidadeID, o.ClienteID, o.VendedorID, o.Etapa)
	}
	return o, nil
}

// UpdateOportunidade atualiza os campos editáveis de uma oportunidade
// existente. Retorna ErrOportunidadeNaoEncontrada se não existir.
func (s *OportunidadeService) UpdateOportunidade(ctx context.Context, db *sql.DB, id int64, input OportunidadeInput) (*models.Oportunidade, error) {
	o, err := s.validarOportunidadeInput(ctx, db, input, false)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Update(ctx, db, id, o); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrOportunidadeNaoEncontrada
		}
		return nil, err
	}

	atualizada, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[oportunidades] atualizada: id=%d etapa=%s", id, o.Etapa)
	}
	return atualizada, nil
}
