package models

import "time"

// Produto representa um produto do catálogo (importado de dados/erp/produtos.csv).
type Produto struct {
	ID             int64      `json:"id"`
	SKU            string     `json:"sku"`
	Descricao      string     `json:"descricao"`
	Categoria      string     `json:"categoria"`
	Marca          string     `json:"marca"`
	NotaOlfativa   string     `json:"nota_olfativa"`
	PrecoTabela    float64    `json:"preco_tabela"`
	CustoUnitario  float64    `json:"custo_unitario"`
	Unidade        string     `json:"unidade"`
	DataLancamento *time.Time `json:"data_lancamento"`
	Ativo          bool       `json:"ativo"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
