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
	refreshRepo  *repositories.RefreshTokenRepository
	Cfg          *config.Config
}

// NewVendedorService cria um VendedorService com pool de conexão injetado.
func NewVendedorService(db *sql.DB, cfg *config.Config) *VendedorService {
	return &VendedorService{
		repo:         repositories.NewVendedorRepository(),
		carteiraRepo: repositories.NewCarteiraRepository(),
		clienteRepo:  repositories.NewClienteRepository(),
		usuarioRepo:  repositories.NewUsuarioRepository(),
		refreshRepo:  repositories.NewRefreshTokenRepository(),
		Cfg:          cfg,
	}
}

// ListVendedores retorna TODOS os vendedores (ativos e inativos), ordenados
// por nome. Cada item traz DataDesligamento (nil = ativo) para que o
// chamador (frontend) possa marcar visualmente os inativos.
func (s *VendedorService) ListVendedores(ctx context.Context, db *sql.DB) ([]repositories.VendedorResumo, error) {
	vlog.Printf("vendedor_service.go", "VendedorService.ListVendedores", "chamando s.repo.List e declarando vendedores, err")
	vendedores, err := s.repo.List(ctx, db)
	vlog.Printf("vendedor_service.go", "VendedorService.ListVendedores", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[vendedores] list: total=%d", len(vendedores))
	}
	return vendedores, nil
}

