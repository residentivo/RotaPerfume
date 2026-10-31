// Package vendas é o domínio do contexto Vendas (ARQ-05): o aggregate root
// Pedido com os seus itens (ItemPedido), a entidade Pagamento, os enums de
// status/canal/forma, o VO DescontoPct, as ordenações tipadas (C-02) e as
// regras puras de exclusão e de atualização de pedido (V-01, V-03).
//
// O pacote é puro (docs/arquitetura-ddd.md §4.1): importa só a stdlib e o
// kernel. Não loga, não conhece SQL nem HTTP e não tem tags JSON. As
// entidades têm campos exportados com os mesmos nomes e tipos dos models
// legados (registro ARQ-02, divergência b).
package vendas

import "github.com/rotaperfumes/shared/kernel"

// Erros de escopo de carteira (E-05), com as mensagens do contrato: o mesmo
// texto do Comercial, declarado aqui porque um domínio não importa outro
// (§3.1). "Não é seu" nunca é KindProibido: fora da carteira responde como
// inexistente (E-04).
var (
	ErrSemVendedor           = kernel.NovoErro(kernel.KindProibido, "usuário sem vendedor vinculado")
	ErrClienteForaDaCarteira = kernel.NovoErro(kernel.KindInvalido, "cliente não pertence à carteira deste vendedor")
)

// Erros de Pedido (mensagens públicas do pedido_handler.go legado).
var (
	ErrPedidoNaoEncontrado    = kernel.NovoErro(kernel.KindNaoEncontrado, "pedido não encontrado")
	ErrClienteIDObrigatorio   = kernel.NovoErro(kernel.KindInvalido, "cliente_id é obrigatório")
	ErrVendedorIDObrigatorio  = kernel.NovoErro(kernel.KindInvalido, "vendedor_id é obrigatório")
	ErrDataPedidoInvalida     = kernel.NovoErro(kernel.KindInvalido, "data_pedido inválida (use o formato AAAA-MM-DD)")
	ErrCanalInvalido          = kernel.NovoErro(kernel.KindInvalido, "canal inválido (use: App, Telefone, Visita, WhatsApp)")
	ErrStatusInvalido         = kernel.NovoErro(kernel.KindInvalido, "status inválido (use: Cancelado, Em separação, Entregue, Faturado)")
	ErrItensObrigatorios      = kernel.NovoErro(kernel.KindInvalido, "o pedido deve ter ao menos um item")
	ErrItensDemais            = kernel.NovoErro(kernel.KindInvalido, "o pedido deve ter no máximo 200 itens") // SEC-14: MaxItensPedido
	ErrProdutoIDObrigatorio   = kernel.NovoErro(kernel.KindInvalido, "produto_id é obrigatório em todos os itens")
	ErrQuantidadeInvalida     = kernel.NovoErro(kernel.KindInvalido, "quantidade deve ser maior que zero em todos os itens")
	ErrPrecoPraticadoInvalido = kernel.NovoErro(kernel.KindInvalido, "preco_praticado deve ser maior ou igual a zero em todos os itens")
	ErrDescontoPctInvalido    = kernel.NovoErro(kernel.KindInvalido, "desconto_pct deve estar entre 0 e 100 em todos os itens")
	// ErrPedidoJaFaturadoNaoPodeAlterarItens: pedido faturado (ou entregue,
	// que é pós-faturamento) não troca os itens (dessincronizaria o estoque
	// já baixado). O texto "apenas o status" é o contrato legado (R6).
	ErrPedidoJaFaturadoNaoPodeAlterarItens = kernel.NovoErro(kernel.KindConflito, "pedido já faturado: não é possível alterar os itens, apenas o status")
	// Máquina de status (BUG-16, BUG-20): Faturado só segue para Entregue,
	// Entregue é terminal e só se chega a Entregue a partir de Faturado (a
	// baixa de estoque acontece na entrada em Faturado; pular essa etapa,
	// na criação ou no PUT, burlaria a baixa). Erros de validação: 400.
	ErrPedidoFaturadoSoPodeIrParaEntregue = kernel.NovoErro(kernel.KindInvalido, "pedido faturado só pode ter o status alterado para Entregue")
	ErrPedidoEntregueNaoPodeMudarStatus   = kernel.NovoErro(kernel.KindInvalido, "pedido entregue não pode ter o status alterado")
	ErrPedidoEntregueSemFaturamento       = kernel.NovoErro(kernel.KindInvalido, "pedido só pode ser marcado como Entregue depois de Faturado")
	// ErrPedidoPossuiPagamentosVinculados e ErrPedidoFaturadoNaoPodeSerExcluido
	// são as duas travas da exclusão (V-01).
	ErrPedidoPossuiPagamentosVinculados = kernel.NovoErro(kernel.KindConflito, "pedido possui pagamentos vinculados")
	ErrPedidoFaturadoNaoPodeSerExcluido = kernel.NovoErro(kernel.KindConflito, "pedido faturado não pode ser excluído, apenas ter o status alterado")
	// ErrPedidoEntregueNaoPodeSerExcluido (BUG-16): Entregue é pós-faturamento
	// (estoque já baixado) e terminal; mesma trava da exclusão de Faturado.
	ErrPedidoEntregueNaoPodeSerExcluido = kernel.NovoErro(kernel.KindConflito, "pedido entregue não pode ser excluído")
	// ErrValorForaDoLimite (N-9): o cálculo de valor_bruto/valor_total
	// estourou o float64 (+Inf), o que só acontece com entradas absurdas.
	// O legado mandava o +Inf ao banco, que o recusava (DECIMAL(15,2)) com
	// 500; aqui o domínio recusa antes, com o mesmo 500 "erro interno".
	ErrValorForaDoLimite = kernel.NovoErro(kernel.KindInterno, "erro interno")
)

