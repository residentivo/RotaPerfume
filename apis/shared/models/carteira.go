package models

import "time"

// Carteira representa o vínculo histórico entre um Cliente e um Vendedor
// (importado de dados/crm/carteira.csv). Um cliente pode ter múltiplas
// carteiras ao longo do tempo (troca de vendedor); a carteira ativa é a que
// tem DataFim == nil.
type Carteira struct {
	ID               int64      `json:"id"`
	CarteiraIDOrigem int64      `json:"carteira_id_origem"`
	ClienteID        int64      `json:"cliente_id"`
	VendedorID       int64      `json:"vendedor_id"`
	DataInicio       time.Time  `json:"data_inicio"`
	DataFim          *time.Time `json:"data_fim"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
