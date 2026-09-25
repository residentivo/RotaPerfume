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
	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
)

// ClienteHandler trata as rotas /api/clientes/*.
type ClienteHandler struct {
	db           *sql.DB
	svc          *services.ClienteService
	carteiraRepo *repositories.CarteiraRepository
}

// NewClienteHandler cria um ClienteHandler com pool de conexão injetado.
func NewClienteHandler(db *sql.DB, cfg *config.Config) *ClienteHandler {
	return &ClienteHandler{
		db:           db,
		svc:          services.NewClienteService(db, cfg),
		carteiraRepo: repositories.NewCarteiraRepository(),
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

	scope, err := resolverVendedorScope(r, h.db)
	if err != nil {
		responderErroEscopo(w, "[clientes] ListClientes", err)
		return
	}
	if scope.SemAcesso() {
		// Usuário normal sem vendedor vinculado: nenhuma carteira, nenhum
		// resultado — nunca cai no "sem filtro" (que exporia todos os
		// clientes de todos os vendedores).
		writeJSONWithPagination(w, http.StatusOK, []any{}, page, limit, 0, 0)
		return
	}

	filtro := services.ClienteFiltro{
		UF:       strings.TrimSpace(r.URL.Query().Get("uf")),
		Segmento: strings.TrimSpace(r.URL.Query().Get("segmento")),
		Ativo:    parseAtivoQuery(r.URL.Query().Get("ativo")),
		Q:        strings.TrimSpace(r.URL.Query().Get("q")),
		OrderBy:  strings.TrimSpace(r.URL.Query().Get("order_by")),
		OrderDir: parseOrderDirQuery(r.URL.Query().Get("order_dir")),
	}
	if scope.Restrito {
		// Usuário role=normal: força o filtro à própria carteira, ignorando
		// qualquer tentativa de bypass via query string (não há parâmetro
		// vendedor_id nesta rota hoje, mas o campo é sempre sobrescrito).
		filtro.VendedorID = scope.VendedorID
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

	scope, err := resolverVendedorScope(r, h.db)
	if err != nil {
		responderErroEscopo(w, "[clientes] GetCliente", err)
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "cliente não encontrado")
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

	if scope.Restrito {
		if _, err := h.carteiraRepo.GetVinculoAtivo(r.Context(), h.db, scope.VendedorID, id); err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				// Cliente existe, mas não pertence à carteira do usuário:
				// resposta 404 (mesma mensagem do "não existe") para não
				// vazar a existência de clientes de outros vendedores.
				writeJSON(w, http.StatusNotFound, nil, "cliente não encontrado")
				return
			}
			log.Printf("[clientes] GetCliente vinculo carteira: %v", err)
			writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
			return
		}
	}

	writeJSON(w, http.StatusOK, cliente, "")
}

// autorizarEscritaCliente aplica o escopo de carteira (SEC-01) às rotas que
// alteram um cliente existente (Update/Toggle), ANTES de ler o body:
//   - erro ao resolver escopo → 403 (vendedor desligado) ou 500;
//   - usuário normal sem vendedor vinculado → 404 "cliente não encontrado";
//   - usuário normal cujo vendedor não tem vínculo ativo com o cliente →
//     404 "cliente não encontrado" (mesma mensagem do cliente inexistente,
//     para não vazar a existência de clientes de outras carteiras);
//   - admin → sem restrição.
//
// Devolve false quando a resposta de erro já foi escrita.
//
// Regra: os DTOs de Update/Toggle não têm vendedor_id. Se um dia ganharem,
// o valor deve ser ignorado ou forçado a scope.VendedorID para usuário
// normal (mesmo padrão de CreateOportunidade), nunca aceito do payload.
func (h *ClienteHandler) autorizarEscritaCliente(w http.ResponseWriter, r *http.Request, handler string, clienteID int64) bool {
	scope, err := resolverVendedorScope(r, h.db)
	if err != nil {
		responderErroEscopo(w, "[clientes] "+handler, err)
		return false
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusNotFound, nil, "cliente não encontrado")
		return false
	}
	if !scope.Restrito {
		return true
	}

	pertence, err := clienteNaCarteiraDoVendedor(r.Context(), h.db, scope.VendedorID, clienteID)
	if err != nil {
		log.Printf("[clientes] %s checar carteira: %v", handler, err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return false
	}
	if !pertence {
		writeJSON(w, http.StatusNotFound, nil, "cliente não encontrado")
		return false
	}
	return true
}

