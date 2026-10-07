// Package services (da API) orquestra regras de negócio dos endpoints.
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	repo         *repositories.ClienteRepository
	carteiraRepo *repositories.CarteiraRepository
	Cfg          *config.Config
}

// NewClienteService cria um ClienteService com pool de conexão injetado.
func NewClienteService(db *sql.DB, cfg *config.Config) *ClienteService {
	return &ClienteService{
		repo:         repositories.NewClienteRepository(),
		carteiraRepo: repositories.NewCarteiraRepository(),
		Cfg:          cfg,
	}
}

// ListClientes pagina clientes aplicando os filtros informados.
// page/limit são validados (limit max 100) no repositório.
func (s *ClienteService) ListClientes(ctx context.Context, db *sql.DB, page, limit int, filtro ClienteFiltro) ([]models.Cliente, int, error) {
	if s.Cfg.Verbose {
		log.Printf("[clientes] list page=%d limit=%d uf=%q segmento=%q ativo=%v q=%q vendedor_id=%d",
			page, limit, filtro.UF, filtro.Segmento, filtro.Ativo, filtro.Q, filtro.VendedorID)
	}
	vlog.Printf("cliente_service.go", "ClienteService.ListClientes", "montando literal repositories.ClienteFiltro e declarando repoFiltro")
	repoFiltro := repositories.ClienteFiltro{
		UF:         filtro.UF,
		Segmento:   filtro.Segmento,
		Ativo:      filtro.Ativo,
		Q:          filtro.Q,
		QCNPJ:      termoBuscaCNPJ(filtro.Q),
		VendedorID: filtro.VendedorID,
		OrderBy:    filtro.OrderBy,
		OrderDir:   filtro.OrderDir,
	}
	return s.repo.List(ctx, db, page, limit, repoFiltro)
}

