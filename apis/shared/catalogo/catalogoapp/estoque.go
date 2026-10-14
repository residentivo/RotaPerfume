package catalogoapp

import (
	"context"

	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqEstoque = "estoque.go"

// ConsultaEstoque é a consulta da listagem de estoque, com as datas ainda
// cruas (AAAA-MM-DD): a validação delas é regra do domínio.
type ConsultaEstoque struct {
	SKU       string
	DataDe    string
	DataAte   string
	Ruptura   *bool
	Ordenacao kernel.Ordenacao[catalogo.CampoEstoque]
	// Historico=true devolve a série temporal completa; o padrão é só a
	// última posição de cada SKU (respeitando o filtro de datas).
	Historico bool
}

// ListarEstoque pagina o estoque: última posição por SKU (padrão) ou a
// série histórica. Data malformada → ErrEstoqueDataInvalida, sem consultar.
func (c *Casos) ListarEstoque(ctx context.Context, q ConsultaEstoque, p kernel.Pagina) (res kernel.Resultado[catalogo.RegistroEstoque], err error) {
	defer vlog.Trace(arqEstoque, "Catalogo.ListarEstoque", "page=%d limit=%d historico=%t ordem=%s",
		p.Numero(), p.Limite(), q.Historico, q.Ordenacao.Campo())(&err)
	f, err := catalogo.NovoFiltroEstoque(q.SKU, q.DataDe, q.DataAte, q.Ruptura, q.Ordenacao)
	if err != nil {
		return kernel.Resultado[catalogo.RegistroEstoque]{}, err
	}
	if q.Historico {
		return c.estoques.Historico(ctx, f, p)
	}
	return c.estoques.UltimaPosicao(ctx, f, p)
}

// ObterEstoque devolve o registro ou catalogo.ErrEstoqueNaoEncontrado.
func (c *Casos) ObterEstoque(ctx context.Context, id int64) (e *catalogo.RegistroEstoque, err error) {
	defer vlog.Trace(arqEstoque, "Catalogo.ObterEstoque", "id=%d", id)(&err)
	return c.estoques.PorID(ctx, id)
}

// AjustarEstoque cria um ajuste MANUAL: valida o registro, exige que o SKU
// exista em produtos e devolve o registro relido.
func (c *Casos) AjustarEstoque(ctx context.Context, d catalogo.DadosEstoque) (e *catalogo.RegistroEstoque, err error) {
	defer vlog.Trace(arqEstoque, "Catalogo.AjustarEstoque", "saldo=%d", d.Saldo)(&err)
	e, err = catalogo.NovoRegistroEstoque(d, c.agora())
	if err != nil {
		return nil, err
	}
	existe, err := c.produtos.ExisteSKU(ctx, e.SKU)
	if err != nil {
		return nil, err
	}
	if !existe {
		vlog.Printf(arqEstoque, "Catalogo.AjustarEstoque", "decisão sku inexistente em produtos")
		return nil, catalogo.ErrEstoqueSKUInexistente
	}
	if err = c.estoques.Inserir(ctx, e); err != nil {
		return nil, err
	}
	return relerCriado(ctx, "[estoque]", e.ID, e, c.estoques.PorID), nil
}

// AtualizarSaldo é o PUT de estoque: só o saldo muda (SKU e data são a chave
// de negócio); a ruptura é recalculada.
func (c *Casos) AtualizarSaldo(ctx context.Context, id int64, saldo int) (e *catalogo.RegistroEstoque, err error) {
	defer vlog.Trace(arqEstoque, "Catalogo.AtualizarSaldo", "id=%d saldo=%d", id, saldo)(&err)
	e, err = c.estoques.PorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = e.AjustarSaldo(saldo); err != nil {
		return nil, err
	}
	if err = c.estoques.AtualizarSaldo(ctx, id, e.Saldo, e.Ruptura); err != nil {
		return nil, err
	}
	return relerAtualizado(ctx, id, c.estoques.PorID)
}