// ToggleAtivoClienteRequest body do PATCH /api/clientes/{id}/inativar.
type ToggleAtivoClienteRequest struct {
	Ativo *bool `json:"ativo"` // omitido = toggle
}

// ToggleAtivoCliente PATCH /api/clientes/{id}/inativar
//
// Body opcional: { "ativo": bool } — omitido = toggle
// Response: {success, data: cliente atualizado, error}
// Acesso comum: usuário role=normal só altera clientes da própria carteira
// ativa (fora dela, ou sem vendedor vinculado → 404 "cliente não encontrado").
func (h *ClienteHandler) ToggleAtivoCliente(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	// Escopo antes do decode (SEC-01): sem acesso → 404 mesmo com body inválido.
	if !h.autorizarEscritaCliente(w, r, "ToggleAtivoCliente", id) {
		return
	}

	ativo, ok := lerAtivoOpcional(w, r) // vazio/null/{} = toggle; inválido = 400
	if !ok {
		return
	}

	cliente, err := h.svc.ToggleAtivoCliente(r.Context(), h.db, id, ativo)
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
	case errors.Is(err, services.ErrCNPJInvalido):
		return http.StatusBadRequest, "cnpj inválido", true
	case errors.Is(err, services.ErrCNPJDuplicado):
		// Mensagem genérica (NEG-01): nunca revela id, vendedor ou razão
		// social do cliente que já usa o CNPJ.
		return http.StatusConflict, "cnpj já cadastrado", true
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
// Acesso comum. Usuário role=normal: sem vendedor vinculado → 403 "usuário
// sem vendedor vinculado"; com vendedor, o cliente é criado já vinculado à
// carteira desse vendedor (data_inicio = hoje), na mesma transação. Admin cria
// só o cliente, sem carteira.
func (h *ClienteHandler) CreateCliente(w http.ResponseWriter, r *http.Request) {
	role, _ := middleware.GetRole(r.Context())

	scope, err := resolverVendedorScope(r, h.db)
	if err != nil {
		responderErroEscopo(w, "[clientes] CreateCliente", err)
		return
	}
	if scope.SemAcesso() {
		writeJSON(w, http.StatusForbidden, nil, "usuário sem vendedor vinculado")
		return
	}

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

	cliente, err := h.criarCliente(r, scope, input)
	if err != nil {
		if status, msg, ok := clienteErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[clientes] CreateCliente: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	if scope.Restrito {
		log.Printf("[clientes] criado: id=%d por usuario role=%s vendedor_id=%d carteira_vinculada=true",
			cliente.ClienteIDOrigem, role, scope.VendedorID)
	} else {
		log.Printf("[clientes] criado: id=%d por usuario role=%s", cliente.ClienteIDOrigem, role)
	}
	writeJSON(w, http.StatusCreated, cliente, "")
}

// criarCliente escolhe o fluxo de criação conforme o escopo: usuário normal
// cria cliente + vínculo de carteira com o próprio vendedor numa única
// transação (SEC-01); admin cria só o cliente, sem carteira.
func (h *ClienteHandler) criarCliente(r *http.Request, scope vendedorScope, input services.ClienteInput) (*models.Cliente, error) {
	if scope.Restrito {
		return h.svc.CreateClienteNaCarteira(r.Context(), h.db, input, scope.VendedorID)
	}
	return h.svc.CreateCliente(r.Context(), h.db, input)
}

// UpdateCliente PUT /api/clientes/{id}
//
// Body: { "cnpj": string, "razao_social": string, "segmento": string, "cidade": string, "uf": string, "bairro": string, "data_cadastro": "AAAA-MM-DD" }
// cliente_id_origem e ativo não são editáveis por esta rota.
// Retorna: 200 com o cliente atualizado, 404 se não existir, 400 se o payload for inválido.
// Acesso comum: usuário role=normal só edita clientes da própria carteira
// ativa (fora dela, ou sem vendedor vinculado → 404 "cliente não encontrado").
func (h *ClienteHandler) UpdateCliente(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	if !h.autorizarEscritaCliente(w, r, "UpdateCliente", id) {
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