// GetClienteByID busca um cliente por id. Retorna ErrClienteNaoEncontrado se não existir.
func (s *ClienteService) GetClienteByID(ctx context.Context, db *sql.DB, id int64) (*models.Cliente, error) {
	vlog.Printf("cliente_service.go", "ClienteService.GetClienteByID", "chamando s.repo.GetByID e declarando c, err")
	c, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("cliente_service.go", "ClienteService.GetClienteByID", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("cliente_service.go", "ClienteService.GetClienteByID", "verificando condição errors.Is(err, repositories.ErrNotFound)")
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
	vlog.Printf("cliente_service.go", "ClienteService.ToggleAtivoCliente", "chamando s.repo.GetByID e declarando c, err")
	c, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("cliente_service.go", "ClienteService.ToggleAtivoCliente", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("cliente_service.go", "ClienteService.ToggleAtivoCliente", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrClienteNaoEncontrado
		}
		return nil, err
	}
	vlog.Printf("cliente_service.go", "ClienteService.ToggleAtivoCliente", "declarando newAtivo com !c.Ativo")
	newAtivo := !c.Ativo
	vlog.Printf("cliente_service.go", "ClienteService.ToggleAtivoCliente", "verificando condição ativo != nil")
	if ativo != nil {
		vlog.Printf("cliente_service.go", "ClienteService.ToggleAtivoCliente", "atribuindo *ativo a newAtivo")
		newAtivo = *ativo
	}
	vlog.Printf("cliente_service.go", "ClienteService.ToggleAtivoCliente", "chamando s.repo.SetAtivo e declarando err e verificando condição err != nil")
	if err := s.repo.SetAtivo(ctx, db, id, newAtivo); err != nil {
		vlog.Printf("cliente_service.go", "ClienteService.ToggleAtivoCliente", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrClienteNaoEncontrado
		}
		return nil, err
	}
	vlog.Printf("cliente_service.go", "ClienteService.ToggleAtivoCliente", "atribuindo newAtivo a c.Ativo")
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
	vlog.Printf("cliente_service.go", "validarClienteInput", "chamando strings.TrimSpace e atribuindo a razaoSocial")
	razaoSocial = strings.TrimSpace(input.RazaoSocial)
	vlog.Printf("cliente_service.go", "validarClienteInput", "chamando strings.TrimSpace e atribuindo a cnpj")
	cnpj = strings.TrimSpace(input.CNPJ)
	vlog.Printf("cliente_service.go", "validarClienteInput", "chamando strings.TrimSpace e atribuindo a segmento")
	segmento = strings.TrimSpace(input.Segmento)
	vlog.Printf("cliente_service.go", "validarClienteInput", "chamando strings.TrimSpace e atribuindo a cidade")
	cidade = strings.TrimSpace(input.Cidade)
	vlog.Printf("cliente_service.go", "validarClienteInput", "chamando strings.ToUpper e atribuindo a uf")
	uf = strings.ToUpper(strings.TrimSpace(input.UF))
	vlog.Printf("cliente_service.go", "validarClienteInput", "chamando strings.TrimSpace e atribuindo a bairro")
	bairro = strings.TrimSpace(input.Bairro)
	vlog.Printf("cliente_service.go", "validarClienteInput", "chamando strings.TrimSpace e declarando dataCadastroStr")
	dataCadastroStr := strings.TrimSpace(input.DataCadastro)

	vlog.Printf("cliente_service.go", "validarClienteInput", "verificando condição razaoSocial == \"\"")
	if razaoSocial == "" {
		vlog.Printf("cliente_service.go", "validarClienteInput", "atribuindo ErrRazaoSocialObrigatoria a err")
		err = ErrRazaoSocialObrigatoria
		return
	}
	vlog.Printf("cliente_service.go", "validarClienteInput", "verificando condição cnpj == \"\"")
	if cnpj == "" {
		vlog.Printf("cliente_service.go", "validarClienteInput", "atribuindo ErrCNPJObrigatorio a err")
		err = ErrCNPJObrigatorio
		return
	}
	// NEG-01/NEG-02: aceita máscara e minúsculas; grava sem máscara e em
	// MAIÚSCULAS (numérico ou alfanumérico). O dígito verificador é checado
	// à parte, em toda gravação (Create e Update — NEG-04).
	vlog.Printf("cliente_service.go", "validarClienteInput", "chamando normalizarCNPJ e declarando cnpjNormalizado, cnpjOK")
	cnpjNormalizado, cnpjOK := normalizarCNPJ(cnpj)
	vlog.Printf("cliente_service.go", "validarClienteInput", "verificando condição !cnpjOK")
	if !cnpjOK {
		vlog.Printf("cliente_service.go", "validarClienteInput", "atribuindo ErrCNPJInvalido a err")
		err = ErrCNPJInvalido
		return
	}
	vlog.Printf("cliente_service.go", "validarClienteInput", "atribuindo cnpjNormalizado a cnpj")
	cnpj = cnpjNormalizado
	vlog.Printf("cliente_service.go", "validarClienteInput", "verificando condição segmento == \"\"")
	if segmento == "" {
		vlog.Printf("cliente_service.go", "validarClienteInput", "atribuindo ErrSegmentoObrigatorio a err")
		err = ErrSegmentoObrigatorio
		return
	}
	vlog.Printf("cliente_service.go", "validarClienteInput", "verificando condição cidade == \"\"")
	if cidade == "" {
		vlog.Printf("cliente_service.go", "validarClienteInput", "atribuindo ErrCidadeObrigatoria a err")
		err = ErrCidadeObrigatoria
		return
	}
	vlog.Printf("cliente_service.go", "validarClienteInput", "verificando condição len(uf) != 2")
	if len(uf) != 2 {
		vlog.Printf("cliente_service.go", "validarClienteInput", "atribuindo ErrUFInvalida a err")
		err = ErrUFInvalida
		return
	}

	vlog.Printf("cliente_service.go", "validarClienteInput", "verificando condição dataCadastroStr == \"\"")
	if dataCadastroStr == "" {
		vlog.Printf("cliente_service.go", "validarClienteInput", "verificando condição defaultHoje")
		if defaultHoje {
			vlog.Printf("cliente_service.go", "validarClienteInput", "chamando time.Now e atribuindo a dataCadastro")
			dataCadastro = time.Now()
			return
		}
		vlog.Printf("cliente_service.go", "validarClienteInput", "atribuindo ErrDataCadastroInvalida a err")
		err = ErrDataCadastroInvalida
		return
	}
	vlog.Printf("cliente_service.go", "validarClienteInput", "chamando time.ParseInLocation e declarando dataCadastro, parseErr")
	dataCadastro, parseErr := time.ParseInLocation(dataCadastroLayout, dataCadastroStr, time.Local)
	vlog.Printf("cliente_service.go", "validarClienteInput", "verificando condição parseErr != nil")
	if parseErr != nil {
		vlog.Printf("cliente_service.go", "validarClienteInput", "atribuindo ErrDataCadastroInvalida a err")
		err = ErrDataCadastroInvalida
		return
	}
	return
}

// novoClienteValidado valida o input de criação e monta o models.Cliente
// (ativo, data_cadastro default hoje). Compartilhado por CreateCliente e
// CreateClienteNaCarteira.
func novoClienteValidado(input ClienteInput) (*models.Cliente, error) {
	vlog.Printf("cliente_service.go", "novoClienteValidado", "chamando validarClienteInput e declarando razaoSocial, cnpj, segmento, cidade, uf, bairro, dataCadastro, err")
	razaoSocial, cnpj, segmento, cidade, uf, bairro, dataCadastro, err := validarClienteInput(input, true)
	vlog.Printf("cliente_service.go", "novoClienteValidado", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("cliente_service.go", "novoClienteValidado", "verificando condição !cnpjDigitosValidos(cnpj)")
	if !cnpjDigitosValidos(cnpj) {
		return nil, ErrCNPJInvalido
	}
	return &models.Cliente{
		CNPJ:         cnpj,
		RazaoSocial:  razaoSocial,
		Segmento:     segmento,
		Cidade:       cidade,
		UF:           uf,
		Bairro:       bairro,
		DataCadastro: dataCadastro,
		Ativo:        true,
	}, nil
}

// CreateCliente cria um novo cliente, validando os campos obrigatórios.
// cliente_id_origem é gerado nativamente pelo AUTO_INCREMENT do MySQL.
// Usado pelo admin: NÃO cria vínculo de carteira.
func (s *ClienteService) CreateCliente(ctx context.Context, db *sql.DB, input ClienteInput) (*models.Cliente, error) {
	vlog.Printf("cliente_service.go", "ClienteService.CreateCliente", "chamando novoClienteValidado e declarando c, err")
	c, err := novoClienteValidado(input)
	vlog.Printf("cliente_service.go", "ClienteService.CreateCliente", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("cliente_service.go", "ClienteService.CreateCliente", "chamando s.repo.Create e declarando err e verificando condição err != nil")
	if err := s.repo.Create(ctx, db, c); err != nil {
		return nil, mapearErroCNPJDuplicado(err)
	}

	if s.Cfg.Verbose {
		log.Printf("[clientes] criado: cliente_id_origem=%d razao_social=%s", c.ClienteIDOrigem, c.RazaoSocial)
	}
	return s.relerClienteCriado(ctx, db, c), nil
}

// relerClienteCriado relê do banco o cliente recém-gravado (BUG-08), para
// devolver created_at/updated_at preenchidos pelo MySQL. O INSERT já foi
// confirmado: se a releitura falhar, loga e devolve o objeto em memória
// (timestamps zerados) em vez de responder erro para uma gravação que deu
// certo — um retry do cliente esbarraria no CNPJ duplicado.
func (s *ClienteService) relerClienteCriado(ctx context.Context, db *sql.DB, c *models.Cliente) *models.Cliente {
	vlog.Printf("cliente_service.go", "ClienteService.relerClienteCriado", "chamando s.repo.GetByID e declarando gravado, err")
	gravado, err := s.repo.GetByID(ctx, db, c.ClienteIDOrigem)
	vlog.Printf("cliente_service.go", "ClienteService.relerClienteCriado", "verificando condição err != nil")
	if err != nil {
		log.Printf("[clientes] criado, mas falhou a releitura: cliente_id_origem=%d: %v", c.ClienteIDOrigem, err)
		return c
	}
	return gravado
}

// CreateClienteNaCarteira cria um novo cliente e, na MESMA transação, o
// vínculo de carteira ativo (data_inicio = hoje, data_fim = NULL) com o
// vendedor informado. Usado pelo usuário role=normal (SEC-01), para que o
// cliente recém-criado já fique na carteira de quem o cadastrou — sem isso
// ele não conseguiria ver/editar o próprio cadastro.
//
// Erros de validação são devolvidos antes de abrir a transação. Qualquer
// falha de banco faz rollback (nem cliente nem carteira ficam gravados).
func (s *ClienteService) CreateClienteNaCarteira(ctx context.Context, db *sql.DB, input ClienteInput, vendedorID int64) (*models.Cliente, error) {
	vlog.Printf("cliente_service.go", "ClienteService.CreateClienteNaCarteira", "chamando novoClienteValidado e declarando c, err")
	c, err := novoClienteValidado(input)
	vlog.Printf("cliente_service.go", "ClienteService.CreateClienteNaCarteira", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("cliente_service.go", "ClienteService.CreateClienteNaCarteira", "chamando db.BeginTx e declarando tx, err")
	tx, err := db.BeginTx(ctx, nil)
	vlog.Printf("cliente_service.go", "ClienteService.CreateClienteNaCarteira", "verificando condição err != nil")
	if err != nil {
		return nil, fmt.Errorf("services: begin tx criar cliente na carteira: %w", err)
	}
	vlog.Printf("cliente_service.go", "ClienteService.CreateClienteNaCarteira", "agendando defer: tx.Rollback")
	defer tx.Rollback() //nolint:errcheck // rollback é no-op após commit bem-sucedido

	vlog.Printf("cliente_service.go", "ClienteService.CreateClienteNaCarteira", "chamando s.repo.Create e declarando err e verificando condição err != nil")
	if err := s.repo.Create(ctx, tx, c); err != nil {
		return nil, mapearErroCNPJDuplicado(err)
	}
	vlog.Printf("cliente_service.go", "ClienteService.CreateClienteNaCarteira", "montando &models.Carteira e declarando vinculo")
	vinculo := &models.Carteira{
		ClienteID:  c.ClienteIDOrigem,
		VendedorID: vendedorID,
		DataInicio: inicioDoDia(time.Now()),
		DataFim:    nil,
	}
	vlog.Printf("cliente_service.go", "ClienteService.CreateClienteNaCarteira", "chamando s.carteiraRepo.Create e declarando err e verificando condição err != nil")
	if err := s.carteiraRepo.Create(ctx, tx, vinculo); err != nil {
		return nil, err
	}
	vlog.Printf("cliente_service.go", "ClienteService.CreateClienteNaCarteira", "chamando tx.Commit e declarando err e verificando condição err != nil")
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("services: commit criar cliente na carteira: %w", err)
	}

	if s.Cfg.Verbose {
		log.Printf("[clientes] criado na carteira: cliente_id_origem=%d vendedor_id=%d carteira_id=%d",
			c.ClienteIDOrigem, vendedorID, vinculo.CarteiraIDOrigem)
	}
	return s.relerClienteCriado(ctx, db, c), nil
}

// mapearErroCNPJDuplicado traduz a violação do índice UNIQUE de CNPJ
// (repositories.ErrCNPJDuplicado) em ErrCNPJDuplicado; demais erros passam
// inalterados.
func mapearErroCNPJDuplicado(err error) error {
	vlog.Printf("cliente_service.go", "mapearErroCNPJDuplicado", "verificando condição errors.Is(err, repositories.ErrCNPJDuplicado)")
	if errors.Is(err, repositories.ErrCNPJDuplicado) {
		log.Printf("[clientes] cnpj duplicado recusado (uq_clientes_cnpj)")
		return ErrCNPJDuplicado
	}
	return err
}

// inicioDoDia devolve a meia-noite (time.Local) do dia de t — usada para
// colunas DATE, evitando carregar hora/minuto no valor gravado.
func inicioDoDia(t time.Time) time.Time {
	vlog.Printf("cliente_service.go", "inicioDoDia", "chamando t.In(time.Local).Date e declarando y, m, d")
	y, m, d := t.In(time.Local).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}

// UpdateCliente atualiza os campos editáveis de um cliente existente
// (cliente_id_origem e ativo não são alterados por aqui).
// Retorna ErrClienteNaoEncontrado se não existir.
func (s *ClienteService) UpdateCliente(ctx context.Context, db *sql.DB, id int64, input ClienteInput) (*models.Cliente, error) {
	vlog.Printf("cliente_service.go", "ClienteService.UpdateCliente", "chamando validarClienteInput e declarando razaoSocial, cnpj, segmento, cidade, uf, bairro, dataCadastro, err")
	razaoSocial, cnpj, segmento, cidade, uf, bairro, dataCadastro, err := validarClienteInput(input, false)
	vlog.Printf("cliente_service.go", "ClienteService.UpdateCliente", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	// NEG-04: o dígito verificador é exigido em TODA gravação, inclusive
	// quando o CNPJ enviado é igual ao atual. Cliente legado/importado com
	// DV inválido só volta a ser salvo depois de ter o CNPJ corrigido.
	// A existência do cliente (404) é verificada pelo próprio repo.Update
	// (RowsAffected=0 com clientFoundRows=true).
	vlog.Printf("cliente_service.go", "ClienteService.UpdateCliente", "verificando condição !cnpjDigitosValidos(cnpj)")
	if !cnpjDigitosValidos(cnpj) {
		if s.Cfg.Verbose {
			log.Printf("[clientes] update recusado: id=%d cnpj com dígito verificador inválido", id)
		}
		return nil, ErrCNPJInvalido
	}

	vlog.Printf("cliente_service.go", "ClienteService.UpdateCliente", "montando &models.Cliente e declarando c")
	c := &models.Cliente{
		CNPJ:         cnpj,
		RazaoSocial:  razaoSocial,
		Segmento:     segmento,
		Cidade:       cidade,
		UF:           uf,
		Bairro:       bairro,
		DataCadastro: dataCadastro,
	}
	vlog.Printf("cliente_service.go", "ClienteService.UpdateCliente", "chamando s.repo.Update e declarando err e verificando condição err != nil")
	if err := s.repo.Update(ctx, db, id, c); err != nil {
		vlog.Printf("cliente_service.go", "ClienteService.UpdateCliente", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrClienteNaoEncontrado
		}
		return nil, mapearErroCNPJDuplicado(err)
	}

	vlog.Printf("cliente_service.go", "ClienteService.UpdateCliente", "chamando s.repo.GetByID e declarando atualizado, err")
	atualizado, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("cliente_service.go", "ClienteService.UpdateCliente", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[clientes] atualizado: id=%d razao_social=%s", id, razaoSocial)
	}
	return atualizado, nil
}
