package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"backend/erp/internal/models"
	"backend/erp/internal/services"

	"github.com/gorilla/mux"
)

// ===================== HANDLER STRUCTS =====================

type ProdutoHandler struct {
	service *services.ProdutoService
}

type PedidoHandler struct {
	service *services.PedidoService
}

type ItemPedidoHandler struct {
	service *services.ItemPedidoService
}

type PagamentoHandler struct {
	service *services.PagamentoService
}

type EstoqueHandler struct {
	service *services.EstoqueService
}

type DashboardHandler struct {
	service *services.DashboardService
}

// ===================== CONSTRUCTORS =====================

func NewProdutoHandler() *ProdutoHandler {
	return &ProdutoHandler{service: services.NewProdutoService()}
}

func NewPedidoHandler() *PedidoHandler {
	return &PedidoHandler{service: services.NewPedidoService()}
}

func NewItemPedidoHandler() *ItemPedidoHandler {
	return &ItemPedidoHandler{service: services.NewItemPedidoService()}
}

func NewPagamentoHandler() *PagamentoHandler {
	return &PagamentoHandler{service: services.NewPagamentoService()}
}

func NewEstoqueHandler() *EstoqueHandler {
	return &EstoqueHandler{service: services.NewEstoqueService()}
}

func NewDashboardHandler() *DashboardHandler {
	return &DashboardHandler{service: services.NewDashboardService()}
}

// ===================== PRODUTO HANDLERS =====================

func (h *ProdutoHandler) ListarTodos(w http.ResponseWriter, r *http.Request) {
	produtos, err := h.service.ListarTodos(r.Context())
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(produtos)
}

func (h *ProdutoHandler) BuscarPorSKU(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	sku := params["sku"]

	produto, err := h.service.BuscarPorSKU(r.Context(), sku)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if produto == nil {
		http.Error(w, `{"erro": "Produto nao encontrado"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(produto)
}

func (h *ProdutoHandler) Criar(w http.ResponseWriter, r *http.Request) {
	var req models.Produto
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"erro": "Dados invalidos"}`, http.StatusBadRequest)
		return
	}

	if req.SKU == "" || req.Descricao == "" {
		http.Error(w, `{"erro": "SKU e Descricao sao obrigatorios"}`, http.StatusBadRequest)
		return
	}

	err := h.service.Criar(r.Context(), req)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"mensagem": "Produto criado com sucesso"})
}

func (h *ProdutoHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	sku := params["sku"]

	produto, err := h.service.BuscarPorSKU(r.Context(), sku)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if produto == nil {
		http.Error(w, `{"erro": "Produto nao encontrado"}`, http.StatusNotFound)
		return
	}

	var req models.Produto
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"erro": "Dados invalidos"}`, http.StatusBadRequest)
		return
	}

	req.SKU = sku
	err = h.service.Atualizar(r.Context(), sku, req)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"mensagem": "Produto atualizado com sucesso"})
}

// ===================== PEDIDO HANDLERS =====================

func (h *PedidoHandler) ListarTodos(w http.ResponseWriter, r *http.Request) {
	pedidos, err := h.service.ListarTodos(r.Context())
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pedidos)
}

func (h *PedidoHandler) BuscarPorID(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	idStr := params["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"erro": "ID invalido"}`, http.StatusBadRequest)
		return
	}

	pedido, err := h.service.BuscarPorID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}
	if pedido == nil {
		http.Error(w, `{"erro": "Pedido nao encontrado"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pedido)
}

func (h *PedidoHandler) Criar(w http.ResponseWriter, r *http.Request) {
	var req models.PedidoInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"erro": "Dados invalidos"}`, http.StatusBadRequest)
		return
	}

	pedidoID, err := h.service.Criar(r.Context(), req)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensagem": "Pedido criado com sucesso",
		"id":       pedidoID,
	})
}

// ===================== ITEM PEDIDO HANDLERS =====================

func (h *ItemPedidoHandler) ListarPorPedido(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	pedidoIDStr := params["id"]

	pedidoID, err := strconv.ParseInt(pedidoIDStr, 10, 64)
	if err != nil {
		http.Error(w, `{"erro": "ID invalido"}`, http.StatusBadRequest)
		return
	}

	itens, err := h.service.ListarPorPedido(r.Context(), pedidoID)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(itens)
}

// ===================== PAGAMENTO HANDLERS =====================

func (h *PagamentoHandler) ListarTodos(w http.ResponseWriter, r *http.Request) {
	pagamentos, err := h.service.ListarTodos(r.Context())
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pagamentos)
}

func (h *PagamentoHandler) Criar(w http.ResponseWriter, r *http.Request) {
	var req models.Pagamento
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"erro": "Dados invalidos"}`, http.StatusBadRequest)
		return
	}

	if req.PedidoID == 0 || req.FormaPagamento == "" || req.Valor == 0 {
		http.Error(w, `{"erro": "PedidoID, FormaPagamento e Valor sao obrigatorios"}`, http.StatusBadRequest)
		return
	}

	if req.DataVencimento.IsZero() {
		req.DataVencimento = time.Now().AddDate(0, 0, 30)
	}
	if req.StatusPagamento == "" {
		req.StatusPagamento = "pendente"
	}

	pagamentoID, err := h.service.Criar(r.Context(), req)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensagem": "Pagamento criado com sucesso",
		"id":       pagamentoID,
	})
}

func (h *PagamentoHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	idStr := params["id"]

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"erro": "ID invalido"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		StatusPagamento string `json:"status_pagamento"`
		DataPagamento   string `json:"data_pagamento"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"erro": "Dados invalidos"}`, http.StatusBadRequest)
		return
	}

	pagamento := models.Pagamento{
		StatusPagamento: req.StatusPagamento,
	}
	if req.DataPagamento != "" {
		t, err := time.Parse("2006-01-02", req.DataPagamento)
		if err == nil {
			pagamento.DataPagamento = sql.NullTime{Time: t, Valid: true}
		}
	}

	err = h.service.Atualizar(r.Context(), id, pagamento)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"mensagem": "Pagamento atualizado com sucesso"})
}

// ===================== ESTOQUE HANDLERS =====================

func (h *EstoqueHandler) ListarPorSKU(w http.ResponseWriter, r *http.Request) {
	sku := r.URL.Query().Get("sku")

	if sku == "" {
		http.Error(w, `{"erro": "Parametro sku e obrigatorio"}`, http.StatusBadRequest)
		return
	}

	estoques, err := h.service.ListarPorSKU(r.Context(), sku)
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(estoques)
}

// ===================== DASHBOARD HANDLERS =====================

func (h *DashboardHandler) ObterTotaisFinanceiro(w http.ResponseWriter, r *http.Request) {
	totais, err := h.service.ObterTotaisPorFormaPagamento(r.Context())
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(totais)
}

func (h *DashboardHandler) ObterRupturas(w http.ResponseWriter, r *http.Request) {
	rupturas, err := h.service.ListarEmRuptura(r.Context())
	if err != nil {
		http.Error(w, `{"erro": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rupturas)
}
