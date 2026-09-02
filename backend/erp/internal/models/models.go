package models

import (
	"database/sql"
	"time"
)

// Produto representa um produto do ERP
type Produto struct {
	SKU            string  `json:"sku"`
	Descricao      string  `json:"descricao"`
	Categoria      string  `json:"categoria"`
	Marca          string  `json:"marca"`
	NotaOlfativa   string  `json:"nota_olfativa"`
	PrecoTabela    float64 `json:"preco_tabela"`
	CustoUnitario  float64 `json:"custo_unitario"`
	Unidade        string  `json:"unidade"`
	Ativo          string  `json:"ativo"`
	DataLancamento sql.NullTime `json:"data_lancamento"`
}

// Pedido representa um pedido
type Pedido struct {
	ID         int64     `json:"id"`
	ClienteID  int64     `json:"cliente_id"`
	VendedorID int64     `json:"vendedor_id"`
	DataPedido time.Time `json:"data_pedido"`
	Canal      string    `json:"canal"`
	Status     string    `json:"status"`
	ValorTotal float64   `json:"valor_total"`
}

// PedidoInput representa dados de entrada de pedido com seus itens
type PedidoInput struct {
	ClienteID  int64         `json:"cliente_id"`
	VendedorID int64         `json:"vendedor_id"`
	Canal      string        `json:"canal"`
	Status     string        `json:"status"`
	Itens      []ItemPedido  `json:"itens"`
}

// ItemPedido representa um item de um pedido
type ItemPedido struct {
	ID             int64   `json:"id"`
	PedidoID       int64   `json:"pedido_id"`
	SKU            string  `json:"sku"`
	Quantidade     int     `json:"quantidade"`
	PrecoPraticado float64 `json:"preco_praticado"`
	DescontoPct    float64 `json:"desconto_pct"`
	ValorBruto     float64 `json:"valor_bruto"`
}

// Estoque representa o estoque de um produto em uma data
type Estoque struct {
	DataSnapshot time.Time `json:"data_snapshot"`
	SKU          string    `json:"sku"`
	Saldo        int       `json:"saldo"`
	Ruptura      string    `json:"ruptura"`
}

// Pagamento representa um pagamento
type Pagamento struct {
	ID              int64        `json:"id"`
	PedidoID        int64        `json:"pedido_id"`
	FormaPagamento  string       `json:"forma_pagamento"`
	Parcelas        int          `json:"parcelas"`
	Valor           float64      `json:"valor"`
	TaxaPct         float64      `json:"taxa_pct"`
	ValorLiquido    float64      `json:"valor_liquido"`
	DataVencimento  time.Time    `json:"data_vencimento"`
	DataPagamento   sql.NullTime `json:"data_pagamento"`
	StatusPagamento string       `json:"status_pagamento"`
}

// PagamentoInput representa dados de entrada de pagamento
type PagamentoInput struct {
	PedidoID        int64   `json:"pedido_id"`
	FormaPagamento  string  `json:"forma_pagamento"`
	Parcelas        int     `json:"parcelas"`
	Valor           float64 `json:"valor"`
	TaxaPct         float64 `json:"taxa_pct"`
	DataVencimento  string  `json:"data_vencimento"`
	DataPagamento   string  `json:"data_pagamento"`
	StatusPagamento string  `json:"status_pagamento"`
}

// Usuario representa um usuário do sistema
type Usuario struct {
	ID            int64         `json:"id"`
	Login         string        `json:"login"`
	Email         string        `json:"email"`
	SenhaHash     string        `json:"-"`
	TipoUsuario   string        `json:"tipo_usuario"`
	VendedorID    sql.NullInt64 `json:"vendedor_id"`
	GerenciadoPor sql.NullInt64 `json:"gerenciado_por"`
	Ativo         string        `json:"ativo"`
}

// DashboardFinanceiroTotais representa os totais por forma de pagamento
type DashboardFinanceiroTotais struct {
	FormaPagamento string  `json:"forma_pagamento"`
	TotalValor     float64 `json:"total_valor"`
	TotalLiquido   float64 `json:"total_liquido"`
	Quantidade     int     `json:"quantidade"`
}

// DashboardRuptura representa produtos em ruptura
type DashboardRuptura struct {
	SKU       string `json:"sku"`
	Descricao string `json:"descricao"`
	Saldo     int    `json:"saldo"`
}

// Request/Response structs

type LoginRequest struct {
	Login string `json:"login"`
	Senha string `json:"senha"`
}

type LoginResponse struct {
	Token       string   `json:"token"`
	TipoUsuario string   `json:"tipo_usuario"`
	Usuario     *Usuario `json:"usuario"`
}

type CriarPedidoRequest struct {
	ClienteID  int64              `json:"cliente_id"`
	VendedorID int64              `json:"vendedor_id"`
	Canal      string             `json:"canal"`
	Status     string             `json:"status"`
	Itens      []ItemPedidoInput  `json:"itens"`
}

type ItemPedidoInput struct {
	SKU            string  `json:"sku"`
	Quantidade     int     `json:"quantidade"`
	PrecoPraticado float64 `json:"preco_praticado"`
	DescontoPct    float64 `json:"desconto_pct"`
}
