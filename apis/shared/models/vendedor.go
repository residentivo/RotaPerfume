package models

// Vendedor representa um vendedor do CRM ao qual um Usuario pode estar vinculado.
type Vendedor struct {
	ID     int64  `json:"id"`
	Nome   string `json:"nome"`
	Regiao string `json:"regiao"`
	UF     string `json:"uf"`
}
