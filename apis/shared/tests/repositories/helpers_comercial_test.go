package repositories_test

// ARQ-04: os repositórios de vendedores, carteiras, clientes, visitas e
// oportunidades migraram para comercial/comercialmysql (SQL idêntico). Os
// testes mantêm os nomes e exercitam os adapters novos por estes
// repassadores finos, que só traduzem a assinatura antiga (db por
// parâmetro, page/limit e order_by em string, vendedorID int64 de escopo)
// para a nova. A tradução do vendedorID de escopo é a do §7.2-5: 0 (o
// "global" do legado) vira kernel.EscopoIrrestrito de um admin; > 0 vira
// kernel.EscopoDoVendedor. Erros não são traduzidos: as asserções usam os
// sentinelas de comercial. As métricas de cliente do Dashboard (ARQ-06)
// vão para o analyticsmysql (helpers_analytics_test.go).
// Sai no ARQ-07, quando os testes forem reescritos sobre a API nova.

import (
	"context"
	"database/sql"
	"time"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/comercial/comercialmysql"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/repositories"
)

// escopoLegado traduz o vendedorID de escopo do legado (0 = admin).
func escopoLegado(vendedorID int64) kernel.Escopo {
	if vendedorID > 0 {
		e, _ := kernel.EscopoDoVendedor(vendedorID)
		return e
	}
	admin, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	e, _ := kernel.EscopoIrrestrito(admin)
	return e
}

// ---------------------------------------------------------------------------
// vendedores
// ---------------------------------------------------------------------------

type vendedorRepo struct{}

// NewVendedorRepository substitui repositories.NewVendedorRepository.
func NewVendedorRepository() vendedorRepo { return vendedorRepo{} }

func (vendedorRepo) List(ctx context.Context, q db.Querier) ([]comercial.VendedorResumo, error) {
	return comercialmysql.NovoVendedores(q).ListarResumo(ctx, escopoLegado(0))
}

func (vendedorRepo) ListResumoByID(ctx context.Context, q db.Querier, id int64) ([]comercial.VendedorResumo, error) {
	return comercialmysql.NovoVendedores(q).ListarResumo(ctx, escopoLegado(id))
}

func (vendedorRepo) ExistsByID(ctx context.Context, q db.Querier, id int64) (bool, error) {
	return comercialmysql.NovoVendedores(q).Existe(ctx, id)
}

func (vendedorRepo) IsDesligado(ctx context.Context, q db.Querier, id int64) (bool, error) {
	return comercialmysql.NovoVendedores(q).Desligado(ctx, id)
}

func (vendedorRepo) GetByID(ctx context.Context, q db.Querier, id int64) (*comercial.Vendedor, error) {
	return comercialmysql.NovoVendedores(q).PorID(ctx, id)
}

func (vendedorRepo) Create(ctx context.Context, q db.Querier, v *comercial.Vendedor) error {
	return comercialmysql.NovoVendedores(q).Inserir(ctx, v)
}

func (vendedorRepo) Update(ctx context.Context, q db.Querier, id int64, v *comercial.Vendedor) error {
	return comercialmysql.NovoVendedores(q).Atualizar(ctx, id, v)
}

func (vendedorRepo) SetDataDesligamento(ctx context.Context, q db.Querier, id int64, d *sql.NullTime) error {
	var quando *time.Time
	if d != nil && d.Valid {
		quando = &d.Time
	}
	return comercialmysql.NovoVendedores(q).DefinirDesligamento(ctx, id, quando)
}

func (vendedorRepo) MarcarDesligamento(ctx context.Context, q db.Querier, id int64, quando time.Time) error {
	return comercialmysql.NovoVendedores(q).MarcarDesligamento(ctx, id, quando)
}

// ---------------------------------------------------------------------------
// carteiras
// ---------------------------------------------------------------------------

type carteiraRepo struct{}

// NewCarteiraRepository substitui repositories.NewCarteiraRepository.
func NewCarteiraRepository() carteiraRepo { return carteiraRepo{} }

func (carteiraRepo) ListClientesByVendedorID(ctx context.Context, q db.Querier, vendedorID int64) ([]comercial.ClienteResumo, error) {
	return comercialmysql.NovoCarteiras(q).ClientesDoVendedor(ctx, vendedorID)
}

