package models

import "time"

// Visita representa uma visita de um vendedor a um cliente no CRM
// (tabela visitas).
type Visita struct {
	VisitaID   int64     `json:"visita_id"`
	ClienteID  int64     `json:"cliente_id"`
	VendedorID int64     `json:"vendedor_id"`
	DataVisita time.Time `json:"data_visita"`
	Resultado  string    `json:"resultado"`
	DuracaoMin int       `json:"duracao_min"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
