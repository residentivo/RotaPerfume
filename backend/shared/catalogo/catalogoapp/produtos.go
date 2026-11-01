package catalogoapp

import (
	"context"
	"errors"
	"log"

	"github.com/rotaperfumes/shared/appkit"
	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqProdutos = "produtos.go"

// ListarProdutos pagina os produtos que casam com o filtro.
func (c *Casos) ListarProdutos(ctx context.Context, f catalogo.FiltroProduto, p kernel.Pagina) (res kernel.Resultado[catalogo.Produto], err error) {
	defer vlog.Trace(arqProdutos, "Catalogo.ListarProdutos", "page=%d limit=%d busca=%t ordem=%s",
		p.Numero(), p.Limite(), f.Busca != "", f.Ordenacao.Campo())(&err)
	return c.produtos.Listar(ctx, f, p)
}

// ObterProduto devolve o produto ou catalogo.ErrProdutoNaoEncontrado.
func (c *Casos) ObterProduto(ctx context.Context, id int64) (p *catalogo.Produto, err error) {
	defer vlog.Trace(arqProdutos, "Catalogo.ObterProduto", "id=%d", id)(&err)
	return c.produtos.PorID(ctx, id)
}

// CriarProduto valida, grava (sempre ativo) e devolve o produto relido.
func (c *Casos) CriarProduto(ctx context.Context, d catalogo.DadosProduto) (p *catalogo.Produto, err error) {
	defer vlog.Trace(arqProdutos, "Catalogo.CriarProduto", "")(&err)
	p, err = catalogo.NovoProduto(d)
	if err != nil {
		return nil, err
	}
	if err = c.produtos.Inserir(ctx, p); err != nil {
		if errors.Is(err, catalogo.ErrSKUDuplicado) {
			log.Printf("[produtos] sku duplicado recusado (uk_produtos_sku)")
		}
		return nil, err
	}
	return appkit.RelerCriado(ctx, "[produtos] criado, mas falhou a releitura: id", p.ID, p, c.produtos.PorID), nil
}

// AtualizarProduto valida e grava os campos editáveis (SKU e ativo não
// mudam) e devolve o produto relido.
func (c *Casos) AtualizarProduto(ctx context.Context, id int64, d catalogo.DadosProduto) (p *catalogo.Produto, err error) {
	defer vlog.Trace(arqProdutos, "Catalogo.AtualizarProduto", "id=%d", id)(&err)
	campos, err := catalogo.NovosCamposProduto(d)
	if err != nil {
		return nil, err
	}
	if err = c.produtos.Atualizar(ctx, id, campos); err != nil {
		return nil, err
	}
	return appkit.RelerAtualizado(ctx, "catalogoapp", id, c.produtos.PorID)
}

// AlternarAtivoProduto aplica o PATCH /inativar (exclusão lógica): ativo nil
// inverte o status no próprio UPDATE (BUG-17: dois toggles simultâneos não
// se anulam por ler-inverter-gravar); um valor o define. A resposta é o
// produto relido depois da escrita (com o updated_at real).
func (c *Casos) AlternarAtivoProduto(ctx context.Context, id int64, ativo *bool) (p *catalogo.Produto, err error) {
	defer vlog.Trace(arqProdutos, "Catalogo.AlternarAtivoProduto", "id=%d toggle=%t", id, ativo == nil)(&err)
	if ativo == nil {
		err = c.produtos.AlternarAtivo(ctx, id)
	} else {
		err = c.produtos.DefinirAtivo(ctx, id, *ativo)
	}
	if err != nil {
		return nil, err
	}
	if p, err = c.produtos.PorID(ctx, id); err != nil {
		return nil, err
	}
	vlog.Printf(arqProdutos, "Catalogo.AlternarAtivoProduto", "decisão ativo=%t id=%d", p.Ativo, id)
	return p, nil
}
