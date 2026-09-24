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
	ErrClienteNaoEncontrado   = errors.New("cliente não encontrado")
	ErrRazaoSocialObrigatoria = errors.New("razão social é obrigatória")
	ErrCNPJObrigatorio        = errors.New("cnpj é obrigatório")
	ErrSegmentoObrigatorio    = errors.New("segmento é obrigatório")
	ErrCidadeObrigatoria      = errors.New("cidade é obrigatória")
	ErrUFInvalida             = errors.New("uf deve ter 2 letras")
	ErrDataCadastroInvalida   = errors.New("data_cadastro inválida (use o formato AAAA-MM-DD)")
)

// dataCadastroLayout é o formato aceito para o campo data_cadastro no
// payload de criação/edição de clientes (mesmo formato de DATE do MySQL).
const dataCadastroLayout = "2006-01-02"

// ClienteFiltro agrupa os filtros opcionais aceitos por ListClientes.
type ClienteFiltro struct {
	UF         string
	Segmento   string
	Ativo      *bool
	Q          string
	VendedorID int64 // > 0 restringe aos clientes na carteira ativa desse vendedor
	OrderBy    string
	OrderDir   string
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
		log.Printf("[clientes] list page=%d limit=%d uf=%q segmento=%q ativo=%v q=%q vendedor_id=%d",
			page, limit, filtro.UF, filtro.Segmento, filtro.Ativo, filtro.Q, filtro.VendedorID)
	}
	repoFiltro := repositories.ClienteFiltro{
		UF:         filtro.UF,
		Segmento:   filtro.Segmento,
		Ativo:      filtro.Ativo,
		Q:          filtro.Q,
		VendedorID: filtro.VendedorID,
		OrderBy:    filtro.OrderBy,
		OrderDir:   filtro.OrderDir,
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

// ClienteInput agrupa os campos editáveis de um cliente, usados tanto na
// criação quanto na edição.
type ClienteInput struct {
	CNPJ         string
	RazaoSocial  string
	Segmento     string
	Cidade       string
	UF           string
	Bairro       string
	DataCadastro string // formato AAAA-MM-DD; vazio = default (hoje, apenas na criação)
}

// validarClienteInput aplica as validações comuns a criação e edição,
// normaliza os campos (trim/uppercase de UF) e resolve data_cadastro.
// defaultHoje controla se data_cadastro vazio vira a data atual (criação)
// ou é considerado erro (edição, onde o campo já deveria existir).
func validarClienteInput(input ClienteInput, defaultHoje bool) (razaoSocial, cnpj, segmento, cidade, uf, bairro string, dataCadastro time.Time, err error) {
	razaoSocial = strings.TrimSpace(input.RazaoSocial)
	cnpj = strings.TrimSpace(input.CNPJ)
	segmento = strings.TrimSpace(input.Segmento)
	cidade = strings.TrimSpace(input.Cidade)
	uf = strings.ToUpper(strings.TrimSpace(input.UF))
	bairro = strings.TrimSpace(input.Bairro)
	dataCadastroStr := strings.TrimSpace(input.DataCadastro)

	if razaoSocial == "" {
		err = ErrRazaoSocialObrigatoria
		return
	}
	if cnpj == "" {
		err = ErrCNPJObrigatorio
		return
	}
	if segmento == "" {
		err = ErrSegmentoObrigatorio
		return
	}
	if cidade == "" {
		err = ErrCidadeObrigatoria
		return
	}
	if len(uf) != 2 {
		err = ErrUFInvalida
		return
	}

	if dataCadastroStr == "" {
		if defaultHoje {
			dataCadastro = time.Now()
			return
		}
		err = ErrDataCadastroInvalida
		return
	}
	dataCadastro, parseErr := time.ParseInLocation(dataCadastroLayout, dataCadastroStr, time.Local)
	if parseErr != nil {
		err = ErrDataCadastroInvalida
		return
	}
	return
}

// CreateCliente cria um novo cliente, validando os campos obrigatórios.
// cliente_id_origem é gerado nativamente pelo AUTO_INCREMENT do MySQL.
func (s *ClienteService) CreateCliente(ctx context.Context, db *sql.DB, input ClienteInput) (*models.Cliente, error) {
	razaoSocial, cnpj, segmento, cidade, uf, bairro, dataCadastro, err := validarClienteInput(input, true)
	if err != nil {
		return nil, err
	}

	c := &models.Cliente{
		CNPJ:         cnpj,
		RazaoSocial:  razaoSocial,
		Segmento:     segmento,
		Cidade:       cidade,
		UF:           uf,
		Bairro:       bairro,
		DataCadastro: dataCadastro,
		Ativo:        true,
	}
	if err := s.repo.Create(ctx, db, c); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[clientes] criado: cliente_id_origem=%d razao_social=%s", c.ClienteIDOrigem, c.RazaoSocial)
	}
	return c, nil
}

// UpdateCliente atualiza os campos editáveis de um cliente existente
// (cliente_id_origem e ativo não são alterados por aqui).
// Retorna ErrClienteNaoEncontrado se não existir.
func (s *ClienteService) UpdateCliente(ctx context.Context, db *sql.DB, id int64, input ClienteInput) (*models.Cliente, error) {
	razaoSocial, cnpj, segmento, cidade, uf, bairro, dataCadastro, err := validarClienteInput(input, false)
	if err != nil {
		return nil, err
	}

	c := &models.Cliente{
		CNPJ:         cnpj,
		RazaoSocial:  razaoSocial,
		Segmento:     segmento,
		Cidade:       cidade,
		UF:           uf,
		Bairro:       bairro,
		DataCadastro: dataCadastro,
	}
	if err := s.repo.Update(ctx, db, id, c); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrClienteNaoEncontrado
		}
		return nil, err
	}

	atualizado, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[clientes] atualizado: id=%d razao_social=%s", id, razaoSocial)
	}
	return atualizado, nil
}
