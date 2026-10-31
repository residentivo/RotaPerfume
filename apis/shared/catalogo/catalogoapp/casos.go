package catalogoapp

import (
	"time"

	"github.com/rotaperfumes/shared/catalogo"
)

// Casos agrupa os casos de uso do Catálogo (um método por caso de uso).
// Não recebem Principal: a única regra de acesso do contexto é a política
// da rota (C-01: leitura de produto Authenticated; escrita de produto e todo
// o estoque AdminOnly), aplicada pelo httpx antes do handler.
type Casos struct {
	produtos Produtos
	estoques Estoques
	// imagens nil: casos de uso de imagem desligados (ErrImagensIndisponiveis).
	imagens *Imagens
	agora   func() time.Time
}

// Novo monta os casos de uso. Dependência nil é erro de montagem: panic.
func Novo(produtos Produtos, estoques Estoques) *Casos {
	if produtos == nil || estoques == nil {
		panic("catalogoapp: Novo exige os repositórios de produtos e estoques")
	}
	return &Casos{produtos: produtos, estoques: estoques, agora: time.Now}
}

// NovoComImagens monta os casos de uso com a imagem de produto (IMG-01).
// Qualquer dependência de imagem nil é erro de montagem: panic.
func NovoComImagens(produtos Produtos, estoques Estoques, img Imagens) *Casos {
	if img.Armazenamento == nil || img.Normalizador == nil || img.Tx == nil {
		panic("catalogoapp: NovoComImagens exige armazenamento, normalizador e transactor")
	}
	c := Novo(produtos, estoques)
	c.imagens = &img
	return c
}

// imagensLigadas devolve as dependências de imagem ou
// catalogo.ErrImagensIndisponiveis (montado sem NovoComImagens).
func (c *Casos) imagensLigadas() (*Imagens, error) {
	if c.imagens == nil {
		return nil, catalogo.ErrImagensIndisponiveis
	}
	return c.imagens, nil
}
