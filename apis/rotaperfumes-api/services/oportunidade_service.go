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
	vlog.Printf("oportunidade_service.go", "OportunidadeService.ListOportunidades", "montando literal repositories.OportunidadeFiltro e declarando repoFiltro")
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
	vlog.Printf("oportunidade_service.go", "OportunidadeService.GetOportunidadeByID", "chamando s.repo.GetByID e declarando o, err")
	o, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("oportunidade_service.go", "OportunidadeService.GetOportunidadeByID", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("oportunidade_service.go", "OportunidadeService.GetOportunidadeByID", "verificando condição errors.Is(err, repositories.ErrNotFound)")
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
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "chamando strings.TrimSpace e declarando origem")
	origem := strings.TrimSpace(input.Origem)
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "chamando strings.TrimSpace e declarando etapa")
	etapa := strings.TrimSpace(input.Etapa)
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "chamando strings.TrimSpace e declarando motivoPerda")
	motivoPerda := strings.TrimSpace(input.MotivoPerda)
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "chamando strings.TrimSpace e declarando dataAberturaStr")
	dataAberturaStr := strings.TrimSpace(input.DataAbertura)
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "chamando strings.TrimSpace e declarando dataFechamentoStr")
	dataFechamentoStr := strings.TrimSpace(input.DataFechamento)

	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição input.ClienteID <= 0")
	if input.ClienteID <= 0 {
		return nil, ErrOportunidadeClienteInvalido
	}
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição input.VendedorID <= 0")
	if input.VendedorID <= 0 {
		return nil, ErrOportunidadeVendedorInvalido
	}
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição origem == \"\"")
	if origem == "" {
		return nil, ErrOportunidadeOrigemInvalida
	}
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição etapa == \"\"")
	if etapa == "" {
		return nil, ErrOportunidadeEtapaInvalida
	}
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição input.ProbabilidadePct < 0 || input.ProbabilidadePct > 100")
	if input.ProbabilidadePct < 0 || input.ProbabilidadePct > 100 {
		return nil, ErrOportunidadeProbabilidadeInvalida
	}
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição input.ValorEstimado < 0")
	if input.ValorEstimado < 0 {
		return nil, ErrOportunidadeValorEstimadoInvalido
	}
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição etapa == etapaFechadoPerdido && motivoPerda == \"\"")
	if etapa == etapaFechadoPerdido && motivoPerda == "" {
		return nil, ErrOportunidadeMotivoPerdaObrigatorio
	}

	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "chamando s.clienteRepo.ExistsByID e declarando clienteExiste, err")
	clienteExiste, err := s.clienteRepo.ExistsByID(ctx, db, input.ClienteID)
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição !clienteExiste")
	if !clienteExiste {
		return nil, ErrOportunidadeClienteInvalido
	}

	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "chamando s.vendedorRepo.ExistsByID e declarando vendedorExiste, err")
	vendedorExiste, err := s.vendedorRepo.ExistsByID(ctx, db, input.VendedorID)
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição !vendedorExiste")
	if !vendedorExiste {
		return nil, ErrOportunidadeVendedorInvalido
	}

	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "declarando dataAbertura")
	var dataAbertura time.Time
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição dataAberturaStr == \"\"")
	if dataAberturaStr == "" {
		vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição !defaultHoje")
		if !defaultHoje {
			return nil, ErrOportunidadeDataAberturaInvalida
		}
		vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "chamando time.Now e atribuindo a dataAbertura")
		dataAbertura = time.Now()
	} else {
		vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "chamando time.ParseInLocation e atribuindo a dataAbertura, err")
		dataAbertura, err = time.ParseInLocation(dataOportunidadeLayout, dataAberturaStr, time.Local)
		vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição err != nil")
		if err != nil {
			return nil, ErrOportunidadeDataAberturaInvalida
		}
	}

	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "declarando dataFechamento")
	var dataFechamento *time.Time
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição dataFechamentoStr != \"\"")
	if dataFechamentoStr != "" {
		vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "chamando time.ParseInLocation e declarando parsed, err")
		parsed, err := time.ParseInLocation(dataOportunidadeLayout, dataFechamentoStr, time.Local)
		vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição err != nil")
		if err != nil {
			return nil, ErrOportunidadeDataFechamentoInvalida
		}
		vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "atribuindo &parsed a dataFechamento")
		dataFechamento = &parsed
	}

	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "declarando motivoPerdaPtr")
	var motivoPerdaPtr *string
	vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "verificando condição motivoPerda != \"\"")
	if motivoPerda != "" {
		vlog.Printf("oportunidade_service.go", "OportunidadeService.validarOportunidadeInput", "atribuindo &motivoPerda a motivoPerdaPtr")
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
	vlog.Printf("oportunidade_service.go", "OportunidadeService.CreateOportunidade", "chamando s.validarOportunidadeInput e declarando o, err")
	o, err := s.validarOportunidadeInput(ctx, db, input, true)
	vlog.Printf("oportunidade_service.go", "OportunidadeService.CreateOportunidade", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("oportunidade_service.go", "OportunidadeService.CreateOportunidade", "chamando s.repo.Create e declarando err e verificando condição err != nil")
	if err := s.repo.Create(ctx, db, o); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[oportunidades] criada: id=%d cliente_id=%d vendedor_id=%d etapa=%s", o.OportunidadeID, o.ClienteID, o.VendedorID, o.Etapa)
	}
	return s.relerOportunidadeCriada(ctx, db, o), nil
}