func (carteiraRepo) GetByID(ctx context.Context, q db.Querier, id int64) (*comercial.Carteira, error) {
	return comercialmysql.NovoCarteiras(q).PorID(ctx, id)
}

func (carteiraRepo) GetVinculoAtivo(ctx context.Context, q db.Querier, vendedorID, clienteID int64) (*comercial.Carteira, error) {
	return comercialmysql.NovoCarteiras(q).VinculoAtivo(ctx, vendedorID, clienteID)
}

func (carteiraRepo) GetVinculoAtivoByClienteID(ctx context.Context, q db.Querier, clienteID int64) (*comercial.Carteira, error) {
	return comercialmysql.NovoCarteiras(q).VinculoAtivoDoCliente(ctx, clienteID)
}

func (carteiraRepo) GetVinculoByClienteVendedorData(ctx context.Context, q db.Querier, clienteID, vendedorID int64, dia time.Time) (*comercial.Carteira, error) {
	return comercialmysql.NovoCarteiras(q).VinculoDoDia(ctx, clienteID, vendedorID, dia)
}

func (carteiraRepo) ReativarVinculo(ctx context.Context, q db.Querier, id int64) error {
	return comercialmysql.NovoCarteiras(q).Reabrir(ctx, id)
}

func (carteiraRepo) Create(ctx context.Context, q db.Querier, c *comercial.Carteira) error {
	return comercialmysql.NovoCarteiras(q).Inserir(ctx, c)
}

func (carteiraRepo) EncerrarVinculo(ctx context.Context, q db.Querier, id int64, fim time.Time) error {
	return comercialmysql.NovoCarteiras(q).Encerrar(ctx, id, fim)
}

func (carteiraRepo) Delete(ctx context.Context, q db.Querier, id int64) error {
	return comercialmysql.NovoCarteiras(q).Excluir(ctx, id)
}

// ---------------------------------------------------------------------------
// clientes
// ---------------------------------------------------------------------------

// ClienteFiltro é o filtro legado. QCNPJ é ignorado: o adapter calcula o
// termo da busca em cnpj com comercial.TermoBuscaCNPJ(Q), a mesma regra que
// o service legado usava para preenchê-lo.
type ClienteFiltro struct {
	UF, Segmento string
	Ativo        *bool
	Q, QCNPJ     string
	VendedorID   int64
	OrderBy      string
	OrderDir     string
}

// clienteRepo repassa as operações de cadastro ao comercialmysql e as
// métricas do Dashboard ao analyticsmysql (helpers_analytics_test.go).
type clienteRepo struct{}

// NewClienteRepository substitui repositories.NewClienteRepository.
func NewClienteRepository() clienteRepo {
	return clienteRepo{}
}

