package vendasapp

import (
	"context"
	"time"

	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vendas"
)

// Deps são as dependências dos casos de uso, todas obrigatórias.
type Deps struct {
	Pedidos    Pedidos
	Pagamentos Pagamentos
	Escopo     EscopoCarteira
	// TxPedido é o UoW do pedido: cabeçalho + itens (+ baixa de estoque).
	TxPedido Transactor[TxPedido]
	// Relogio é a fonte de "agora" (dia da baixa de estoque).
	Relogio func() time.Time
}

// Casos agrupa os casos de uso de Vendas (um método por caso de uso). Todos
// recebem o Principal e o passam ao autorizador: Principal inválido (rota
// sem usuário) é SemAcesso, o escopo mais restritivo, sem consulta
// (contrato legado; registro ARQ-04, divergência b).
type Casos struct{ d Deps }

// Novo monta os casos de uso. Dependência nil é erro de montagem: panic.
func Novo(d Deps) *Casos {
	for _, dep := range []any{d.Pedidos, d.Pagamentos, d.Escopo, d.TxPedido} {
		if dep == nil {
			panic("vendasapp: Novo exige todas as dependências")
		}
	}
	if d.Relogio == nil {
		panic("vendasapp: Novo exige o relógio")
	}
	return &Casos{d: d}
}

// noEscopo resolve o escopo e traduz SemAcesso em semAcesso (o erro de
// cada operação: 404 no detalhe e na escrita de registro existente, 403 na
// criação).
func (c *Casos) noEscopo(ctx context.Context, p kernel.Principal, semAcesso error) (kernel.Escopo, error) {
	e, err := c.d.Escopo.Resolver(ctx, p)
	if err != nil {
		return kernel.Escopo{}, err
	}
	if e.SemAcesso() {
		return kernel.Escopo{}, semAcesso
	}
	return e, nil
}

// restrito informa se o escopo limita a carteira, pela única tradução do
// escopo (Filtro, O1): o escopo de vendedor filtra; o zero value (erro) conta
// como restrito (fail-closed). Só o Irrestrito (admin) devolve false.
func restrito(e kernel.Escopo) bool {
	_, filtra, err := e.Filtro()
	return err != nil || filtra
}

// vendedorDoComando aplica a política de escrita (E-05): no escopo de
// vendedor, o vendedor_id do payload é IGNORADO (vale o do escopo) e o
// cliente tem de estar na carteira ativa (fora: 400); no irrestrito vale o
// payload. SemAcesso já foi barrado por noEscopo antes do decode; o zero
// value volta como kernel.ErrEscopoInvalido (500, fail-closed).
func (c *Casos) vendedorDoComando(ctx context.Context, e kernel.Escopo, clienteID, doPayload int64) (int64, error) {
	vendedorID, filtra, err := e.Filtro()
	if err != nil {
		return 0, err
	}
	if !filtra {
		return doPayload, nil
	}
	naCarteira, err := c.d.Escopo.ClienteNaCarteira(ctx, e, clienteID)
	if err != nil {
		return 0, err
	}
	if !naCarteira {
		return 0, vendas.ErrClienteForaDaCarteira
	}
	return vendedorID, nil
}

// vazio é a listagem de quem não tem carteira (E-03): 200 com "data": [] e
// sem tocar no banco. O slice não nulo distingue do "sem linhas" do banco
// (que o legado serializa como null).
func vazio[T any](p kernel.Pagina) kernel.Resultado[T] {
	return kernel.NovoResultado(make([]T, 0), p, 0)
}
