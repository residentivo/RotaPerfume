package comercialapp

import (
	"context"
	"time"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/kernel"
)

// Deps são as dependências dos casos de uso, todas obrigatórias.
type Deps struct {
	Vendedores    Vendedores
	Carteiras     Carteiras
	Clientes      Clientes
	Visitas       Visitas
	Oportunidades Oportunidades
	Vinculo       VinculoUsuario
	// Desligamento é o UoW do desligamento de vendedor (I-09).
	Desligamento Transactor[TxDesligamento]
	// ClienteNaCarteira é o UoW do cadastro de cliente por usuário
	// restrito (cliente + vínculo).
	ClienteNaCarteira Transactor[TxClienteNaCarteira]
	// Vinculacao é o UoW da vinculação de cliente a vendedor (BUG-14).
	Vinculacao Transactor[TxVinculo]
	// Relogio é a fonte de "agora" (datas padrão, desligamento, vínculos).
	Relogio func() time.Time
}

// Casos agrupa os casos de uso do contexto (um método por caso de uso).
// Todos recebem o Principal e falham fechado com ele inválido (O5).
type Casos struct {
	d      Deps
	escopo *AutorizadorEscopo
}

// Novo monta os casos de uso. Dependência nil é erro de montagem: panic.
func Novo(d Deps) *Casos {
	obrigatorias := []any{d.Vendedores, d.Carteiras, d.Clientes, d.Visitas, d.Oportunidades,
		d.Vinculo, d.Desligamento, d.ClienteNaCarteira, d.Vinculacao}
	for _, dep := range obrigatorias {
		if dep == nil {
			panic("comercialapp: Novo exige todas as dependências")
		}
	}
	if d.Relogio == nil {
		panic("comercialapp: Novo exige o relógio")
	}
	return &Casos{d: d, escopo: NovoAutorizadorEscopo(d.Vinculo, d.Vendedores, d.Carteiras)}
}

// Escopo devolve o autorizador (a mesma regra para Vendas, porta
// vendasapp.EscopoCarteira, e para Analytics, porta
// analyticsapp.EscopoAnalytics; ligadas no bootstrap).
func (c *Casos) Escopo() *AutorizadorEscopo { return c.escopo }

func (c *Casos) agora() time.Time { return c.d.Relogio() }

// Leitor adia a leitura do comando (o body) até o caso de uso autorizar:
// "escopo antes do decode" (E-05). O handler passa a decodificação; o caso
// de uso só a chama depois de resolver o escopo e checar a posse.
type Leitor[T any] func() (T, error)

// ErrAcessoRestritoAdmin é a defesa em profundidade da gestão de vendedores
// (a rota já é AdminOnly; mesma mensagem do 403 do httpx).
var ErrAcessoRestritoAdmin = kernel.NovoErro(kernel.KindProibido, "acesso restrito a administradores")

// exigirAdmin: Principal inválido falha fechado (500); não admin, 403.
func exigirAdmin(p kernel.Principal) error {
	if !p.Valido() {
		return kernel.ErrPrincipalInvalido
	}
	if !p.EhAdmin() {
		return ErrAcessoRestritoAdmin
	}
	return nil
}

// noEscopo resolve o escopo e traduz SemAcesso em semAcesso (o erro de
// cada operação: 404 no detalhe e na escrita de registro existente, 403 na
// criação).
func (c *Casos) noEscopo(ctx context.Context, p kernel.Principal, semAcesso error) (kernel.Escopo, error) {
	e, err := c.escopo.Resolver(ctx, p)
	if err != nil {
		return kernel.Escopo{}, err
	}
	if e.SemAcesso() {
		return kernel.Escopo{}, semAcesso
	}
	return e, nil
}

// obterNoEscopo lê um registro de carteira (visita, oportunidade) e o
// esconde com naoEncontrado quando é de outro vendedor (E-04: mesma
// resposta de "não existe", nunca 403).
func obterNoEscopo[T any](ctx context.Context, e kernel.Escopo, id int64, ler func(context.Context, int64) (*T, error), vendedorDe func(*T) int64, naoEncontrado error) (*T, error) {
	v, err := ler(ctx, id)
	if err != nil {
		return nil, err
	}
	if !e.Permite(vendedorDe(v)) {
		return nil, naoEncontrado
	}
	return v, nil
}

// vendedorDoComando aplica a política de escrita (E-05): no escopo de
// vendedor, força o vendedor do escopo e exige o cliente na carteira ativa
// (fora dela: 400 ErrClienteForaDaCarteira); no irrestrito, vale o payload.
func (c *Casos) vendedorDoComando(ctx context.Context, e kernel.Escopo, clienteID, doPayload int64) (int64, error) {
	vendedorID, restrito, err := comercial.VendedorDaEscrita(e, doPayload)
	if err != nil || !restrito {
		return vendedorID, err
	}
	naCarteira, err := c.escopo.ClienteNaCarteira(ctx, e, clienteID)
	if err != nil {
		return 0, err
	}
	if !naCarteira {
		return 0, comercial.ErrClienteForaDaCarteira
	}
	return vendedorID, nil
}

// exigirClienteEVendedor confere a existência dos dois (400 nas mensagens
// de visita/oportunidade), na ordem do legado: cliente, depois vendedor.
func (c *Casos) exigirClienteEVendedor(ctx context.Context, clienteID, vendedorID int64) error {
	existe, err := c.d.Clientes.Existe(ctx, clienteID)
	if err != nil {
		return err
	}
	if !existe {
		return comercial.ErrClienteInvalido
	}
	if existe, err = c.d.Vendedores.Existe(ctx, vendedorID); err != nil {
		return err
	}
	if !existe {
		return comercial.ErrVendedorInvalido
	}
	return nil
}

// vazio é a listagem de quem não tem carteira (E-03): 200 com "data": [] e
// sem tocar no banco. O slice não nulo distingue do "sem linhas" do banco
// (que o legado serializa como null).
func vazio[T any](p kernel.Pagina) kernel.Resultado[T] {
	return kernel.NovoResultado(make([]T, 0), p, 0)
}
