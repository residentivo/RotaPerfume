package catalogoapp

import (
	"context"
	"fmt"
	"log"
	"time"
)

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

// lerPorID é a forma comum de PorID dos dois repositórios.
type lerPorID[T any] func(ctx context.Context, id int64) (*T, error)

// relerCriado relê o registro recém-inserido para devolver created_at e
// updated_at do banco (BUG-09). O INSERT já foi confirmado: se a releitura
// falhar, loga e devolve o objeto em memória (timestamps zerados) em vez de
// responder erro, porque um retry esbarraria na chave duplicada.
func relerCriado[T any](ctx context.Context, tag string, id int64, emMemoria *T, ler lerPorID[T]) *T {
	gravado, err := ler(ctx, id)
	if err != nil {
		log.Printf("%s criado, mas falhou a releitura: id=%d: %v", tag, id, err)
		return emMemoria
	}
	return gravado
}

// relerAtualizado relê o registro depois de um UPDATE. Qualquer falha aqui é
// 500, inclusive "não encontrado" (corrida com remoção): o erro é
// reembrulhado SEM %w para o sentinela não virar 404, como no legado, em que
// a releitura devolvia o erro cru do repositório.
func relerAtualizado[T any](ctx context.Context, id int64, ler lerPorID[T]) (*T, error) {
	v, err := ler(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("catalogoapp: releitura após atualizar: %v", err)
	}
	return v, nil
}
