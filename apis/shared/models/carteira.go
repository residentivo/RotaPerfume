package models

import "time"

// Carteira representa o vínculo histórico entre um Cliente e um Vendedor
// (importado de dados/crm/carteira.csv). Um cliente pode ter múltiplas
// carteiras ao longo do tempo (troca de vendedor); a carteira ativa é a que
// tem DataFim == nil. CarteiraIDOrigem é a PK BIGINT AUTO_INCREMENT da
// tabela (corresponde 1:1 ao carteira_id do CSV de origem) — não há coluna
// id separada.
type Carteira struct {
	CarteiraIDOrigem int64      `json:"carteira_id_origem"`
	ClienteID        int64      `json:"cliente_id"`
	VendedorID       int64      `json:"vendedor_id"`
	DataInicio       time.Time  `json:"data_inicio"`
	DataFim          *time.Time `json:"data_fim"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
