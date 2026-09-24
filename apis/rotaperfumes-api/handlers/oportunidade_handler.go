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

// OportunidadeHandler trata as rotas /api/oportunidades/*.
type OportunidadeHandler struct {
	db  *sql.DB
	svc *services.OportunidadeService
}

// NewOportunidadeHandler cria um OportunidadeHandler com pool de conexão injetado.
func NewOportunidadeHandler(db *sql.DB, cfg *config.Config) *OportunidadeHandler {
	return &OportunidadeHandler{
		db:  db,
		svc: services.NewOportunidadeService(db, cfg),
	}
}

// ListOportunidades GET /api/oportunidades
//
// Query params (opcionais): page (default 1), limit (default 20, max 100),
// cliente_id, vendedor_id, etapa, origem, data_abertura_de, data_abertura_ate
// (formato AAAA-MM-DD), q (busca em origem OU etapa), order_by
// (id|data_abertura|valor_estimado|probabilidade_pct|etapa|origem|created_at|
// updated_at; default id), order_dir (asc|desc; default asc).
// Response: {success, data: [oportunidade...], error, pagination: {page, limit, total, pages}}
// Acesso comum: usuário role=normal só enxerga oportunidades da própria
// carteira (vendedor_id da query é ignorado e forçado ao vendedor vinculado).
func (h *OportunidadeHandler) ListOportunidades(w http.ResponseWriter, r *http.Request) {
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	scope, err := resolverVendedorScope(r, h.db)
	if err != nil {
		responderErroEscopo(w, "[oportunidades] ListOportunidades", err)
		return
	}
	if scope.SemAcesso() {
		writeJSONWithPagination(w, http.StatusOK, []any{}, page, limit, 0, 0)
		return
	}

	filtro := services.OportunidadeFiltro{
		ClienteID:       parseInt64Query(r.URL.Query().Get("cliente_id")),
		VendedorID:      parseInt64Query(r.URL.Query().Get("vendedor_id")),
		Etapa:           strings.TrimSpace(r.URL.Query().Get("etapa")),
		Origem:          strings.TrimSpace(r.URL.Query().Get("origem")),
		DataAberturaDe:  strings.TrimSpace(r.URL.Query().Get("data_abertura_de")),
		DataAberturaAte: strings.TrimSpace(r.URL.Query().Get("data_abertura_ate")),
		Q:               strings.TrimSpace(r.URL.Query().Get("q")),
		OrderBy:         strings.TrimSpace(r.URL.Query().Get("order_by")),
		OrderDir:        parseOrderDirQuery(r.URL.Query().Get("order_dir")),
	}
	if scope.Restrito {
		// Usuário role=normal: força o filtro à própria carteira, ignorando
		// qualquer vendedor_id vindo da query string (evita bypass via URL).
		filtro.VendedorID = scope.VendedorID
	}

	oportunidades, total, err := h.svc.ListOportunidades(r.Context(), h.db, page, limit, filtro)
	if err != nil {
		log.Printf("[oportunidades] ListOportunidades: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}

	writeJSONWithPagination(w, http.StatusOK, oportunidades, page, limit, total, pages)
}

// GetOportunidade GET /api/oportunidades/{id}
//
// Response: {success, data: oportunidade, error}
// Acesso comum: usuário role=normal só enxerga oportunidade da própria
// carteira (404 — não 403 — se pertencer a outro vendedor, para não permitir
// enumeração de IDs).
func (h *OportunidadeHandler) GetOportunidade(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	scope, err := resolverVendedorScope(r, h.db)
	if err != nil {
		responderErroEscopo(w, "[oportunidades] GetOportunidade", err)
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "oportunidade não encontrada")
		return
	}

	oportunidade, err := h.svc.GetOportunidadeByID(r.Context(), h.db, id)
	if err != nil {
		if errors.Is(err, services.ErrOportunidadeNaoEncontrada) {
			writeJSON(w, http.StatusNotFound, nil, "oportunidade não encontrada")
			return
		}
		log.Printf("[oportunidades] GetOportunidade: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	if scope.Restrito && !scope.PermiteVendedor(oportunidade.VendedorID) {
		writeJSON(w, http.StatusNotFound, nil, "oportunidade não encontrada")
		return
	}

	writeJSON(w, http.StatusOK, oportunidade, "")
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// CreateOportunidadeRequest body do POST /api/oportunidades.
type CreateOportunidadeRequest struct {
	ClienteID        int64   `json:"cliente_id"`
	VendedorID       int64   `json:"vendedor_id"`
	Origem           string  `json:"origem"`
	DataAbertura     string  `json:"data_abertura"` // opcional, formato AAAA-MM-DD; vazio = hoje
	Etapa            string  `json:"etapa"`
	ProbabilidadePct float64 `json:"probabilidade_pct"`
	ValorEstimado    float64 `json:"valor_estimado"`
	DataFechamento   string  `json:"data_fechamento"` // opcional, formato AAAA-MM-DD
	CicloDias        *int    `json:"ciclo_dias"`      // opcional
	MotivoPerda      string  `json:"motivo_perda"`    // obrigatório se etapa = "Fechado perdido"
}

// UpdateOportunidadeRequest body do PUT /api/oportunidades/{id}.
type UpdateOportunidadeRequest struct {
	ClienteID        int64   `json:"cliente_id"`
	VendedorID       int64   `json:"vendedor_id"`
	Origem           string  `json:"origem"`
	DataAbertura     string  `json:"data_abertura"` // formato AAAA-MM-DD
	Etapa            string  `json:"etapa"`
	ProbabilidadePct float64 `json:"probabilidade_pct"`
	ValorEstimado    float64 `json:"valor_estimado"`
	DataFechamento   string  `json:"data_fechamento"` // opcional, formato AAAA-MM-DD
	CicloDias        *int    `json:"ciclo_dias"`      // opcional
	MotivoPerda      string  `json:"motivo_perda"`    // obrigatório se etapa = "Fechado perdido"
}

// oportunidadeErroParaStatus mapeia erros de validação/negócio do
// OportunidadeService para o status HTTP e mensagem apropriados. Retorna
// ok=false se o erro não for reconhecido (cabe ao chamador tratar como erro
// interno).
func oportunidadeErroParaStatus(err error) (status int, msg string, ok bool) {
	switch {
	case errors.Is(err, services.ErrOportunidadeNaoEncontrada):
		return http.StatusNotFound, "oportunidade não encontrada", true
	case errors.Is(err, services.ErrOportunidadeOrigemInvalida):
		return http.StatusBadRequest, "origem é obrigatória", true
	case errors.Is(err, services.ErrOportunidadeEtapaInvalida):
		return http.StatusBadRequest, "etapa é obrigatória", true
	case errors.Is(err, services.ErrOportunidadeClienteInvalido):
		return http.StatusBadRequest, "cliente_id é obrigatório e deve existir", true
	case errors.Is(err, services.ErrOportunidadeVendedorInvalido):
		return http.StatusBadRequest, "vendedor_id é obrigatório e deve existir", true
	case errors.Is(err, services.ErrOportunidadeProbabilidadeInvalida):
		return http.StatusBadRequest, "probabilidade_pct deve estar entre 0 e 100", true
	case errors.Is(err, services.ErrOportunidadeValorEstimadoInvalido):
		return http.StatusBadRequest, "valor_estimado deve ser maior ou igual a zero", true
	case errors.Is(err, services.ErrOportunidadeDataAberturaInvalida):
		return http.StatusBadRequest, "data_abertura inválida (use o formato AAAA-MM-DD)", true
	case errors.Is(err, services.ErrOportunidadeDataFechamentoInvalida):
		return http.StatusBadRequest, "data_fechamento inválida (use o formato AAAA-MM-DD)", true
	case errors.Is(err, services.ErrOportunidadeMotivoPerdaObrigatorio):
		return http.StatusBadRequest, "motivo_perda é obrigatório quando etapa = \"Fechado perdido\"", true
	default:
		return 0, "", false
	}
}

// CreateOportunidade POST /api/oportunidades
//
// Body: { "cliente_id": number, "vendedor_id": number, "origem": string,
// "data_abertura": "AAAA-MM-DD" (opcional, default hoje), "etapa": string,
// "probabilidade_pct": number, "valor_estimado": number,
// "data_fechamento": "AAAA-MM-DD" (opcional), "ciclo_dias": number (opcional),
// "motivo_perda": string (obrigatório se etapa = "Fechado perdido") }
// Retorna: 201 com a oportunidade criada.
// Acesso comum: usuário role=normal só pode criar oportunidade para a
// própria carteira (vendedor_id do payload é ignorado e forçado ao vendedor
// vinculado; cliente_id deve pertencer à carteira ativa desse vendedor).
func (h *OportunidadeHandler) CreateOportunidade(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.GetRole(r.Context())

	scope, err := resolverVendedorScope(r, h.db)
	if err != nil {
		responderErroEscopo(w, "[oportunidades] CreateOportunidade", err)
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusForbidden, nil, "usuário sem vendedor vinculado")
		return
	}

	var req CreateOportunidadeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	if scope.Restrito {
		// Usuário role=normal: nunca confia no vendedor_id do payload — força
		// à própria carteira (evita forjar registro para outro vendedor).
		req.VendedorID = scope.VendedorID

		pertence, err := clienteNaCarteiraDoVendedor(r.Context(), h.db, scope.VendedorID, req.ClienteID)
		if err != nil {
			log.Printf("[oportunidades] CreateOportunidade checar carteira: %v", err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return
		}
		if !pertence {
			writeJSON(w, http.StatusBadRequest, nil, "cliente não pertence à carteira deste vendedor")
			return
		}
	}

	input := services.OportunidadeInput{
		ClienteID:        req.ClienteID,
		VendedorID:       req.VendedorID,
		Origem:           req.Origem,
		DataAbertura:     req.DataAbertura,
		Etapa:            req.Etapa,
		ProbabilidadePct: req.ProbabilidadePct,
		ValorEstimado:    req.ValorEstimado,
		DataFechamento:   req.DataFechamento,
		CicloDias:        req.CicloDias,
		MotivoPerda:      req.MotivoPerda,
	}

	oportunidade, err := h.svc.CreateOportunidade(r.Context(), h.db, input)
	if err != nil {
		if status, msg, ok := oportunidadeErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[oportunidades] CreateOportunidade: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	log.Printf("[oportunidades] criada: id=%d por usuario role=%s", oportunidade.OportunidadeID, role)
	writeJSON(w, http.StatusCreated, oportunidade, "")
}

// UpdateOportunidade PUT /api/oportunidades/{id}
//
// Body: igual ao de CreateOportunidade (data_abertura obrigatório na edição).
// Retorna: 200 com a oportunidade atualizada, 404 se não existir, 400 se o
// payload for inválido.
// Acesso comum: usuário role=normal só pode atualizar oportunidade da
// própria carteira (404 se pertencer a outro vendedor) e não pode reatribuir
// vendedor_id para outro vendedor.
func (h *OportunidadeHandler) UpdateOportunidade(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	scope, err := resolverVendedorScope(r, h.db)
	if err != nil {
		responderErroEscopo(w, "[oportunidades] UpdateOportunidade", err)
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "oportunidade não encontrada")
		return
	}

	atual, err := h.svc.GetOportunidadeByID(r.Context(), h.db, id)
	if err != nil {
		if errors.Is(err, services.ErrOportunidadeNaoEncontrada) {
			writeJSON(w, http.StatusNotFound, nil, "oportunidade não encontrada")
			return
		}
		log.Printf("[oportunidades] UpdateOportunidade buscar atual: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.Restrito && !scope.PermiteVendedor(atual.VendedorID) {
		writeJSON(w, http.StatusNotFound, nil, "oportunidade não encontrada")
		return
	}

	var req UpdateOportunidadeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	if scope.Restrito {
		// Usuário role=normal: nunca confia no vendedor_id do payload — força
		// à própria carteira (evita reatribuir o registro a outro vendedor).
		req.VendedorID = scope.VendedorID

		pertence, err := clienteNaCarteiraDoVendedor(r.Context(), h.db, scope.VendedorID, req.ClienteID)
		if err != nil {
			log.Printf("[oportunidades] UpdateOportunidade checar carteira: %v", err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return
		}
		if !pertence {
			writeJSON(w, http.StatusBadRequest, nil, "cliente não pertence à carteira deste vendedor")
			return
		}
	}

	input := services.OportunidadeInput{
		ClienteID:        req.ClienteID,
		VendedorID:       req.VendedorID,
		Origem:           req.Origem,
		DataAbertura:     req.DataAbertura,
		Etapa:            req.Etapa,
		ProbabilidadePct: req.ProbabilidadePct,
		ValorEstimado:    req.ValorEstimado,
		DataFechamento:   req.DataFechamento,
		CicloDias:        req.CicloDias,
		MotivoPerda:      req.MotivoPerda,
	}

	oportunidade, err := h.svc.UpdateOportunidade(r.Context(), h.db, id, input)
	if err != nil {
		if status, msg, ok := oportunidadeErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[oportunidades] UpdateOportunidade: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	role, _ := middleware.GetRole(r.Context())
	log.Printf("[oportunidades] atualizada: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, oportunidade, "")
}

// DeleteOportunidade DELETE /api/oportunidades/{id}
//
// Hard delete: remove a oportunidade definitivamente (não há soft-delete
// para oportunidades).
// Retorna: 204 sem corpo, 404 se não existir (ou pertencer a outro
// vendedor).
// Acesso comum: usuário role=normal só pode excluir oportunidade da própria
// carteira (404 — não 403 — se pertencer a outro vendedor).
func (h *OportunidadeHandler) DeleteOportunidade(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	scope, err := resolverVendedorScope(r, h.db)
	if err != nil {
		responderErroEscopo(w, "[oportunidades] DeleteOportunidade", err)
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "oportunidade não encontrada")
		return
	}

	atual, err := h.svc.GetOportunidadeByID(r.Context(), h.db, id)
	if err != nil {
		if errors.Is(err, services.ErrOportunidadeNaoEncontrada) {
			writeJSON(w, http.StatusNotFound, nil, "oportunidade não encontrada")
			return
		}
		log.Printf("[oportunidades] DeleteOportunidade buscar atual: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.Restrito && !scope.PermiteVendedor(atual.VendedorID) {
		writeJSON(w, http.StatusNotFound, nil, "oportunidade não encontrada")
		return
	}

	if err := h.svc.DeleteOportunidade(r.Context(), h.db, id); err != nil {
		if status, msg, ok := oportunidadeErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[oportunidades] DeleteOportunidade: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	role, _ := middleware.GetRole(r.Context())
	log.Printf("[oportunidades] excluída: id=%d por usuario role=%s", id, role)
	writeNoContent(w)
}
