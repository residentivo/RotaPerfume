package comercialapp

import (
	"context"
	"errors"
	"log"

	"github.com/rotaperfumes/shared/comercial"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqEscopo = "escopo.go"

// desligamentos é o que o autorizador lê de vendedores.
type desligamentos interface {
	Desligado(ctx context.Context, id int64) (bool, error)
}

// vinculosAtivos é o que o autorizador lê de carteiras.
type vinculosAtivos interface {
	VinculoAtivo(ctx context.Context, vendedorID, clienteID int64) (*comercial.Carteira, error)
}

// AutorizadorEscopo resolve o escopo de carteira de um Principal A CADA
// chamada, com consultas próprias (sem cache, R4: um desligamento ou uma
// troca de vínculo vale na requisição seguinte). É o ÚNICO construtor de
// kernel.Escopo fora do kernel (O5, TestARQ_Escopo_SoAutorizador).
//
// Vendas (ARQ-05) e Analytics (ARQ-06) usam este mesmo autorizador pelas
// portas declaradas nos seus <ctx>app (sem ponte): uma regra só.
type AutorizadorEscopo struct {
	vinculo    VinculoUsuario
	vendedores desligamentos
	carteiras  vinculosAtivos
}

// NovoAutorizadorEscopo exige as três portas (nil: panic na montagem).
func NovoAutorizadorEscopo(v VinculoUsuario, vend desligamentos, cart vinculosAtivos) *AutorizadorEscopo {
	if v == nil || vend == nil || cart == nil {
		panic("comercialapp: NovoAutorizadorEscopo exige vínculo, vendedores e carteiras")
	}
	return &AutorizadorEscopo{vinculo: v, vendedores: vend, carteiras: cart}
}

// Resolver devolve o escopo da requisição (E-01):
//   - Principal inválido (zero value: request sem usuário, o que não ocorre
//     em rota protegida) → SemAcesso, sem consulta: o escopo mais restritivo,
//     como no legado (fail-closed sem 500; TestEscopo_SemUsuarioNoContexto);
//   - admin → Irrestrito, sem consulta;
//   - outro role (inclusive desconhecido, R-a) → restrito ao vendedor do
//     usuário; sem vínculo, usuário inexistente ou vínculo órfão →
//     SemAcesso;
//   - vendedor desligado → comercial.ErrVendedorDesligado (403);
//   - erro de banco → erro (500), nunca "sem filtro" (R3).
func (a *AutorizadorEscopo) Resolver(ctx context.Context, p kernel.Principal) (kernel.Escopo, error) {
	e, vendedorID, desligado, err := a.resolver(ctx, p)
	if err != nil {
		return kernel.Escopo{}, err
	}
	if desligado {
		log.Printf("[escopo] bloqueado vendedor_desligado user_id=%d vendedor_id=%d rota=%s", p.UserID(), vendedorID, kernel.RotaDe(ctx))
		return kernel.Escopo{}, comercial.ErrVendedorDesligado
	}
	return e, nil
}

// ResolverParaAnalytics é o escopo do Dashboard (D-01): o desligado NÃO é
// erro, vira SemAcesso com desligado=true (resposta 200 zerada com o aviso);
// vínculo órfão também é SemAcesso, sem o aviso. Nunca "zera o vendedor e
// segue" (o escopo SemAcesso não traduz para SQL).
func (a *AutorizadorEscopo) ResolverParaAnalytics(ctx context.Context, p kernel.Principal) (e kernel.Escopo, desligado bool, err error) {
	e, _, desligado, err = a.resolver(ctx, p)
	if err != nil {
		return kernel.Escopo{}, false, err
	}
	if desligado {
		return kernel.EscopoSemAcesso(), true, nil
	}
	return e, false, nil
}

// resolver faz as consultas comuns: id_vendedor do usuário e, se houver,
// o desligamento do vendedor.
func (a *AutorizadorEscopo) resolver(ctx context.Context, p kernel.Principal) (e kernel.Escopo, vendedorID int64, desligado bool, err error) {
	if !p.Valido() {
		log.Printf("[escopo] principal invalido") // N3: tag fixa para alerta (rota com escopo sem usuário)
		vlog.Printf(arqEscopo, "AutorizadorEscopo.resolver", "decisão sem_acesso motivo=principal_invalido")
		return kernel.EscopoSemAcesso(), 0, false, nil
	}
	if p.EhAdmin() {
		e, err = kernel.EscopoIrrestrito(p)
		return e, 0, false, err
	}
	vendedorID, vinculado, err := a.vinculo.VendedorDoUsuario(ctx, p.UserID())
	if err != nil {
		return kernel.Escopo{}, 0, false, err
	}
	if !vinculado || vendedorID <= 0 {
		vlog.Printf(arqEscopo, "AutorizadorEscopo.resolver", "decisão sem_acesso user_id=%d motivo=sem_vinculo", p.UserID())
		return kernel.EscopoSemAcesso(), 0, false, nil
	}
	desligado, err = a.vendedores.Desligado(ctx, vendedorID)
	switch {
	case errors.Is(err, comercial.ErrVendedorNaoEncontrado):
		vlog.Printf(arqEscopo, "AutorizadorEscopo.resolver", "decisão sem_acesso user_id=%d motivo=vinculo_orfao", p.UserID())
		return kernel.EscopoSemAcesso(), 0, false, nil
	case err != nil:
		return kernel.Escopo{}, 0, false, err
	}
	e, err = kernel.EscopoDoVendedor(vendedorID)
	return e, vendedorID, desligado, err
}

// ClienteNaCarteira informa se o cliente está na carteira ATIVA do escopo
// (E-05): sempre true no irrestrito; no escopo de vendedor, consulta o
// vínculo ativo (data_fim nula). SemAcesso/zero value → erro (O1).
func (a *AutorizadorEscopo) ClienteNaCarteira(ctx context.Context, e kernel.Escopo, clienteID int64) (bool, error) {
	vendedorID, restrito, err := e.Filtro()
	if err != nil || !restrito {
		return err == nil, err
	}
	_, err = a.carteiras.VinculoAtivo(ctx, vendedorID, clienteID)
	if errors.Is(err, comercial.ErrVinculoNaoEncontrado) {
		return false, nil
	}
	return err == nil, err
}
