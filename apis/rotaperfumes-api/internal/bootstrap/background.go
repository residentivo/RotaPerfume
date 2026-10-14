package bootstrap

import (
	"context"
	"log"
	"time"
)

// pararTarefasDeFundo cancela as tarefas em segundo plano e espera o término
// por até timeout, para não fechar o pool no meio de um DELETE (I-10).
func pararTarefasDeFundo(cancel context.CancelFunc, done <-chan struct{}, timeout time.Duration) {
	cancel()
	select {
	case <-done:
		log.Printf("[server] tarefas de fundo encerradas")
	case <-time.After(timeout):
		log.Printf("[server] tarefas de fundo não encerraram em %s — seguindo com o shutdown", timeout)
	}
}
