// Package handlers contém handlers HTTP.
package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/vlog"
)

// VendedorHandler trata as rotas /api/vendedores/*.
type VendedorHandler struct {
	db  *sql.DB
	svc *services.VendedorService
}

// NewVendedorHandler cria um VendedorHandler com pool de conexão injetado.
func NewVendedorHandler(db *sql.DB, cfg *config.Config) *VendedorHandler {
	return &VendedorHandler{
		db:  db,
		svc: services.NewVendedorService(db, cfg),
	}
}

// ListVendedores GET /api/vendedores
//
// Sem paginação: usado para popular listas de seleção (ex: combobox no
// admin de usuários/pedidos). Retorna TODOS os vendedores, ativos e
// inativos — cada item traz data_desligamento (null = ativo) para que o
// frontend decida como sinalizar os inativos (ex: marcador "[inativo]").
// Antes filtrava só ativos, o que fazia vendedores inativos vinculados a
// registros existentes sumirem das opções (bug corrigido).
// Response: {success, data: [{id, nome, regiao, uf, data_desligamento}], error}
//
// Escopo (SEC-03, contrato 🟣 SecBrain):
//   - admin: todos os vendedores (ativos e desligados);
//   - normal com vendedor desligado: 403 "acesso bloqueado: vendedor desligado";
//   - normal sem vínculo (ou vínculo órfão): 200 com data [];
//   - normal com vínculo: 200 com data [só o próprio vendedor].
func (h *VendedorHandler) ListVendedores(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("vendedor_handler.go", "VendedorHandler.ListVendedores", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.ListVendedores", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[vendedores] ListVendedores", err)
		return
	}
	vlog.Printf("vendedor_handler.go", "VendedorHandler.ListVendedores", "verificando se scope.SemAcesso()")
	if scope.SemAcesso() {
		writeJSON(w, http.StatusOK, []any{}, "")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.ListVendedores", "declarando variável vendedores")
	var vendedores any
	vlog.Printf("vendedor_handler.go", "VendedorHandler.ListVendedores", "verificando se scope.Restrito")
	if scope.Restrito {
		vlog.Printf("vendedor_handler.go", "VendedorHandler.ListVendedores", "chamando h.svc.ListVendedorProprio e atribuindo resultado a vendedores, err")
		vendedores, err = h.svc.ListVendedorProprio(r.Context(), h.db, scope.VendedorID)
	} else {
		vlog.Printf("vendedor_handler.go", "VendedorHandler.ListVendedores", "chamando h.svc.ListVendedores e atribuindo resultado a vendedores, err")
		vendedores, err = h.svc.ListVendedores(r.Context(), h.db)
	}
	vlog.Printf("vendedor_handler.go", "VendedorHandler.ListVendedores", "verificando se err != nil")
	if err != nil {
		log.Printf("[vendedores] ListVendedores: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, vendedores, "")
}

// GetVendedor GET /api/vendedores/{id}
//
// Response: {success, data: vendedor com a lista de clientes vinculados
// (carteira ativa), error}
// Admin only (JWTMiddleware com requireAdmin=true; role=normal recebe 403).
func (h *VendedorHandler) GetVendedor(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("vendedor_handler.go", "VendedorHandler.GetVendedor", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.GetVendedor", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.GetVendedor", "chamando h.svc.GetVendedorDetalhe e atribuindo resultado a vendedor, err")
	vendedor, err := h.svc.GetVendedorDetalhe(r.Context(), h.db, id)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.GetVendedor", "verificando se err != nil")
	if err != nil {
		vlog.Printf("vendedor_handler.go", "VendedorHandler.GetVendedor", "verificando se errors.Is(...)")
		if errors.Is(err, services.ErrVendedorNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "vendedor não encontrado")
			return
		}
		log.Printf("[vendedores] GetVendedor: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, vendedor, "")
}

// ListClientesDoVendedor GET /api/vendedores/{id}/clientes
//
// Lista os clientes vinculados (carteira ativa, data_fim IS NULL) a um
// vendedor. Usado pelo dropdown em cascata do frontend (Oportunidades: ao
// escolher o vendedor, filtra os clientes possíveis).
// Response: {success, data: [ClienteResumo...], error}
// Acesso comum.
func (h *VendedorHandler) ListClientesDoVendedor(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("vendedor_handler.go", "VendedorHandler.ListClientesDoVendedor", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.ListClientesDoVendedor", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.ListClientesDoVendedor", "chamando resolverVendedorScope e atribuindo resultado a scope, err")
	scope, err := resolverVendedorScope(r, h.db)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.ListClientesDoVendedor", "verificando se err != nil")
	if err != nil {
		responderErroEscopo(w, "[vendedores] ListClientesDoVendedor", err)
		return
	}
	vlog.Printf("vendedor_handler.go", "VendedorHandler.ListClientesDoVendedor", "verificando se scope.Restrito && !scope.PermiteVendedor(...)")
	if scope.Restrito && !scope.PermiteVendedor(id) {
		// Usuário role=normal tentando ver a carteira de outro vendedor:
		// resposta idêntica à de vendedor inexistente, para não confirmar a
		// existência do id informado.
		writeJSON(w, http.StatusNotFound, nil, "vendedor não encontrado")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.ListClientesDoVendedor", "chamando h.svc.ListClientesDoVendedor e atribuindo resultado a clientes, err")
	clientes, err := h.svc.ListClientesDoVendedor(r.Context(), h.db, id)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.ListClientesDoVendedor", "verificando se err != nil")
	if err != nil {
		vlog.Printf("vendedor_handler.go", "VendedorHandler.ListClientesDoVendedor", "verificando se errors.Is(...)")
		if errors.Is(err, services.ErrVendedorNaoEncontrado) {
			writeJSON(w, http.StatusNotFound, nil, "vendedor não encontrado")
			return
		}
		log.Printf("[vendedores] ListClientesDoVendedor: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, clientes, "")
}

// ---------------------------------------------------------------------------
// Request DTOs
// ---------------------------------------------------------------------------

// CreateVendedorRequest body do POST /api/vendedores.
type CreateVendedorRequest struct {
	Nome         string  `json:"nome"`
	Regiao       string  `json:"regiao"`
	UF           string  `json:"uf"`
	DataAdmissao string  `json:"data_admissao"` // opcional, formato AAAA-MM-DD; vazio = hoje
	MetaMensal   float64 `json:"meta_mensal"`
}

// UpdateVendedorRequest body do PUT /api/vendedores/{id}.
type UpdateVendedorRequest struct {
	Nome         string  `json:"nome"`
	Regiao       string  `json:"regiao"`
	UF           string  `json:"uf"`
	DataAdmissao string  `json:"data_admissao"` // formato AAAA-MM-DD
	MetaMensal   float64 `json:"meta_mensal"`
}

// vendedorErroParaStatus mapeia erros de validação/negócio do VendedorService
// para o status HTTP e mensagem apropriados. Retorna ok=false se o erro não
// for reconhecido (cabe ao chamador tratar como erro interno).
func vendedorErroParaStatus(err error) (status int, msg string, ok bool) {
	vlog.Printf("vendedor_handler.go", "vendedorErroParaStatus", "avaliando switch de condições")
	switch {
	case errors.Is(err, services.ErrVendedorNaoEncontrado):
		return http.StatusNotFound, "vendedor não encontrado", true
	case errors.Is(err, services.ErrVendedorNomeObrigatorio):
		return http.StatusBadRequest, "nome é obrigatório", true
	case errors.Is(err, services.ErrVendedorRegiaoObrigatoria):
		return http.StatusBadRequest, "regiao é obrigatória", true
	case errors.Is(err, services.ErrVendedorUFInvalida):
		return http.StatusBadRequest, "uf deve ter 2 letras", true
	case errors.Is(err, services.ErrVendedorDataAdmissaoInvalida):
		return http.StatusBadRequest, "data_admissao inválida (use o formato AAAA-MM-DD)", true
	case errors.Is(err, services.ErrVendedorMetaMensalInvalida):
		return http.StatusBadRequest, "meta_mensal deve ser maior ou igual a zero", true
	case errors.Is(err, services.ErrClienteNaoEncontrado):
		return http.StatusNotFound, "cliente não encontrado", true
	case errors.Is(err, services.ErrVinculoNaoEncontrado):
		return http.StatusNotFound, "vínculo entre cliente e vendedor não encontrado", true
	default:
		return 0, "", false
	}
}

// CreateVendedor POST /api/vendedores
//
// Body: { "nome": string, "regiao": string, "uf": string, "data_admissao": "AAAA-MM-DD" (opcional, default hoje), "meta_mensal": number }
// Retorna: 201 com o vendedor criado.
// Admin only (JWTMiddleware com requireAdmin=true; role=normal recebe 403).
func (h *VendedorHandler) CreateVendedor(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("vendedor_handler.go", "VendedorHandler.CreateVendedor", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())

	vlog.Printf("vendedor_handler.go", "VendedorHandler.CreateVendedor", "declarando variável req")
	var req CreateVendedorRequest
	vlog.Printf("vendedor_handler.go", "VendedorHandler.CreateVendedor", "chamando json.NewDecoder(...).Decode e atribuindo resultado a err e verificando se err != nil")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.CreateVendedor", "montando services.VendedorInput em input")
	input := services.VendedorInput{
		Nome:         req.Nome,
		Regiao:       req.Regiao,
		UF:           req.UF,
		DataAdmissao: req.DataAdmissao,
		MetaMensal:   req.MetaMensal,
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.CreateVendedor", "chamando h.svc.CreateVendedor e atribuindo resultado a vendedor, err")
	vendedor, err := h.svc.CreateVendedor(r.Context(), h.db, input)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.CreateVendedor", "verificando se err != nil")
	if err != nil {
		vlog.Printf("vendedor_handler.go", "VendedorHandler.CreateVendedor", "chamando vendedorErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := vendedorErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[vendedores] CreateVendedor: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	log.Printf("[vendedores] criado: id=%d por usuario role=%s", vendedor.ID, role)
	writeJSON(w, http.StatusCreated, vendedor, "")
}

// UpdateVendedor PUT /api/vendedores/{id}
//
// Body: { "nome": string, "regiao": string, "uf": string, "data_admissao": "AAAA-MM-DD", "meta_mensal": number }
// data_desligamento não é editável por esta rota (ver DeleteVendedor).
// Retorna: 200 com o vendedor atualizado, 404 se não existir, 400 se o payload for inválido.
// Admin only (JWTMiddleware com requireAdmin=true; role=normal recebe 403).
func (h *VendedorHandler) UpdateVendedor(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("vendedor_handler.go", "VendedorHandler.UpdateVendedor", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.UpdateVendedor", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.UpdateVendedor", "declarando variável req")
	var req UpdateVendedorRequest
	vlog.Printf("vendedor_handler.go", "VendedorHandler.UpdateVendedor", "chamando json.NewDecoder(...).Decode e atribuindo resultado a err e verificando se err != nil")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.UpdateVendedor", "montando services.VendedorInput em input")
	input := services.VendedorInput{
		Nome:         req.Nome,
		Regiao:       req.Regiao,
		UF:           req.UF,
		DataAdmissao: req.DataAdmissao,
		MetaMensal:   req.MetaMensal,
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.UpdateVendedor", "chamando h.svc.UpdateVendedor e atribuindo resultado a vendedor, err")
	vendedor, err := h.svc.UpdateVendedor(r.Context(), h.db, id, input)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.UpdateVendedor", "verificando se err != nil")
	if err != nil {
		vlog.Printf("vendedor_handler.go", "VendedorHandler.UpdateVendedor", "chamando vendedorErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := vendedorErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[vendedores] UpdateVendedor: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.UpdateVendedor", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[vendedores] atualizado: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, vendedor, "")
}

// DeleteVendedor DELETE /api/vendedores/{id}
//
// Soft-delete: define data_desligamento = hoje, preservando o histórico de
// carteiras/pedidos vinculados ao vendedor (ver ReativarVendedor para
// reverter).
// Retorna: 200 com o vendedor atualizado, 404 se não existir.
// Admin only (JWTMiddleware com requireAdmin=true; role=normal recebe 403).
func (h *VendedorHandler) DeleteVendedor(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("vendedor_handler.go", "VendedorHandler.DeleteVendedor", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.DeleteVendedor", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.DeleteVendedor", "chamando h.svc.DeleteVendedor e atribuindo resultado a vendedor, err")
	vendedor, err := h.svc.DeleteVendedor(r.Context(), h.db, id)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.DeleteVendedor", "verificando se err != nil")
	if err != nil {
		vlog.Printf("vendedor_handler.go", "VendedorHandler.DeleteVendedor", "chamando vendedorErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := vendedorErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[vendedores] DeleteVendedor: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.DeleteVendedor", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[vendedores] inativado: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, vendedor, "")
}

// ReativarVendedor POST /api/vendedores/{id}/reativar
//
// Reverte o soft-delete: limpa data_desligamento, tornando o vendedor ativo
// novamente (ver DeleteVendedor).
// Retorna: 200 com o vendedor atualizado, 404 se não existir.
// Admin only (JWTMiddleware com requireAdmin=true; role=normal recebe 403).
func (h *VendedorHandler) ReativarVendedor(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("vendedor_handler.go", "VendedorHandler.ReativarVendedor", "chamando strconv.ParseInt e atribuindo resultado a id, err")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.ReativarVendedor", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.ReativarVendedor", "chamando h.svc.ReativarVendedor e atribuindo resultado a vendedor, err")
	vendedor, err := h.svc.ReativarVendedor(r.Context(), h.db, id)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.ReativarVendedor", "verificando se err != nil")
	if err != nil {
		vlog.Printf("vendedor_handler.go", "VendedorHandler.ReativarVendedor", "chamando vendedorErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := vendedorErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[vendedores] ReativarVendedor: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.ReativarVendedor", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[vendedores] reativado: id=%d por usuario role=%s", id, role)
	writeJSON(w, http.StatusOK, vendedor, "")
}

// VincularClienteRequest body do POST /api/vendedores/{id}/clientes.
type VincularClienteRequest struct {
	ClienteID int64 `json:"cliente_id"`
}

// VincularCliente POST /api/vendedores/{id}/clientes
//
// Inclui um cliente na carteira ativa do vendedor. Se o cliente já estiver
// vinculado a outro vendedor, o vínculo anterior é encerrado automaticamente
// e a carteira é transferida para o vendedor informado.
// Body: { "cliente_id": number }
// Retorna: 201 com o ClienteResumo do cliente vinculado, 404 se vendedor ou
// cliente não existirem, 400 se o payload for inválido.
// Admin only (JWTMiddleware com requireAdmin=true; role=normal recebe 403).
func (h *VendedorHandler) VincularCliente(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("vendedor_handler.go", "VendedorHandler.VincularCliente", "chamando strconv.ParseInt e atribuindo resultado a vendedorID, err")
	vendedorID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.VincularCliente", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.VincularCliente", "declarando variável req")
	var req VincularClienteRequest
	vlog.Printf("vendedor_handler.go", "VendedorHandler.VincularCliente", "chamando json.NewDecoder(...).Decode e atribuindo resultado a err e verificando se err != nil")
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}
	vlog.Printf("vendedor_handler.go", "VendedorHandler.VincularCliente", "verificando se req.ClienteID <= 0")
	if req.ClienteID <= 0 {
		writeJSON(w, http.StatusBadRequest, nil, "cliente_id é obrigatório")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.VincularCliente", "chamando h.svc.VincularCliente e atribuindo resultado a clienteResumo, err")
	clienteResumo, err := h.svc.VincularCliente(r.Context(), h.db, vendedorID, req.ClienteID)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.VincularCliente", "verificando se err != nil")
	if err != nil {
		vlog.Printf("vendedor_handler.go", "VendedorHandler.VincularCliente", "chamando vendedorErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := vendedorErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[vendedores] VincularCliente: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.VincularCliente", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[vendedores] cliente vinculado: vendedor_id=%d cliente_id=%d por usuario role=%s", vendedorID, req.ClienteID, role)
	writeJSON(w, http.StatusCreated, clienteResumo, "")
}

// DesvincularCliente DELETE /api/vendedores/{id}/clientes/{clienteId}
//
// Encerra o vínculo ativo entre o vendedor e o cliente informados (não afeta
// vínculos do cliente com outros vendedores).
// Retorna: 200 se encerrado com sucesso, 404 se não houver vínculo ativo
// entre os dois.
// Admin only (JWTMiddleware com requireAdmin=true; role=normal recebe 403).
func (h *VendedorHandler) DesvincularCliente(w http.ResponseWriter, r *http.Request) {
	vlog.Printf("vendedor_handler.go", "VendedorHandler.DesvincularCliente", "chamando strconv.ParseInt e atribuindo resultado a vendedorID, err")
	vendedorID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.DesvincularCliente", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "id inválido")
		return
	}
	vlog.Printf("vendedor_handler.go", "VendedorHandler.DesvincularCliente", "chamando strconv.ParseInt e atribuindo resultado a clienteID, err")
	clienteID, err := strconv.ParseInt(r.PathValue("clienteId"), 10, 64)
	vlog.Printf("vendedor_handler.go", "VendedorHandler.DesvincularCliente", "verificando se err != nil")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "cliente id inválido")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.DesvincularCliente", "chamando h.svc.DesvincularCliente e atribuindo resultado a err e verificando se err != nil")
	if err := h.svc.DesvincularCliente(r.Context(), h.db, vendedorID, clienteID); err != nil {
		vlog.Printf("vendedor_handler.go", "VendedorHandler.DesvincularCliente", "chamando vendedorErroParaStatus e atribuindo resultado a status, msg, ok e verificando se ok")
		if status, msg, ok := vendedorErroParaStatus(err); ok {
			writeJSON(w, status, nil, msg)
			return
		}
		log.Printf("[vendedores] DesvincularCliente: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vlog.Printf("vendedor_handler.go", "VendedorHandler.DesvincularCliente", "chamando middleware.GetRole e atribuindo resultado a role, _")
	role, _ := middleware.GetRole(r.Context())
	log.Printf("[vendedores] cliente desvinculado: vendedor_id=%d cliente_id=%d por usuario role=%s", vendedorID, clienteID, role)
	writeJSON(w, http.StatusOK, nil, "")
}
