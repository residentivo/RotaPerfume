package identidadeapp

import (
	"context"
	"log"
	"time"
)

// timeoutLimpeza limita cada execução da limpeza periódica.
const timeoutLimpeza = 2 * time.Minute

// LimparRefreshExpirados apaga, em lotes de LoteLimpezaRefresh, os tokens
// com expires_at < agora - retenção (CHORE-02, I-10). A retenção nunca fica
// abaixo de RetencaoRefreshMinima: nenhum token ainda útil para detectar
// reuso é apagado. Para quando um lote vem incompleto ou o ctx é cancelado;
// em erro devolve o total parcial junto com o erro.
func (c *Casos) LimparRefreshExpirados(ctx context.Context) (int64, error) {
	retencao := max(c.o.RetencaoRefresh, RetencaoRefreshMinima)
	corte := c.agora().Add(-retencao)
	var total int64
	for ctx.Err() == nil {
		n, err := c.d.Tokens.ApagarExpirados(ctx, corte, LoteLimpezaRefresh)
		total += n
		if err != nil {
			log.Printf("[refresh] cleanup: falha após %d tokens removidos (corte=%s): %v", total, corte.Format(time.RFC3339), err)
			return total, err
		}
		if n < LoteLimpezaRefresh {
			break
		}
	}
	if err := ctx.Err(); err != nil {
		log.Printf("[refresh] cleanup: interrompido após %d tokens removidos (corte=%s): %v", total, corte.Format(time.RFC3339), err)
		return total, err
	}
	log.Printf("[refresh] cleanup: %d tokens removidos (corte=%s)", total, corte.Format(time.RFC3339))
	return total, nil
}

// IniciarLimpezaPeriodica roda LimparRefreshExpirados já no início e a cada
// intervalo até o ctx ser cancelado; o canal devolvido fecha ao terminar
// (o shutdown espera por ele antes de fechar o pool, I-10). intervalo <= 0
// desativa (canal já fechado). Cada execução tem timeout próprio e um panic
// é recuperado e logado, sem parar as execuções seguintes.
func IniciarLimpezaPeriodica(ctx context.Context, c *Casos, intervalo time.Duration) <-chan struct{} {
	done := make(chan struct{})
	if intervalo <= 0 {
		log.Printf("[refresh] cleanup: limpeza periódica desativada (REFRESH_CLEANUP_INTERVAL=0)")
		close(done)
		return done
	}
	go func() {
		defer close(done)
		log.Printf("[refresh] cleanup: limpeza periódica iniciada (intervalo=%s)", intervalo)
		executarLimpeza(ctx, c)
		ticker := time.NewTicker(intervalo)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Printf("[refresh] cleanup: limpeza periódica encerrada")
				return
			case <-ticker.C:
				executarLimpeza(ctx, c)
			}
		}
	}()
	return done
}

func executarLimpeza(ctx context.Context, c *Casos) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[refresh] cleanup: panic recuperado: %v", r)
		}
	}()
	if ctx.Err() != nil {
		return
	}
	execCtx, cancel := context.WithTimeout(ctx, timeoutLimpeza)
	defer cancel()
	if _, err := c.LimparRefreshExpirados(execCtx); err != nil {
		log.Printf("[refresh] cleanup: erro: %v", err)
	}
}
