// Package vendashttp é a camada HTTP do contexto Vendas (ARQ-05): handlers
// finos (parse/DTO → caso de uso → envelope httpx). As rotas e as políticas
// ficam na tabela do bootstrap; o escopo de carteira (herdado do pedido no
// caso do pagamento) é decidido pelos casos de uso (vendasapp), nunca aqui.
//
// Os DTOs têm as mesmas chaves, ordem e tipos dos models legados (contrato
// JSON inalterado). valor_bruto e valor_total não existem no payload: o
// backend calcula (V-03); pedido_id não existe no payload de edição de
// pagamento (V-02).
package vendashttp

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
)

// ---------------------------------------------------------------------------
// Respostas
// ---------------------------------------------------------------------------

// pedidoJSON é o pedido da listagem (sem a chave "itens").
type pedidoJSON struct {
	PedidoIDOrigem int64     `json:"pedido_id_origem"`
	ClienteID      int64     `json:"cliente_id"`
	VendedorID     int64     `json:"vendedor_id"`
	DataPedido     time.Time `json:"data_pedido"`
	Canal          string    `json:"canal"`
	Status         string    `json:"status"`
	ValorTotal     float64   `json:"valor_total"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	ClienteNome    string    `json:"cliente_nome"`
	VendedorNome   string    `json:"vendedor_nome"`
}

func novoPedidoJSON(p vendas.Pedido) pedidoJSON {
	return pedidoJSON{PedidoIDOrigem: p.PedidoIDOrigem, ClienteID: p.ClienteID, VendedorID: p.VendedorID,
		DataPedido: p.DataPedido, Canal: p.Canal, Status: p.Status, ValorTotal: p.ValorTotal,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, ClienteNome: p.ClienteNome, VendedorNome: p.VendedorNome}
}

// pedidoDetalheJSON é o pedido (campos "achatados") + "itens" (null sem
// itens, como no legado).
type pedidoDetalheJSON struct {
	pedidoJSON
	Itens []itemPedidoJSON `json:"itens"`
}

func novoPedidoDetalheJSON(p vendas.Pedido) pedidoDetalheJSON {
	return pedidoDetalheJSON{pedidoJSON: novoPedidoJSON(p), Itens: httpx.Lista(p.Itens, novoItemPedidoJSON)}
}

type itemPedidoJSON struct {
	ItemIDOrigem     int64     `json:"item_id_origem"`
	PedidoID         int64     `json:"pedido_id"`
	ProdutoID        int64     `json:"produto_id"`
	Quantidade       int       `json:"quantidade"`
	PrecoPraticado   float64   `json:"preco_praticado"`
	DescontoPct      float64   `json:"desconto_pct"`
	ValorBruto       float64   `json:"valor_bruto"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	ProdutoSKU       string    `json:"produto_sku"`
	ProdutoDescricao string    `json:"produto_descricao"`
}

func novoItemPedidoJSON(it vendas.ItemPedido) itemPedidoJSON { return itemPedidoJSON(it) }

