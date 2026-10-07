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
	"github.com/rotaperfumes/shared/vlog"
)

// VisitaHandler trata as rotas /api/visitas/*.
type VisitaHandler struct {
	db  *sql.DB
	svc *services.VisitaService
}

// NewVisitaHandler cria um VisitaHandler com pool de conexão injetado.
func NewVisitaHandler(db *sql.DB, cfg *config.Config) *VisitaHandler {
	return &VisitaHandler{
		db:  db,
		svc: services.NewVisitaService(db, cfg),
	}
}

// ListVisitas GET /api/visitas
//
// Query params (opcionais): page (default 1), limit (default 20, max 100),
// cliente_id, vendedor_id, resultado, data_visita_de, data_visita_ate
// (formato AAAA-MM-DD), q (busca em resultado), order_by
// (id|visita_id|data_visita|duracao_min|resultado|created_at|updated_at;
// default id), order_dir (asc|desc; default asc).
// Response: {success, data: [visita...], error, pagination: {page, limit, total, pages}}
// Acesso comum: usuário role=normal só enxerga visitas da própria carteira
// (vendedor_id da query é ignorado e forçado ao vendedor vinculado).
func (h *VisitaHandler) ListVisitas(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("visita_handler.go", "VisitaHandler.ListVisitas", "chamando services.ParsePagination e atribuindo resultado a page, limit")
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	vlog.Printf("visita_handler.go", "VisitaHandler.ListVisitas", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("visita_handler.go", "VisitaHandler.ListVisitas", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[visitas] ListVisitas", err)
		return
	}
	vlog.Printf("visita_handler.go", "VisitaHandler.ListVisitas", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSONWithPagination(w, http.StatusOK, []any{}, page, limit, 0, 0)
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.ListVisitas", "montando services.VisitaFiltro em filtro")
	filtro := services.VisitaFiltro{
		ClienteID:     parseInt64Query(r.URL.Query().Get("cliente_id")),
		VendedorID:    parseInt64Query(r.URL.Query().Get("vendedor_id")),
		Resultado:     strings.TrimSpace(r.URL.Query().Get("resultado")),
		DataVisitaDe:  strings.TrimSpace(r.URL.Query().Get("data_visita_de")),
		DataVisitaAte: strings.TrimSpace(r.URL.Query().Get("data_visita_ate")),
		Q:             strings.TrimSpace(r.URL.Query().Get("q")),
		OrderBy:       strings.TrimSpace(r.URL.Query().Get("order_by")),
		OrderDir:      parseOrderDirQuery(r.URL.Query().Get("order_dir")),
	}
	vlog.Printf("visita_handler.go", "VisitaHandler.ListVisitas", "verificando se scope.Restrito")
	if scope.Restrito {
		vlog.Printf("visita_handler.go", "VisitaHandler.ListVisitas", "atribuindo filtro.VendedorID = scope.VendedorID")
		// Usuário role=normal: força o filtro à própria carteira, ignorando
		// qualquer vendedor_id vindo da query string (evita bypass via URL).
		filtro.VendedorID = scope.VendedorID
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.ListVisitas", "chamando h.svc.ListVisitas e atribuindo resultado a visitas, total, err")
	visitas, total, err := h.svc.ListVisitas(r.Context(), h.db, page, limit, filtro)
	vlog.Printf("visita_handler.go", "VisitaHandler.ListVisitas", "verificando se err != nil")
	if err != nil {
		log.Printf("[visitas] ListVisitas: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.ListVisitas", "definindo pages := total / limit")
	pages := total / limit
	vlog.Printf("visita_handler.go", "VisitaHandler.ListVisitas", "verificando se total%%limit != 0")
	if total%limit != 0 {
		vlog.Printf("visita_handler.go", "VisitaHandler.ListVisitas", "incrementando pages")
		pages++
	}

	writeJSONWithPagination(w, http.StatusOK, visitas, page, limit, total, pages)
}

// GetVisita GET /api/visitas/{id}
//
// Response: {success, data: visita, error}
// Acesso comum: usuário role=normal só enxerga visita da própria carteira
// (404 — não 403 — se pertencer a outro vendedor, para não permitir
// enumeração de IDs).
func (h *VisitaHandler) GetVisita(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("visita_handler.go", "VisitaHandler.GetVisita", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("visita_handler.go", "VisitaHandler.GetVisita", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.GetVisita", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("visita_handler.go", "VisitaHandler.GetVisita", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[visitas] GetVisita", err)
		return
	}
	vlog.Printf("visita_handler.go", "VisitaHandler.GetVisita", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.GetVisita", "chamando h.svc.GetVisitaByID e atribuindo resultado a visita, err")
	visita, err := h.svc.GetVisitaByID(r.Context(), h.db, id)
	vlog.Printf("visita_handler.go", "VisitaHandler.GetVisita", "verificando se err != nil")
	if err != nil {
		vlog.Printf("visita_handler.go", "VisitaHandler.GetVisita", "verificando se errors.Is(...)")
		if errors.Is(err, services.ErrVisitaNaoEncontrada) {
			writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
			return
		}
		log.Printf("[visitas] GetVisita: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.GetVisita", "verificando se scope.Restrito && !scope.PermiteVendedor(...)")
	if scope.Restrito && !scope.PermiteVendedor(visita.VendedorID) {
		writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
		return
	}

	writeJSON(w, http.StatusOK, visita, "")
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// CreateVisitaRequest body do POST /api/visitas.
type CreateVisitaRequest struct {
	ClienteID  int64  `json:"cliente_id"`
	VendedorID int64  `json:"vendedor_id"`
	DataVisita string `json:"data_visita"` // formato AAAA-MM-DD
	Resultado  string `json:"resultado"`
	DuracaoMin int    `json:"duracao_min"`
}

// UpdateVisitaRequest body do PUT /api/visitas/{id}.
type UpdateVisitaRequest struct {
	ClienteID  int64  `json:"cliente_id"`
	VendedorID int64  `json:"vendedor_id"`
	DataVisita string `json:"data_visita"` // formato AAAA-MM-DD
	Resultado  string `json:"resultado"`
	DuracaoMin int    `json:"duracao_min"`
}

// visitaErroParaStatus mapeia erros de validação/negócio do VisitaService
// para o status HTTP e mensagem apropriados. Retorna ok=false se o erro não
// for reconhecido (cabe ao chamador tratar como erro interno).
func visitaErroParaStatus(err error) (status int, msg string, ok bool) {
	vlog.Printf("visita_handler.go", "visitaErroParaStatus", "avaliando switch de condições")
	switch {
	case errors.Is(err, services.ErrVisitaNaoEncontrada):
		return http.StatusNotFound, "visita não encontrada", true
	case errors.Is(err, services.ErrVisitaClienteInvalido):
		return http.StatusBadRequest, "cliente_id é obrigatório e deve existir", true
	case errors.Is(err, services.ErrVisitaVendedorInvalido):
		return http.StatusBadRequest, "vendedor_id é obrigatório e deve existir", true
	case errors.Is(err, services.ErrVisitaDataInvalida):
		return http.StatusBadRequest, "data_visita inválida (use o formato AAAA-MM-DD)", true
	case errors.Is(err, services.ErrVisitaResultadoInvalido):
		return http.StatusBadRequest, "resultado é obrigatório", true
	case errors.Is(err, services.ErrVisitaDuracaoInvalida):
		return http.StatusBadRequest, "duracao_min deve ser maior ou igual a zero", true
	default:
		return 0, "", false
	}
}

// CreateVisita POST /api/visitas
//
// Body: { "cliente_id": number, "vendedor_id": number,
// "data_visita": "AAAA-MM-DD", "resultado": string, "duracao_min": number }
// Retorna: 201 com a visita criada.
// Acesso comum: usuário role=normal só pode criar visita para a própria
// carteira (vendedor_id do payload é ignorado e forçado ao vendedor
// vinculado; cliente_id deve pertencer à carteira ativa desse vendedor).
func (h *VisitaHandler) CreateVisita(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())

	vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[visitas] CreateVisita", err)
		return
	}
	vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusForbidden, nil, "usuário sem vendedor vinculado")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "declarando variável req")
	var req CreateVisitaRequest
	vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "chamando json.NewDecoder(...).Decode e atribuindo resultado a err e verificando se err != nil")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "verificando se scope.Restrito")
	if scope.Restrito {
		vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "atribuindo req.VendedorID = scope.VendedorID")
		// Usuário role=normal: nunca confia no vendedor_id do payload — força
		// à própria carteira (evita forjar registro para outro vendedor).
		req.VendedorID = scope.VendedorID

		vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "chamando clienteNaCarteiraDoVendedor e atribuindo resultado a pertence, err")
		pertence, err := clienteNaCarteiraDoVendedor(r.Context(), h.db, scope.VendedorID, req.ClienteID)
		vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "verificando se err != nil")
		if err != nil {
			log.Printf("[visitas] CreateVisita checar carteira: %v", err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return
		}
		vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "verificando se !pertence")
		if !pertence {
			writeJSON(w, http.StatusBadRequest, nil, "cliente não pertence à carteira deste vendedor")
			return
		}
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "montando services.VisitaInput em input")
	input := services.VisitaInput{
		ClienteID:  req.ClienteID,
		VendedorID: req.VendedorID,
		DataVisita: req.DataVisita,
		Resultado:  req.Resultado,
		DuracaoMin: req.DuracaoMin,
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "chamando h.svc.CreateVisita e atribuindo resultado a visita, err")
	visita, err := h.svc.CreateVisita(r.Context(), h.db, input)
	vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "verificando se err != nil")
	if err != nil {
		vlog.Printf("visita_handler.go", "VisitaHandler.CreateVisita", "chamando visitaErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := visitaErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[visitas] CreateVisita: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	log.Printf("[visitas] criada: id=%d por usuario role=%s", visita.VisitaID, role)
	writeJSON(w, http.StatusCreated, visita, "")
}

// UpdateVisita PUT /api/visitas/{id}
//
// Body: igual ao de CreateVisita.
// Retorna: 200 com a visita atualizada, 404 se não existir, 400 se o
// payload for inválido.
// Acesso comum: usuário role=normal só pode atualizar visita da própria
// carteira (404 se pertencer a outro vendedor) e não pode reatribuir
// vendedor_id para outro vendedor.
func (h *VisitaHandler) UpdateVisita(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[visitas] UpdateVisita", err)
		return
	}
	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "chamando h.svc.GetVisitaByID e atribuindo resultado a atual, err")
	atual, err := h.svc.GetVisitaByID(r.Context(), h.db, id)
	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "verificando se err != nil")
	if err != nil {
		vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "verificando se errors.Is(...)")
		if errors.Is(err, services.ErrVisitaNaoEncontrada) {
			writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
			return
		}
		log.Printf("[visitas] UpdateVisita buscar atual: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "verificando se scope.Restrito && !scope.PermiteVendedor(...)")
	if scope.Restrito && !scope.PermiteVendedor(atual.VendedorID) {
		writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "declarando variável req")
	var req UpdateVisitaRequest
	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "chamando json.NewDecoder(...).Decode e atribuindo resultado a err e verificando se err != nil")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "verificando se scope.Restrito")
	if scope.Restrito {
		vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "atribuindo req.VendedorID = scope.VendedorID")
		// Usuário role=normal: nunca confia no vendedor_id do payload — força
		// à própria carteira (evita reatribuir o registro a outro vendedor).
		req.VendedorID = scope.VendedorID

		vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "chamando clienteNaCarteiraDoVendedor e atribuindo resultado a pertence, err")
		pertence, err := clienteNaCarteiraDoVendedor(r.Context(), h.db, scope.VendedorID, req.ClienteID)
		vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "verificando se err != nil")
		if err != nil {
			log.Printf("[visitas] UpdateVisita checar carteira: %v", err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return
		}
		vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "verificando se !pertence")
		if !pertence {
			writeJSON(w, http.StatusBadRequest, nil, "cliente não pertence à carteira deste vendedor")
			return
		}
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "montando services.VisitaInput em input")
	input := services.VisitaInput{
		ClienteID:  req.ClienteID,
		VendedorID: req.VendedorID,
		DataVisita: req.DataVisita,
		Resultado:  req.Resultado,
		DuracaoMin: req.DuracaoMin,
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "chamando h.svc.UpdateVisita e atribuindo resultado a visita, err")
	visita, err := h.svc.UpdateVisita(r.Context(), h.db, id, input)
	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "verificando se err != nil")
	if err != nil {
		vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "chamando visitaErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := visitaErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[visitas] UpdateVisita: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.UpdateVisita", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[visitas] atualizada: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, visita, "")
}