// Erros de Pagamento (mensagens públicas do pagamento_handler.go legado).
var (
	ErrPagamentoNaoEncontrado    = kernel.NovoErro(kernel.KindNaoEncontrado, "pagamento não encontrado")
	ErrPedidoIDObrigatorio       = kernel.NovoErro(kernel.KindInvalido, "pedido_id é obrigatório")
	ErrFormaPagamentoInvalida    = kernel.NovoErro(kernel.KindInvalido, "forma_pagamento inválida")
	ErrStatusPagamentoInvalido   = kernel.NovoErro(kernel.KindInvalido, "status_pagamento inválido")
	ErrParcelasInvalidas         = kernel.NovoErro(kernel.KindInvalido, "parcelas deve ser maior ou igual a 1")
	ErrValorInvalido             = kernel.NovoErro(kernel.KindInvalido, "valor deve ser maior ou igual a zero")
	ErrTaxaPctInvalida           = kernel.NovoErro(kernel.KindInvalido, "taxa_pct deve estar entre 0 e 30")
	ErrValorLiquidoInvalido      = kernel.NovoErro(kernel.KindInvalido, "valor_liquido deve ser maior ou igual a zero")
	ErrDataVencimentoObrigatoria = kernel.NovoErro(kernel.KindInvalido, "data_vencimento é obrigatória (use o formato AAAA-MM-DD)")
	ErrDataVencimentoInvalida    = kernel.NovoErro(kernel.KindInvalido, "data_vencimento inválida (use o formato AAAA-MM-DD)")
	ErrDataPagamentoInvalida     = kernel.NovoErro(kernel.KindInvalido, "data_pagamento inválida (use o formato AAAA-MM-DD)")
	// ErrPagamentoJaQuitadoNaoPodeSerExcluido preserva a trilha financeira.
	ErrPagamentoJaQuitadoNaoPodeSerExcluido = kernel.NovoErro(kernel.KindConflito, "pagamento já quitado não pode ser excluído")
	// ErrPagamentoJaQuitadoNaoPodeSerEditado (SEC-27): só o admin edita um
	// pagamento quitado; para o perfil restrito, 409 (espelha a exclusão).
	ErrPagamentoJaQuitadoNaoPodeSerEditado = kernel.NovoErro(kernel.KindConflito, "pagamento já quitado não pode ser editado")
)
