package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
)

// PagamentoHandler trata as rotas /api/pagamentos/*.
//
// Acesso comum: diferente das demais telas administrativas (produtos,
// clientes, pedidos), Pagamentos é liberado a qualquer usuário autenticado
// (admin ou normal). A autenticação (JWT válido, sem exigir admin) é feita
// pela cadeia de middleware registrada em routes.go
// (middleware.JWTMiddleware(cfg, true, false)); o escopo por carteira
// (role=normal só enxerga/altera pagamentos de pedidos do próprio vendedor,
// via pedidos.vendedor_id) é aplicado em cada método com
// resolverVendedorScope.
type PagamentoHandler struct {
	db  *sql.DB
	svc *services.PagamentoService
}

// NewPagamentoHandler cria um PagamentoHandler com pool de conexão injetado.
func NewPagamentoHandler(db *sql.DB, cfg *config.Config) *PagamentoHandler {
	return &PagamentoHandler{
		db:  db,
		svc: services.NewPagamentoService(db, cfg),
	}
}

// ListPagamentos GET /api/pagamentos
//
// Query params (opcionais): page (default 1), limit (default 20, max 100),
// status_pagamento, forma_pagamento, pedido_id, vencimento_de (AAAA-MM-DD),
// vencimento_ate (AAAA-MM-DD),
// order_by (pagamento_id|pedido_id|forma_pagamento|parcelas|valor|taxa_pct|
// valor_liquido|data_vencimento|data_pagamento|status_pagamento|created_at|
// updated_at; default pagamento_id), order_dir (asc|desc; default asc).
// Response: {success, data: [pagamento...], error, pagination: {page, limit, total, pages}}
// Acesso comum (qualquer usuário autenticado).
func (h *PagamentoHandler) ListPagamentos(w http.ResponseWriter, r *http.Request) {
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	scope, err := resolverVendedorScope(r.Context(), h.db)
	if err != nil {
		log.Printf("[pagamentos] ListPagamentos escopo: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.SemAcesso() {
		writeJSONWithPagination(w, http.StatusOK, []any{}, page, limit, 0, 0)
		return
	}

	var pedidoID int64
	if v := strings.TrimSpace(r.URL.Query().Get("pedido_id")); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, nil, "pedido_id inválido")
			return
		}
		pedidoID = id
	}

	filtro := services.PagamentoFiltro{
		StatusPagamento: strings.TrimSpace(r.URL.Query().Get("status_pagamento")),
		FormaPagamento:  strings.TrimSpace(r.URL.Query().Get("forma_pagamento")),
		PedidoID:        pedidoID,
		VencimentoDe:    strings.TrimSpace(r.URL.Query().Get("vencimento_de")),
		VencimentoAte:   strings.TrimSpace(r.URL.Query().Get("vencimento_ate")),
		OrderBy:         strings.TrimSpace(r.URL.Query().Get("order_by")),
		OrderDir:        parseOrderDirQuery(r.URL.Query().Get("order_dir")),
	}
	if scope.Restrito {
		// Usuário role=normal: força o filtro à própria carteira.
		filtro.VendedorID = scope.VendedorID
	}

	pagamentos, total, err := h.svc.ListPagamentos(r.Context(), h.db, page, limit, filtro)
	if err != nil {
		log.Printf("[pagamentos] ListPagamentos: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}

	writeJSONWithPagination(w, http.StatusOK, pagamentos, page, limit, total, pages)
}

// GetPagamento GET /api/pagamentos/{id}
//
// Response: {success, data: pagamento, error}
// Acesso comum (qualquer usuário autenticado).
func (h *PagamentoHandler) GetPagamento(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	scope, err := resolverVendedorScope(r.Context(), h.db)
	if err != nil {
		log.Printf("[pagamentos] GetPagamento escopo: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
		return
	}

	pagamento, err := h.svc.GetPagamentoByID(r.Context(), h.db, id)
	if err != nil {
		if errors.Is(err, services.ErrPagamentoNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
			return
		}
		log.Printf("[pagamentos] GetPagamento: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	if scope.Restrito {
		vendedorID, err := h.svc.VendedorIDDoPedido(r.Context(), h.db, pagamento.PedidoID)
		if err != nil {
			// Pedido do pagamento não encontrado é inesperado (FK garante
			// integridade), mas por segurança trata como "sem acesso" em vez
			// de vazar erro interno.
			log.Printf("[pagamentos] GetPagamento vendedor do pedido: %v", err)
			writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
			return
		}
		if !scope.PermiteVendedor(vendedorID) {
			writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
			return
		}
	}

	writeJSON(w, http.StatusOK, pagamento, "")
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// CreatePagamentoRequest body do POST /api/pagamentos.
type CreatePagamentoRequest struct {
	PedidoID        int64   `json:"pedido_id"`
	FormaPagamento  string  `json:"forma_pagamento"`
	Parcelas        uint8   `json:"parcelas"`
	Valor           float64 `json:"valor"`
	TaxaPct         float64 `json:"taxa_pct"`
	ValorLiquido    float64 `json:"valor_liquido"`
	DataVencimento  string  `json:"data_vencimento"` // formato AAAA-MM-DD, obrigatório
	DataPagamento   string  `json:"data_pagamento"`  // formato AAAA-MM-DD, opcional
	StatusPagamento string  `json:"status_pagamento"`
}

// UpdatePagamentoRequest body do PUT /api/pagamentos/{id}.
// pagamento_id e pedido_id não são editáveis por esta rota.
type UpdatePagamentoRequest struct {
	FormaPagamento  string  `json:"forma_pagamento"`
	Parcelas        uint8   `json:"parcelas"`
	Valor           float64 `json:"valor"`
	TaxaPct         float64 `json:"taxa_pct"`
	ValorLiquido    float64 `json:"valor_liquido"`
	DataVencimento  string  `json:"data_vencimento"` // formato AAAA-MM-DD, obrigatório
	DataPagamento   string  `json:"data_pagamento"`  // formato AAAA-MM-DD, opcional
	StatusPagamento string  `json:"status_pagamento"`
}

// pagamentoErroParaStatus mapeia erros de validação/negócio do
// PagamentoService para o status HTTP e mensagem apropriados. Retorna
// ok=false se o erro não for reconhecido (cabe ao chamador tratar como erro
// interno).
func pagamentoErroParaStatus(err error) (status int, msg string, ok bool) {
	switch {
	case errors.Is(err, services.ErrPagamentoNaoEncontrado):
		return http.StatusNotFound, "pagamento não encontrado", true
	case errors.Is(err, services.ErrPedidoIDObrigatorio):
		return http.StatusBadRequest, "pedido_id é obrigatório", true
	case errors.Is(err, services.ErrPedidoNaoEncontrado):
		// 404 igual ao do escopo restrito (pedidoNoEscopo): mesmo status e
		// corpo para "inexistente" e "de outro vendedor", sem enumeração.
		return http.StatusNotFound, "pedido não encontrado", true
	case errors.Is(err, services.ErrFormaPagamentoInvalida):
		return http.StatusBadRequest, "forma_pagamento inválida", true
	case errors.Is(err, services.ErrStatusPagamentoInvalido):
		return http.StatusBadRequest, "status_pagamento inválido", true
	case errors.Is(err, services.ErrParcelasInvalidas):
		return http.StatusBadRequest, "parcelas deve ser maior ou igual a 1", true
	case errors.Is(err, services.ErrValorInvalido):
		return http.StatusBadRequest, "valor deve ser maior ou igual a zero", true
	case errors.Is(err, services.ErrTaxaPctInvalida):
		return http.StatusBadRequest, "taxa_pct deve ser maior ou igual a zero", true
	case errors.Is(err, services.ErrValorLiquidoInvalido):
		return http.StatusBadRequest, "valor_liquido deve ser maior ou igual a zero", true
	case errors.Is(err, services.ErrDataVencimentoObrigatoria):
		return http.StatusBadRequest, "data_vencimento é obrigatória (use o formato AAAA-MM-DD)", true
	case errors.Is(err, services.ErrDataVencimentoInvalida):
		return http.StatusBadRequest, "data_vencimento inválida (use o formato AAAA-MM-DD)", true
	case errors.Is(err, services.ErrDataPagamentoInvalida):
		return http.StatusBadRequest, "data_pagamento inválida (use o formato AAAA-MM-DD)", true
	case errors.Is(err, services.ErrPagamentoJaQuitadoNaoPodeSerExcluido):
		return http.StatusConflict, "pagamento já quitado não pode ser excluído", true
	default:
		return 0, "", false
	}
}

// pedidoNoEscopo verifica se o pedido informado pertence à carteira do
// escopo restrito. Se não pertencer (ou não existir), escreve 404 com
// msgNaoEncontrado — mesmo corpo de "inexistente", para não revelar registros
// de terceiros — e retorna false. Erro inesperado de banco vira 500.
// Retorna true quando o chamador pode prosseguir.
func (h *PagamentoHandler) pedidoNoEscopo(w http.ResponseWriter, r *http.Request, scope vendedorScope, pedidoID int64, op, msgNaoEncontrado string) bool {
	vendedorID, err := h.svc.VendedorIDDoPedido(r.Context(), h.db, pedidoID)
	if err != nil {
		if errors.Is(err, services.ErrPedidoNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, msgNaoEncontrado)
			return false
		}
		log.Printf("[pagamentos] %s vendedor do pedido: %v", op, err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return false
	}
	if !scope.PermiteVendedor(vendedorID) {
		writeJSON(w, http.StatusNotFound, nil, msgNaoEncontrado)
		return false
	}
	return true
}

// CreatePagamento POST /api/pagamentos
//
// Body: { "pedido_id": number, "forma_pagamento": string, "parcelas": number,
//
//	"valor": number, "taxa_pct": number, "valor_liquido": number,
//	"data_vencimento": "AAAA-MM-DD", "data_pagamento": "AAAA-MM-DD" (opcional),
//	"status_pagamento": string }
//
// valor_liquido é exigido explicitamente no payload (não é calculado
// automaticamente) — ver comentário de services.PagamentoInput.
// Retorna: 201 com o pagamento criado.
// Acesso comum (escopo por carteira): usuário role=normal só pode lançar
// pagamento em pedido da própria carteira (404 "pedido não encontrado" se o
// pedido for de outro vendedor ou não existir). 403 se o usuário normal não
// tiver vendedor vinculado.
func (h *PagamentoHandler) CreatePagamento(w http.ResponseWriter, r *http.Request) {
	scope, err := resolverVendedorScope(r.Context(), h.db)
	if err != nil {
		log.Printf("[pagamentos] CreatePagamento escopo: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusForbidden, nil, "usuário sem vendedor vinculado")
		return
	}

	var req CreatePagamentoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	// pedido_id <= 0 segue para o service, que responde 400 "pedido_id é
	// obrigatório" (nenhum registro é criado nesse caso).
	if scope.Restrito && req.PedidoID > 0 {
		if !h.pedidoNoEscopo(w, r, scope, req.PedidoID, "CreatePagamento", "pedido não encontrado") {
			return
		}
	}

	input := services.PagamentoInput{
		PedidoID:        req.PedidoID,
		FormaPagamento:  req.FormaPagamento,
		Parcelas:        req.Parcelas,
		Valor:           req.Valor,
		TaxaPct:         req.TaxaPct,
		ValorLiquido:    req.ValorLiquido,
		DataVencimento:  req.DataVencimento,
		DataPagamento:   req.DataPagamento,
		StatusPagamento: req.StatusPagamento,
	}

	pagamento, err := h.svc.CreatePagamento(r.Context(), h.db, input)
	if err != nil {
		if status, msg, ok := pagamentoErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[pagamentos] CreatePagamento: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	log.Printf("[pagamentos] criado: pagamento_id=%d pedido_id=%d", pagamento.PagamentoID, pagamento.PedidoID)
	writeJSON(w, http.StatusCreated, pagamento, "")
}

// UpdatePagamento PUT /api/pagamentos/{id}
//
// Body: { "forma_pagamento": string, "parcelas": number, "valor": number,
//
//	"taxa_pct": number, "valor_liquido": number,
//	"data_vencimento": "AAAA-MM-DD", "data_pagamento": "AAAA-MM-DD" (opcional),
//	"status_pagamento": string }
//
// pagamento_id e pedido_id não são editáveis por esta rota.
// Retorna: 200 com o pagamento atualizado, 404 se não existir, 400 se o payload for inválido.
// Acesso comum (escopo por carteira): usuário role=normal só pode atualizar
// pagamento de pedido da própria carteira (404 "pagamento não encontrado"
// caso contrário). Como pedido_id não é editável (UpdatePagamentoRequest não
// o expõe e o service não o altera), não há como mover o pagamento para um
// pedido de outro vendedor.
func (h *PagamentoHandler) UpdatePagamento(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	scope, err := resolverVendedorScope(r.Context(), h.db)
	if err != nil {
		log.Printf("[pagamentos] UpdatePagamento escopo: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
		return
	}

	if scope.Restrito {
		atual, err := h.svc.GetPagamentoByID(r.Context(), h.db, id)
		if err != nil {
			if errors.Is(err, services.ErrPagamentoNaoEncontrado) {
				writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
				return
			}
			log.Printf("[pagamentos] UpdatePagamento buscar atual: %v", err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return
		}
		if !h.pedidoNoEscopo(w, r, scope, atual.PedidoID, "UpdatePagamento", "pagamento não encontrado") {
			return
		}
	}

	var req UpdatePagamentoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	input := services.PagamentoInput{
		FormaPagamento:  req.FormaPagamento,
		Parcelas:        req.Parcelas,
		Valor:           req.Valor,
		TaxaPct:         req.TaxaPct,
		ValorLiquido:    req.ValorLiquido,
		DataVencimento:  req.DataVencimento,
		DataPagamento:   req.DataPagamento,
		StatusPagamento: req.StatusPagamento,
	}

	pagamento, err := h.svc.UpdatePagamento(r.Context(), h.db, id, input)
	if err != nil {
		if status, msg, ok := pagamentoErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[pagamentos] UpdatePagamento: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	log.Printf("[pagamentos] atualizado: pagamento_id=%d", id)
	writeJSON(w, http.StatusOK, pagamento, "")
}

// DeletePagamento DELETE /api/pagamentos/{id}
//
// Hard delete: remove o pagamento definitivamente (não há soft-delete para
// pagamentos). Bloqueado (409) se status_pagamento já for "Pago" ou "Pago
// com atraso" — preserva a trilha financeira de pagamentos já quitados.
// Retorna: 204 sem corpo, 404 se não existir (ou fora do escopo do
// vendedor), 409 se já estiver quitado.
// Acesso comum (qualquer usuário autenticado).
func (h *PagamentoHandler) DeletePagamento(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	scope, err := resolverVendedorScope(r.Context(), h.db)
	if err != nil {
		log.Printf("[pagamentos] DeletePagamento escopo: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
		return
	}

	pagamento, err := h.svc.GetPagamentoByID(r.Context(), h.db, id)
	if err != nil {
		if errors.Is(err, services.ErrPagamentoNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
			return
		}
		log.Printf("[pagamentos] DeletePagamento: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	if scope.Restrito {
		vendedorID, err := h.svc.VendedorIDDoPedido(r.Context(), h.db, pagamento.PedidoID)
		if err != nil {
			log.Printf("[pagamentos] DeletePagamento vendedor do pedido: %v", err)
			writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
			return
		}
		if !scope.PermiteVendedor(vendedorID) {
			writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
			return
		}
	}

	if err := h.svc.DeletePagamento(r.Context(), h.db, id); err != nil {
		if status, msg, ok := pagamentoErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[pagamentos] DeletePagamento: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	log.Printf("[pagamentos] excluído: pagamento_id=%d", id)
	writeNoContent(w)
}
