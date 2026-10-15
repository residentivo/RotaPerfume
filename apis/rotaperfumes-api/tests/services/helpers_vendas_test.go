package services_test

// ARQ-05: PedidoService e PagamentoService saíram de rotaperfumes-api/
// services. Os testes (mesmos nomes e asserções) usam os nomes antigos,
// agora declarados AQUI como repassadores finos para vendasapp.Casos,
// montado pelo bootstrap sobre os adapters reais (vendasmysql,
// catalogomysql na tx do pedido) e o db do sqlmock. Nenhuma regra mora
// aqui; os sentinelas são os de shared/vendas (mesmos nomes do legado).
//
// O legado não tinha Principal nos services: os repassadores chamam os
// casos de uso como admin (escopo Irrestrito, sem consulta de escopo; o
// AutorizadorEscopo vem de casosComercial, helpers_comercial_test.go). O
// único filtro que o legado recebia com o vendedor do escopo
// (PagamentoFiltro.VendedorID) usa um usuário restrito cujo vínculo vem de
// um fake SEM SQL. Sai no ARQ-07.

import (
	"context"
	"database/sql"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
	"github.com/rotaperfumes/shared/vendas/vendasapp"
)

// Comandos e filtros com os nomes antigos (mesmos campos).
type (
	PedidoInput     = vendas.DadosPedido
	ItemPedidoInput = vendas.DadosItem
	PedidoFiltro    struct {
		Status, Canal         string
		ClienteID, VendedorID int64
		DataInicio, DataFim   string
		Q, OrderBy, OrderDir  string
	}
	PagamentoInput struct {
		PedidoID        int64
		FormaPagamento  string
		Parcelas        uint8
		Valor, TaxaPct  float64
		ValorLiquido    float64
		DataVencimento  string
		DataPagamento   string
		StatusPagamento string
	}
	PagamentoFiltro struct {
		StatusPagamento, FormaPagamento string
		PedidoID                        int64
		VencimentoDe, VencimentoAte     string
		VendedorID                      int64
		OrderBy, OrderDir               string
	}
)

func (in PagamentoInput) dados() vendas.DadosPagamento {
	return vendas.DadosPagamento{FormaPagamento: in.FormaPagamento, Parcelas: in.Parcelas, Valor: in.Valor,
		TaxaPct: in.TaxaPct, ValorLiquido: in.ValorLiquido, DataVencimento: in.DataVencimento,
		DataPagamento: in.DataPagamento, StatusPagamento: in.StatusPagamento}
}

// casosVendas monta os casos de uso pelo bootstrap; vendedorID > 0 liga o
// vínculo fixo do usuário restrito.
func casosVendas(db *sql.DB, vendedorID int64) *vendasapp.Casos {
	return bootstrap.NovoVendas(db, casosComercial(db, vendedorID).Escopo())
}

func lerFixo[T any](v T) vendasapp.Leitor[T] { return func() (T, error) { return v, nil } }

// ---------------------------------------------------------------------------
// PedidoService
// ---------------------------------------------------------------------------

type PedidoService struct{ db *sql.DB }

func NewPedidoService(db *sql.DB, _ *config.Config) *PedidoService { return &PedidoService{db: db} }

