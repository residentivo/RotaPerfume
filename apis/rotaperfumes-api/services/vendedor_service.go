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
)

// Erros exportados para uso em handlers.
//
// ErrVendedorNaoEncontrado já é declarado em usuario_service.go (mesmo
// pacote services) e é reaproveitado aqui.
var (
	ErrVendedorNomeObrigatorio      = errors.New("nome é obrigatório")
	ErrVendedorRegiaoObrigatoria    = errors.New("regiao é obrigatória")
	ErrVendedorUFInvalida           = errors.New("uf deve ter 2 letras")
	ErrVendedorDataAdmissaoInvalida = errors.New("data_admissao inválida (use o formato AAAA-MM-DD)")
	ErrVendedorMetaMensalInvalida   = errors.New("meta_mensal deve ser maior ou igual a zero")
	ErrVinculoNaoEncontrado         = errors.New("vínculo entre cliente e vendedor não encontrado")
)

// dataAdmissaoLayout é o formato aceito para o campo data_admissao no
// payload de criação/edição de vendedores (mesmo formato de DATE do MySQL).
const dataAdmissaoLayout = "2006-01-02"

// VendedorDetalhe agrega os dados de um vendedor com a lista de clientes
// atualmente vinculados a ele (carteira ativa), no mesmo padrão
// master-detail usado em PedidoDetalhe.
type VendedorDetalhe struct {
	models.Vendedor
	Clientes []repositories.ClienteResumo `json:"clientes"`
}

// VendedorService agrega regras de negócio sobre vendedores.
type VendedorService struct {
	repo         *repositories.VendedorRepository
	carteiraRepo *repositories.CarteiraRepository
	clienteRepo  *repositories.ClienteRepository
	usuarioRepo  *repositories.UsuarioRepository
	Cfg          *config.Config
}

// NewVendedorService cria um VendedorService com pool de conexão injetado.
func NewVendedorService(db *sql.DB, cfg *config.Config) *VendedorService {
	return &VendedorService{
		repo:         repositories.NewVendedorRepository(),
		carteiraRepo: repositories.NewCarteiraRepository(),
		clienteRepo:  repositories.NewClienteRepository(),
		usuarioRepo:  repositories.NewUsuarioRepository(),
		Cfg:          cfg,
	}
}

// ListVendedores retorna TODOS os vendedores (ativos e inativos), ordenados
// por nome. Cada item traz DataDesligamento (nil = ativo) para que o
// chamador (frontend) possa marcar visualmente os inativos.
func (s *VendedorService) ListVendedores(ctx context.Context, db *sql.DB) ([]repositories.VendedorResumo, error) {
	vendedores, err := s.repo.List(ctx, db)
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[vendedores] list: total=%d", len(vendedores))
	}
	return vendedores, nil
}

// GetVendedorDetalhe busca um vendedor e a lista de clientes atualmente
// vinculados a ele (carteira ativa). Retorna ErrVendedorNaoEncontrado se o
// vendedor não existir.
func (s *VendedorService) GetVendedorDetalhe(ctx context.Context, db *sql.DB, vendedorID int64) (*VendedorDetalhe, error) {
	v, err := s.repo.GetByID(ctx, db, vendedorID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVendedorNaoEncontrado
		}
		return nil, err
	}

	clientes, err := s.carteiraRepo.ListClientesByVendedorID(ctx, db, vendedorID)
	if err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[vendedores] detalhe: id=%d clientes=%d", vendedorID, len(clientes))
	}

	return &VendedorDetalhe{
		Vendedor: *v,
		Clientes: clientes,
	}, nil
}

// ListClientesDoVendedor retorna os clientes vinculados (carteira ativa,
// data_fim IS NULL) a um vendedor. Retorna ErrVendedorNaoEncontrado se o
// vendedor não existir.
func (s *VendedorService) ListClientesDoVendedor(ctx context.Context, db *sql.DB, vendedorID int64) ([]repositories.ClienteResumo, error) {
	vendedorExiste, err := s.repo.ExistsByID(ctx, db, vendedorID)
	if err != nil {
		return nil, err
	}
	if !vendedorExiste {
		return nil, ErrVendedorNaoEncontrado
	}

	clientes, err := s.carteiraRepo.ListClientesByVendedorID(ctx, db, vendedorID)
	if err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[vendedores] clientes do vendedor: vendedor_id=%d total=%d", vendedorID, len(clientes))
	}
	return clientes, nil
}

