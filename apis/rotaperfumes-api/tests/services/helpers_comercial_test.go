package services_test

// ARQ-04: ClienteService, VendedorService, VisitaService e
// OportunidadeService saíram de rotaperfumes-api/services. Os testes (mesmos
// nomes e asserções) usam os nomes antigos, agora declarados AQUI como
// repassadores finos para comercialapp.Casos montado sobre os adapters reais
// (comercialmysql, identidademysql) e o db do sqlmock. Nenhuma regra mora
// aqui.
//
// O legado não tinha Principal nos services: os repassadores chamam os casos
// de uso como admin (escopo Irrestrito, sem consulta de escopo, a mesma
// sequência de SQL do service legado). Os dois métodos que o legado recebia
// com o vendedor do escopo (CreateClienteNaCarteira, ListVendedorProprio e o
// ListClientes com VendedorID) usam um usuário restrito cujo vínculo e
// desligamento vêm de um fake SEM SQL (o escopo em si é testado nas suítes
// Escopo* e em shared/tests/comercial). Sai no ARQ-07.

import (
	"context"
	"database/sql"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/comercial/comercialapp"
	"github.com/rotaperfumes/shared/comercial/comercialmysql"
	"github.com/rotaperfumes/shared/config"
	shareddb "github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"github.com/rotaperfumes/shared/kernel"
)

// Comandos com os nomes antigos (mesmos campos).
type (
	ClienteInput      = comercial.DadosCliente
	VendedorInput     = comercial.DadosVendedor
	VisitaInput       = comercial.DadosVisita
	OportunidadeInput = comercial.DadosOportunidade
)

// Filtros legados (o VendedorID do cliente era o escopo; o de visita e
// oportunidade, filtro de negócio).
type (
	ClienteFiltro struct {
		UF, Segmento string
		Ativo        *bool
		Q            string
		VendedorID   int64
		OrderBy      string
		OrderDir     string
	}
	VisitaFiltro struct {
		ClienteID, VendedorID       int64
		Resultado                   string
		DataVisitaDe, DataVisitaAte string
		Q, OrderBy, OrderDir        string
	}
	OportunidadeFiltro struct {
		ClienteID, VendedorID           int64
		Etapa, Origem                   string
		DataAberturaDe, DataAberturaAte string
		Q, OrderBy, OrderDir            string
	}
)

// vinculoFixo é o vínculo usuário→vendedor do usuário restrito (sem SQL).
type vinculoFixo struct{ vendedorID int64 }

func (v vinculoFixo) VendedorDoUsuario(context.Context, int64) (int64, bool, error) {
	return v.vendedorID, v.vendedorID > 0, nil
}

// vendedoresAtivos não consulta o desligamento (o resto é o adapter real).
type vendedoresAtivos struct{ comercialapp.Vendedores }

func (vendedoresAtivos) Desligado(context.Context, int64) (bool, error) { return false, nil }

// casosComercial monta os casos de uso sobre o db; vendedorID > 0 liga o
// vínculo fixo do usuário restrito.
func casosComercial(db *sql.DB, vendedorID int64) *comercialapp.Casos {
	return comercialapp.Novo(comercialapp.Deps{
		Vendedores:    vendedoresAtivos{comercialmysql.NovoVendedores(db)},
		Carteiras:     comercialmysql.NovoCarteiras(db),
		Clientes:      comercialmysql.NovoClientes(db),
		Visitas:       comercialmysql.NovoVisitas(db),
		Oportunidades: comercialmysql.NovoOportunidades(db),
		Vinculo:       vinculoFixo{vendedorID: vendedorID},
		Desligamento: shareddb.NewTransactor(db, func(q shareddb.Querier) comercialapp.TxDesligamento {
			return comercialapp.TxDesligamento{Vendedores: comercialmysql.NovoVendedores(q), Identidade: identidadeNaTx{q: q}}
		}),
		ClienteNaCarteira: shareddb.NewTransactor(db, func(q shareddb.Querier) comercialapp.TxClienteNaCarteira {
			return comercialapp.TxClienteNaCarteira{Clientes: comercialmysql.NovoClientes(q), Carteiras: comercialmysql.NovoCarteiras(q)}
		}),
		Relogio: time.Now,
	})
}

// identidadeNaTx é a parte de Identidade da tx do desligamento (I-09), como
// no bootstrap.
type identidadeNaTx struct{ q shareddb.Querier }

func (i identidadeNaTx) InativarUsuariosDoVendedor(ctx context.Context, vendedorID int64) (int64, error) {
	return identidademysql.NovoUsuarios(i.q).InativarDoVendedor(ctx, vendedorID)
}

func (i identidadeNaTx) RevogarSessoesDoVendedor(ctx context.Context, vendedorID int64) (int64, error) {
	return identidademysql.NovoRefreshTokens(i.q).RevogarDoVendedor(ctx, vendedorID, "inativacao")
}

func principalAdmin() kernel.Principal {
	p, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	return p
}

func principalRestrito() kernel.Principal {
	p, _ := kernel.NovoPrincipal(7, kernel.RoleNormal)
	return p
}

