// Package handlers contém handlers HTTP.
package handlers

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
)

// VendedorHandler trata as rotas /api/vendedores/*.
type VendedorHandler struct {
	db  *sql.DB
	svc *services.VendedorService
}

// NewVendedorHandler cria um VendedorHandler com pool de conexão injetado.
func NewVendedorHandler(db *sql.DB, cfg *config.Config) *VendedorHandler {
	return &VendedorHandler{
		db:  db,
		svc: services.NewVendedorService(db, cfg),
	}
}

// ListVendedores GET /api/vendedores
//
// Sem paginação: usado para popular listas de seleção (ex: combobox no
// admin de usuários). Retorna apenas vendedores ativos.
// Response: {success, data: [{id, nome, regiao, uf}], error}
// Acesso comum.
func (h *VendedorHandler) ListVendedores(w http.ResponseWriter, r *http.Request) {
	vendedores, err := h.svc.ListVendedores(r.Context(), h.db)
	if err != nil {
		log.Printf("[vendedores] ListVendedores: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, vendedores, "")
}
