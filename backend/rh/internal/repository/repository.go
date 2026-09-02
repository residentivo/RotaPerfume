package repository

import "backend/rh/internal/models"

// UsuarioRepository define a interface para operações de usuário
type UsuarioRepository interface {
	// Login busca um usuário pelo login
	Login(login string) (*models.Usuario, error)
	// BuscarPorID busca um usuário pelo ID
	BuscarPorID(id int) (*models.Usuario, error)
	// ListarTodos lista todos os usuários
	ListarTodos() ([]models.Usuario, error)
	// Criar cria um novo usuário
	Criar(usuario *models.Usuario) error
	// Atualizar atualiza um usuário existente
	Atualizar(usuario *models.Usuario) error
	// Deletar remove um usuário (soft delete - muda ativo para N)
	Deletar(id int) error
	// BuscarPorVendedorID busca um usuário pelo ID do vendedor
	BuscarPorVendedorID(vendedorID int) (*models.Usuario, error)
	// BuscarPorLogin busca um usuário pelo login
	BuscarPorLogin(login string) (*models.Usuario, error)
	// AtualizarSenha atualiza a senha de um usuário
	AtualizarSenha(id int, novaSenhaHash string) error
}

// VendedorRepository define a interface para operações de vendedor
type VendedorRepository interface {
	// ListarTodos lista todos os vendedores com filtros
	ListarTodos(filtros models.VendedorFiltros) ([]models.Vendedor, error)
	// BuscarPorID busca um vendedor pelo ID
	BuscarPorID(id int) (*models.Vendedor, error)
	// Criar cria um novo vendedor
	Criar(vendedor *models.Vendedor) error
	// Atualizar atualiza um vendedor existente
	Atualizar(vendedor *models.Vendedor) error
}