func (clienteRepo) List(ctx context.Context, q db.Querier, page, limit int, f ClienteFiltro) ([]comercial.Cliente, int, error) {
	filtro := comercial.FiltroCliente{UF: f.UF, Segmento: f.Segmento, Ativo: f.Ativo, Busca: f.Q,
		Ordenacao: kernel.ParseOrdenacao(f.OrderBy, f.OrderDir, comercial.CamposOrdenacaoCliente(), comercial.OrdenacaoClientePadrao())}
	r, err := comercialmysql.NovoClientes(q).Listar(ctx, escopoLegado(f.VendedorID), filtro, kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

func (clienteRepo) ExistsByID(ctx context.Context, q db.Querier, id int64) (bool, error) {
	return comercialmysql.NovoClientes(q).Existe(ctx, id)
}

func (clienteRepo) GetByID(ctx context.Context, q db.Querier, id int64) (*comercial.Cliente, error) {
	return comercialmysql.NovoClientes(q).PorID(ctx, id)
}

func (clienteRepo) SetAtivo(ctx context.Context, q db.Querier, id int64, ativo bool) error {
	return comercialmysql.NovoClientes(q).DefinirAtivo(ctx, id, ativo)
}

// Create aceita o Execer legado (pool ou tx; os dois são db.Querier).
func (clienteRepo) Create(ctx context.Context, e repositories.Execer, c *comercial.Cliente) error {
	return comercialmysql.NovoClientes(e.(db.Querier)).Inserir(ctx, c)
}

func (clienteRepo) Update(ctx context.Context, q db.Querier, id int64, c *comercial.Cliente) error {
	return comercialmysql.NovoClientes(q).Atualizar(ctx, id, c)
}

// ---------------------------------------------------------------------------
// visitas
// ---------------------------------------------------------------------------

// VisitaFiltro é o filtro legado.
type VisitaFiltro struct {
	ClienteID, VendedorID       int64
	Resultado                   string
	DataVisitaDe, DataVisitaAte string
	Q, OrderBy, OrderDir        string
}

type visitaRepo struct{}

// NewVisitaRepository substitui repositories.NewVisitaRepository.
func NewVisitaRepository() visitaRepo { return visitaRepo{} }

func (visitaRepo) List(ctx context.Context, q db.Querier, page, limit int, f VisitaFiltro) ([]comercial.Visita, int, error) {
	filtro := comercial.FiltroVisita{ClienteID: f.ClienteID, VendedorID: f.VendedorID, Resultado: f.Resultado,
		De: f.DataVisitaDe, Ate: f.DataVisitaAte, Busca: f.Q,
		Ordenacao: kernel.ParseOrdenacao(f.OrderBy, f.OrderDir, comercial.CamposOrdenacaoVisita(), comercial.OrdenacaoVisitaPadrao())}
	r, err := comercialmysql.NovoVisitas(q).Listar(ctx, escopoLegado(0), filtro, kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

func (visitaRepo) GetByID(ctx context.Context, q db.Querier, id int64) (*comercial.Visita, error) {
	return comercialmysql.NovoVisitas(q).PorID(ctx, id)
}

func (visitaRepo) Create(ctx context.Context, q db.Querier, v *comercial.Visita) error {
	return comercialmysql.NovoVisitas(q).Inserir(ctx, v)
}

func (visitaRepo) Update(ctx context.Context, q db.Querier, id int64, v *comercial.Visita) error {
	return comercialmysql.NovoVisitas(q).Atualizar(ctx, id, v)
}

func (visitaRepo) Delete(ctx context.Context, q db.Querier, id int64) error {
	return comercialmysql.NovoVisitas(q).Excluir(ctx, id)
}

// ---------------------------------------------------------------------------
// oportunidades
// ---------------------------------------------------------------------------

// OportunidadeFiltro é o filtro legado.
type OportunidadeFiltro struct {
	ClienteID, VendedorID           int64
	Etapa, Origem                   string
	DataAberturaDe, DataAberturaAte string
	Q, OrderBy, OrderDir            string
}

type oportunidadeRepo struct{}

// NewOportunidadeRepository substitui repositories.NewOportunidadeRepository.
func NewOportunidadeRepository() oportunidadeRepo { return oportunidadeRepo{} }

func (oportunidadeRepo) List(ctx context.Context, q db.Querier, page, limit int, f OportunidadeFiltro) ([]comercial.Oportunidade, int, error) {
	filtro := comercial.FiltroOportunidade{ClienteID: f.ClienteID, VendedorID: f.VendedorID, Etapa: f.Etapa, Origem: f.Origem,
		De: f.DataAberturaDe, Ate: f.DataAberturaAte, Busca: f.Q,
		Ordenacao: kernel.ParseOrdenacao(f.OrderBy, f.OrderDir, comercial.CamposOrdenacaoOportunidade(), comercial.OrdenacaoOportunidadePadrao())}
	r, err := comercialmysql.NovoOportunidades(q).Listar(ctx, escopoLegado(0), filtro, kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

func (oportunidadeRepo) GetByID(ctx context.Context, q db.Querier, id int64) (*comercial.Oportunidade, error) {
	return comercialmysql.NovoOportunidades(q).PorID(ctx, id)
}

func (oportunidadeRepo) Create(ctx context.Context, q db.Querier, o *comercial.Oportunidade) error {
	return comercialmysql.NovoOportunidades(q).Inserir(ctx, o)
}

func (oportunidadeRepo) Update(ctx context.Context, q db.Querier, id int64, o *comercial.Oportunidade) error {
	return comercialmysql.NovoOportunidades(q).Atualizar(ctx, id, o)
}

func (oportunidadeRepo) Delete(ctx context.Context, q db.Querier, id int64) error {
	return comercialmysql.NovoOportunidades(q).Excluir(ctx, id)
}