// ListVendedorProprio retorna, no mesmo formato de ListVendedores, apenas o
// vendedor vendedorID (escopo do usuário normal). O filtro é feito no SQL.
// Lista vazia (não nula) se o vendedor não existir.
func (s *VendedorService) ListVendedorProprio(ctx context.Context, db *sql.DB, vendedorID int64) ([]repositories.VendedorResumo, error) {
	vlog.Printf("vendedor_service.go", "VendedorService.ListVendedorProprio", "chamando s.repo.ListResumoByID e declarando vendedores, err")
	vendedores, err := s.repo.ListResumoByID(ctx, db, vendedorID)
	vlog.Printf("vendedor_service.go", "VendedorService.ListVendedorProprio", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	if s.Cfg.Verbose {
		log.Printf("[vendedores] list escopo: vendedor_id=%d total=%d", vendedorID, len(vendedores))
	}
	return vendedores, nil
}

// GetVendedorDetalhe busca um vendedor e a lista de clientes atualmente
// vinculados a ele (carteira ativa). Retorna ErrVendedorNaoEncontrado se o
// vendedor não existir.
func (s *VendedorService) GetVendedorDetalhe(ctx context.Context, db *sql.DB, vendedorID int64) (*VendedorDetalhe, error) {
	vlog.Printf("vendedor_service.go", "VendedorService.GetVendedorDetalhe", "chamando s.repo.GetByID e declarando v, err")
	v, err := s.repo.GetByID(ctx, db, vendedorID)
	vlog.Printf("vendedor_service.go", "VendedorService.GetVendedorDetalhe", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("vendedor_service.go", "VendedorService.GetVendedorDetalhe", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVendedorNaoEncontrado
		}
		return nil, err
	}

	vlog.Printf("vendedor_service.go", "VendedorService.GetVendedorDetalhe", "chamando s.carteiraRepo.ListClientesByVendedorID e declarando clientes, err")
	clientes, err := s.carteiraRepo.ListClientesByVendedorID(ctx, db, vendedorID)
	vlog.Printf("vendedor_service.go", "VendedorService.GetVendedorDetalhe", "verificando condição err != nil")
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
	vlog.Printf("vendedor_service.go", "VendedorService.ListClientesDoVendedor", "chamando s.repo.ExistsByID e declarando vendedorExiste, err")
	vendedorExiste, err := s.repo.ExistsByID(ctx, db, vendedorID)
	vlog.Printf("vendedor_service.go", "VendedorService.ListClientesDoVendedor", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("vendedor_service.go", "VendedorService.ListClientesDoVendedor", "verificando condição !vendedorExiste")
	if !vendedorExiste {
		return nil, ErrVendedorNaoEncontrado
	}

	vlog.Printf("vendedor_service.go", "VendedorService.ListClientesDoVendedor", "chamando s.carteiraRepo.ListClientesByVendedorID e declarando clientes, err")
	clientes, err := s.carteiraRepo.ListClientesByVendedorID(ctx, db, vendedorID)
	vlog.Printf("vendedor_service.go", "VendedorService.ListClientesDoVendedor", "verificando condição err != nil")
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
	vlog.Printf("vendedor_service.go", "validarVendedorInput", "chamando strings.TrimSpace e atribuindo a nome")
	nome = strings.TrimSpace(input.Nome)
	vlog.Printf("vendedor_service.go", "validarVendedorInput", "chamando strings.TrimSpace e atribuindo a regiao")
	regiao = strings.TrimSpace(input.Regiao)
	vlog.Printf("vendedor_service.go", "validarVendedorInput", "chamando strings.ToUpper e atribuindo a uf")
	uf = strings.ToUpper(strings.TrimSpace(input.UF))
	vlog.Printf("vendedor_service.go", "validarVendedorInput", "chamando strings.TrimSpace e declarando dataAdmissaoStr")
	dataAdmissaoStr := strings.TrimSpace(input.DataAdmissao)
	vlog.Printf("vendedor_service.go", "validarVendedorInput", "atribuindo input.MetaMensal a metaMensal")
	metaMensal = input.MetaMensal

	vlog.Printf("vendedor_service.go", "validarVendedorInput", "verificando condição nome == \"\"")
	if nome == "" {
		vlog.Printf("vendedor_service.go", "validarVendedorInput", "atribuindo ErrVendedorNomeObrigatorio a err")
		err = ErrVendedorNomeObrigatorio
		return
	}
	vlog.Printf("vendedor_service.go", "validarVendedorInput", "verificando condição regiao == \"\"")
	if regiao == "" {
		vlog.Printf("vendedor_service.go", "validarVendedorInput", "atribuindo ErrVendedorRegiaoObrigatoria a err")
		err = ErrVendedorRegiaoObrigatoria
		return
	}
	vlog.Printf("vendedor_service.go", "validarVendedorInput", "verificando condição len(uf) != 2")
	if len(uf) != 2 {
		vlog.Printf("vendedor_service.go", "validarVendedorInput", "atribuindo ErrVendedorUFInvalida a err")
		err = ErrVendedorUFInvalida
		return
	}
	vlog.Printf("vendedor_service.go", "validarVendedorInput", "verificando condição metaMensal < 0")
	if metaMensal < 0 {
		vlog.Printf("vendedor_service.go", "validarVendedorInput", "atribuindo ErrVendedorMetaMensalInvalida a err")
		err = ErrVendedorMetaMensalInvalida
		return
	}

	vlog.Printf("vendedor_service.go", "validarVendedorInput", "verificando condição dataAdmissaoStr == \"\"")
	if dataAdmissaoStr == "" {
		vlog.Printf("vendedor_service.go", "validarVendedorInput", "verificando condição defaultHoje")
		if defaultHoje {
			vlog.Printf("vendedor_service.go", "validarVendedorInput", "chamando time.Now e atribuindo a dataAdmissao")
			dataAdmissao = time.Now()
			return
		}
		vlog.Printf("vendedor_service.go", "validarVendedorInput", "atribuindo ErrVendedorDataAdmissaoInvalida a err")
		err = ErrVendedorDataAdmissaoInvalida
		return
	}
	vlog.Printf("vendedor_service.go", "validarVendedorInput", "chamando time.ParseInLocation e declarando dataAdmissao, parseErr")
	dataAdmissao, parseErr := time.ParseInLocation(dataAdmissaoLayout, dataAdmissaoStr, time.Local)
	vlog.Printf("vendedor_service.go", "validarVendedorInput", "verificando condição parseErr != nil")
	if parseErr != nil {
		vlog.Printf("vendedor_service.go", "validarVendedorInput", "atribuindo ErrVendedorDataAdmissaoInvalida a err")
		err = ErrVendedorDataAdmissaoInvalida
		return
	}
	return
}

// CreateVendedor cria um novo vendedor, validando os campos obrigatórios.
func (s *VendedorService) CreateVendedor(ctx context.Context, db *sql.DB, input VendedorInput) (*models.Vendedor, error) {
	vlog.Printf("vendedor_service.go", "VendedorService.CreateVendedor", "chamando validarVendedorInput e declarando nome, regiao, uf, dataAdmissao, metaMensal, err")
	nome, regiao, uf, dataAdmissao, metaMensal, err := validarVendedorInput(input, true)
	vlog.Printf("vendedor_service.go", "VendedorService.CreateVendedor", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("vendedor_service.go", "VendedorService.CreateVendedor", "montando &models.Vendedor e declarando v")
	v := &models.Vendedor{
		Nome:         nome,
		Regiao:       regiao,
		UF:           uf,
		DataAdmissao: dataAdmissao,
		MetaMensal:   metaMensal,
	}
	vlog.Printf("vendedor_service.go", "VendedorService.CreateVendedor", "chamando s.repo.Create e declarando err e verificando condição err != nil")
	if err := s.repo.Create(ctx, db, v); err != nil {
		return nil, err
	}

	if s.Cfg.Verbose {
		log.Printf("[vendedores] criado: id=%d nome=%s", v.ID, v.Nome)
	}
	return s.relerVendedorCriado(ctx, db, v), nil
}

// relerVendedorCriado relê do banco o vendedor recém-gravado (BUG-09), para
// devolver created_at/updated_at preenchidos pelo MySQL. O INSERT já foi
// confirmado: se a releitura falhar, loga e devolve o objeto em memória
// (timestamps zerados) em vez de responder erro para uma gravação que deu certo.
func (s *VendedorService) relerVendedorCriado(ctx context.Context, db *sql.DB, v *models.Vendedor) *models.Vendedor {
	vlog.Printf("vendedor_service.go", "VendedorService.relerVendedorCriado", "chamando s.repo.GetByID e declarando gravado, err")
	gravado, err := s.repo.GetByID(ctx, db, v.ID)
	vlog.Printf("vendedor_service.go", "VendedorService.relerVendedorCriado", "verificando condição err != nil")
	if err != nil {
		log.Printf("[vendedores] criado, mas falhou a releitura: id=%d: %v", v.ID, err)
		return v
	}
	return gravado
}

// UpdateVendedor atualiza os campos editáveis de um vendedor existente
// (data_desligamento não é alterado por aqui — ver DeleteVendedor).
// Retorna ErrVendedorNaoEncontrado se não existir.
func (s *VendedorService) UpdateVendedor(ctx context.Context, db *sql.DB, id int64, input VendedorInput) (*models.Vendedor, error) {
	vlog.Printf("vendedor_service.go", "VendedorService.UpdateVendedor", "chamando validarVendedorInput e declarando nome, regiao, uf, dataAdmissao, metaMensal, err")
	nome, regiao, uf, dataAdmissao, metaMensal, err := validarVendedorInput(input, false)
	vlog.Printf("vendedor_service.go", "VendedorService.UpdateVendedor", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("vendedor_service.go", "VendedorService.UpdateVendedor", "montando &models.Vendedor e declarando v")
	v := &models.Vendedor{
		Nome:         nome,
		Regiao:       regiao,
		UF:           uf,
		DataAdmissao: dataAdmissao,
		MetaMensal:   metaMensal,
	}
	vlog.Printf("vendedor_service.go", "VendedorService.UpdateVendedor", "chamando s.repo.Update e declarando err e verificando condição err != nil")
	if err := s.repo.Update(ctx, db, id, v); err != nil {
		vlog.Printf("vendedor_service.go", "VendedorService.UpdateVendedor", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVendedorNaoEncontrado
		}
		return nil, err
	}

	vlog.Printf("vendedor_service.go", "VendedorService.UpdateVendedor", "chamando s.repo.GetByID e declarando atualizado, err")
	atualizado, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("vendedor_service.go", "VendedorService.UpdateVendedor", "verificando condição err != nil")
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
	vlog.Printf("vendedor_service.go", "VendedorService.DeleteVendedor", "chamando s.inativarVendedorEUsuarios e declarando usuariosInativados, err")
	usuariosInativados, err := s.inativarVendedorEUsuarios(ctx, db, id)
	vlog.Printf("vendedor_service.go", "VendedorService.DeleteVendedor", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}

	vlog.Printf("vendedor_service.go", "VendedorService.DeleteVendedor", "chamando s.repo.GetByID e declarando v, err")
	v, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("vendedor_service.go", "VendedorService.DeleteVendedor", "verificando condição err != nil")
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
	vlog.Printf("vendedor_service.go", "VendedorService.inativarVendedorEUsuarios", "chamando db.BeginTx e declarando tx, err")
	tx, err := db.BeginTx(ctx, nil)
	vlog.Printf("vendedor_service.go", "VendedorService.inativarVendedorEUsuarios", "verificando condição err != nil")
	if err != nil {
		return 0, fmt.Errorf("services: begin tx inativar vendedor: %w", err)
	}
	vlog.Printf("vendedor_service.go", "VendedorService.inativarVendedorEUsuarios", "agendando defer: tx.Rollback")
	defer tx.Rollback() //nolint:errcheck // rollback é no-op após commit bem-sucedido

	// COALESCE (BUG-04): desligar de novo um vendedor já desligado preserva a
	// data original e responde sucesso; InativarByVendedorID é idempotente.
	vlog.Printf("vendedor_service.go", "VendedorService.inativarVendedorEUsuarios", "chamando s.repo.MarcarDesligamento e declarando err e verificando condição err != nil")
	if err := s.repo.MarcarDesligamento(ctx, tx, id, time.Now()); err != nil {
		vlog.Printf("vendedor_service.go", "VendedorService.inativarVendedorEUsuarios", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return 0, ErrVendedorNaoEncontrado
		}
		return 0, err
	}

	vlog.Printf("vendedor_service.go", "VendedorService.inativarVendedorEUsuarios", "chamando s.usuarioRepo.InativarByVendedorID e declarando usuariosInativados, err")
	usuariosInativados, err := s.usuarioRepo.InativarByVendedorID(ctx, tx, id)
	vlog.Printf("vendedor_service.go", "VendedorService.inativarVendedorEUsuarios", "verificando condição err != nil")
	if err != nil {
		return 0, err
	}

	// SEC-06: na mesma transação, derruba as sessões de refresh de todos os
	// usuários do vendedor. Idempotente (0 tokens pendentes não é erro);
	// falha aqui faz rollback do desligamento inteiro.
	vlog.Printf("vendedor_service.go", "VendedorService.inativarVendedorEUsuarios", "chamando s.refreshRepo.RevokeAllByVendedorID e declarando tokensRevogados, err")
	tokensRevogados, err := s.refreshRepo.RevokeAllByVendedorID(ctx, tx, id, repositories.RevokeReasonInativacao)
	vlog.Printf("vendedor_service.go", "VendedorService.inativarVendedorEUsuarios", "verificando condição err != nil")
	if err != nil {
		return 0, err
	}

	vlog.Printf("vendedor_service.go", "VendedorService.inativarVendedorEUsuarios", "chamando tx.Commit e declarando err e verificando condição err != nil")
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("services: commit inativar vendedor: %w", err)
	}
	if s.Cfg.Verbose {
		log.Printf("[vendedores] desligamento id=%d: refresh_tokens_revogados=%d", id, tokensRevogados)
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
	vlog.Printf("vendedor_service.go", "VendedorService.ReativarVendedor", "chamando s.repo.SetDataDesligamento e declarando err e verificando condição err != nil")
	if err := s.repo.SetDataDesligamento(ctx, db, id, &sql.NullTime{Valid: false}); err != nil {
		vlog.Printf("vendedor_service.go", "VendedorService.ReativarVendedor", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrVendedorNaoEncontrado
		}
		return nil, err
	}

	vlog.Printf("vendedor_service.go", "VendedorService.ReativarVendedor", "chamando s.repo.GetByID e declarando v, err")
	v, err := s.repo.GetByID(ctx, db, id)
	vlog.Printf("vendedor_service.go", "VendedorService.ReativarVendedor", "verificando condição err != nil")
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
	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "chamando s.repo.ExistsByID e declarando vendedorExiste, err")
	vendedorExiste, err := s.repo.ExistsByID(ctx, db, vendedorID)
	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "verificando condição err != nil")
	if err != nil {
		return nil, err
	}
	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "verificando condição !vendedorExiste")
	if !vendedorExiste {
		return nil, ErrVendedorNaoEncontrado
	}

	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "chamando s.clienteRepo.GetByID e declarando cliente, err")
	cliente, err := s.clienteRepo.GetByID(ctx, db, clienteID)
	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrClienteNaoEncontrado
		}
		return nil, err
	}

	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "chamando time.Now e declarando agora")
	agora := time.Now()

	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "chamando s.carteiraRepo.GetVinculoAtivoByClienteID e declarando vinculoAnterior, err")
	vinculoAnterior, err := s.carteiraRepo.GetVinculoAtivoByClienteID(ctx, db, clienteID)
	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "verificando condição err != nil && !errors.Is(err, repositories.ErrNotFound)")
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, err
	}
	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "verificando condição err == nil")
	if err == nil {
		vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "verificando condição vinculoAnterior.VendedorID == vendedorID")
		if vinculoAnterior.VendedorID == vendedorID {
			// Já vinculado a este mesmo vendedor: nada a fazer, retorna o resumo atual.
			return clienteResumoDoVinculo(cliente, vinculoAnterior), nil
		}
		vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "chamando s.carteiraRepo.EncerrarVinculo e declarando err e verificando condição err != nil")
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
	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "chamando s.carteiraRepo.GetVinculoByClienteVendedorData e declarando vinculoDoDia, err")
	vinculoDoDia, err := s.carteiraRepo.GetVinculoByClienteVendedorData(ctx, db, clienteID, vendedorID, agora)
	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "verificando condição err != nil && !errors.Is(err, repositories.ErrNotFound)")
	if err != nil && !errors.Is(err, repositories.ErrNotFound) {
		return nil, err
	}
	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "verificando condição err == nil")
	if err == nil {
		vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "verificando condição vinculoDoDia.DataFim == nil")
		if vinculoDoDia.DataFim == nil {
			// Já ativo (mesmo cenário do vinculoAnterior acima, mas
			// alcançável se o registro do dia não for o vínculo ativo mais
			// recente do cliente por algum motivo): idempotente.
			return clienteResumoDoVinculo(cliente, vinculoDoDia), nil
		}
		vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "chamando s.carteiraRepo.ReativarVinculo e declarando err e verificando condição err != nil")
		if err := s.carteiraRepo.ReativarVinculo(ctx, db, vinculoDoDia.CarteiraIDOrigem); err != nil {
			return nil, err
		}
		vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "atribuindo nil a vinculoDoDia.DataFim")
		vinculoDoDia.DataFim = nil

		if s.Cfg.Verbose {
			log.Printf("[vendedores] vinculo reativado (mesmo dia): vendedor_id=%d cliente_id=%d carteira_id=%d", vendedorID, clienteID, vinculoDoDia.CarteiraIDOrigem)
		}
		return clienteResumoDoVinculo(cliente, vinculoDoDia), nil
	}

	// carteira_id_origem é gerado nativamente pelo AUTO_INCREMENT do MySQL.
	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "montando &models.Carteira e declarando novoVinculo")
	novoVinculo := &models.Carteira{
		ClienteID:  clienteID,
		VendedorID: vendedorID,
		DataInicio: agora,
	}
	vlog.Printf("vendedor_service.go", "VendedorService.VincularCliente", "chamando s.carteiraRepo.Create e declarando err e verificando condição err != nil")
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
	vlog.Printf("vendedor_service.go", "VendedorService.DesvincularCliente", "chamando s.carteiraRepo.GetVinculoAtivo e declarando vinculo, err")
	vinculo, err := s.carteiraRepo.GetVinculoAtivo(ctx, db, vendedorID, clienteID)
	vlog.Printf("vendedor_service.go", "VendedorService.DesvincularCliente", "verificando condição err != nil")
	if err != nil {
		vlog.Printf("vendedor_service.go", "VendedorService.DesvincularCliente", "verificando condição errors.Is(err, repositories.ErrNotFound)")
		if errors.Is(err, repositories.ErrNotFound) {
			return ErrVinculoNaoEncontrado
		}
		return err
	}

	vlog.Printf("vendedor_service.go", "VendedorService.DesvincularCliente", "chamando s.carteiraRepo.EncerrarVinculo e declarando err e verificando condição err != nil")
	if err := s.carteiraRepo.EncerrarVinculo(ctx, db, vinculo.CarteiraIDOrigem, time.Now()); err != nil {
		vlog.Printf("vendedor_service.go", "VendedorService.DesvincularCliente", "verificando condição errors.Is(err, repositories.ErrNotFound)")
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