// fixo é o Leitor de um comando já pronto.
func fixo[T any](v T) comercialapp.Leitor[T] { return func() (T, error) { return v, nil } }

// ---------------------------------------------------------------------------
// ClienteService
// ---------------------------------------------------------------------------

type ClienteService struct{ db *sql.DB }

func NewClienteService(db *sql.DB, _ *config.Config) *ClienteService { return &ClienteService{db: db} }

func (s *ClienteService) ListClientes(ctx context.Context, _ *sql.DB, page, limit int, f ClienteFiltro) ([]comercial.Cliente, int, error) {
	p := principalAdmin()
	if f.VendedorID > 0 {
		p = principalRestrito()
	}
	filtro := comercial.FiltroCliente{UF: f.UF, Segmento: f.Segmento, Ativo: f.Ativo, Busca: f.Q,
		Ordenacao: kernel.ParseOrdenacao(f.OrderBy, f.OrderDir, comercial.CamposOrdenacaoCliente(), comercial.OrdenacaoClientePadrao())}
	r, err := casosComercial(s.db, f.VendedorID).ListarClientes(ctx, p, filtro, kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

func (s *ClienteService) GetClienteByID(ctx context.Context, _ *sql.DB, id int64) (*comercial.Cliente, error) {
	return casosComercial(s.db, 0).ObterCliente(ctx, principalAdmin(), id)
}

func (s *ClienteService) ToggleAtivoCliente(ctx context.Context, _ *sql.DB, id int64, ativo *bool) (*comercial.Cliente, error) {
	return casosComercial(s.db, 0).AlternarAtivoCliente(ctx, principalAdmin(), id, fixo(ativo))
}

func (s *ClienteService) CreateCliente(ctx context.Context, _ *sql.DB, in ClienteInput) (*comercial.Cliente, error) {
	return casosComercial(s.db, 0).CriarCliente(ctx, principalAdmin(), fixo(in))
}

func (s *ClienteService) CreateClienteNaCarteira(ctx context.Context, _ *sql.DB, in ClienteInput, vendedorID int64) (*comercial.Cliente, error) {
	return casosComercial(s.db, vendedorID).CriarCliente(ctx, principalRestrito(), fixo(in))
}

func (s *ClienteService) UpdateCliente(ctx context.Context, _ *sql.DB, id int64, in ClienteInput) (*comercial.Cliente, error) {
	return casosComercial(s.db, 0).AtualizarCliente(ctx, principalAdmin(), id, fixo(in))
}

// ---------------------------------------------------------------------------
// VendedorService
// ---------------------------------------------------------------------------

type VendedorService struct{ db *sql.DB }

func NewVendedorService(db *sql.DB, _ *config.Config) *VendedorService {
	return &VendedorService{db: db}
}

func (s *VendedorService) ListVendedores(ctx context.Context, _ *sql.DB) ([]comercial.VendedorResumo, error) {
	return casosComercial(s.db, 0).ListarVendedores(ctx, principalAdmin())
}

func (s *VendedorService) ListVendedorProprio(ctx context.Context, _ *sql.DB, vendedorID int64) ([]comercial.VendedorResumo, error) {
	return casosComercial(s.db, vendedorID).ListarVendedores(ctx, principalRestrito())
}

func (s *VendedorService) GetVendedorDetalhe(ctx context.Context, _ *sql.DB, id int64) (*comercial.DetalheVendedor, error) {
	return casosComercial(s.db, 0).ObterVendedor(ctx, principalAdmin(), id)
}

func (s *VendedorService) ListClientesDoVendedor(ctx context.Context, _ *sql.DB, id int64) ([]comercial.ClienteResumo, error) {
	return casosComercial(s.db, 0).ClientesDoVendedor(ctx, principalAdmin(), id)
}

func (s *VendedorService) CreateVendedor(ctx context.Context, _ *sql.DB, in VendedorInput) (*comercial.Vendedor, error) {
	return casosComercial(s.db, 0).CriarVendedor(ctx, principalAdmin(), in)
}

func (s *VendedorService) UpdateVendedor(ctx context.Context, _ *sql.DB, id int64, in VendedorInput) (*comercial.Vendedor, error) {
	return casosComercial(s.db, 0).AtualizarVendedor(ctx, principalAdmin(), id, in)
}

func (s *VendedorService) DeleteVendedor(ctx context.Context, _ *sql.DB, id int64) (*comercial.Vendedor, error) {
	return casosComercial(s.db, 0).DesligarVendedor(ctx, principalAdmin(), id)
}

func (s *VendedorService) ReativarVendedor(ctx context.Context, _ *sql.DB, id int64) (*comercial.Vendedor, error) {
	return casosComercial(s.db, 0).ReativarVendedor(ctx, principalAdmin(), id)
}

func (s *VendedorService) VincularCliente(ctx context.Context, _ *sql.DB, vendedorID, clienteID int64) (*comercial.ClienteResumo, error) {
	return casosComercial(s.db, 0).VincularCliente(ctx, principalAdmin(), vendedorID, clienteID)
}

func (s *VendedorService) DesvincularCliente(ctx context.Context, _ *sql.DB, vendedorID, clienteID int64) error {
	return casosComercial(s.db, 0).DesvincularCliente(ctx, principalAdmin(), vendedorID, clienteID)
}

// ---------------------------------------------------------------------------
// VisitaService
// ---------------------------------------------------------------------------

type VisitaService struct{ db *sql.DB }

func NewVisitaService(db *sql.DB, _ *config.Config) *VisitaService { return &VisitaService{db: db} }

func (s *VisitaService) ListVisitas(ctx context.Context, _ *sql.DB, page, limit int, f VisitaFiltro) ([]comercial.Visita, int, error) {
	filtro := comercial.FiltroVisita{ClienteID: f.ClienteID, VendedorID: f.VendedorID, Resultado: f.Resultado,
		De: f.DataVisitaDe, Ate: f.DataVisitaAte, Busca: f.Q,
		Ordenacao: kernel.ParseOrdenacao(f.OrderBy, f.OrderDir, comercial.CamposOrdenacaoVisita(), comercial.OrdenacaoVisitaPadrao())}
	r, err := casosComercial(s.db, 0).ListarVisitas(ctx, principalAdmin(), filtro, kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

func (s *VisitaService) GetVisitaByID(ctx context.Context, _ *sql.DB, id int64) (*comercial.Visita, error) {
	return casosComercial(s.db, 0).ObterVisita(ctx, principalAdmin(), id)
}

func (s *VisitaService) CreateVisita(ctx context.Context, _ *sql.DB, in VisitaInput) (*comercial.Visita, error) {
	return casosComercial(s.db, 0).CriarVisita(ctx, principalAdmin(), fixo(in))
}

func (s *VisitaService) UpdateVisita(ctx context.Context, _ *sql.DB, id int64, in VisitaInput) (*comercial.Visita, error) {
	return casosComercial(s.db, 0).AtualizarVisita(ctx, principalAdmin(), id, fixo(in))
}

func (s *VisitaService) DeleteVisita(ctx context.Context, _ *sql.DB, id int64) error {
	return casosComercial(s.db, 0).ExcluirVisita(ctx, principalAdmin(), id)
}

// ---------------------------------------------------------------------------
// OportunidadeService
// ---------------------------------------------------------------------------

type OportunidadeService struct{ db *sql.DB }

func NewOportunidadeService(db *sql.DB, _ *config.Config) *OportunidadeService {
	return &OportunidadeService{db: db}
}

func (s *OportunidadeService) ListOportunidades(ctx context.Context, _ *sql.DB, page, limit int, f OportunidadeFiltro) ([]comercial.Oportunidade, int, error) {
	filtro := comercial.FiltroOportunidade{ClienteID: f.ClienteID, VendedorID: f.VendedorID, Etapa: f.Etapa, Origem: f.Origem,
		De: f.DataAberturaDe, Ate: f.DataAberturaAte, Busca: f.Q,
		Ordenacao: kernel.ParseOrdenacao(f.OrderBy, f.OrderDir, comercial.CamposOrdenacaoOportunidade(), comercial.OrdenacaoOportunidadePadrao())}
	r, err := casosComercial(s.db, 0).ListarOportunidades(ctx, principalAdmin(), filtro, kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

func (s *OportunidadeService) GetOportunidadeByID(ctx context.Context, _ *sql.DB, id int64) (*comercial.Oportunidade, error) {
	return casosComercial(s.db, 0).ObterOportunidade(ctx, principalAdmin(), id)
}

func (s *OportunidadeService) CreateOportunidade(ctx context.Context, _ *sql.DB, in OportunidadeInput) (*comercial.Oportunidade, error) {
	return casosComercial(s.db, 0).CriarOportunidade(ctx, principalAdmin(), fixo(in))
}

func (s *OportunidadeService) UpdateOportunidade(ctx context.Context, _ *sql.DB, id int64, in OportunidadeInput) (*comercial.Oportunidade, error) {
	return casosComercial(s.db, 0).AtualizarOportunidade(ctx, principalAdmin(), id, fixo(in))
}

func (s *OportunidadeService) DeleteOportunidade(ctx context.Context, _ *sql.DB, id int64) error {
	return casosComercial(s.db, 0).ExcluirOportunidade(ctx, principalAdmin(), id)
}

// ARQ-04: editar ou excluir visita/oportunidade lê o registro antes, para
// checar a posse (E-04). No legado essa leitura ficava no handler (que os
// testes de handler já esperam); nos testes de service ela entra por estes
// helpers, uma linha antes das expectativas de cada caso.
func esperarPosseVisita(m sqlmock.Sqlmock, id int64) {
	m.ExpectQuery(`SELECT ` + visitaColunasRegex + ` FROM visitas WHERE visita_id = \? LIMIT 1`).
		WithArgs(id).WillReturnRows(visitaRows())
}

func esperarPosseOportunidade(m sqlmock.Sqlmock, id int64) {
	m.ExpectQuery(`SELECT ` + oportunidadeColunasRegex + ` FROM oportunidades WHERE oportunidade_id = \? LIMIT 1`).
		WithArgs(id).WillReturnRows(oportunidadeRows())
}