// VendedorInput agrupa os campos editáveis de um vendedor, usados tanto na
// criação quanto na edição.
type VendedorInput struct {
	Nome         string
	Regiao       string
	UF           string
	DataAdmissao string // formato AAAA-MM-DD; vazio = default (hoje, apenas na criação)
	MetaMensal   float64
}

// validarVendedorInput aplica as validações comuns a criação e edição,
// normaliza os campos (trim/uppercase de UF) e resolve data_admissao.
// defaultHoje controla se data_admissao vazio vira a data atual (criação)
// ou é considerado erro (edição, onde o campo já deveria existir).
func validarVendedorInput(input VendedorInput, defaultHoje bool) (nome, regiao, uf string, dataAdmissao time.Time, metaMensal float64, err error) {
	nome = strings.TrimSpace(input.Nome)
	regiao = strings.TrimSpace(input.Regiao)
	uf = strings.ToUpper(strings.TrimSpace(input.UF))
	dataAdmissaoStr := strings.TrimSpace(input.DataAdmissao)
	metaMensal = input.MetaMensal

	if nome == "" {
		err = ErrVendedorNomeObrigatorio
		return
	}
	if regiao == "" {
		err = ErrVendedorRegiaoObrigatoria
		return
	}
	if len(uf) != 2 {
		err = ErrVendedorUFInvalida
		return
	}
	if metaMensal < 0 {
		err = ErrVendedorMetaMensalInvalida
		return
	}

	if dataAdmissaoStr == "" {
		if defaultHoje {
			dataAdmissao = time.Now()
			return
		}
		err = ErrVendedorDataAdmissaoInvalida
		return
	}
	dataAdmissao, parseErr := time.ParseInLocation(dataAdmissaoLayout, dataAdmissaoStr, time.Local)
	if parseErr != nil {
		err = ErrVendedorDataAdmissaoInvalida
		return
	}
	return
}

// CreateVendedor cria um novo vendedor, validando os campos obrigatórios.
func (s *VendedorService) CreateVendedor(ctx context.Context, db *sql.DB, input VendedorInput) (*models.Vendedor, error) {
	nome, regiao, uf, dataAdmissao, metaMensal, err := validarVendedorInput(input, true)
	if err != nil {
		return nil, err
	}

	v := &models.Vendedor{
		Nome:         nome,
		Regiao:       regiao,
		UF:           uf,
		DataAdmissao: dataAdmissao,
		MetaMensal:   metaMensal,
	}
	if err := s.repo.Create(ctx, db, v); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[vendedores] criado: id=%d nome=%s", v.ID, v.Nome)
	}
	return v, nil
}

// UpdateVendedor atualiza os campos editáveis de um vendedor existente
// (data_desligamento não é alterado por aqui — ver DeleteVendedor).
// Retorna ErrVendedorNaoEncontrado se não existir.
func (s *VendedorService) UpdateVendedor(ctx context.Context, db *sql.DB, id int64, input VendedorInput) (*models.Vendedor, error) {
	nome, regiao, uf, dataAdmissao, metaMensal, err := validarVendedorInput(input, false)
	if err != nil {
		return nil, err
	}

	v := &models.Vendedor{
		Nome:         nome,
		Regiao:       regiao,
		UF:           uf,
		DataAdmissao: dataAdmissao,
		MetaMensal:   metaMensal,
	}
	if err := s.repo.Update(ctx, db, id, v); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVendedorNaoEncontrado
		}
		return nil, err
	}

	atualizado, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[vendedores] atualizado: id=%d nome=%s", id, nome)
	}
	return atualizado, nil
}

