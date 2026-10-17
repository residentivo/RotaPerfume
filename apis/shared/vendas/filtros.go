package vendas

import "github.com/rotaperfumes/shared/kernel"

// CampoPedido é o enum de ordenação de pedidos (C-02). O valor é o nome
// aceito em ?order_by=; a coluna sai de um switch no adapter.
type CampoPedido string

// Campos de ordenação de pedidos.
const (
	OrdemPedidoID           CampoPedido = "id"
	OrdemPedidoData         CampoPedido = "data_pedido"
	OrdemPedidoCanal        CampoPedido = "canal"
	OrdemPedidoStatus       CampoPedido = "status"
	OrdemPedidoValorTotal   CampoPedido = "valor_total"
	OrdemPedidoCriadoEm     CampoPedido = "created_at"
	OrdemPedidoAtualizadoEm CampoPedido = "updated_at"
	OrdemPedidoClienteNome  CampoPedido = "cliente_nome"
	OrdemPedidoVendedorNome CampoPedido = "vendedor_nome"
)

// CamposOrdenacaoPedido devolve a whitelist de pedidos.
func CamposOrdenacaoPedido() []CampoPedido {
	return []CampoPedido{
		OrdemPedidoID, OrdemPedidoData, OrdemPedidoCanal, OrdemPedidoStatus, OrdemPedidoValorTotal,
		OrdemPedidoCriadoEm, OrdemPedidoAtualizadoEm, OrdemPedidoClienteNome, OrdemPedidoVendedorNome,
	}
}

// OrdenacaoPedidoPadrao é "id DESC" (o mais recente primeiro).
func OrdenacaoPedidoPadrao() kernel.Ordenacao[CampoPedido] {
	return kernel.NovaOrdenacao(OrdemPedidoID, kernel.Desc)
}

// CampoPagamento é o enum de ordenação de pagamentos (C-02).
type CampoPagamento string

// Campos de ordenação de pagamentos.
const (
	OrdemPagamentoID             CampoPagamento = "pagamento_id"
	OrdemPagamentoPedidoID       CampoPagamento = "pedido_id"
	OrdemPagamentoForma          CampoPagamento = "forma_pagamento"
	OrdemPagamentoParcelas       CampoPagamento = "parcelas"
	OrdemPagamentoValor          CampoPagamento = "valor"
	OrdemPagamentoTaxaPct        CampoPagamento = "taxa_pct"
	OrdemPagamentoValorLiquido   CampoPagamento = "valor_liquido"
	OrdemPagamentoDataVencimento CampoPagamento = "data_vencimento"
	OrdemPagamentoDataPagamento  CampoPagamento = "data_pagamento"
	OrdemPagamentoStatus         CampoPagamento = "status_pagamento"
	OrdemPagamentoCriadoEm       CampoPagamento = "created_at"
	OrdemPagamentoAtualizadoEm   CampoPagamento = "updated_at"
)

// CamposOrdenacaoPagamento devolve a whitelist de pagamentos.
func CamposOrdenacaoPagamento() []CampoPagamento {
	return []CampoPagamento{
		OrdemPagamentoID, OrdemPagamentoPedidoID, OrdemPagamentoForma, OrdemPagamentoParcelas,
		OrdemPagamentoValor, OrdemPagamentoTaxaPct, OrdemPagamentoValorLiquido, OrdemPagamentoDataVencimento,
		OrdemPagamentoDataPagamento, OrdemPagamentoStatus, OrdemPagamentoCriadoEm, OrdemPagamentoAtualizadoEm,
	}
}

// OrdenacaoPagamentoPadrao é "pagamento_id ASC".
func OrdenacaoPagamentoPadrao() kernel.Ordenacao[CampoPagamento] {
	return kernel.NovaOrdenacao(OrdemPagamentoID, kernel.Asc)
}

// FiltroPedido é o query object da listagem (Specification como dado,
// §5.5). VendedorID é filtro de negócio (admin): no escopo de vendedor o
// adapter o SOBRESCREVE pelo do escopo, ignorando a query (E-03). Os demais
// valores vão por placeholder; as datas, como texto AAAA-MM-DD (legado).
type FiltroPedido struct {
	Status     string
	Canal      string
	ClienteID  int64 // 0 = sem filtro
	VendedorID int64 // 0 = sem filtro (só vale no escopo irrestrito)
	DataInicio string
	DataFim    string
	Busca      string // na razão social do cliente (LIKE)
	Ordenacao  kernel.Ordenacao[CampoPedido]
}

// FiltroPagamento é o query object da listagem de pagamentos. Não existe
// filtro de vendedor de negócio: o vendedor só entra pelo escopo, via
// pedidos.vendedor_id (V-02).
type FiltroPagamento struct {
	StatusPagamento string
	FormaPagamento  string
	PedidoID        int64 // 0 = sem filtro
	VencimentoDe    string
	VencimentoAte   string
	Ordenacao       kernel.Ordenacao[CampoPagamento]
}