// relerOportunidadeCriada relê do banco a oportunidade recém-gravada (BUG-09),
// para devolver created_at/updated_at preenchidos pelo MySQL. O INSERT já foi
// confirmado: se a releitura falhar, loga e devolve o objeto em memória
// (timestamps zerados) em vez de responder erro para uma gravação que deu
// certo — um retry duplicaria a oportunidade.
func (s *OportunidadeService) relerOportunidadeCriada(ctx context.Context, db *sql.DB, o *models.Oportunidade) *models.Oportunidade {
	vlog.Printf("oportunidade_service.go", "OportunidadeService.relerOportunidadeCriada", "chamando s.repo.GetByID e declarando gravada, err")
	gravada, err := s.repo.GetByID(ctx, db, o.OportunidadeID)
	vlog.Printf("oportunidade_service.go", "OportunidadeService.relerOportunidadeCriada", "verificando condição err != nil")
	if err != nil {
		log.Printf("[oportunidades] criada, mas falhou a releitura: id=%d: %v", o.OportunidadeID, err)
		return o
	}
	return gravada
}

// UpdateOportunidade atualiza os campos editáveis de uma oportunidade
// existente. Retorna ErrOportunidadeNaoEncontrada se não existir.
func (s *OportunidadeService) UpdateOportunidade(ctx context.Context, db *sql.DB, id int64, input OportunidadeInput) (*models.Oportunidade, error) {
	vlog.Printf("oportunidade_service.go", "OportunidadeService.UpdateOportunidade", "chamando s.validarOportunidadeInput e declarando o, err")
	o, err := s.validarOportunidadeInput(ctx, db, input, false)
	vlog.Printf("oportunidade_service.go", "OportunidadeService.UpdateOportunidade", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("oportunidade_service.go", "OportunidadeService.UpdateOportunidade", "chamando s.repo.Update e declarando err e verificando condição err != nil")
	if err := s.repo.Update(ctx, db, id, o); err != nil {
		vlog.Printf("oportunidade_service.go", "OportunidadeService.UpdateOportunidade", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrOportunidadeNaoEncontrada
		}
		return nil, err
	}

	vlog.Printf("oportunidade_service.go", "OportunidadeService.UpdateOportunidade", "chamando s.repo.GetByID e declarando atualizada, err")
	atualizada, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("oportunidade_service.go", "OportunidadeService.UpdateOportunidade", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[oportunidades] atualizada: id=%d etapa=%s", id, o.Etapa)
	}
	return atualizada, nil
}

// DeleteOportunidade remove uma oportunidade (hard delete). Retorna
// ErrOportunidadeNaoEncontrada se não existir. O scope check por carteira é
// responsabilidade do handler chamador, feito antes de invocar este método.
func (s *OportunidadeService) DeleteOportunidade(ctx context.Context, db *sql.DB, id int64) error {
	vlog.Printf("oportunidade_service.go", "OportunidadeService.DeleteOportunidade", "chamando s.repo.Delete e declarando err e verificando condição err != nil")
	if err := s.repo.Delete(ctx, db, id); err != nil {
		vlog.Printf("oportunidade_service.go", "OportunidadeService.DeleteOportunidade", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrOportunidadeNaoEncontrada
		}
		return err
	}

	if s.Cfg.Verbose {
		log.Printf("[oportunidades] excluída: id=%d", id)
	}
	return nil
}