// DeleteVendedor inativa um vendedor (soft-delete via data_desligamento =
// hoje), preservando o histórico de carteiras/pedidos, e inativa na MESMA
// transação todos os usuários vinculados a ele (usuarios.id_vendedor), para
// que não continuem acessando o sistema. Retorna ErrVendedorNaoEncontrado se
// o vendedor não existir (nesse caso nenhum usuário é alterado).
func (s *VendedorService) DeleteVendedor(ctx context.Context, db *sql.DB, id int64) (*models.Vendedor, error) {
	usuariosInativados, err := s.inativarVendedorEUsuarios(ctx, db, id)
	if err != nil {
		return nil, err
	}

	v, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[vendedores] inativado (soft-delete): id=%d usuarios_inativados=%d", id, usuariosInativados)
	}
	return v, nil
}

// inativarVendedorEUsuarios grava data_desligamento e inativa os usuários
// vinculados ao vendedor de forma atômica. Retorna quantos usuários foram
// inativados.
func (s *VendedorService) inativarVendedorEUsuarios(ctx context.Context, db *sql.DB, id int64) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("services: begin tx inativar vendedor: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback é no-op após commit bem-sucedido

	// COALESCE (BUG-04): desligar de novo um vendedor já desligado preserva a
	// data original e responde sucesso; InativarByVendedorID é idempotente.
	if err := s.repo.MarcarDesligamento(ctx, tx, id, time.Now()); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return 0, ErrVendedorNaoEncontrado
		}
		return 0, err
	}

	usuariosInativados, err := s.usuarioRepo.InativarByVendedorID(ctx, tx, id)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("services: commit inativar vendedor: %w", err)
	}
	return usuariosInativados, nil
}

// ReativarVendedor reverte o soft-delete de um vendedor, limpando
// data_desligamento. Retorna ErrVendedorNaoEncontrado se não existir.
//
// Os usuários vinculados NÃO são reativados automaticamente: o admin pode
// tê-los inativado por outro motivo. A reativação do usuário continua manual
// (tela de usuários).
func (s *VendedorService) ReativarVendedor(ctx context.Context, db *sql.DB, id int64) (*models.Vendedor, error) {
	if err := s.repo.SetDataDesligamento(ctx, db, id, &sql.NullTime{Valid: false}); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVendedorNaoEncontrado
		}
		return nil, err
	}

	v, err := s.repo.GetByID(ctx, db, id)
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[vendedores] reativado: id=%d", id)
	}
	return v, nil
}