type pagamentoJSON struct {
	PagamentoID     int64      `json:"pagamento_id"`
	PedidoID        int64      `json:"pedido_id"`
	FormaPagamento  string     `json:"forma_pagamento"`
	Parcelas        uint8      `json:"parcelas"`
	Valor           float64    `json:"valor"`
	TaxaPct         float64    `json:"taxa_pct"`
	ValorLiquido    float64    `json:"valor_liquido"`
	DataVencimento  time.Time  `json:"data_vencimento"`
	DataPagamento   *time.Time `json:"data_pagamento"`
	StatusPagamento string     `json:"status_pagamento"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func novoPagamentoJSON(p vendas.Pagamento) pagamentoJSON { return pagamentoJSON(p) }

// ---------------------------------------------------------------------------
// Requisições (um DTO por operação; o de edição de pagamento não tem
// pedido_id, V-02)
// ---------------------------------------------------------------------------

type itemPedidoRequest struct {
	ProdutoID      int64   `json:"produto_id"`
	Quantidade     int     `json:"quantidade"`
	PrecoPraticado float64 `json:"preco_praticado"`
	DescontoPct    float64 `json:"desconto_pct"`
}

// pedidoRequest é o body de POST e PUT /api/pedidos (os dois legados eram
// idênticos). vendedor_id é ignorado para usuário restrito (E-05).
type pedidoRequest struct {
	ClienteID  int64               `json:"cliente_id"`
	VendedorID int64               `json:"vendedor_id"`
	DataPedido string              `json:"data_pedido"`
	Canal      string              `json:"canal"`
	Status     string              `json:"status"`
	Itens      []itemPedidoRequest `json:"itens"`
}

func dadosPedido(q pedidoRequest) vendas.DadosPedido {
	itens := make([]vendas.DadosItem, 0, len(q.Itens))
	for _, it := range q.Itens {
		itens = append(itens, vendas.DadosItem(it))
	}
	return vendas.DadosPedido{ClienteID: q.ClienteID, VendedorID: q.VendedorID, DataPedido: q.DataPedido,
		Canal: q.Canal, Status: q.Status, Itens: itens}
}

// pagamentoRequest é o body do PUT /api/pagamentos/{id}: SEM pedido_id (um
// pedido_id enviado é ignorado pelo decoder).
type pagamentoRequest struct {
	FormaPagamento  string  `json:"forma_pagamento"`
	Parcelas        uint8   `json:"parcelas"`
	Valor           float64 `json:"valor"`
	TaxaPct         float64 `json:"taxa_pct"`
	ValorLiquido    float64 `json:"valor_liquido"`
	DataVencimento  string  `json:"data_vencimento"`
	DataPagamento   string  `json:"data_pagamento"`
	StatusPagamento string  `json:"status_pagamento"`
}

func dadosPagamento(q pagamentoRequest) vendas.DadosPagamento { return vendas.DadosPagamento(q) }

// novoPagamentoRequest é o body do POST /api/pagamentos.
type novoPagamentoRequest struct {
	PedidoID int64 `json:"pedido_id"`
	pagamentoRequest
}

func dadosNovoPagamento(q novoPagamentoRequest) vendas.DadosNovoPagamento {
	return vendas.DadosNovoPagamento{PedidoID: q.PedidoID, DadosPagamento: dadosPagamento(q.pagamentoRequest)}
}

// ---------------------------------------------------------------------------
// Query strings
// ---------------------------------------------------------------------------

// errPedidoIDInvalido é o 400 do ?pedido_id= não numérico na listagem.
var errPedidoIDInvalido = kernel.NovoErro(kernel.KindInvalido, "pedido_id inválido")

func texto(r *http.Request, nome string) string { return strings.TrimSpace(r.URL.Query().Get(nome)) }

// filtroPedidoDaQuery: q acima de 100 caracteres é httpx.ErrBuscaMuitoLonga
// (SEC-25); o handler responde 400 antes do caso de uso.
func filtroPedidoDaQuery(r *http.Request) (vendas.FiltroPedido, error) {
	busca, err := httpx.BuscaDaQuery(r)
	if err != nil {
		return vendas.FiltroPedido{}, err
	}
	return vendas.FiltroPedido{
		Status: texto(r, "status"), Canal: texto(r, "canal"),
		ClienteID: httpx.Int64DaQuery(r, "cliente_id"), VendedorID: httpx.Int64DaQuery(r, "vendedor_id"),
		DataInicio: texto(r, "data_inicio"), DataFim: texto(r, "data_fim"), Busca: busca,
		Ordenacao: httpx.OrdenacaoDaQuery(r, vendas.CamposOrdenacaoPedido(), vendas.OrdenacaoPedidoPadrao()),
	}, nil
}

// leitorFiltroPagamento adia o parse do filtro para depois do escopo: o
// pedido_id inválido só é 400 para quem tem acesso (ordem do legado).
func leitorFiltroPagamento(r *http.Request) func() (vendas.FiltroPagamento, error) {
	return func() (vendas.FiltroPagamento, error) {
		f := vendas.FiltroPagamento{
			StatusPagamento: texto(r, "status_pagamento"), FormaPagamento: texto(r, "forma_pagamento"),
			VencimentoDe: texto(r, "vencimento_de"), VencimentoAte: texto(r, "vencimento_ate"),
			Ordenacao: httpx.OrdenacaoDaQuery(r, vendas.CamposOrdenacaoPagamento(), vendas.OrdenacaoPagamentoPadrao()),
		}
		if v := texto(r, "pedido_id"); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return vendas.FiltroPagamento{}, errPedidoIDInvalido
			}
			f.PedidoID = id
		}
		return f, nil
	}
}

// principalDoEscopo é o Principal das rotas com escopo de carteira. A falta
// dele NÃO vira 500 aqui: o autorizador trata o Principal inválido como
// SemAcesso, sem consulta (lição 2 do ARQ-04; TestEscopo_SemUsuarioNoContexto).
func principalDoEscopo(r *http.Request) kernel.Principal {
	p, _ := httpx.PrincipalFrom(r)
	return p
}
