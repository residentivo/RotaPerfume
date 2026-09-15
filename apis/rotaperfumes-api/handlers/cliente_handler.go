// Package handlers contém handlers HTTP.
package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
)

// ClienteHandler trata as rotas /api/clientes/*.
type ClienteHandler struct {
	db  *sql.DB
	svc *services.ClienteService
}

// NewClienteHandler cria um ClienteHandler com pool de conexão injetado.
func NewClienteHandler(db *sql.DB, cfg *config.Config) *ClienteHandler {
	return &ClienteHandler{
		db:  db,
		svc: services.NewClienteService(db, cfg),
	}
}

// parseAtivoQuery lê o query param "ativo" ("true"/"false"). Qualquer outro
// valor (incluindo ausente/vazio) é tratado como "sem filtro" (nil).
func parseAtivoQuery(v string) *bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true":
		b := true
		return &b
	case "false":
		b := false
		return &b
	default:
		return nil
	}
}

// ListClientes GET /api/clientes
//
// Query params (opcionais): page (default 1), limit (default 20, max 100),
// uf, segmento, ativo (true|false), q (busca em razao_social OU cnpj),
// order_by (id|razao_social|cnpj|segmento|cidade|uf|data_cadastro|ativo|
// created_at|updated_at; default id), order_dir (asc|desc; default asc).
// Response: {success, data: [cliente...], error, pagination: {page, limit, total, pages}}
// Acesso comum.
func (h *ClienteHandler) ListClientes(w http.ResponseWriter, r *http.Request) {
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	filtro := services.ClienteFiltro{
		UF:       strings.TrimSpace(r.URL.Query().Get("uf")),
		Segmento: strings.TrimSpace(r.URL.Query().Get("segmento")),
		Ativo:    parseAtivoQuery(r.URL.Query().Get("ativo")),
		Q:        strings.TrimSpace(r.URL.Query().Get("q")),
		OrderBy:  strings.TrimSpace(r.URL.Query().Get("order_by")),
		OrderDir: parseOrderDirQuery(r.URL.Query().Get("order_dir")),
	}

	clientes, total, err := h.svc.ListClientes(r.Context(), h.db, page, limit, filtro)
	if err != nil {
		log.Printf("[clientes] ListClientes: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}

	writeJSONWithPagination(w, http.StatusOK, clientes, page, limit, total, pages)
}

// GetCliente GET /api/clientes/{id}
//
// Response: {success, data: cliente, error}
// Acesso comum.
func (h *ClienteHandler) GetCliente(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	cliente, err := h.svc.GetClienteByID(r.Context(), h.db, id)
	if err != nil {
		if errors.Is(err, services.ErrClienteNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "cliente não encontrado")
			return
		}
		log.Printf("[clientes] GetCliente: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, cliente, "")
}

// ToggleAtivoClienteRequest body do PATCH /api/clientes/{id}/inativar.
type ToggleAtivoClienteRequest struct {
	Ativo *bool `json:"ativo"` // omitido = toggle
}

// ToggleAtivoCliente PATCH /api/clientes/{id}/inativar
//
// Body opcional: { "ativo": bool } — omitido = toggle
// Response: {success, data: cliente atualizado, error}
// Acesso comum.
func (h *ClienteHandler) ToggleAtivoCliente(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	var req ToggleAtivoClienteRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // body opcional, ignora erro

	cliente, err := h.svc.ToggleAtivoCliente(r.Context(), h.db, id, req.Ativo)
	if err != nil {
		if errors.Is(err, services.ErrClienteNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "cliente não encontrado")
			return
		}
		log.Printf("[clientes] ToggleAtivoCliente: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	role, _ := middleware.GetRole(r.Context())
	log.Printf("[clientes] ativo=%t: id=%d por usuario role=%s", cliente.Ativo, id, role)
	writeJSON(w, http.StatusOK, cliente, "")
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// CreateClienteRequest body do POST /api/clientes.
type CreateClienteRequest struct {
	CNPJ         string `json:"cnpj"`
	RazaoSocial  string `json:"razao_social"`
	Segmento     string `json:"segmento"`
	Cidade       string `json:"cidade"`
	UF           string `json:"uf"`
	Bairro       string `json:"bairro"`
	DataCadastro string `json:"data_cadastro"` // opcional, formato AAAA-MM-DD; vazio = hoje
}

// UpdateClienteRequest body do PUT /api/clientes/{id}.
type UpdateClienteRequest struct {
	CNPJ         string `json:"cnpj"`
	RazaoSocial  string `json:"razao_social"`
	Segmento     string `json:"segmento"`
	Cidade       string `json:"cidade"`
	UF           string `json:"uf"`
	Bairro       string `json:"bairro"`
	DataCadastro string `json:"data_cadastro"` // formato AAAA-MM-DD
}

// clienteErroParaStatus mapeia erros de validação/negócio do ClienteService
// para o status HTTP e mensagem apropriados. Retorna ok=false se o erro não
// for reconhecido (cabe ao chamador tratar como erro interno).
func clienteErroParaStatus(err error) (status int, msg string, ok bool) {
	switch {
	case errors.Is(err, services.ErrClienteNaoEncontrado):
		return http.StatusNotFound, "cliente não encontrado", true
	case errors.Is(err, services.ErrRazaoSocialObrigatoria):
		return http.StatusBadRequest, "razão social é obrigatória", true
	case errors.Is(err, services.ErrCNPJObrigatorio):
		return http.StatusBadRequest, "cnpj é obrigatório", true
	case errors.Is(err, services.ErrSegmentoObrigatorio):
		return http.StatusBadRequest, "segmento é obrigatório", true
	case errors.Is(err, services.ErrCidadeObrigatoria):
		return http.StatusBadRequest, "cidade é obrigatória", true
	case errors.Is(err, services.ErrUFInvalida):
		return http.StatusBadRequest, "uf deve ter 2 letras", true
	case errors.Is(err, services.ErrDataCadastroInvalida):
		return http.StatusBadRequest, "data_cadastro inválida (use o formato AAAA-MM-DD)", true
	default:
		return 0, "", false
	}
}

// CreateCliente POST /api/clientes
//
// Body: { "cnpj": string, "razao_social": string, "segmento": string, "cidade": string, "uf": string, "bairro": string, "data_cadastro": "AAAA-MM-DD" (opcional, default hoje) }
// cliente_id_origem é gerado automaticamente pelo sistema.
// Retorna: 201 com o cliente criado.
// Acesso comum.
func (h *ClienteHandler) CreateCliente(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.GetRole(r.Context())

	var req CreateClienteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	input := services.ClienteInput{
		CNPJ:         req.CNPJ,
		RazaoSocial:  req.RazaoSocial,
		Segmento:     req.Segmento,
		Cidade:       req.Cidade,
		UF:           req.UF,
		Bairro:       req.Bairro,
		DataCadastro: req.DataCadastro,
	}

	cliente, err := h.svc.CreateCliente(r.Context(), h.db, input)
	if err != nil {
		if status, msg, ok := clienteErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[clientes] CreateCliente: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	log.Printf("[clientes] criado: id=%d por usuario role=%s", cliente.ClienteIDOrigem, role)
	writeJSON(w, http.StatusCreated, cliente, "")
}

// UpdateCliente PUT /api/clientes/{id}
//
// Body: { "cnpj": string, "razao_social": string, "segmento": string, "cidade": string, "uf": string, "bairro": string, "data_cadastro": "AAAA-MM-DD" }
// cliente_id_origem e ativo não são editáveis por esta rota.
// Retorna: 200 com o cliente atualizado, 404 se não existir, 400 se o payload for inválido.
// Acesso comum.
func (h *ClienteHandler) UpdateCliente(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	var req UpdateClienteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	input := services.ClienteInput{
		CNPJ:         req.CNPJ,
		RazaoSocial:  req.RazaoSocial,
		Segmento:     req.Segmento,
		Cidade:       req.Cidade,
		UF:           req.UF,
		Bairro:       req.Bairro,
		DataCadastro: req.DataCadastro,
	}

	cliente, err := h.svc.UpdateCliente(r.Context(), h.db, id, input)
	if err != nil {
		if status, msg, ok := clienteErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[clientes] UpdateCliente: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	role, _ := middleware.GetRole(r.Context())
	log.Printf("[clientes] atualizado: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, cliente, "")
}
