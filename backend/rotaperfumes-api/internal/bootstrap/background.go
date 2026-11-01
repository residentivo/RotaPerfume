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

// alertador é o que o shutdown precisa do notificador de alertas.
type alertador interface {
	Aguardar(timeout time.Duration) bool
}

// aguardarAlertas espera os alertas de segurança em andamento (busca do
// usuário + envio) antes do pool.Close (NB-6), por até timeout.
func aguardarAlertas(n alertador, timeout time.Duration) {
	if n.Aguardar(timeout) {
		log.Printf("[server] alertas de segurança encerrados")
		return
	}
	log.Printf("[server] alertas de segurança não encerraram em %s — seguindo com o shutdown", timeout)
}
