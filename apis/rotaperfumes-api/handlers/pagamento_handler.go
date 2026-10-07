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
	"github.com/rotaperfumes/shared/vlog"
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
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "chamando services.ParsePagination e atribuindo resultado a page, limit")
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[pagamentos] ListPagamentos", err)
		return
	}
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSONWithPagination(w, http.StatusOK, []any{}, page, limit, 0, 0)
		return
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "declarando variável pedidoID")
	var pedidoID int64
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "chamando strings.TrimSpace e atribuindo resultado a v e verificando se v != \"\"")
	if v := strings.TrimSpace(r.URL.Query().Get("pedido_id")); v != "" {
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "chamando strconv.ParseInt e atribuindo resultado a id, err")
		id, err := strconv.ParseInt(v, 10, 64)
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "verificando se err != nil")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, nil, "pedido_id inválido")
			return
		}
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "atribuindo pedidoID = id")
		pedidoID = id
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "montando services.PagamentoFiltro em filtro")
	filtro := services.PagamentoFiltro{
		StatusPagamento: strings.TrimSpace(r.URL.Query().Get("status_pagamento")),
		FormaPagamento:  strings.TrimSpace(r.URL.Query().Get("forma_pagamento")),
		PedidoID:        pedidoID,
		VencimentoDe:    strings.TrimSpace(r.URL.Query().Get("vencimento_de")),
		VencimentoAte:   strings.TrimSpace(r.URL.Query().Get("vencimento_ate")),
		OrderBy:         strings.TrimSpace(r.URL.Query().Get("order_by")),
		OrderDir:        parseOrderDirQuery(r.URL.Query().Get("order_dir")),
	}
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "verificando se scope.Restrito")
	if scope.Restrito {
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "atribuindo filtro.VendedorID = scope.VendedorID")
		// Usuário role=normal: força o filtro à própria carteira.
		filtro.VendedorID = scope.VendedorID
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "chamando h.svc.ListPagamentos e atribuindo resultado a pagamentos, total, err")
	pagamentos, total, err := h.svc.ListPagamentos(r.Context(), h.db, page, limit, filtro)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "verificando se err != nil")
	if err != nil {
		log.Printf("[pagamentos] ListPagamentos: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "definindo pages := total / limit")
	pages := total / limit
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "verificando se total%%limit != 0")
	if total%limit != 0 {
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.ListPagamentos", "incrementando pages")
		pages++
	}

	writeJSONWithPagination(w, http.StatusOK, pagamentos, page, limit, total, pages)
}

