package models

import "time"

// ItemPedido representa um item (linha) de pedido do ERP (importado de
// dados/erp/itens_pedido.csv).
type ItemPedido struct {
	ID             int64     `json:"id"`
	ItemIDOrigem   int64     `json:"item_id_origem"`
	PedidoID       int64     `json:"pedido_id"`
	ProdutoID      int64     `json:"produto_id"`
	Quantidade     int       `json:"quantidade"`
	PrecoPraticado float64   `json:"preco_praticado"`
	DescontoPct    float64   `json:"desconto_pct"`
	ValorBruto     float64   `json:"valor_bruto"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
