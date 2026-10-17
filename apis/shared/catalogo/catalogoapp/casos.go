package catalogoapp

import "time"

// Casos agrupa os casos de uso do Catálogo (um método por caso de uso).
// Não recebem Principal: a única regra de acesso do contexto é a política
// da rota (C-01: leitura de produto Authenticated; escrita de produto e todo
// o estoque AdminOnly), aplicada pelo httpx antes do handler.
type Casos struct {
	produtos Produtos
	estoques Estoques
	agora    func() time.Time
}

// Novo monta os casos de uso. Dependência nil é erro de montagem: panic.
func Novo(produtos Produtos, estoques Estoques) *Casos {
	if produtos == nil || estoques == nil {
		panic("catalogoapp: Novo exige os repositórios de produtos e estoques")
	}
	return &Casos{produtos: produtos, estoques: estoques, agora: time.Now}
}
