package comercial

import "time"

// Carteira é o vínculo histórico Cliente↔Vendedor. CarteiraIDOrigem é a PK
// (AUTO_INCREMENT). Um cliente pode ter várias carteiras ao longo do tempo;
// a ativa é a de DataFim nil.
type Carteira struct {
	CarteiraIDOrigem int64
	ClienteID        int64
	VendedorID       int64
	DataInicio       time.Time
	DataFim          *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Ativa informa se o vínculo está em vigor (data_fim nula).
func (c Carteira) Ativa() bool { return c.DataFim == nil }

// ClienteResumo é o cliente com os dados do vínculo de carteira (lista de
// clientes de um vendedor e resposta do vínculo).
type ClienteResumo struct {
	ID          int64
	CNPJ        string
	RazaoSocial string
	Segmento    string
	Cidade      string
	UF          string
	CarteiraID  int64
	DataInicio  time.Time
	DataFim     *time.Time
}

// ResumoDoVinculo monta o resumo a partir do cliente e do vínculo já
// carregados (sem nova consulta).
func ResumoDoVinculo(c *Cliente, v *Carteira) *ClienteResumo {
	return &ClienteResumo{
		ID: c.ClienteIDOrigem, CNPJ: c.CNPJ, RazaoSocial: c.RazaoSocial, Segmento: c.Segmento,
		Cidade: c.Cidade, UF: c.UF, CarteiraID: v.CarteiraIDOrigem, DataInicio: v.DataInicio, DataFim: v.DataFim,
	}
}

// NovaCarteiraDoDia cria o vínculo ativo do cliente com o vendedor, com
// data_inicio = meia-noite local do dia de agora (SEC-01/RISCO-01: o
// instante é convertido para time.Local antes de truncar).
func NovaCarteiraDoDia(clienteID, vendedorID int64, agora time.Time) *Carteira {
	a, m, d := agora.In(time.Local).Date()
	return &Carteira{ClienteID: clienteID, VendedorID: vendedorID, DataInicio: time.Date(a, m, d, 0, 0, 0, 0, time.Local)}
}
