// Package vendasapp tem os casos de uso do contexto Vendas (ARQ-05) e as
// portas que eles consomem. As portas são declaradas aqui, pelo consumidor;
// vendasmysql as satisfaz implicitamente. As portas para outros contextos
// (EscopoCarteira → Comercial; BaixaEstoque → Catálogo) são ligadas no
// bootstrap (§3.1): este pacote não importa nenhum outro contexto.
package vendasapp

import (
	"context"
	"time"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
)

// Pedidos é o repositório de leitura de pedidos (fora da tx).
type Pedidos interface {
	// Listar: Irrestrito → filtro de negócio da query; Vendedor → o
	// vendedor do escopo sobrescreve o da query; SemAcesso/zero value →
	// erro SEM SQL (O1).
	Listar(ctx context.Context, e kernel.Escopo, f vendas.FiltroPedido, p kernel.Pagina) (kernel.Resultado[vendas.Pedido], error)
	// PorID devolve o cabeçalho (sem itens) ou vendas.ErrPedidoNaoEncontrado.
	PorID(ctx context.Context, id int64) (*vendas.Pedido, error)
	// Itens devolve os itens do pedido (nil sem linhas).
	Itens(ctx context.Context, pedidoID int64) ([]vendas.ItemPedido, error)
	Existe(ctx context.Context, id int64) (bool, error)
}

// Pagamentos é o repositório de pagamentos.
type Pagamentos interface {
	// Listar: o escopo de vendedor filtra pelos pedidos do vendedor
	// (pedidos.vendedor_id, V-02); SemAcesso/zero value → erro SEM SQL (O1).
	Listar(ctx context.Context, e kernel.Escopo, f vendas.FiltroPagamento, p kernel.Pagina) (kernel.Resultado[vendas.Pagamento], error)
	// PorID devolve vendas.ErrPagamentoNaoEncontrado se não existir.
	PorID(ctx context.Context, id int64) (*vendas.Pagamento, error)
	// Inserir grava e preenche PagamentoID.
	Inserir(ctx context.Context, p *vendas.Pagamento) error
	// Atualizar grava os campos editáveis (nunca pedido_id, V-02);
	// ErrPagamentoNaoEncontrado se nenhuma linha casar.
	Atualizar(ctx context.Context, id int64, p *vendas.Pagamento) error
	Excluir(ctx context.Context, id int64) error
	ExistePorPedido(ctx context.Context, pedidoID int64) (bool, error)
}

// EscopoCarteira é a porta para Comercial (comercialapp.AutorizadorEscopo
// a satisfaz): o escopo resolvido A CADA chamada, sem cache (R4).
type EscopoCarteira interface {
	// Resolver: Principal inválido → SemAcesso; admin → Irrestrito; restrito
	// → o vendedor do usuário (desligado → 403; erro → 500).
	Resolver(ctx context.Context, p kernel.Principal) (kernel.Escopo, error)
	// ClienteNaCarteira: o cliente está na carteira ATIVA do escopo (E-05).
	ClienteNaCarteira(ctx context.Context, e kernel.Escopo, clienteID int64) (bool, error)
}

// PedidosNaTx são as escritas de pedido ligadas à tx do pedido (C9).
type PedidosNaTx interface {
	// Inserir grava o cabeçalho e os itens; preenche PedidoIDOrigem e o
	// ItemIDOrigem/PedidoID de cada item (no próprio slice p.Itens).
	Inserir(ctx context.Context, p *vendas.Pedido) error
	// StatusParaAtualizar lê o status com SELECT … FOR UPDATE (serializa o
	// PUT concorrente); ErrPedidoNaoEncontrado sem linha.
	StatusParaAtualizar(ctx context.Context, id int64) (string, error)
	// ItensAtuais lê produto, quantidade, preço e desconto dos itens.
	ItensAtuais(ctx context.Context, id int64) ([]vendas.ItemPedido, error)
	// AtualizarCabecalho: ErrPedidoNaoEncontrado se nenhuma linha casar.
	AtualizarCabecalho(ctx context.Context, id int64, p *vendas.Pedido) error
	// SubstituirItens apaga os itens e insere os novos (preenche os ids).
	SubstituirItens(ctx context.Context, id int64, itens []vendas.ItemPedido) error
	// Excluir apaga os itens e o pedido; ErrPedidoNaoEncontrado sem linha.
	Excluir(ctx context.Context, id int64) error
}

// BaixaEstoque é a porta para Catálogo DENTRO da tx do pedido: a baixa de
// estoque do faturamento (catalogomysql.Estoques a satisfaz, ligada ao
// mesmo *sql.Tx no bootstrap).
type BaixaEstoque interface {
	// SKUDoProduto: a tabela estoque referencia produtos por sku. Produto
	// inexistente é erro de infraestrutura (500), nunca sentinela.
	SKUDoProduto(ctx context.Context, produtoID int64) (string, error)
	// AjustarSaldoPorFaturamento aplica o DELTA (saldo -= delta) no dia, com
	// SELECT … FOR UPDATE + upsert atômico.
	AjustarSaldoPorFaturamento(ctx context.Context, sku string, dia time.Time, delta int) error
}

// TxPedido são os repositórios ligados à tx do pedido: cabeçalho, itens e
// a baixa de estoque do faturamento no MESMO *sql.Tx (§5.2).
type TxPedido struct {
	Pedidos PedidosNaTx
	Estoque BaixaEstoque
}

// Transactor é o Unit of Work (db.Transactor satisfaz).
type Transactor[R any] interface {
	WithTx(ctx context.Context, fn func(R) error) error
}

// Leitor adia a leitura do comando (o body ou a query) até o caso de uso
// autorizar: "escopo antes do decode" (E-05, lição 3 do ARQ-04).
type Leitor[T any] func() (T, error)
