package services

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// refreshCleanupTimeout limita cada execução da limpeza de refresh_tokens.
const refreshCleanupTimeout = 2 * time.Minute

// IniciarLimpezaRefreshTokens roda CleanupExpired imediatamente e a cada
// intervalo até ctx ser cancelado. Devolve um canal fechado ao terminar.
//
// CHORE-02: intervalo <= 0 desativa a limpeza (canal já fechado, nada roda).
// Cada execução tem timeout próprio e um panic é recuperado e logado, sem
// derrubar o processo nem parar as execuções seguintes.
func IniciarLimpezaRefreshTokens(ctx context.Context, db *sql.DB, svc *RefreshTokenService, intervalo time.Duration) <-chan struct{} {
	done := make(chan struct{})
	if intervalo <= 0 {
		log.Printf("[refresh] cleanup: limpeza periódica desativada (REFRESH_CLEANUP_INTERVAL=0)")
		close(done)
		return done
	}

	go func() {
		defer close(done)
		log.Printf("[refresh] cleanup: limpeza periódica iniciada (intervalo=%s)", intervalo)

		executarLimpezaRefresh(ctx, db, svc)

		ticker := time.NewTicker(intervalo)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Printf("[refresh] cleanup: limpeza periódica encerrada")
				return
			case <-ticker.C:
				executarLimpezaRefresh(ctx, db, svc)
			}
		}
	}()
	return done
}

// executarLimpezaRefresh roda uma limpeza com timeout e recupera panic.
func executarLimpezaRefresh(ctx context.Context, db *sql.DB, svc *RefreshTokenService) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[refresh] cleanup: panic recuperado: %v", r)
		}
	}()
	if ctx.Err() != nil {
		return
	}

	runCtx, cancel := context.WithTimeout(ctx, refreshCleanupTimeout)
	defer cancel()
	if _, err := svc.CleanupExpired(runCtx, db); err != nil {
		log.Printf("[refresh] cleanup: erro: %v", err)
	}
}
