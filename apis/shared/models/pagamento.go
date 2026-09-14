package models

import "time"

// Pagamento representa um pagamento do ERP (importado de
// dados/erp/pagamentos.csv). Diferente das demais tabelas, a PK é
// literalmente pagamento_id (não há coluna id separada) — ver
// sql/12_ddl_pagamentos.sql para o motivo do desvio de padrão.
type Pagamento struct {
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
