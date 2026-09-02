package models

import (
	"database/sql"
	"time"
)

// Cliente representa um cliente do CRM
type Cliente struct {
	ID           int64          `json:"id"`
	CNPJ         string         `json:"cnpj"`
	RazaoSocial  string         `json:"razao_social"`
	Segmento     string         `json:"segmento"`
	Cidade       string         `json:"cidade"`
	Uf           string         `json:"uf"`
	Bairro       string         `json:"bairro"`
	DataCadastro time.Time      `json:"data_cadastro"`
	Ativo        string         `json:"ativo"`
}

// ClienteInput representa os dados de entrada para criar/atualizar cliente
type ClienteInput struct {
	CNPJ        string `json:"cnpj"`
	RazaoSocial string `json:"razao_social"`
	Segmento    string `json:"segmento"`
	Cidade      string `json:"cidade"`
	Uf          string `json:"uf"`
	Bairro      string `json:"bairro"`
	Ativo       string `json:"ativo"`
}

// ClienteFiltro representa filtros para listagem de clientes
type ClienteFiltro struct {
	UF       string
	Segmento string
	Ativo    *string
	Cidade   string
}

// Vendedor representa um vendedor do CRM
type Vendedor struct {
	ID              int64          `json:"id"`
	Nome            string         `json:"nome"`
	Regiao          string         `json:"regiao"`
	Uf              string         `json:"uf"`
	DataAdmissao    time.Time      `json:"data_admissao"`
	DataDesligamento sql.NullTime  `json:"data_desligamento"`
	MetaMensal      float64        `json:"meta_mensal"`
}

// VendedorFiltro representa filtros para listagem de vendedores
type VendedorFiltro struct {
	UF      string
	Regiao  string
	Ativo   bool
}

// Carteira representa a associação entre cliente e vendedor
type Carteira struct {
	ID          int64         `json:"id"`
	ClienteID   int64         `json:"cliente_id"`
	VendedorID  int64         `json:"vendedor_id"`
	DataInicio  time.Time     `json:"data_inicio"`
	DataFim     sql.NullTime  `json:"data_fim"`
	// Dados extras para JOIN
	ClienteNome   string `json:"cliente_nome,omitempty"`
	VendedorNome  string `json:"vendedor_nome,omitempty"`
}

// CarteiraInput representa dados de entrada para criar associação
type CarteiraInput struct {
	ClienteID  int64 `json:"cliente_id"`
	VendedorID int64 `json:"vendedor_id"`
}

// Visita representa uma visita a um cliente
type Visita struct {
	ID            int64     `json:"id"`
	ClienteID     int64     `json:"cliente_id"`
	VendedorID    int64     `json:"vendedor_id"`
	DataVisita    time.Time `json:"data_visita"`
	Resultado     string    `json:"resultado"`
	DuracaoMin    int       `json:"duracao_min"`
	// Dados extras para JOIN
	ClienteNome  string `json:"cliente_nome,omitempty"`
	VendedorNome string `json:"vendedor_nome,omitempty"`
}

// VisitaInput representa dados de entrada para criar visita
type VisitaInput struct {
	ClienteID  int64 `json:"cliente_id"`
	VendedorID int64 `json:"vendedor_id"`
	DataVisita string `json:"data_visita"`
	Resultado  string `json:"resultado"`
	DuracaoMin int    `json:"duracao_min"`
}

// Oportunidade representa uma oportunidade de venda
type Oportunidade struct {
	ID               int64          `json:"id"`
	ClienteID        int64          `json:"cliente_id"`
	VendedorID       int64          `json:"vendedor_id"`
	Origem           string         `json:"origem"`
	DataAbertura     time.Time      `json:"data_abertura"`
	Etapa            string         `json:"etapa"`
	ProbabilidadePct int            `json:"probabilidade_pct"`
	ValorEstimado    float64        `json:"valor_estimado"`
	DataFechamento   sql.NullTime   `json:"data_fechamento"`
	CicloDias        int            `json:"ciclo_dias"`
	MotivoPerda      sql.NullString `json:"motivo_perda"`
	// Dados extras para JOIN
	ClienteNome  string `json:"cliente_nome,omitempty"`
	VendedorNome string `json:"vendedor_nome,omitempty"`
}

// OportunidadeInput representa dados de entrada para criar/atualizar oportunidade
type OportunidadeInput struct {
	ClienteID        int64   `json:"cliente_id"`
	VendedorID       int64   `json:"vendedor_id"`
	Origem           string  `json:"origem"`
	Etapa            string  `json:"etapa"`
	ProbabilidadePct int     `json:"probabilidade_pct"`
	ValorEstimado    float64 `json:"valor_estimado"`
	DataFechamento   string  `json:"data_fechamento"`
	MotivoPerda      string  `json:"motivo_perda"`
}

// OportunidadeFiltro representa filtros para listagem de oportunidades
type OportunidadeFiltro struct {
	VendedorID int64
	Etapa      string
}

// Usuario representa um usuário do sistema
type Usuario struct {
	ID           int64          `json:"id"`
	Login        string         `json:"login"`
	Email        string         `json:"email"`
	SenhaHash    string         `json:"-"`
	TipoUsuario  string         `json:"tipo_usuario"`
	VendedorID   sql.NullInt64  `json:"vendedor_id"`
	GerenciadoPor sql.NullInt64 `json:"gerenciado_por"`
	Ativo        string         `json:"ativo"`
}

// LoginRequest representa a requisição de login
type LoginRequest struct {
	Login string `json:"login"`
	Senha string `json:"senha"`
}

// LoginResponse representa a resposta de login
type LoginResponse struct {
	Token   string   `json:"token"`
	Usuario *Usuario `json:"usuario"`
}

// RankingVendedor representa ranking de vendedores por vendas fechadas
type RankingVendedor struct {
	VendedorID    int64   `json:"vendedor_id"`
	VendedorNome  string  `json:"vendedor_nome"`
	TotalFechado  float64 `json:"total_fechado"`
	Quantidade    int     `json:"quantidade"`
}

// DistribuicaoCarteira representa distribuição de clientes por vendedor
type DistribuicaoCarteira struct {
	VendedorID   int64  `json:"vendedor_id"`
	VendedorNome string `json:"vendedor_nome"`
	Quantidade   int    `json:"quantidade"`
}

// ErrorResponse representa uma resposta de erro
type ErrorResponse struct {
	Erro string `json:"erro"`
}

// SuccessResponse representa uma resposta de sucesso
type SuccessResponse struct {
	Mensagem string      `json:"mensagem"`
	Dados    interface{} `json:"dados,omitempty"`
}
