package models

import "time"

// Estoque representa um snapshot diário de saldo/ruptura de um SKU
// (importado de dados/erp/estoque.csv). Diferente de Produto, múltiplos
// registros de Estoque existem por SKU ao longo do tempo (série temporal).
type Estoque struct {
	ID           int64     `json:"id"`
	DataSnapshot time.Time `json:"data_snapshot"`
	SKU          string    `json:"sku"`
	// ProdutoDescricao é preenchido via JOIN com produtos (produtos.descricao)
	// nas queries de listagem, para exibição ao lado do SKU no frontend.
	ProdutoDescricao string    `json:"produto_descricao"`
	Saldo            int       `json:"saldo"`
	Ruptura          bool      `json:"ruptura"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
