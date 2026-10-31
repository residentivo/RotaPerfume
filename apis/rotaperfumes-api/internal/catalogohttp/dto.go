// Package catalogohttp é a camada HTTP do Catálogo (ARQ-02): handlers finos
// (parse/DTO → caso de uso → envelope httpx). As rotas e as políticas (C-01)
// ficam na tabela do bootstrap.
package catalogohttp

import (
	"net/http"
	"strings"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/catalogo"
	"github.com/rotaperfumes/shared/catalogo/catalogoapp"
)

// produtoResposta tem as mesmas chaves, ordem e tipos do models.Produto
// legado (contrato JSON inalterado).
type produtoResposta struct {
	ID             int64      `json:"id"`
	SKU            string     `json:"sku"`
	Descricao      string     `json:"descricao"`
	Categoria      string     `json:"categoria"`
	Marca          string     `json:"marca"`
	NotaOlfativa   string     `json:"nota_olfativa"`
	PrecoTabela    float64    `json:"preco_tabela"`
	CustoUnitario  float64    `json:"custo_unitario"`
	Unidade        string     `json:"unidade"`
	DataLancamento *time.Time `json:"data_lancamento"`
	Ativo          bool       `json:"ativo"`
	// ImagemURL (IMG-01) é o caminho público da imagem, ou null sem imagem.
	ImagemURL *string   `json:"imagem_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func produtoParaResposta(p catalogo.Produto) produtoResposta {
	return produtoResposta{
		ID: p.ID, SKU: p.SKU.String(), Descricao: p.Descricao, Categoria: p.Categoria, Marca: p.Marca,
		NotaOlfativa: p.NotaOlfativa, PrecoTabela: p.PrecoTabela, CustoUnitario: p.CustoUnitario,
		Unidade: p.Unidade, DataLancamento: p.DataLancamento, Ativo: p.Ativo,
		ImagemURL: urlImagem(p.Imagem), CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
}

// urlImagem monta o caminho público só para nomes válidos (nunca expõe o
// diretório); sem imagem → nil (JSON null).
func urlImagem(nome string) *string {
	if !catalogo.NomeImagemValido(nome) {
		return nil
	}
	u := PrefixoURLImagem + nome
	return &u
}

// criarProdutoRequest é o body do POST de produto.
type criarProdutoRequest struct {
	SKU string `json:"sku"`
	atualizarProdutoRequest
}

// atualizarProdutoRequest é o body do PUT de produto. Não tem sku: como no
// legado, um "sku" no PUT é ignorado (e não vira 400 por tipo errado).
type atualizarProdutoRequest struct {
	Descricao      string  `json:"descricao"`
	Categoria      string  `json:"categoria"`
	Marca          string  `json:"marca"`
	NotaOlfativa   string  `json:"nota_olfativa"`
	PrecoTabela    float64 `json:"preco_tabela"`
	CustoUnitario  float64 `json:"custo_unitario"`
	Unidade        string  `json:"unidade"`
	DataLancamento string  `json:"data_lancamento"` // opcional, AAAA-MM-DD
}

func (r criarProdutoRequest) paraDados() catalogo.DadosProduto {
	d := r.atualizarProdutoRequest.paraDados()
	d.SKU = r.SKU
	return d
}

func (r atualizarProdutoRequest) paraDados() catalogo.DadosProduto {
	return catalogo.DadosProduto{
		Descricao: r.Descricao, Categoria: r.Categoria, Marca: r.Marca, NotaOlfativa: r.NotaOlfativa,
		PrecoTabela: r.PrecoTabela, CustoUnitario: r.CustoUnitario, Unidade: r.Unidade,
		DataLancamento: r.DataLancamento,
	}
}

// filtroProdutoDaQuery lê categoria, marca, ativo, q e a ordenação.
func filtroProdutoDaQuery(r *http.Request) catalogo.FiltroProduto {
	q := r.URL.Query()
	return catalogo.FiltroProduto{
		Categoria: strings.TrimSpace(q.Get("categoria")),
		Marca:     strings.TrimSpace(q.Get("marca")),
		Ativo:     httpx.BoolDaQuery(r, "ativo"),
		Busca:     strings.TrimSpace(q.Get("q")),
		Ordenacao: httpx.OrdenacaoDaQuery(r, catalogo.CamposOrdenacaoProduto(), catalogo.OrdenacaoProdutoPadrao()),
	}
}

// estoqueResposta tem as mesmas chaves, ordem e tipos do models.Estoque
// legado.
type estoqueResposta struct {
	ID               int64     `json:"id"`
	DataSnapshot     time.Time `json:"data_snapshot"`
	SKU              string    `json:"sku"`
	ProdutoDescricao string    `json:"produto_descricao"`
	Saldo            int       `json:"saldo"`
	Ruptura          bool      `json:"ruptura"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func estoqueParaResposta(e catalogo.RegistroEstoque) estoqueResposta {
	return estoqueResposta{
		ID: e.ID, DataSnapshot: e.DataSnapshot, SKU: e.SKU.String(), ProdutoDescricao: e.ProdutoDescricao,
		Saldo: e.Saldo, Ruptura: e.Ruptura, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

// criarEstoqueRequest é o body do POST de estoque (ajuste manual). ruptura
// NÃO é aceita: é derivada do saldo.
type criarEstoqueRequest struct {
	SKU          string `json:"sku"`
	DataSnapshot string `json:"data_snapshot"` // AAAA-MM-DD
	Saldo        int    `json:"saldo"`
}

// atualizarEstoqueRequest é o body do PUT de estoque: só o saldo (sku e
// data_snapshot no body são ignorados, como no legado).
type atualizarEstoqueRequest struct {
	Saldo int `json:"saldo"`
}

// consultaEstoqueDaQuery lê sku, data_de, data_ate, ruptura, historico e a
// ordenação.
func consultaEstoqueDaQuery(r *http.Request) catalogoapp.ConsultaEstoque {
	q := r.URL.Query()
	return catalogoapp.ConsultaEstoque{
		SKU:       strings.TrimSpace(q.Get("sku")),
		DataDe:    strings.TrimSpace(q.Get("data_de")),
		DataAte:   strings.TrimSpace(q.Get("data_ate")),
		Ruptura:   httpx.BoolDaQuery(r, "ruptura"),
		Ordenacao: httpx.OrdenacaoDaQuery(r, catalogo.CamposOrdenacaoEstoque(), catalogo.OrdenacaoEstoquePadrao()),
		Historico: strings.EqualFold(strings.TrimSpace(q.Get("historico")), "true"),
	}
}