// DeleteVisita DELETE /api/visitas/{id}
//
// Hard delete: remove a visita definitivamente (não há soft-delete para
// visitas).
// Retorna: 204 sem corpo, 404 se não existir (ou pertencer a outro
// vendedor).
// Acesso comum: usuário role=normal só pode excluir visita da própria
// carteira (404 — não 403 — se pertencer a outro vendedor).
func (h *VisitaHandler) DeleteVisita(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("visita_handler.go", "VisitaHandler.DeleteVisita", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("visita_handler.go", "VisitaHandler.DeleteVisita", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.DeleteVisita", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("visita_handler.go", "VisitaHandler.DeleteVisita", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[visitas] DeleteVisita", err)
		return
	}
	vlog.Printf("visita_handler.go", "VisitaHandler.DeleteVisita", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.DeleteVisita", "chamando h.svc.GetVisitaByID e atribuindo resultado a atual, err")
	atual, err := h.svc.GetVisitaByID(r.Context(), h.db, id)
	vlog.Printf("visita_handler.go", "VisitaHandler.DeleteVisita", "verificando se err != nil")
	if err != nil {
		vlog.Printf("visita_handler.go", "VisitaHandler.DeleteVisita", "verificando se errors.Is(...)")
		if errors.Is(err, services.ErrVisitaNaoEncontrada) {
			writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
			return
		}
		log.Printf("[visitas] DeleteVisita buscar atual: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	vlog.Printf("visita_handler.go", "VisitaHandler.DeleteVisita", "verificando se scope.Restrito && !scope.PermiteVendedor(...)")
	if scope.Restrito && !scope.PermiteVendedor(atual.VendedorID) {
		writeJSON(w, http.StatusNotFound, nil, "visita não encontrada")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.DeleteVisita", "chamando h.svc.DeleteVisita e atribuindo resultado a err e verificando se err != nil")
	if err := h.svc.DeleteVisita(r.Context(), h.db, id); err != nil {
		vlog.Printf("visita_handler.go", "VisitaHandler.DeleteVisita", "chamando visitaErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := visitaErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[visitas] DeleteVisita: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("visita_handler.go", "VisitaHandler.DeleteVisita", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[visitas] excluída: id=%d por usuario role=%s", id, role)
	writeNoContent(w)
}
