package models

import "time"

// Cliente representa um cliente do CRM (importado de dados/crm/clientes.csv).
type Cliente struct {
	ID              int64     `json:"id"`
	ClienteIDOrigem int64     `json:"cliente_id_origem"`
	CNPJ            string    `json:"cnpj"`
	RazaoSocial     string    `json:"razao_social"`
	Segmento        string    `json:"segmento"`
	Cidade          string    `json:"cidade"`
	UF              string    `json:"uf"`
	Bairro          string    `json:"bairro"`
	DataCadastro    time.Time `json:"data_cadastro"`
	Ativo           bool      `json:"ativo"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
