package bootstrap

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
)

// iniciarLimpezaRefresh sobe a limpeza periódica de refresh_tokens
// (CHORE-02): roda já no início e a cada REFRESH_CLEANUP_INTERVAL ("0"
// desativa), com a retenção da config (I-10). O canal fecha ao terminar.
func iniciarLimpezaRefresh(ctx context.Context, pool *sql.DB, cfg *config.Config) <-chan struct{} {
	limpeza := services.NewRefreshTokenService()
	limpeza.SetRetencao(cfg.RefreshTokenRetencao)
	return services.IniciarLimpezaRefreshTokens(ctx, pool, limpeza, cfg.RefreshCleanupInterval)
}

// pararTarefasDeFundo cancela as tarefas em segundo plano e espera o término
// por até timeout, para não fechar o pool no meio de um DELETE.
func pararTarefasDeFundo(cancel context.CancelFunc, done <-chan struct{}, timeout time.Duration) {
	cancel()
	select {
	case <-done:
		log.Printf("[server] tarefas de fundo encerradas")
	case <-time.After(timeout):
		log.Printf("[server] tarefas de fundo não encerraram em %s — seguindo com o shutdown", timeout)
	}
}