func (s *PedidoService) ListPedidos(ctx context.Context, _ *sql.DB, page, limit int, f PedidoFiltro) ([]vendas.Pedido, int, error) {
	filtro := vendas.FiltroPedido{Status: f.Status, Canal: f.Canal, ClienteID: f.ClienteID, VendedorID: f.VendedorID,
		DataInicio: f.DataInicio, DataFim: f.DataFim, Busca: f.Q,
		Ordenacao: kernel.ParseOrdenacao(f.OrderBy, f.OrderDir, vendas.CamposOrdenacaoPedido(), vendas.OrdenacaoPedidoPadrao())}
	r, err := casosVendas(s.db, 0).ListarPedidos(ctx, principalAdmin(), filtro, kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

func (s *PedidoService) GetPedidoDetalhe(ctx context.Context, _ *sql.DB, id int64) (*vendas.Pedido, error) {
	return casosVendas(s.db, 0).ObterPedido(ctx, principalAdmin(), id)
}

func (s *PedidoService) CreatePedido(ctx context.Context, _ *sql.DB, in PedidoInput) (*vendas.Pedido, error) {
	return casosVendas(s.db, 0).CriarPedido(ctx, principalAdmin(), lerFixo(in))
}

func (s *PedidoService) UpdatePedido(ctx context.Context, _ *sql.DB, id int64, in PedidoInput) (*vendas.Pedido, error) {
	return casosVendas(s.db, 0).AtualizarPedido(ctx, principalAdmin(), id, lerFixo(in))
}

func (s *PedidoService) DeletePedido(ctx context.Context, _ *sql.DB, id int64) error {
	return casosVendas(s.db, 0).ExcluirPedido(ctx, principalAdmin(), id)
}

// ---------------------------------------------------------------------------
// PagamentoService
// ---------------------------------------------------------------------------

type PagamentoService struct{ db *sql.DB }

func NewPagamentoService(db *sql.DB, _ *config.Config) *PagamentoService {
	return &PagamentoService{db: db}
}

func (s *PagamentoService) ListPagamentos(ctx context.Context, _ *sql.DB, page, limit int, f PagamentoFiltro) ([]vendas.Pagamento, int, error) {
	p := principalAdmin()
	if f.VendedorID > 0 {
		p = principalRestrito()
	}
	filtro := vendas.FiltroPagamento{StatusPagamento: f.StatusPagamento, FormaPagamento: f.FormaPagamento,
		PedidoID: f.PedidoID, VencimentoDe: f.VencimentoDe, VencimentoAte: f.VencimentoAte,
		Ordenacao: kernel.ParseOrdenacao(f.OrderBy, f.OrderDir, vendas.CamposOrdenacaoPagamento(), vendas.OrdenacaoPagamentoPadrao())}
	r, err := casosVendas(s.db, f.VendedorID).ListarPagamentos(ctx, p, lerFixo(filtro), kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

func (s *PagamentoService) GetPagamentoByID(ctx context.Context, _ *sql.DB, id int64) (*vendas.Pagamento, error) {
	return casosVendas(s.db, 0).ObterPagamento(ctx, principalAdmin(), id)
}

func (s *PagamentoService) CreatePagamento(ctx context.Context, _ *sql.DB, in PagamentoInput) (*vendas.Pagamento, error) {
	d := vendas.DadosNovoPagamento{PedidoID: in.PedidoID, DadosPagamento: in.dados()}
	return casosVendas(s.db, 0).CriarPagamento(ctx, principalAdmin(), lerFixo(d))
}

// UpdatePagamento: o comando de edição não tem pedido_id (V-02); o
// in.PedidoID do legado é descartado aqui, como o handler legado fazia.
func (s *PagamentoService) UpdatePagamento(ctx context.Context, _ *sql.DB, id int64, in PagamentoInput) (*vendas.Pagamento, error) {
	return casosVendas(s.db, 0).AtualizarPagamento(ctx, principalAdmin(), id, lerFixo(in.dados()))
}

func (s *PagamentoService) DeletePagamento(ctx context.Context, _ *sql.DB, id int64) error {
	return casosVendas(s.db, 0).ExcluirPagamento(ctx, principalAdmin(), id)
}

// ARQ-05: excluir pedido ou pagamento lê o registro antes, para checar a
// posse (E-04, V-02). No legado essa leitura ficava no handler (que os
// testes de handler já esperam); nos testes de service ela entra por estes
// helpers, uma linha antes das expectativas de cada caso (lição 1 do
// ARQ-04).
func esperarPossePedido(m sqlmock.Sqlmock, id int64) {
	m.ExpectQuery(`SELECT ` + pedidoColunasRegex + pedidoFromRegex + ` WHERE p\.pedido_id_origem = \? LIMIT 1`).
		WithArgs(id).WillReturnRows(pedidoRowsComStatus(id, 230.0, "Em separação"))
	m.ExpectQuery(`SELECT ` + itemPedidoColunasRegex + itemPedidoFromRegex + ` WHERE i\.pedido_id = \? ORDER BY i\.item_id_origem ASC`).
		WithArgs(id).WillReturnRows(itemPedidoRows(id))
}

func esperarPossePagamento(m sqlmock.Sqlmock, id int64) {
	m.ExpectQuery(`SELECT ` + pagamentoColunasRegex + pagamentoFromRegex + ` WHERE pagamento_id = \? LIMIT 1`).
		WithArgs(id).WillReturnRows(pagamentoRows(id, 1, 100.0))
}
