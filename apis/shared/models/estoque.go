package models

import "time"

// Estoque representa um snapshot diário de saldo/ruptura de um SKU
// (importado de dados/erp/estoque.csv). Diferente de Produto, múltiplos
// registros de Estoque existem por SKU ao longo do tempo (série temporal).
type Estoque struct {
	ID           int64     `json:"id"`
	DataSnapshot time.Time `json:"data_snapshot"`
	SKU          string    `json:"sku"`
	Saldo        int       `json:"saldo"`
	Ruptura      bool      `json:"ruptura"`
	Origem       string    `json:"origem"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Valores possíveis de Estoque.Origem (ver ENUM `origem` em
// sql/17_ddl_estoque.sql). Rastreiam qual processo gravou/atualizou por
// último cada snapshot (data_snapshot, sku), evitando que o import diário
// do CSV do ERP e a baixa por faturamento de pedidos se sobrescrevam
// silenciosamente.
const (
	EstoqueOrigemImportCSV   = "import_csv"
	EstoqueOrigemFaturamento = "faturamento"
	EstoqueOrigemManual      = "manual"
)
