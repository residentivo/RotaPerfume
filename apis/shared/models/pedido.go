package models

import "time"

// Pedido representa um pedido do ERP (importado de dados/erp/pedidos.csv).
type Pedido struct {
	ID             int64     `json:"id"`
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
