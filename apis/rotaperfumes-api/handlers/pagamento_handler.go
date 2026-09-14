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
// (admin ou normal) — por isso, ao contrário de ProdutoHandler/PedidoHandler,
// os métodos abaixo NÃO verificam middleware.GetRole/RoleAdmin. O controle de
// acesso (exigir JWT válido, sem exigir admin) é feito inteiramente pela
// cadeia de middleware registrada em routes.go
// (middleware.JWTMiddleware(cfg, true, false)).
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
		return http.StatusBadRequest, "pedido não encontrado", true
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
	default:
		return 0, "", false
	}
}

// CreatePagamento POST /api/pagamentos
//
// Body: { "pedido_id": number, "forma_pagamento": string, "parcelas": number,
//         "valor": number, "taxa_pct": number, "valor_liquido": number,
//         "data_vencimento": "AAAA-MM-DD", "data_pagamento": "AAAA-MM-DD" (opcional),
//         "status_pagamento": string }
// valor_liquido é exigido explicitamente no payload (não é calculado
// automaticamente) — ver comentário de services.PagamentoInput.
// Retorna: 201 com o pagamento criado.
// Acesso comum (qualquer usuário autenticado).
func (h *PagamentoHandler) CreatePagamento(w http.ResponseWriter, r *http.Request) {
	var req CreatePagamentoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
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
//         "taxa_pct": number, "valor_liquido": number,
//         "data_vencimento": "AAAA-MM-DD", "data_pagamento": "AAAA-MM-DD" (opcional),
//         "status_pagamento": string }
// pagamento_id e pedido_id não são editáveis por esta rota.
// Retorna: 200 com o pagamento atualizado, 404 se não existir, 400 se o payload for inválido.
// Acesso comum (qualquer usuário autenticado).
func (h *PagamentoHandler) UpdatePagamento(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
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
