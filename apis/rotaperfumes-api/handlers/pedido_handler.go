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

// PedidoHandler trata as rotas /api/pedidos/*.
type PedidoHandler struct {
	db  *sql.DB
	svc *services.PedidoService
}

// NewPedidoHandler cria um PedidoHandler com pool de conexão injetado.
func NewPedidoHandler(db *sql.DB, cfg *config.Config) *PedidoHandler {
	return &PedidoHandler{
		db:  db,
		svc: services.NewPedidoService(db, cfg),
	}
}

// ListPedidos GET /api/pedidos
//
// Query params (opcionais): page (default 1), limit (default 20, max 100),
// status, canal, cliente_id, vendedor_id, data_inicio, data_fim (AAAA-MM-DD),
// q (busca livre pela razão social do cliente),
// order_by (id|data_pedido|canal|status|valor_total|created_at|updated_at|
// cliente_nome|vendedor_nome; default id), order_dir (asc|desc; default desc).
// Response: {success, data: [pedido...], error, pagination: {page, limit, total, pages}}
// Acesso comum.
func (h *PedidoHandler) ListPedidos(w http.ResponseWriter, r *http.Request) {
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	scope, err := resolverVendedorScope(r.Context(), h.db)
	if err != nil {
		log.Printf("[pedidos] ListPedidos escopo: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.SemAcesso() {
		writeJSONWithPagination(w, http.StatusOK, []any{}, page, limit, 0, 0)
		return
	}

	filtro := services.PedidoFiltro{
		Status:     strings.TrimSpace(r.URL.Query().Get("status")),
		Canal:      strings.TrimSpace(r.URL.Query().Get("canal")),
		ClienteID:  parseInt64Query(r.URL.Query().Get("cliente_id")),
		VendedorID: parseInt64Query(r.URL.Query().Get("vendedor_id")),
		DataInicio: strings.TrimSpace(r.URL.Query().Get("data_inicio")),
		DataFim:    strings.TrimSpace(r.URL.Query().Get("data_fim")),
		Q:          strings.TrimSpace(r.URL.Query().Get("q")),
		OrderBy:    strings.TrimSpace(r.URL.Query().Get("order_by")),
		OrderDir:   parseOrderDirQuery(r.URL.Query().Get("order_dir")),
	}
	if scope.Restrito {
		// Usuário role=normal: força o filtro à própria carteira, ignorando
		// qualquer vendedor_id vindo da query string (evita bypass via URL).
		filtro.VendedorID = scope.VendedorID
	}

	pedidos, total, err := h.svc.ListPedidos(r.Context(), h.db, page, limit, filtro)
	if err != nil {
		log.Printf("[pedidos] ListPedidos: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	pages := total / limit
	if total%limit != 0 {
		pages++
	}

	writeJSONWithPagination(w, http.StatusOK, pedidos, page, limit, total, pages)
}

// parseInt64Query lê um query param numérico opcional. Retorna 0 (sem
// filtro) se ausente ou inválido.
func parseInt64Query(v string) int64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// GetPedido GET /api/pedidos/{id}
//
// Response: {success, data: pedido com itens, error}
// Acesso comum.
func (h *PedidoHandler) GetPedido(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	scope, err := resolverVendedorScope(r.Context(), h.db)
	if err != nil {
		log.Printf("[pedidos] GetPedido escopo: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "pedido não encontrado")
		return
	}

	pedido, err := h.svc.GetPedidoDetalhe(r.Context(), h.db, id)
	if err != nil {
		if errors.Is(err, services.ErrPedidoNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "pedido não encontrado")
			return
		}
		log.Printf("[pedidos] GetPedido: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	if scope.Restrito && !scope.PermiteVendedor(pedido.VendedorID) {
		writeJSON(w, http.StatusNotFound, nil, "pedido não encontrado")
		return
	}

	writeJSON(w, http.StatusOK, pedido, "")
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// ItemPedidoRequest representa um item no payload de criação/edição de pedido.
type ItemPedidoRequest struct {
	ProdutoID      int64   `json:"produto_id"`
	Quantidade     int     `json:"quantidade"`
	PrecoPraticado float64 `json:"preco_praticado"`
	DescontoPct    float64 `json:"desconto_pct"`
}

// CreatePedidoRequest body do POST /api/pedidos.
type CreatePedidoRequest struct {
	ClienteID  int64               `json:"cliente_id"`
	VendedorID int64               `json:"vendedor_id"`
	DataPedido string              `json:"data_pedido"` // formato AAAA-MM-DD
	Canal      string              `json:"canal"`
	Status     string              `json:"status"`
	Itens      []ItemPedidoRequest `json:"itens"`
}

// UpdatePedidoRequest body do PUT /api/pedidos/{id}.
type UpdatePedidoRequest struct {
	ClienteID  int64               `json:"cliente_id"`
	VendedorID int64               `json:"vendedor_id"`
	DataPedido string              `json:"data_pedido"` // formato AAAA-MM-DD
	Canal      string              `json:"canal"`
	Status     string              `json:"status"`
	Itens      []ItemPedidoRequest `json:"itens"`
}

// itensRequestToInput converte os itens do payload HTTP para o input do
// service.
func itensRequestToInput(itens []ItemPedidoRequest) []services.ItemPedidoInput {
	out := make([]services.ItemPedidoInput, 0, len(itens))
	for _, it := range itens {
		out = append(out, services.ItemPedidoInput{
			ProdutoID:      it.ProdutoID,
			Quantidade:     it.Quantidade,
			PrecoPraticado: it.PrecoPraticado,
			DescontoPct:    it.DescontoPct,
		})
	}
	return out
}

// pedidoErroParaStatus mapeia erros de validação/negócio do PedidoService
// para o status HTTP e mensagem apropriados. Retorna ok=false se o erro não
// for reconhecido (cabe ao chamador tratar como erro interno).
func pedidoErroParaStatus(err error) (status int, msg string, ok bool) {
	switch {
	case errors.Is(err, services.ErrPedidoNaoEncontrado):
		return http.StatusNotFound, "pedido não encontrado", true
	case errors.Is(err, services.ErrClienteIDObrigatorio):
		return http.StatusBadRequest, "cliente_id é obrigatório", true
	case errors.Is(err, services.ErrVendedorIDObrigatorio):
		return http.StatusBadRequest, "vendedor_id é obrigatório", true
	case errors.Is(err, services.ErrDataPedidoInvalida):
		return http.StatusBadRequest, "data_pedido inválida (use o formato AAAA-MM-DD)", true
	case errors.Is(err, services.ErrCanalInvalido):
		return http.StatusBadRequest, "canal inválido (use: App, Telefone, Visita, WhatsApp)", true
	case errors.Is(err, services.ErrStatusInvalido):
		return http.StatusBadRequest, "status inválido (use: Cancelado, Em separação, Entregue, Faturado)", true
	case errors.Is(err, services.ErrItensObrigatorios):
		return http.StatusBadRequest, "o pedido deve ter ao menos um item", true
	case errors.Is(err, services.ErrProdutoIDObrigatorio):
		return http.StatusBadRequest, "produto_id é obrigatório em todos os itens", true
	case errors.Is(err, services.ErrQuantidadeInvalida):
		return http.StatusBadRequest, "quantidade deve ser maior que zero em todos os itens", true
	case errors.Is(err, services.ErrPrecoPraticadoInvalido):
		return http.StatusBadRequest, "preco_praticado deve ser maior ou igual a zero em todos os itens", true
	case errors.Is(err, services.ErrDescontoPctInvalido):
		return http.StatusBadRequest, "desconto_pct deve estar entre 0 e 100 em todos os itens", true
	default:
		return 0, "", false
	}
}

// CreatePedido POST /api/pedidos
//
// Body: { "cliente_id": number, "vendedor_id": number, "data_pedido": "AAAA-MM-DD", "canal": string, "status": string, "itens": [{"produto_id": number, "quantidade": number, "preco_praticado": number, "desconto_pct": number}] }
// valor_bruto de cada item e valor_total do pedido são calculados no backend.
// pedido_id_origem é gerado automaticamente pelo sistema.
// Retorna: 201 com o pedido criado (incluindo itens).
// Acesso comum.
func (h *PedidoHandler) CreatePedido(w http.ResponseWriter, r *http.Request) {
	var req CreatePedidoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	input := services.PedidoInput{
		ClienteID:  req.ClienteID,
		VendedorID: req.VendedorID,
		DataPedido: req.DataPedido,
		Canal:      req.Canal,
		Status:     req.Status,
		Itens:      itensRequestToInput(req.Itens),
	}

	pedido, err := h.svc.CreatePedido(r.Context(), h.db, input)
	if err != nil {
		if status, msg, ok := pedidoErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[pedidos] CreatePedido: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	role, _ := middleware.GetRole(r.Context())
	log.Printf("[pedidos] criado: id=%d por usuario role=%s", pedido.PedidoIDOrigem, role)
	writeJSON(w, http.StatusCreated, pedido, "")
}

// UpdatePedido PUT /api/pedidos/{id}
//
// Body: { "cliente_id": number, "vendedor_id": number, "data_pedido": "AAAA-MM-DD", "canal": string, "status": string, "itens": [{"produto_id": number, "quantidade": number, "preco_praticado": number, "desconto_pct": number}] }
// A lista de itens é substituída integralmente (delete + insert). valor_bruto
// e valor_total são recalculados no backend.
// Retorna: 200 com o pedido atualizado (incluindo itens), 404 se não existir,
// 400 se o payload for inválido.
// Acesso comum.
func (h *PedidoHandler) UpdatePedido(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	var req UpdatePedidoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	input := services.PedidoInput{
		ClienteID:  req.ClienteID,
		VendedorID: req.VendedorID,
		DataPedido: req.DataPedido,
		Canal:      req.Canal,
		Status:     req.Status,
		Itens:      itensRequestToInput(req.Itens),
	}

	pedido, err := h.svc.UpdatePedido(r.Context(), h.db, id, input)
	if err != nil {
		if status, msg, ok := pedidoErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[pedidos] UpdatePedido: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	role, _ := middleware.GetRole(r.Context())
	log.Printf("[pedidos] atualizado: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, pedido, "")
}
