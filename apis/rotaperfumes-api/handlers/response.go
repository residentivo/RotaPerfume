// Package handlers contém handlers HTTP.
package handlers

import (
	"encoding/json"
	"net/http"
)

// writeJSON escreve uma resposta JSON padronizada.
func writeJSON(w http.ResponseWriter, status int, data any, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]any{
		"success": errMsg == "",
	}
	if data != nil {
		resp["data"] = data
	}
	if errMsg != "" {
		resp["error"] = errMsg
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// writeJSONWithPagination é igual a writeJSON + bloco pagination.
func writeJSONWithPagination(w http.ResponseWriter, status int, data any, page, limit, total, pages int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]any{
		"success": true,
		"data":    data,
		"pagination": map[string]int{
			"page":  page,
			"limit": limit,
			"total": total,
			"pages": pages,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}