// VincularCliente inclui um cliente na carteira ativa de um vendedor.
//
// Se o cliente já possuir um vínculo ativo com OUTRO vendedor, esse vínculo
// anterior é encerrado automaticamente (data_fim = agora) e um novo vínculo é
// criado para o vendedor informado — ou seja, a "transferência de carteira"
// é implícita nesta operação, preservando o histórico via múltiplas linhas
// em `carteiras`. Retorna ErrVendedorNaoEncontrado ou ErrClienteNaoEncontrado
// se algum dos dois não existir.
func (s *VendedorService) VincularCliente(ctx context.Context, db *sql.DB, vendedorID, clienteID int64) (*repositories.ClienteResumo, error) {
	vendedorExiste, err := s.repo.ExistsByID(ctx, db, vendedorID)
	if err != nil {
		return nil, err
	}
	if !vendedorExiste {
		return nil, ErrVendedorNaoEncontrado
	}

	cliente, err := s.clienteRepo.GetByID(ctx, db, clienteID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrClienteNaoEncontrado
		}
		return nil, err
	}

	agora := time.Now()

	vinculoAnterior, err := s.carteiraRepo.GetVinculoAtivoByClienteID(ctx, db, clienteID)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, err
	}
	if err == nil {
		if vinculoAnterior.VendedorID == vendedorID {
			// Já vinculado a este mesmo vendedor: nada a fazer, retorna o resumo atual.
			return clienteResumoDoVinculo(cliente, vinculoAnterior), nil
		}
		if err := s.carteiraRepo.EncerrarVinculo(ctx, db, vinculoAnterior.CarteiraIDOrigem, agora); err != nil {
			return nil, err
		}
		if s.Cfg.Verbose {
			log.Printf("[vendedores] carteira transferida: cliente_id=%d de vendedor_id=%d para vendedor_id=%d", clienteID, vinculoAnterior.VendedorID, vendedorID)
		}
	}

	// A coluna data_inicio é DATE (sem hora), com unique key em
	// (cliente_id, vendedor_id, data_inicio). Se este par já tiver uma linha
	// para hoje (ex.: vinculado e desvinculado no mesmo dia), reativamos essa
	// linha em vez de inserir uma nova — evita violar a unique key e
	// preserva a data_inicio original do vínculo daquele dia.
	vinculoDoDia, err := s.carteiraRepo.GetVinculoByClienteVendedorData(ctx, db, clienteID, vendedorID, agora)
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, err
	}
	if err == nil {
		if vinculoDoDia.DataFim == nil {
			// Já ativo (mesmo cenário do vinculoAnterior acima, mas
			// alcançável se o registro do dia não for o vínculo ativo mais
			// recente do cliente por algum motivo): idempotente.
			return clienteResumoDoVinculo(cliente, vinculoDoDia), nil
		}
		if err := s.carteiraRepo.ReativarVinculo(ctx, db, vinculoDoDia.CarteiraIDOrigem); err != nil {
			return nil, err
		}
		vinculoDoDia.DataFim = nil

		if s.Cfg.Verbose {
			log.Printf("[vendedores] vinculo reativado (mesmo dia): vendedor_id=%d cliente_id=%d carteira_id=%d", vendedorID, clienteID, vinculoDoDia.CarteiraIDOrigem)
		}
		return clienteResumoDoVinculo(cliente, vinculoDoDia), nil
	}

	// carteira_id_origem é gerado nativamente pelo AUTO_INCREMENT do MySQL.
	novoVinculo := &models.Carteira{
		ClienteID:  clienteID,
		VendedorID: vendedorID,
		DataInicio: agora,
	}
	if err := s.carteiraRepo.Create(ctx, db, novoVinculo); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[vendedores] cliente vinculado: vendedor_id=%d cliente_id=%d carteira_id=%d", vendedorID, clienteID, novoVinculo.CarteiraIDOrigem)
	}

	return clienteResumoDoVinculo(cliente, novoVinculo), nil
}

// DesvincularCliente encerra o vínculo ativo entre um vendedor e um cliente
// específicos (não afeta vínculos do cliente com outros vendedores). Retorna
// ErrVinculoNaoEncontrado se não houver vínculo ativo entre os dois.
func (s *VendedorService) DesvincularCliente(ctx context.Context, db *sql.DB, vendedorID, clienteID int64) error {
	vinculo, err := s.carteiraRepo.GetVinculoAtivo(ctx, db, vendedorID, clienteID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrVinculoNaoEncontrado
		}
		return err
	}

	if err := s.carteiraRepo.EncerrarVinculo(ctx, db, vinculo.CarteiraIDOrigem, time.Now()); err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrVinculoNaoEncontrado
		}
		return err
	}

	if s.Cfg.Verbose {
		log.Printf("[vendedores] cliente desvinculado: vendedor_id=%d cliente_id=%d", vendedorID, clienteID)
	}
	return nil
}

// clienteResumoDoVinculo monta um ClienteResumo a partir dos dados já
// carregados do cliente e do vínculo de carteira, evitando uma nova query.
func clienteResumoDoVinculo(cliente *models.Cliente, vinculo *models.Carteira) *repositories.ClienteResumo {
	return &repositories.ClienteResumo{
		ID:          cliente.ClienteIDOrigem,
		CNPJ:        cliente.CNPJ,
		RazaoSocial: cliente.RazaoSocial,
		Segmento:    cliente.Segmento,
		Cidade:      cliente.Cidade,
		UF:          cliente.UF,
		CarteiraID:  vinculo.CarteiraIDOrigem,
		DataInicio:  vinculo.DataInicio,
		DataFim:     vinculo.DataFim,
	}
}
