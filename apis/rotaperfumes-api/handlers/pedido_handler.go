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
	vlog.Printf("pedido_handler.go", "PedidoHandler.ListPedidos", "chamando services.ParsePagination e atribuindo resultado a page, limit")
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	vlog.Printf("pedido_handler.go", "PedidoHandler.ListPedidos", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("pedido_handler.go", "PedidoHandler.ListPedidos", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[pedidos] ListPedidos", err)
		return
	}
	vlog.Printf("pedido_handler.go", "PedidoHandler.ListPedidos", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSONWithPagination(w, http.StatusOK, []any{}, page, limit, 0, 0)
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.ListPedidos", "montando services.PedidoFiltro em filtro")
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
	vlog.Printf("pedido_handler.go", "PedidoHandler.ListPedidos", "verificando se scope.Restrito")
	if scope.Restrito {
		vlog.Printf("pedido_handler.go", "PedidoHandler.ListPedidos", "atribuindo filtro.VendedorID = scope.VendedorID")
		// Usuário role=normal: força o filtro à própria carteira, ignorando
		// qualquer vendedor_id vindo da query string (evita bypass via URL).
		filtro.VendedorID = scope.VendedorID
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.ListPedidos", "chamando h.svc.ListPedidos e atribuindo resultado a pedidos, total, err")
	pedidos, total, err := h.svc.ListPedidos(r.Context(), h.db, page, limit, filtro)
	vlog.Printf("pedido_handler.go", "PedidoHandler.ListPedidos", "verificando se err != nil")
	if err != nil {
		log.Printf("[pedidos] ListPedidos: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.ListPedidos", "definindo pages := total / limit")
	pages := total / limit
	vlog.Printf("pedido_handler.go", "PedidoHandler.ListPedidos", "verificando se total%%limit != 0")
	if total%limit != 0 {
		vlog.Printf("pedido_handler.go", "PedidoHandler.ListPedidos", "incrementando pages")
		pages++
	}

	writeJSONWithPagination(w, http.StatusOK, pedidos, page, limit, total, pages)
}

// parseInt64Query lê um query param numérico opcional. Retorna 0 (sem
// filtro) se ausente ou inválido.
func parseInt64Query(v string) int64 {
	vlog.Printf("pedido_handler.go", "parseInt64Query", "chamando strings.TrimSpace e atribuindo resultado a v")
	v = strings.TrimSpace(v)
	vlog.Printf("pedido_handler.go", "parseInt64Query", "verificando se v == \"\"")
	if v == "" {
		return 0
	}
	vlog.Printf("pedido_handler.go", "parseInt64Query", "chamando strconv.ParseInt e atribuindo resultado a n, err")
	n, err := strconv.ParseInt(v, 10, 64)
	vlog.Printf("pedido_handler.go", "parseInt64Query", "verificando se err != nil")
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
	vlog.Printf("pedido_handler.go", "PedidoHandler.GetPedido", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("pedido_handler.go", "PedidoHandler.GetPedido", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.GetPedido", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("pedido_handler.go", "PedidoHandler.GetPedido", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[pedidos] GetPedido", err)
		return
	}
	vlog.Printf("pedido_handler.go", "PedidoHandler.GetPedido", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "pedido não encontrado")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.GetPedido", "chamando h.svc.GetPedidoDetalhe e atribuindo resultado a pedido, err")
	pedido, err := h.svc.GetPedidoDetalhe(r.Context(), h.db, id)
	vlog.Printf("pedido_handler.go", "PedidoHandler.GetPedido", "verificando se err != nil")
	if err != nil {
		vlog.Printf("pedido_handler.go", "PedidoHandler.GetPedido", "verificando se errors.Is(...)")
		if errors.Is(err, services.ErrPedidoNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "pedido não encontrado")
			return
		}
		log.Printf("[pedidos] GetPedido: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.GetPedido", "verificando se scope.Restrito && !scope.PermiteVendedor(...)")
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
	vlog.Printf("pedido_handler.go", "itensRequestToInput", "chamando make e atribuindo resultado a out")
	out := make([]services.ItemPedidoInput, 0, len(itens))
	vlog.Printf("pedido_handler.go", "itensRequestToInput", "iniciando loop sobre itens")
	for _, it := range itens {
		out = append(out, services.ItemPedidoInput{
			ProdutoID:      it.ProdutoID,
			Quantidade:     it.Quantidade,
			PrecoPraticado: it.PrecoPraticado,
			DescontoPct:    it.DescontoPct,
		})
	}
	vlog.Printf("pedido_handler.go", "itensRequestToInput", "loop sobre itens concluído: %d itens", len(itens))
	return out
}

// pedidoErroParaStatus mapeia erros de validação/negócio do PedidoService
// para o status HTTP e mensagem apropriados. Retorna ok=false se o erro não
// for reconhecido (cabe ao chamador tratar como erro interno).
func pedidoErroParaStatus(err error) (status int, msg string, ok bool) {
	vlog.Printf("pedido_handler.go", "pedidoErroParaStatus", "avaliando switch de condições")
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
	case errors.Is(err, services.ErrPedidoJaFaturadoNaoPodeAlterarItens):
		return http.StatusConflict, "pedido já faturado: não é possível alterar os itens, apenas o status", true
	case errors.Is(err, services.ErrPedidoPossuiPagamentosVinculados):
		return http.StatusConflict, "pedido possui pagamentos vinculados", true
	case errors.Is(err, services.ErrPedidoFaturadoNaoPodeSerExcluido):
		return http.StatusConflict, "pedido faturado não pode ser excluído, apenas ter o status alterado", true
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
// Acesso comum (escopo por carteira): usuário role=normal só pode criar
// pedido para a própria carteira (vendedor_id do payload é ignorado e forçado
// ao vendedor vinculado; cliente_id deve pertencer à carteira ativa desse
// vendedor). 403 se o usuário normal não tiver vendedor vinculado.
func (h *PedidoHandler) CreatePedido(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[pedidos] CreatePedido", err)
		return
	}
	vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusForbidden, nil, "usuário sem vendedor vinculado")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "declarando variável req")
	var req CreatePedidoRequest
	vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "chamando json.NewDecoder(...).Decode e atribuindo resultado a err e verificando se err != nil")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "verificando se scope.Restrito")
	if scope.Restrito {
		vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "atribuindo req.VendedorID = scope.VendedorID")
		// Usuário role=normal: nunca confia no vendedor_id do payload — força
		// à própria carteira (evita forjar pedido para outro vendedor).
		req.VendedorID = scope.VendedorID

		vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "chamando clienteNaCarteiraDoVendedor e atribuindo resultado a pertence, err")
		pertence, err := clienteNaCarteiraDoVendedor(r.Context(), h.db, scope.VendedorID, req.ClienteID)
		vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "verificando se err != nil")
		if err != nil {
			log.Printf("[pedidos] CreatePedido checar carteira: %v", err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return
		}
		vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "verificando se !pertence")
		if !pertence {
			writeJSON(w, http.StatusBadRequest, nil, "cliente não pertence à carteira deste vendedor")
			return
		}
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "montando services.PedidoInput em input")
	input := services.PedidoInput{
		ClienteID:  req.ClienteID,
		VendedorID: req.VendedorID,
		DataPedido: req.DataPedido,
		Canal:      req.Canal,
		Status:     req.Status,
		Itens:      itensRequestToInput(req.Itens),
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "chamando h.svc.CreatePedido e atribuindo resultado a pedido, err")
	pedido, err := h.svc.CreatePedido(r.Context(), h.db, input)
	vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "verificando se err != nil")
	if err != nil {
		vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "chamando pedidoErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := pedidoErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[pedidos] CreatePedido: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.CreatePedido", "chamando middleware.GetRole e atribuindo resultado a role, _")
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
// Acesso comum (escopo por carteira): usuário role=normal só pode atualizar
// pedido da própria carteira (404 se pertencer a outro vendedor), não pode
// reatribuir vendedor_id e o cliente_id deve pertencer à sua carteira ativa.
func (h *PedidoHandler) UpdatePedido(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[pedidos] UpdatePedido", err)
		return
	}
	vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "pedido não encontrado")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "verificando se scope.Restrito")
	if scope.Restrito {
		vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "chamando h.svc.GetPedidoDetalhe e atribuindo resultado a atual, err")
		// Só o escopo restrito precisa do registro atual (para checar a
		// posse); admin segue direto ao service, que já retorna 404 se o
		// pedido não existir.
		atual, err := h.svc.GetPedidoDetalhe(r.Context(), h.db, id)
		vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "verificando se err != nil")
		if err != nil {
			vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "verificando se errors.Is(...)")
			if errors.Is(err, services.ErrPedidoNaoEncontrado) {
				writeJSON(w, http.StatusNotFound, nil, "pedido não encontrado")
				return
			}
			log.Printf("[pedidos] UpdatePedido buscar atual: %v", err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return
		}
		vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "verificando se !scope.PermiteVendedor(...)")
		if !scope.PermiteVendedor(atual.VendedorID) {
			writeJSON(w, http.StatusNotFound, nil, "pedido não encontrado")
			return
		}
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "declarando variável req")
	var req UpdatePedidoRequest
	vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "chamando json.NewDecoder(...).Decode e atribuindo resultado a err e verificando se err != nil")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "verificando se scope.Restrito")
	if scope.Restrito {
		vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "atribuindo req.VendedorID = scope.VendedorID")
		// Usuário role=normal: nunca confia no vendedor_id do payload — força
		// à própria carteira (evita reatribuir o pedido a outro vendedor).
		req.VendedorID = scope.VendedorID

		vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "chamando clienteNaCarteiraDoVendedor e atribuindo resultado a pertence, err")
		pertence, err := clienteNaCarteiraDoVendedor(r.Context(), h.db, scope.VendedorID, req.ClienteID)
		vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "verificando se err != nil")
		if err != nil {
			log.Printf("[pedidos] UpdatePedido checar carteira: %v", err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return
		}
		vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "verificando se !pertence")
		if !pertence {
			writeJSON(w, http.StatusBadRequest, nil, "cliente não pertence à carteira deste vendedor")
			return
		}
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "montando services.PedidoInput em input")
	input := services.PedidoInput{
		ClienteID:  req.ClienteID,
		VendedorID: req.VendedorID,
		DataPedido: req.DataPedido,
		Canal:      req.Canal,
		Status:     req.Status,
		Itens:      itensRequestToInput(req.Itens),
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "chamando h.svc.UpdatePedido e atribuindo resultado a pedido, err")
	pedido, err := h.svc.UpdatePedido(r.Context(), h.db, id, input)
	vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "verificando se err != nil")
	if err != nil {
		vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "chamando pedidoErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := pedidoErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[pedidos] UpdatePedido: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.UpdatePedido", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[pedidos] atualizado: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, pedido, "")
}

