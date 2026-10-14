package services

import (
	"strconv"

	"github.com/rotaperfumes/shared/kernel"
)

// ParsePagination lê ?page= e ?limit= da query string, com defaults e
// validação (ARQ-01: delega à regra única kernel.NovaPagina, F-10; valor não
// numérico conta como ausente). Usado pelos handlers legados até o ARQ-07;
// saiu de usuario_service.go no ARQ-03.
func ParsePagination(pageStr, limitStr string) (page, limit int) {
	numero, _ := strconv.Atoi(pageStr)
	limite, _ := strconv.Atoi(limitStr)
	p := kernel.NovaPagina(numero, limite)
	return p.Numero(), p.Limite()
}
