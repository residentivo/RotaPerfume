package models

import "time"

// Oportunidade representa uma oportunidade de venda no funil do CRM
// (tabela oportunidades). DataFechamento, CicloDias e MotivoPerda são
// NULLable no banco (preenchidos apenas quando a oportunidade é fechada,
// ganha ou perdida) — por isso são ponteiros.
type Oportunidade struct {
	OportunidadeID   int64      `json:"oportunidade_id"`
	ClienteID        int64      `json:"cliente_id"`
	VendedorID       int64      `json:"vendedor_id"`
	Origem           string     `json:"origem"`
	DataAbertura     time.Time  `json:"data_abertura"`
	Etapa            string     `json:"etapa"`
	ProbabilidadePct float64    `json:"probabilidade_pct"`
	ValorEstimado    float64    `json:"valor_estimado"`
	DataFechamento   *time.Time `json:"data_fechamento"`
	CicloDias        *int       `json:"ciclo_dias"`
	MotivoPerda      *string    `json:"motivo_perda"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}
