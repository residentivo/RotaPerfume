package services

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/rotaperfumes/shared/vlog"
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
	vlog.Printf("refresh_cleanup.go", "IniciarLimpezaRefreshTokens", "chamando make e declarando done")
	done := make(chan struct{})
	vlog.Printf("refresh_cleanup.go", "IniciarLimpezaRefreshTokens", "verificando condição intervalo <= 0")
	if intervalo <= 0 {
		log.Printf("[refresh] cleanup: limpeza periódica desativada (REFRESH_CLEANUP_INTERVAL=0)")
		vlog.Printf("refresh_cleanup.go", "IniciarLimpezaRefreshTokens", "chamando close")
		close(done)
		return done
	}

	vlog.Printf("refresh_cleanup.go", "IniciarLimpezaRefreshTokens", "disparando goroutine: função anônima")
	go func() {
		vlog.Printf("refresh_cleanup.go", "IniciarLimpezaRefreshTokens.func", "agendando defer: close")
		defer close(done)
		log.Printf("[refresh] cleanup: limpeza periódica iniciada (intervalo=%s)", intervalo)

		vlog.Printf("refresh_cleanup.go", "IniciarLimpezaRefreshTokens.func", "chamando executarLimpezaRefresh")
		executarLimpezaRefresh(ctx, db, svc)

		vlog.Printf("refresh_cleanup.go", "IniciarLimpezaRefreshTokens.func", "chamando time.NewTicker e declarando ticker")
		ticker := time.NewTicker(intervalo)
		vlog.Printf("refresh_cleanup.go", "IniciarLimpezaRefreshTokens.func", "agendando defer: ticker.Stop")
		defer ticker.Stop()
		vlog.Printf("refresh_cleanup.go", "IniciarLimpezaRefreshTokens.func", "iniciando loop for contínuo")
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
	vlog.Printf("refresh_cleanup.go", "executarLimpezaRefresh", "agendando defer: função anônima")
	defer func() {
		vlog.Printf("refresh_cleanup.go", "executarLimpezaRefresh.func", "chamando recover e declarando r e verificando condição r != nil")
		if r := recover(); r != nil {
			log.Printf("[refresh] cleanup: panic recuperado: %v", r)
		}
	}()
	vlog.Printf("refresh_cleanup.go", "executarLimpezaRefresh", "verificando condição ctx.Err() != nil")
	if ctx.Err() != nil {
		return
	}

	vlog.Printf("refresh_cleanup.go", "executarLimpezaRefresh", "chamando context.WithTimeout e declarando runCtx, cancel")
	runCtx, cancel := context.WithTimeout(ctx, refreshCleanupTimeout)
	vlog.Printf("refresh_cleanup.go", "executarLimpezaRefresh", "agendando defer: cancel")
	defer cancel()
	vlog.Printf("refresh_cleanup.go", "executarLimpezaRefresh", "chamando svc.CleanupExpired e declarando _, err e verificando condição err != nil")
	if _, err := svc.CleanupExpired(runCtx, db); err != nil {
		log.Printf("[refresh] cleanup: erro: %v", err)
	}
}
