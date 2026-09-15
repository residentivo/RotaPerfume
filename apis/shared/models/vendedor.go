package models

import "time"

// Vendedor representa um vendedor do CRM ao qual um Usuario pode estar
// vinculado (tabela vendedores, ver sql/01_ddl_usuarios.sql).
// DataDesligamento nil = vendedor ativo.
type Vendedor struct {
	ID               int64      `json:"id"`
	Nome             string     `json:"nome"`
	Regiao           string     `json:"regiao"`
	UF               string     `json:"uf"`
	DataAdmissao     time.Time  `json:"data_admissao"`
	DataDesligamento *time.Time `json:"data_desligamento"`
	MetaMensal       float64    `json:"meta_mensal"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
