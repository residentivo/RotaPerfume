// Package handlers contém handlers HTTP.
package handlers

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/shared/kernel"
)

// ARQ-01: os helpers de resposta do legado delegam ao httpx (regra única do
// envelope). Somem quando o último handler legado for migrado (ARQ-07).

// parseOrderDirQuery lê o query param "order_dir" ("asc"/"desc",
// case-insensitive). Qualquer outro valor (incluindo ausente/vazio) é
// tratado como "sem valor" (string vazia), deixando o default de cada
// entidade prevalecer no repositório.
func parseOrderDirQuery(v string) string {
	if d, ok := kernel.ParseDirecao(v); ok {
		return d.String()
	}
	return ""
}

// writeJSON escreve uma resposta JSON padronizada.
func writeJSON(w http.ResponseWriter, status int, data any, errMsg string) {
	httpx.JSON(w, status, data, errMsg)
}

// writeNoContent escreve uma resposta 204 sem corpo (DELETE bem-sucedido).
func writeNoContent(w http.ResponseWriter) {
	httpx.SemConteudo(w)
}

// writeJSONWithPagination é igual a writeJSON + bloco pagination.
func writeJSONWithPagination(w http.ResponseWriter, status int, data any, page, limit, total, pages int) {
	httpx.JSONComPaginacao(w, status, data, httpx.Paginacao{Limit: limit, Page: page, Pages: pages, Total: total})
}
