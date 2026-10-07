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

// ProdutoHandler trata as rotas /api/produtos/*.
type ProdutoHandler struct {
	db  *sql.DB
	svc *services.ProdutoService
}

// NewProdutoHandler cria um ProdutoHandler com pool de conexão injetado.
func NewProdutoHandler(db *sql.DB, cfg *config.Config) *ProdutoHandler {
	return &ProdutoHandler{
		db:  db,
		svc: services.NewProdutoService(db, cfg),
	}
}

// ListProdutos GET /api/produtos
//
// Query params (opcionais): page (default 1), limit (default 20, max 100),
// categoria, marca, ativo (true|false), q (busca em descricao OU sku),
// order_by (id|sku|descricao|categoria|marca|preco_tabela|custo_unitario|
// data_lancamento|ativo|created_at|updated_at; default id),
// order_dir (asc|desc; default asc).
// Response: {success, data: [produto...], error, pagination: {page, limit, total, pages}}
// Acesso comum.
func (h *ProdutoHandler) ListProdutos(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("produto_handler.go", "ProdutoHandler.ListProdutos", "chamando services.ParsePagination e atribuindo resultado a page, limit")
	page, limit := services.ParsePagination(
		r.URL.Query().Get("page"),
		r.URL.Query().Get("limit"),
	)

	vlog.Printf("produto_handler.go", "ProdutoHandler.ListProdutos", "montando services.ProdutoFiltro em filtro")
	filtro := services.ProdutoFiltro{
		Categoria: strings.TrimSpace(r.URL.Query().Get("categoria")),
		Marca:     strings.TrimSpace(r.URL.Query().Get("marca")),
		Ativo:     parseAtivoQuery(r.URL.Query().Get("ativo")),
		Q:         strings.TrimSpace(r.URL.Query().Get("q")),
		OrderBy:   strings.TrimSpace(r.URL.Query().Get("order_by")),
		OrderDir:  parseOrderDirQuery(r.URL.Query().Get("order_dir")),
	}

	vlog.Printf("produto_handler.go", "ProdutoHandler.ListProdutos", "chamando h.svc.ListProdutos e atribuindo resultado a produtos, total, err")
	produtos, total, err := h.svc.ListProdutos(r.Context(), h.db, page, limit, filtro)
	vlog.Printf("produto_handler.go", "ProdutoHandler.ListProdutos", "verificando se err != nil")
	if err != nil {
		log.Printf("[produtos] ListProdutos: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("produto_handler.go", "ProdutoHandler.ListProdutos", "definindo pages := total / limit")
	pages := total / limit
	vlog.Printf("produto_handler.go", "ProdutoHandler.ListProdutos", "verificando se total%%limit != 0")
	if total%limit != 0 {
		vlog.Printf("produto_handler.go", "ProdutoHandler.ListProdutos", "incrementando pages")
		pages++
	}

	writeJSONWithPagination(w, http.StatusOK, produtos, page, limit, total, pages)
}

// GetProduto GET /api/produtos/{id}
//
// Response: {success, data: produto, error}
// Acesso comum.
func (h *ProdutoHandler) GetProduto(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("produto_handler.go", "ProdutoHandler.GetProduto", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("produto_handler.go", "ProdutoHandler.GetProduto", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("produto_handler.go", "ProdutoHandler.GetProduto", "chamando h.svc.GetProdutoByID e atribuindo resultado a produto, err")
	produto, err := h.svc.GetProdutoByID(r.Context(), h.db, id)
	vlog.Printf("produto_handler.go", "ProdutoHandler.GetProduto", "verificando se err != nil")
	if err != nil {
		vlog.Printf("produto_handler.go", "ProdutoHandler.GetProduto", "verificando se errors.Is(...)")
		if errors.Is(err, services.ErrProdutoNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "produto não encontrado")
			return
		}
		log.Printf("[produtos] GetProduto: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, produto, "")
}

// ToggleAtivoProdutoRequest body do PATCH /api/produtos/{id}/inativar.
type ToggleAtivoProdutoRequest struct {
	Ativo *bool `json:"ativo"` // omitido = toggle
}

// ToggleAtivoProduto PATCH /api/produtos/{id}/inativar
//
// Body opcional: { "ativo": bool } — omitido = toggle
// Response: {success, data: produto atualizado, error}
// Acesso comum.
func (h *ProdutoHandler) ToggleAtivoProduto(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("produto_handler.go", "ProdutoHandler.ToggleAtivoProduto", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("produto_handler.go", "ProdutoHandler.ToggleAtivoProduto", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("produto_handler.go", "ProdutoHandler.ToggleAtivoProduto", "chamando lerAtivoOpcional e atribuindo resultado a ativo, ok")
	ativo, ok := lerAtivoOpcional(w, r) // vazio/null/{} = toggle; inválido = 400
	vlog.Printf("produto_handler.go", "ProdutoHandler.ToggleAtivoProduto", "verificando se !ok")
	if !ok {
		return
	}

	vlog.Printf("produto_handler.go", "ProdutoHandler.ToggleAtivoProduto", "chamando h.svc.ToggleAtivoProduto e atribuindo resultado a produto, err")
	produto, err := h.svc.ToggleAtivoProduto(r.Context(), h.db, id, ativo)
	vlog.Printf("produto_handler.go", "ProdutoHandler.ToggleAtivoProduto", "verificando se err != nil")
	if err != nil {
		vlog.Printf("produto_handler.go", "ProdutoHandler.ToggleAtivoProduto", "verificando se errors.Is(...)")
		if errors.Is(err, services.ErrProdutoNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "produto não encontrado")
			return
		}
		log.Printf("[produtos] ToggleAtivoProduto: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("produto_handler.go", "ProdutoHandler.ToggleAtivoProduto", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[produtos] ativo=%t: id=%d por usuario role=%s", produto.Ativo, id, role)
	writeJSON(w, http.StatusOK, produto, "")
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// CreateProdutoRequest body do POST /api/produtos.
type CreateProdutoRequest struct {
	SKU            string  `json:"sku"`
	Descricao      string  `json:"descricao"`
	Categoria      string  `json:"categoria"`
	Marca          string  `json:"marca"`
	NotaOlfativa   string  `json:"nota_olfativa"`
	PrecoTabela    float64 `json:"preco_tabela"`
	CustoUnitario  float64 `json:"custo_unitario"`
	Unidade        string  `json:"unidade"`
	DataLancamento string  `json:"data_lancamento"` // opcional, formato AAAA-MM-DD
}

// UpdateProdutoRequest body do PUT /api/produtos/{id}.
type UpdateProdutoRequest struct {
	Descricao      string  `json:"descricao"`
	Categoria      string  `json:"categoria"`
	Marca          string  `json:"marca"`
	NotaOlfativa   string  `json:"nota_olfativa"`
	PrecoTabela    float64 `json:"preco_tabela"`
	CustoUnitario  float64 `json:"custo_unitario"`
	Unidade        string  `json:"unidade"`
	DataLancamento string  `json:"data_lancamento"` // opcional, formato AAAA-MM-DD
}

// produtoErroParaStatus mapeia erros de validação/negócio do ProdutoService
// para o status HTTP e mensagem apropriados. Retorna ok=false se o erro não
// for reconhecido (cabe ao chamador tratar como erro interno).
func produtoErroParaStatus(err error) (status int, msg string, ok bool) {
	vlog.Printf("produto_handler.go", "produtoErroParaStatus", "avaliando switch de condições")
	switch {
	case errors.Is(err, services.ErrProdutoNaoEncontrado):
		return http.StatusNotFound, "produto não encontrado", true
	case errors.Is(err, services.ErrSKUObrigatorio):
		return http.StatusBadRequest, "sku é obrigatório", true
	case errors.Is(err, services.ErrDescricaoObrigatoria):
		return http.StatusBadRequest, "descrição é obrigatória", true
	case errors.Is(err, services.ErrCategoriaObrigatoria):
		return http.StatusBadRequest, "categoria é obrigatória", true
	case errors.Is(err, services.ErrMarcaObrigatoria):
		return http.StatusBadRequest, "marca é obrigatória", true
	case errors.Is(err, services.ErrUnidadeObrigatoria):
		return http.StatusBadRequest, "unidade é obrigatória", true
	case errors.Is(err, services.ErrPrecoTabelaInvalido):
		return http.StatusBadRequest, "preco_tabela deve ser maior ou igual a zero", true
	case errors.Is(err, services.ErrCustoUnitarioInvalido):
		return http.StatusBadRequest, "custo_unitario deve ser maior ou igual a zero", true
	case errors.Is(err, services.ErrDataLancamentoInvalida):
		return http.StatusBadRequest, "data_lancamento inválida (use o formato AAAA-MM-DD)", true
	default:
		return 0, "", false
	}
}

// CreateProduto POST /api/produtos
//
// Body: { "sku": string, "descricao": string, "categoria": string, "marca": string, "nota_olfativa": string, "preco_tabela": number, "custo_unitario": number, "unidade": string, "data_lancamento": "AAAA-MM-DD" (opcional) }
// ativo é sempre TRUE na criação (regra de negócio: default ativo).
// Retorna: 201 com o produto criado.
// Acesso comum.
func (h *ProdutoHandler) CreateProduto(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("produto_handler.go", "ProdutoHandler.CreateProduto", "declarando variável req")
	var req CreateProdutoRequest
	vlog.Printf("produto_handler.go", "ProdutoHandler.CreateProduto", "chamando json.NewDecoder(...).Decode e atribuindo resultado a err e verificando se err != nil")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	vlog.Printf("produto_handler.go", "ProdutoHandler.CreateProduto", "montando services.ProdutoInput em input")
	input := services.ProdutoInput{
		SKU:            req.SKU,
		Descricao:      req.Descricao,
		Categoria:      req.Categoria,
		Marca:          req.Marca,
		NotaOlfativa:   req.NotaOlfativa,
		PrecoTabela:    req.PrecoTabela,
		CustoUnitario:  req.CustoUnitario,
		Unidade:        req.Unidade,
		DataLancamento: req.DataLancamento,
	}

	vlog.Printf("produto_handler.go", "ProdutoHandler.CreateProduto", "chamando h.svc.CreateProduto e atribuindo resultado a produto, err")
	produto, err := h.svc.CreateProduto(r.Context(), h.db, input)
	vlog.Printf("produto_handler.go", "ProdutoHandler.CreateProduto", "verificando se err != nil")
	if err != nil {
		vlog.Printf("produto_handler.go", "ProdutoHandler.CreateProduto", "chamando produtoErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := produtoErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[produtos] CreateProduto: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("produto_handler.go", "ProdutoHandler.CreateProduto", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[produtos] criado: id=%d por usuario role=%s", produto.ID, role)
	writeJSON(w, http.StatusCreated, produto, "")
}

// UpdateProduto PUT /api/produtos/{id}
//
// Body: { "descricao": string, "categoria": string, "marca": string, "nota_olfativa": string, "preco_tabela": number, "custo_unitario": number, "unidade": string, "data_lancamento": "AAAA-MM-DD" (opcional) }
// sku e ativo não são editáveis por esta rota.
// Retorna: 200 com o produto atualizado, 404 se não existir, 400 se o payload for inválido.
// Acesso comum.
func (h *ProdutoHandler) UpdateProduto(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("produto_handler.go", "ProdutoHandler.UpdateProduto", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("produto_handler.go", "ProdutoHandler.UpdateProduto", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("produto_handler.go", "ProdutoHandler.UpdateProduto", "declarando variável req")
	var req UpdateProdutoRequest
	vlog.Printf("produto_handler.go", "ProdutoHandler.UpdateProduto", "chamando json.NewDecoder(...).Decode e atribuindo resultado a err e verificando se err != nil")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	vlog.Printf("produto_handler.go", "ProdutoHandler.UpdateProduto", "montando services.ProdutoInput em input")
	input := services.ProdutoInput{
		Descricao:      req.Descricao,
		Categoria:      req.Categoria,
		Marca:          req.Marca,
		NotaOlfativa:   req.NotaOlfativa,
		PrecoTabela:    req.PrecoTabela,
		CustoUnitario:  req.CustoUnitario,
		Unidade:        req.Unidade,
		DataLancamento: req.DataLancamento,
	}

	vlog.Printf("produto_handler.go", "ProdutoHandler.UpdateProduto", "chamando h.svc.UpdateProduto e atribuindo resultado a produto, err")
	produto, err := h.svc.UpdateProduto(r.Context(), h.db, id, input)
	vlog.Printf("produto_handler.go", "ProdutoHandler.UpdateProduto", "verificando se err != nil")
	if err != nil {
		vlog.Printf("produto_handler.go", "ProdutoHandler.UpdateProduto", "chamando produtoErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := produtoErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[produtos] UpdateProduto: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("produto_handler.go", "ProdutoHandler.UpdateProduto", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[produtos] atualizado: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, produto, "")
}
