package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"backend/rh/internal/models"
	"backend/rh/internal/services"

	"github.com/gorilla/mux"
)

// VendedorHandler lida com as requisições de vendedores
type VendedorHandler struct {
	vendedorService *services.VendedorService
}

// NewVendedorHandler cria uma nova instância do handler de vendedores
func NewVendedorHandler(vendedorService *services.VendedorService) *VendedorHandler {
	return &VendedorHandler{vendedorService: vendedorService}
}

// Listar lista todos os vendedores com filtros
// GET /api/vendedores
func (h *VendedorHandler) Listar(w http.ResponseWriter, r *http.Request) {
	filtros := models.VendedorFiltros{
		Nome: r.URL.Query().Get("nome"),
		Uf:   r.URL.Query().Get("uf"),
	}

	if ativo := r.URL.Query().Get("ativo"); ativo != "" {
		v := ativo == "true" || ativo == "1" || ativo == "S"
		filtros.Ativo = &v
	}

	vendedores, err := h.vendedorService.Listar(filtros)
	if err != nil {
		log.Printf("Erro ao listar vendedores: %v", err)
		escreverJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "erro ao listar vendedores"})
		return
	}

	escreverJSON(w, http.StatusOK, vendedores)
}

// BuscarPorID busca um vendedor pelo ID
// GET /api/vendedores/{id}
func (h *VendedorHandler) BuscarPorID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "ID inválido"})
		return
	}

	vendedor, err := h.vendedorService.BuscarPorID(id)
	if err != nil {
		log.Printf("Erro ao buscar vendedor %d: %v", id, err)
		escreverJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "vendedor não encontrado"})
		return
	}

	escreverJSON(w, http.StatusOK, vendedor)
}

// Criar cria um novo vendedor
// POST /api/vendedores
func (h *VendedorHandler) Criar(w http.ResponseWriter, r *http.Request) {
	var vendedor models.Vendedor
	if err := json.NewDecoder(r.Body).Decode(&vendedor); err != nil {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "dados inválidos"})
		return
	}

	if vendedor.Nome == "" {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "nome é obrigatório"})
		return
	}

	if err := h.vendedorService.Criar(&vendedor); err != nil {
		log.Printf("Erro ao criar vendedor: %v", err)
		escreverJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "erro ao criar vendedor"})
		return
	}

	escreverJSON(w, http.StatusCreated, vendedor)
}

// Atualizar atualiza um vendedor existente
// PUT /api/vendedores/{id}
func (h *VendedorHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "ID inválido"})
		return
	}

	vendedor, err := h.vendedorService.BuscarPorID(id)
	if err != nil {
		escreverJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "vendedor não encontrado"})
		return
	}

	var req models.Vendedor
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "dados inválidos"})
		return
	}

	vendedor.Nome = req.Nome
	vendedor.Regiao = req.Regiao
	vendedor.Uf = req.Uf
	vendedor.DataAdmissao = req.DataAdmissao
	vendedor.DataDesligamento = req.DataDesligamento
	vendedor.MetaMensal = req.MetaMensal

	if err := h.vendedorService.Atualizar(vendedor); err != nil {
		log.Printf("Erro ao atualizar vendedor %d: %v", id, err)
		escreverJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "erro ao atualizar vendedor"})
		return
	}

	escreverJSON(w, http.StatusOK, vendedor)
}

// UsuarioHandler lida com as requisições de usuários
type UsuarioHandler struct {
	usuarioService *services.UsuarioService
}

// NewUsuarioHandler cria uma nova instância do handler de usuários
func NewUsuarioHandler(usuarioService *services.UsuarioService) *UsuarioHandler {
	return &UsuarioHandler{usuarioService: usuarioService}
}

// Listar lista todos os usuários
// GET /api/usuarios
func (h *UsuarioHandler) Listar(w http.ResponseWriter, r *http.Request) {
	usuarios, err := h.usuarioService.Listar()
	if err != nil {
		log.Printf("Erro ao listar usuários: %v", err)
		escreverJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "erro ao listar usuários"})
		return
	}

	escreverJSON(w, http.StatusOK, usuarios)
}

// BuscarPorID busca um usuário pelo ID
// GET /api/usuarios/{id}
func (h *UsuarioHandler) BuscarPorID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "ID inválido"})
		return
	}

	usuario, err := h.usuarioService.BuscarPorID(id)
	if err != nil {
		escreverJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "usuário não encontrado"})
		return
	}

	escreverJSON(w, http.StatusOK, usuario)
}

