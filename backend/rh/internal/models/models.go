package models

import "time"

// Vendedor representa um vendedor no sistema
type Vendedor struct {
	ID              int        `json:"id"`
	Nome            string     `json:"nome"`
	Regiao          string     `json:"regiao"`
	Uf              string     `json:"uf"`
	DataAdmissao    *time.Time `json:"data_admissao"`
	DataDesligamento *time.Time `json:"data_desligamento"`
	MetaMensal      float64    `json:"meta_mensal"`
}

// VendedorFiltros é usado para filtrar a listagem de vendedores
type VendedorFiltros struct {
	Nome  string
	Uf    string
	Ativo *bool
}

// Usuario representa um usuário do sistema
type Usuario struct {
	ID            int        `json:"id"`
	Login         string     `json:"login"`
	Email         string     `json:"email"`
	SenhaHash     string     `json:"-"`
	TipoUsuario   string     `json:"tipo_usuario"`
	VendedorID    *int       `json:"vendedor_id"`
	GerenciadoPor *int       `json:"gerenciado_por"`
	Ativo         string     `json:"ativo"`
	DataCriacao   time.Time  `json:"data_criacao"`
	DataAtualizacao *time.Time `json:"data_atualizacao"`
}

// LoginRequest representa o body da requisição de login
type LoginRequest struct {
	Login  string `json:"login"`
	Senha  string `json:"senha"`
}

// LoginResponse representa a resposta do login com sucesso
type LoginResponse struct {
	Token       string `json:"token"`
	TipoUsuario string `json:"tipo_usuario"`
	Nome        string `json:"nome"`
	ID          int    `json:"id"`
}

// TrocarSenhaRequest representa o body da requisição de troca de senha
type TrocarSenhaRequest struct {
	SenhaAtual string `json:"senha_atual"`
	NovaSenha  string `json:"nova_senha"`
}

// ResetSenhaResponse representa a resposta do reset de senha
type ResetSenhaResponse struct {
	NovaSenha string `json:"nova_senha"`
}

// ErrorResponse representa uma resposta de erro
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse representa uma resposta genérica de sucesso
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