// DeletePedido DELETE /api/pedidos/{id}
//
// Hard delete: remove o pedido e seus itens definitivamente (não há
// soft-delete para pedidos). Bloqueado (409) se o pedido tiver pagamentos
// vinculados ou já estiver com status "Faturado" (nesse caso a única
// alteração permitida é a de status, via PUT).
// Retorna: 204 sem corpo, 404 se não existir (ou fora do escopo do
// vendedor), 409 se houver pagamentos vinculados ou o pedido já estiver
// faturado.
// Acesso comum.
func (h *PedidoHandler) DeletePedido(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("pedido_handler.go", "PedidoHandler.DeletePedido", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("pedido_handler.go", "PedidoHandler.DeletePedido", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.DeletePedido", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("pedido_handler.go", "PedidoHandler.DeletePedido", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[pedidos] DeletePedido", err)
		return
	}
	vlog.Printf("pedido_handler.go", "PedidoHandler.DeletePedido", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "pedido não encontrado")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.DeletePedido", "chamando h.svc.GetPedidoDetalhe e atribuindo resultado a pedido, err")
	pedido, err := h.svc.GetPedidoDetalhe(r.Context(), h.db, id)
	vlog.Printf("pedido_handler.go", "PedidoHandler.DeletePedido", "verificando se err != nil")
	if err != nil {
		vlog.Printf("pedido_handler.go", "PedidoHandler.DeletePedido", "verificando se errors.Is(...)")
		if errors.Is(err, services.ErrPedidoNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "pedido não encontrado")
			return
		}
		log.Printf("[pedidos] DeletePedido buscar atual: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	vlog.Printf("pedido_handler.go", "PedidoHandler.DeletePedido", "verificando se scope.Restrito && !scope.PermiteVendedor(...)")
	if scope.Restrito && !scope.PermiteVendedor(pedido.VendedorID) {
		writeJSON(w, http.StatusNotFound, nil, "pedido não encontrado")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.DeletePedido", "chamando h.svc.DeletePedido e atribuindo resultado a err e verificando se err != nil")
	if err := h.svc.DeletePedido(r.Context(), h.db, id); err != nil {
		vlog.Printf("pedido_handler.go", "PedidoHandler.DeletePedido", "chamando pedidoErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := pedidoErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[pedidos] DeletePedido: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("pedido_handler.go", "PedidoHandler.DeletePedido", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[pedidos] excluído: id=%d por usuario role=%s", id, role)
	writeNoContent(w)
}