// Criar cria um novo usuário
// POST /api/usuarios
func (h *UsuarioHandler) Criar(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Login       string `json:"login"`
		Email       string `json:"email"`
		Senha       string `json:"senha"`
		TipoUsuario string `json:"tipo_usuario"`
		VendedorID  *int   `json:"vendedor_id"`
		GerenciadoPor *int  `json:"gerenciado_por"`
		Ativo       string `json:"ativo"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "dados inválidos"})
		return
	}

	if req.Login == "" || req.Email == "" || req.Senha == "" || req.TipoUsuario == "" {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "login, email, senha e tipo_usuario são obrigatórios"})
		return
	}

	if req.TipoUsuario != "vendedor" && req.TipoUsuario != "gerente" && req.TipoUsuario != "rh" {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "tipo_usuario inválido: use vendedor, gerente ou rh"})
		return
	}

	usuario := &models.Usuario{
		Login:         req.Login,
		Email:         req.Email,
		TipoUsuario:   req.TipoUsuario,
		VendedorID:    req.VendedorID,
		GerenciadoPor: req.GerenciadoPor,
		Ativo:         req.Ativo,
	}

	if usuario.Ativo == "" {
		usuario.Ativo = "S"
	}

	if err := h.usuarioService.CriarUsuario(usuario, req.Senha); err != nil {
		log.Printf("Erro ao criar usuário: %v", err)
		escreverJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "erro ao criar usuário"})
		return
	}

	escreverJSON(w, http.StatusCreated, usuario)
}

// Atualizar atualiza um usuário existente
// PUT /api/usuarios/{id}
func (h *UsuarioHandler) Atualizar(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "ID inválido"})
		return
	}

	usuario, err := h.usuarioService.BuscarPorID(id)
	if err != nil {
		escreverJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "usuário não encontrado"})
		return
	}

	var req struct {
		Email          string `json:"email"`
		TipoUsuario    string `json:"tipo_usuario"`
		VendedorID     *int   `json:"vendedor_id"`
		GerenciadoPor  *int   `json:"gerenciado_por"`
		Ativo          string `json:"ativo"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "dados inválidos"})
		return
	}

	if req.Email != "" {
		usuario.Email = req.Email
	}
	if req.TipoUsuario != "" {
		if req.TipoUsuario != "vendedor" && req.TipoUsuario != "gerente" && req.TipoUsuario != "rh" {
			escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "tipo_usuario inválido"})
			return
		}
		usuario.TipoUsuario = req.TipoUsuario
	}
	if req.VendedorID != nil {
		usuario.VendedorID = req.VendedorID
	}
	if req.GerenciadoPor != nil {
		usuario.GerenciadoPor = req.GerenciadoPor
	}
	if req.Ativo != "" {
		usuario.Ativo = req.Ativo
	}

	if err := h.usuarioService.AtualizarUsuario(usuario); err != nil {
		log.Printf("Erro ao atualizar usuário %d: %v", id, err)
		escreverJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "erro ao atualizar usuário"})
		return
	}

	escreverJSON(w, http.StatusOK, usuario)
}

// ResetSenha gera uma nova senha aleatória para o usuário
// POST /api/usuarios/{id}/reset-senha
func (h *UsuarioHandler) ResetSenha(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "ID inválido"})
		return
	}

	novaSenha, err := h.usuarioService.ResetSenha(id)
	if err != nil {
		log.Printf("Erro ao resetar senha do usuário %d: %v", id, err)
		escreverJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "erro ao resetar senha"})
		return
	}

	escreverJSON(w, http.StatusOK, models.ResetSenhaResponse{NovaSenha: novaSenha})
}

// BuscarVendedor retorna o vendedor associado ao usuário
// GET /api/usuarios/{id}/vendedor
func (h *UsuarioHandler) BuscarVendedor(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	// Se o parâmetro for um ID de usuário
	id, err := strconv.Atoi(idStr)
	if err != nil {
		escreverJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "ID inválido"})
		return
	}

	usuario, err := h.usuarioService.BuscarPorID(id)
	if err != nil {
		escreverJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "usuário não encontrado"})
		return
	}

	if usuario.VendedorID == nil {
		escreverJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "usuário não possui vendedor associado"})
		return
	}

	vendedor, err := h.usuarioService.BuscarPorVendedorID(*usuario.VendedorID)
	if err != nil {
		escreverJSON(w, http.StatusNotFound, models.ErrorResponse{Error: "vendedor não encontrado"})
		return
	}

	escreverJSON(w, http.StatusOK, vendedor)
}

// HealthHandler lida com a verificação de saúde da API
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	escreverJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// escreverJSON é um helper para escrever respostas JSON
func escreverJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Erro ao codificar JSON: %v", err)
	}
}
