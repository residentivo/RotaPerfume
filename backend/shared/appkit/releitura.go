// Package appkit tem os passos repetidos pelos casos de uso de todos os
// contextos (lição 4 do ARQ-02): a releitura depois da escrita (BUG-08/09).
// Só depende da stdlib; pode ser importado por qualquer <ctx>app.
package appkit

import (
	"context"
	"fmt"
	"log"
)

// LerPorID é a forma comum do PorID dos repositórios.
type LerPorID[T any] func(ctx context.Context, id int64) (*T, error)

// RelerCriado relê o registro recém-inserido para devolver created_at e
// updated_at do banco (BUG-08/09). O INSERT já foi confirmado: se a
// releitura falhar, loga "<rotulo>=<id>: <erro>" e devolve o objeto em
// memória (timestamps zerados) em vez de responder erro, porque um retry
// esbarraria na chave duplicada (ou duplicaria o registro). rotulo é o texto
// exato do legado, ex.: "[visitas] criada, mas falhou a releitura: id".
func RelerCriado[T any](ctx context.Context, rotulo string, id int64, emMemoria *T, ler LerPorID[T]) *T {
	gravado, err := ler(ctx, id)
	if err != nil {
		log.Printf("%s=%d: %v", rotulo, id, err)
		return emMemoria
	}
	return gravado
}

// RelerAtualizado relê o registro depois de um UPDATE. Qualquer falha é 500,
// inclusive "não encontrado" (corrida com remoção): o erro é reembrulhado SEM
// %w para o sentinela não virar 404, como no legado, em que a releitura
// devolvia o erro cru do repositório. pacote prefixa a mensagem.
func RelerAtualizado[T any](ctx context.Context, pacote string, id int64, ler LerPorID[T]) (*T, error) {
	v, err := ler(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s: releitura após atualizar: %v", pacote, err)
	}
	return v, nil
}
