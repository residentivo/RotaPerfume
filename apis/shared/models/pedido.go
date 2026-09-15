package models

import "time"

// Pedido representa um pedido do ERP (importado de dados/erp/pedidos.csv).
// PedidoIDOrigem é a PK BIGINT AUTO_INCREMENT da tabela (corresponde 1:1 ao
// pedido_id do CSV de origem) — não há coluna id separada.
type Pedido struct {
	PedidoIDOrigem int64     `json:"pedido_id_origem"`
	ClienteID      int64     `json:"cliente_id"`
	VendedorID     int64     `json:"vendedor_id"`
	DataPedido     time.Time `json:"data_pedido"`
	Canal          string    `json:"canal"`
	Status         string    `json:"status"`
	ValorTotal     float64   `json:"valor_total"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
