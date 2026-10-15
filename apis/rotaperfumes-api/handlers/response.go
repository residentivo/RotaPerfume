// Package handlers contém handlers HTTP.
package handlers

import (
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
)

// ARQ-01: os helpers de resposta do legado delegam ao httpx (regra única do
// envelope). Somem quando o último handler legado for migrado (ARQ-07).

// writeJSON escreve uma resposta JSON padronizada.
func writeJSON(w http.ResponseWriter, status int, data any, errMsg string) {
	httpx.JSON(w, status, data, errMsg)
}

// writeJSONWithPagination é igual a writeJSON + bloco pagination.
func writeJSONWithPagination(w http.ResponseWriter, status int, data any, page, limit, total, pages int) {
	httpx.JSONComPaginacao(w, status, data, httpx.Paginacao{Limit: limit, Page: page, Pages: pages, Total: total})
}
