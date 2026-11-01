package vendas

import (
	"strings"
	"time"

	"github.com/rotaperfumes/shared/kernel"
)

// FormaPagamento é o ENUM forma_pagamento de sql/12_ddl_pagamentos.sql.
type FormaPagamento string

// Formas de pagamento aceitas no payload.
const (
	FormaBoleto14      FormaPagamento = "Boleto 14 dias"
	FormaBoleto28      FormaPagamento = "Boleto 28 dias"
	FormaCartaoCredito FormaPagamento = "Cartão de crédito"
	FormaCartaoDebito  FormaPagamento = "Cartão de débito"
	FormaChequePrazo   FormaPagamento = "Cheque a prazo"
	FormaDinheiro      FormaPagamento = "Dinheiro"
	FormaPIX           FormaPagamento = "PIX"
)

// NovaFormaPagamento valida a forma (espaços nas pontas ignorados).
func NovaFormaPagamento(s string) (FormaPagamento, error) {
	switch v := FormaPagamento(strings.TrimSpace(s)); v {
	case FormaBoleto14, FormaBoleto28, FormaCartaoCredito, FormaCartaoDebito, FormaChequePrazo, FormaDinheiro, FormaPIX:
		return v, nil
	}
	return "", ErrFormaPagamentoInvalida
}

// StatusPagamento é o ENUM status_pagamento de sql/12_ddl_pagamentos.sql.
type StatusPagamento string

// Status de pagamento aceitos no payload.
const (
	StatusEmAberto      StatusPagamento = "Em aberto"
	StatusInadimplente  StatusPagamento = "Inadimplente"
	StatusPago          StatusPagamento = "Pago"
	StatusPagoComAtraso StatusPagamento = "Pago com atraso"
)

// NovoStatusPagamento valida o status (espaços nas pontas ignorados).
func NovoStatusPagamento(s string) (StatusPagamento, error) {
	switch v := StatusPagamento(strings.TrimSpace(s)); v {
	case StatusEmAberto, StatusInadimplente, StatusPago, StatusPagoComAtraso:
		return v, nil
	}
	return "", ErrStatusPagamentoInvalido
}

// TaxaPctMaxima é o teto de taxa_pct (SEC-27; o CHECK do banco é o mesmo).
const TaxaPctMaxima = 30.0

// Quitado informa se o status preserva o registro (Pago ou Pago com atraso).
// O UPDATE condicional do adapter (SEC-27) usa as mesmas duas constantes.
func (s StatusPagamento) Quitado() bool { return s == StatusPago || s == StatusPagoComAtraso }

// Pagamento é um pagamento de pedido. A PK é literalmente pagamento_id.
// PedidoID é fixado na criação e nunca regravado (V-02): o UPDATE do
// adapter não tem a coluna e o comando de edição (DadosPagamento) não tem o
// campo; o vínculo com o pedido é o escopo de carteira do pagamento.
type Pagamento struct {
	PagamentoID     int64
	PedidoID        int64
	FormaPagamento  string
	Parcelas        uint8
	Valor           float64
	TaxaPct         float64
	ValorLiquido    float64
	DataVencimento  time.Time
	DataPagamento   *time.Time
	StatusPagamento string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// PodeEditar (SEC-27): pagamento quitado é trilha financeira e só o admin
// o edita; o caso de uso chama esta regra apenas para o perfil restrito.
func (p *Pagamento) PodeEditar() error {
	if StatusPagamento(p.StatusPagamento).Quitado() {
		return ErrPagamentoJaQuitadoNaoPodeSerEditado
	}
	return nil
}

// PodeExcluir: pagamento quitado é trilha financeira e não sai (409).
func (p *Pagamento) PodeExcluir() error {
	if StatusPagamento(p.StatusPagamento).Quitado() {
		return ErrPagamentoJaQuitadoNaoPodeSerExcluido
	}
	return nil
}

// DadosPagamento é o comando de edição: os campos editáveis, SEM pedido_id
// (V-02). valor_liquido vem explícito do payload (não é calculado: o CSV de
// origem traz diferenças de arredondamento em relação a valor e taxa).
type DadosPagamento struct {
	FormaPagamento  string
	Parcelas        uint8
	Valor           float64
	TaxaPct         float64
	ValorLiquido    float64
	DataVencimento  string // AAAA-MM-DD, obrigatória
	DataPagamento   string // AAAA-MM-DD, opcional (vazio = NULL)
	StatusPagamento string
}

// DadosNovoPagamento é o comando de criação: o pedido e os campos.
type DadosNovoPagamento struct {
	PedidoID int64
	DadosPagamento
}

// Pagamento valida o comando na ordem do legado (forma, status, parcelas,
// valor, taxa, valor líquido, vencimento e data de pagamento) e monta a
// entidade do pedido informado, com as datas à meia-noite local (BUG-01).
func (d DadosPagamento) Pagamento(pedidoID int64) (*Pagamento, error) {
	forma, err := NovaFormaPagamento(d.FormaPagamento)
	if err != nil {
		return nil, err
	}
	status, err := NovoStatusPagamento(d.StatusPagamento)
	if err != nil {
		return nil, err
	}
	if err = d.validarValores(); err != nil {
		return nil, err
	}
	vencimento, pagamento, err := d.datas()
	if err != nil {
		return nil, err
	}
	return &Pagamento{PedidoID: pedidoID, FormaPagamento: string(forma), Parcelas: d.Parcelas, Valor: d.Valor,
		TaxaPct: d.TaxaPct, ValorLiquido: d.ValorLiquido, DataVencimento: vencimento,
		DataPagamento: pagamento, StatusPagamento: string(status)}, nil
}

func (d DadosPagamento) validarValores() error {
	if d.Parcelas < 1 {
		return ErrParcelasInvalidas
	}
	if _, err := kernel.NovoDinheiro(d.Valor); err != nil {
		return ErrValorInvalido
	}
	if d.TaxaPct < 0 || d.TaxaPct > TaxaPctMaxima {
		return ErrTaxaPctInvalida
	}
	if _, err := kernel.NovoDinheiro(d.ValorLiquido); err != nil {
		return ErrValorLiquidoInvalido
	}
	return nil
}

func (d DadosPagamento) datas() (vencimento time.Time, pagamento *time.Time, err error) {
	if strings.TrimSpace(d.DataVencimento) == "" {
		return time.Time{}, nil, ErrDataVencimentoObrigatoria
	}
	if vencimento, err = kernel.ParseDataLocal(d.DataVencimento, ErrDataVencimentoInvalida); err != nil {
		return time.Time{}, nil, err
	}
	if strings.TrimSpace(d.DataPagamento) == "" {
		return vencimento, nil, nil
	}
	dp, err := kernel.ParseDataLocal(d.DataPagamento, ErrDataPagamentoInvalida)
	if err != nil {
		return time.Time{}, nil, err
	}
	return vencimento, &dp, nil
}
