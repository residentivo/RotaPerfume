// Package handlers contém handlers HTTP.
package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
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

// GetVendedor GET /api/vendedores/{id}
//
// Response: {success, data: vendedor com a lista de clientes vinculados
// (carteira ativa), error}
// Acesso comum.
func (h *VendedorHandler) GetVendedor(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vendedor, err := h.svc.GetVendedorDetalhe(r.Context(), h.db, id)
	if err != nil {
		if errors.Is(err, services.ErrVendedorNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "vendedor não encontrado")
			return
		}
		log.Printf("[vendedores] GetVendedor: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, vendedor, "")
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// CreateVendedorRequest body do POST /api/vendedores.
type CreateVendedorRequest struct {
	Nome         string  `json:"nome"`
	Regiao       string  `json:"regiao"`
	UF           string  `json:"uf"`
	DataAdmissao string  `json:"data_admissao"` // opcional, formato AAAA-MM-DD; vazio = hoje
	MetaMensal   float64 `json:"meta_mensal"`
}

// UpdateVendedorRequest body do PUT /api/vendedores/{id}.
type UpdateVendedorRequest struct {
	Nome         string  `json:"nome"`
	Regiao       string  `json:"regiao"`
	UF           string  `json:"uf"`
	DataAdmissao string  `json:"data_admissao"` // formato AAAA-MM-DD
	MetaMensal   float64 `json:"meta_mensal"`
}

// vendedorErroParaStatus mapeia erros de validação/negócio do VendedorService
// para o status HTTP e mensagem apropriados. Retorna ok=false se o erro não
// for reconhecido (cabe ao chamador tratar como erro interno).
func vendedorErroParaStatus(err error) (status int, msg string, ok bool) {
	switch {
	case errors.Is(err, services.ErrVendedorNaoEncontrado):
		return http.StatusNotFound, "vendedor não encontrado", true
	case errors.Is(err, services.ErrVendedorNomeObrigatorio):
		return http.StatusBadRequest, "nome é obrigatório", true
	case errors.Is(err, services.ErrVendedorRegiaoObrigatoria):
		return http.StatusBadRequest, "regiao é obrigatória", true
	case errors.Is(err, services.ErrVendedorUFInvalida):
		return http.StatusBadRequest, "uf deve ter 2 letras", true
	case errors.Is(err, services.ErrVendedorDataAdmissaoInvalida):
		return http.StatusBadRequest, "data_admissao inválida (use o formato AAAA-MM-DD)", true
	case errors.Is(err, services.ErrVendedorMetaMensalInvalida):
		return http.StatusBadRequest, "meta_mensal deve ser maior ou igual a zero", true
	case errors.Is(err, services.ErrClienteNaoEncontrado):
		return http.StatusNotFound, "cliente não encontrado", true
	case errors.Is(err, services.ErrVinculoNaoEncontrado):
		return http.StatusNotFound, "vínculo entre cliente e vendedor não encontrado", true
	default:
		return 0, "", false
	}
}

// CreateVendedor POST /api/vendedores
//
// Body: { "nome": string, "regiao": string, "uf": string, "data_admissao": "AAAA-MM-DD" (opcional, default hoje), "meta_mensal": number }
// Retorna: 201 com o vendedor criado.
// Acesso comum.
func (h *VendedorHandler) CreateVendedor(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.GetRole(r.Context())

	var req CreateVendedorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	input := services.VendedorInput{
		Nome:         req.Nome,
		Regiao:       req.Regiao,
		UF:           req.UF,
		DataAdmissao: req.DataAdmissao,
		MetaMensal:   req.MetaMensal,
	}

	vendedor, err := h.svc.CreateVendedor(r.Context(), h.db, input)
	if err != nil {
		if status, msg, ok := vendedorErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[vendedores] CreateVendedor: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	log.Printf("[vendedores] criado: id=%d por usuario role=%s", vendedor.ID, role)
	writeJSON(w, http.StatusCreated, vendedor, "")
}

// UpdateVendedor PUT /api/vendedores/{id}
//
// Body: { "nome": string, "regiao": string, "uf": string, "data_admissao": "AAAA-MM-DD", "meta_mensal": number }
// data_desligamento não é editável por esta rota (ver DeleteVendedor).
// Retorna: 200 com o vendedor atualizado, 404 se não existir, 400 se o payload for inválido.
// Acesso comum.
func (h *VendedorHandler) UpdateVendedor(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	var req UpdateVendedorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	input := services.VendedorInput{
		Nome:         req.Nome,
		Regiao:       req.Regiao,
		UF:           req.UF,
		DataAdmissao: req.DataAdmissao,
		MetaMensal:   req.MetaMensal,
	}

	vendedor, err := h.svc.UpdateVendedor(r.Context(), h.db, id, input)
	if err != nil {
		if status, msg, ok := vendedorErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[vendedores] UpdateVendedor: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	role, _ := middleware.GetRole(r.Context())
	log.Printf("[vendedores] atualizado: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, vendedor, "")
}

// DeleteVendedor DELETE /api/vendedores/{id}
//
// Soft-delete: define data_desligamento = hoje, preservando o histórico de
// carteiras/pedidos vinculados ao vendedor.
// Retorna: 200 com o vendedor atualizado, 404 se não existir.
// Acesso comum.
func (h *VendedorHandler) DeleteVendedor(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vendedor, err := h.svc.DeleteVendedor(r.Context(), h.db, id)
	if err != nil {
		if status, msg, ok := vendedorErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[vendedores] DeleteVendedor: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	role, _ := middleware.GetRole(r.Context())
	log.Printf("[vendedores] inativado: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, vendedor, "")
}

// VincularClienteRequest body do POST /api/vendedores/{id}/clientes.
type VincularClienteRequest struct {
	ClienteID int64 `json:"cliente_id"`
}

// VincularCliente POST /api/vendedores/{id}/clientes
//
// Inclui um cliente na carteira ativa do vendedor. Se o cliente já estiver
// vinculado a outro vendedor, o vínculo anterior é encerrado automaticamente
// e a carteira é transferida para o vendedor informado.
// Body: { "cliente_id": number }
// Retorna: 201 com o ClienteResumo do cliente vinculado, 404 se vendedor ou
// cliente não existirem, 400 se o payload for inválido.
// Acesso comum.
func (h *VendedorHandler) VincularCliente(w http.ResponseWriter, r *http.Request) {
	vendedorID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	var req VincularClienteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}
	if req.ClienteID <= 0 {
		writeJSON(w, http.StatusBadRequest, nil, "cliente_id é obrigatório")
		return
	}

	clienteResumo, err := h.svc.VincularCliente(r.Context(), h.db, vendedorID, req.ClienteID)
	if err != nil {
		if status, msg, ok := vendedorErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[vendedores] VincularCliente: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	role, _ := middleware.GetRole(r.Context())
	log.Printf("[vendedores] cliente vinculado: vendedor_id=%d cliente_id=%d por usuario role=%s", vendedorID, req.ClienteID, role)
	writeJSON(w, http.StatusCreated, clienteResumo, "")
}

// DesvincularCliente DELETE /api/vendedores/{id}/clientes/{clienteId}
//
// Encerra o vínculo ativo entre o vendedor e o cliente informados (não afeta
// vínculos do cliente com outros vendedores).
// Retorna: 200 se encerrado com sucesso, 404 se não houver vínculo ativo
// entre os dois.
// Acesso comum.
func (h *VendedorHandler) DesvincularCliente(w http.ResponseWriter, r *http.Request) {
	vendedorID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}
	clienteID, err := strconv.ParseInt(r.PathValue("clienteId"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "cliente id inválido")
		return
	}

	if err := h.svc.DesvincularCliente(r.Context(), h.db, vendedorID, clienteID); err != nil {
		if status, msg, ok := vendedorErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[vendedores] DesvincularCliente: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	role, _ := middleware.GetRole(r.Context())
	log.Printf("[vendedores] cliente desvinculado: vendedor_id=%d cliente_id=%d por usuario role=%s", vendedorID, clienteID, role)
	writeJSON(w, http.StatusOK, nil, "")
}