// GetPagamento GET /api/pagamentos/{id}
//
// Response: {success, data: pagamento, error}
// Acesso comum (qualquer usuário autenticado).
func (h *PagamentoHandler) GetPagamento(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.GetPagamento", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.GetPagamento", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.GetPagamento", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.GetPagamento", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[pagamentos] GetPagamento", err)
		return
	}
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.GetPagamento", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
		return
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.GetPagamento", "chamando h.svc.GetPagamentoByID e atribuindo resultado a pagamento, err")
	pagamento, err := h.svc.GetPagamentoByID(r.Context(), h.db, id)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.GetPagamento", "verificando se err != nil")
	if err != nil {
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.GetPagamento", "verificando se errors.Is(...)")
		if errors.Is(err, services.ErrPagamentoNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
			return
		}
		log.Printf("[pagamentos] GetPagamento: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.GetPagamento", "verificando se scope.Restrito")
	if scope.Restrito {
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.GetPagamento", "chamando h.svc.VendedorIDDoPedido e atribuindo resultado a vendedorID, err")
		vendedorID, err := h.svc.VendedorIDDoPedido(r.Context(), h.db, pagamento.PedidoID)
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.GetPagamento", "verificando se err != nil")
		if err != nil {
			// Pedido do pagamento não encontrado é inesperado (FK garante
			// integridade), mas por segurança trata como "sem acesso" em vez
			// de vazar erro interno.
			log.Printf("[pagamentos] GetPagamento vendedor do pedido: %v", err)
			writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
			return
		}
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.GetPagamento", "verificando se !scope.PermiteVendedor(...)")
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
	vlog.Printf("pagamento_handler.go", "pagamentoErroParaStatus", "avaliando switch de condições")
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
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.pedidoNoEscopo", "chamando h.svc.VendedorIDDoPedido e atribuindo resultado a vendedorID, err")
	vendedorID, err := h.svc.VendedorIDDoPedido(r.Context(), h.db, pedidoID)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.pedidoNoEscopo", "verificando se err != nil")
	if err != nil {
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.pedidoNoEscopo", "verificando se errors.Is(...)")
		if errors.Is(err, services.ErrPedidoNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, msgNaoEncontrado)
			return false
		}
		log.Printf("[pagamentos] %s vendedor do pedido: %v", op, err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return false
	}
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.pedidoNoEscopo", "verificando se !scope.PermiteVendedor(...)")
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
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.CreatePagamento", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.CreatePagamento", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[pagamentos] CreatePagamento", err)
		return
	}
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.CreatePagamento", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusForbidden, nil, "usuário sem vendedor vinculado")
		return
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.CreatePagamento", "declarando variável req")
	var req CreatePagamentoRequest
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.CreatePagamento", "chamando json.NewDecoder(...).Decode e atribuindo resultado a err e verificando se err != nil")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.CreatePagamento", "verificando se scope.Restrito && req.PedidoID > 0")
	// pedido_id <= 0 segue para o service, que responde 400 "pedido_id é
	// obrigatório" (nenhum registro é criado nesse caso).
	if scope.Restrito && req.PedidoID > 0 {
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.CreatePagamento", "verificando se !h.pedidoNoEscopo(...)")
		if !h.pedidoNoEscopo(w, r, scope, req.PedidoID, "CreatePagamento", "pedido não encontrado") {
			return
		}
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.CreatePagamento", "montando services.PagamentoInput em input")
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

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.CreatePagamento", "chamando h.svc.CreatePagamento e atribuindo resultado a pagamento, err")
	pagamento, err := h.svc.CreatePagamento(r.Context(), h.db, input)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.CreatePagamento", "verificando se err != nil")
	if err != nil {
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.CreatePagamento", "chamando pagamentoErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
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
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[pagamentos] UpdatePagamento", err)
		return
	}
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
		return
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "verificando se scope.Restrito")
	if scope.Restrito {
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "chamando h.svc.GetPagamentoByID e atribuindo resultado a atual, err")
		atual, err := h.svc.GetPagamentoByID(r.Context(), h.db, id)
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "verificando se err != nil")
		if err != nil {
			vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "verificando se errors.Is(...)")
			if errors.Is(err, services.ErrPagamentoNaoEncontrado) {
				writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
				return
			}
			log.Printf("[pagamentos] UpdatePagamento buscar atual: %v", err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return
		}
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "verificando se !h.pedidoNoEscopo(...)")
		if !h.pedidoNoEscopo(w, r, scope, atual.PedidoID, "UpdatePagamento", "pagamento não encontrado") {
			return
		}
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "declarando variável req")
	var req UpdatePagamentoRequest
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "chamando json.NewDecoder(...).Decode e atribuindo resultado a err e verificando se err != nil")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "montando services.PagamentoInput em input")
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

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "chamando h.svc.UpdatePagamento e atribuindo resultado a pagamento, err")
	pagamento, err := h.svc.UpdatePagamento(r.Context(), h.db, id, input)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "verificando se err != nil")
	if err != nil {
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.UpdatePagamento", "chamando pagamentoErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
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
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[pagamentos] DeletePagamento", err)
		return
	}
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
		return
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "chamando h.svc.GetPagamentoByID e atribuindo resultado a pagamento, err")
	pagamento, err := h.svc.GetPagamentoByID(r.Context(), h.db, id)
	vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "verificando se err != nil")
	if err != nil {
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "verificando se errors.Is(...)")
		if errors.Is(err, services.ErrPagamentoNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
			return
		}
		log.Printf("[pagamentos] DeletePagamento: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "verificando se scope.Restrito")
	if scope.Restrito {
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "chamando h.svc.VendedorIDDoPedido e atribuindo resultado a vendedorID, err")
		vendedorID, err := h.svc.VendedorIDDoPedido(r.Context(), h.db, pagamento.PedidoID)
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "verificando se err != nil")
		if err != nil {
			log.Printf("[pagamentos] DeletePagamento vendedor do pedido: %v", err)
			writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
			return
		}
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "verificando se !scope.PermiteVendedor(...)")
		if !scope.PermiteVendedor(vendedorID) {
			writeJSON(w, http.StatusNotFound, nil, "pagamento não encontrado")
			return
		}
	}

	vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "chamando h.svc.DeletePagamento e atribuindo resultado a err e verificando se err != nil")
	if err := h.svc.DeletePagamento(r.Context(), h.db, id); err != nil {
		vlog.Printf("pagamento_handler.go", "PagamentoHandler.DeletePagamento", "chamando pagamentoErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
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
