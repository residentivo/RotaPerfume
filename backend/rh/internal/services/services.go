package services

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"time"

	"backend/rh/internal/models"
	"backend/rh/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// JWTClaims representa as claims do token JWT
type JWTClaims struct {
	UsuarioID   int    `json:"usuario_id"`
	Login       string `json:"login"`
	TipoUsuario string `json:"tipo_usuario"`
	jwt.RegisteredClaims
}

// UsuarioService lida com a lógica de negócio de usuários
type UsuarioService struct {
	repo        repository.UsuarioRepository
	jwtSecret   string
}

// NewUsuarioService cria uma nova instância do serviço de usuário
func NewUsuarioService(repo repository.UsuarioRepository, jwtSecret string) *UsuarioService {
	return &UsuarioService{
		repo:      repo,
		jwtSecret: jwtSecret,
	}
}

// Login autentica um usuário e retorna um token JWT
func (s *UsuarioService) Login(login, senha string) (*models.LoginResponse, error) {
	usuario, err := s.repo.Login(login)
	if err != nil {
		return nil, errors.New("login ou senha inválidos")
	}

	if usuario.Ativo != "S" {
		return nil, errors.New("usuário inativo")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(usuario.SenhaHash), []byte(senha)); err != nil {
		return nil, errors.New("login ou senha inválidos")
	}

	token, err := s.gerarToken(usuario)
	if err != nil {
		log.Printf("Erro ao gerar token: %v", err)
		return nil, errors.New("erro ao gerar token de autenticação")
	}

	nome := login
	if usuario.VendedorID != nil {
		vend, err := s.repo.BuscarPorVendedorID(*usuario.VendedorID)
		if err == nil && vend != nil {
			nome = vend.Login
		}
	}

	return &models.LoginResponse{
		Token:       token,
		TipoUsuario: usuario.TipoUsuario,
		Nome:        nome,
		ID:          usuario.ID,
	}, nil
}

// gerarToken cria um token JWT para o usuário
func (s *UsuarioService) gerarToken(usuario *models.Usuario) (string, error) {
	claims := JWTClaims{
		UsuarioID:   usuario.ID,
		Login:       usuario.Login,
		TipoUsuario: usuario.TipoUsuario,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

// ValidarToken valida um token JWT e retorna as claims
func (s *UsuarioService) ValidarToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("método de assinatura inesperado: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("token inválido")
}

// Listar retorna todos os usuários
func (s *UsuarioService) Listar() ([]models.Usuario, error) {
	return s.repo.ListarTodos()
}

// BuscarPorID retorna um usuário pelo ID
func (s *UsuarioService) BuscarPorID(id int) (*models.Usuario, error) {
	return s.repo.BuscarPorID(id)
}

// CriarUsuario cria um novo usuário
func (s *UsuarioService) CriarUsuario(usuario *models.Usuario, senha string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(senha), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Erro ao gerar hash da senha: %v", err)
		return errors.New("erro ao processar senha")
	}

	usuario.SenhaHash = string(hash)
	if usuario.Ativo == "" {
		usuario.Ativo = "S"
	}

	return s.repo.Criar(usuario)
}

// AtualizarUsuario atualiza um usuário existente
func (s *UsuarioService) AtualizarUsuario(usuario *models.Usuario) error {
	return s.repo.Atualizar(usuario)
}

// TrocarSenha troca a senha de um usuário
func (s *UsuarioService) TrocarSenha(usuarioID int, senhaAtual, novaSenha string) error {
	usuario, err := s.repo.BuscarPorID(usuarioID)
	if err != nil {
		return errors.New("usuário não encontrado")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(usuario.SenhaHash), []byte(senhaAtual)); err != nil {
		return errors.New("senha atual incorreta")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(novaSenha), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Erro ao gerar hash da nova senha: %v", err)
		return errors.New("erro ao processar nova senha")
	}

	return s.repo.AtualizarSenha(usuarioID, string(hash))
}

// ResetSenha gera uma nova senha aleatória para o usuário
func (s *UsuarioService) ResetSenha(usuarioID int) (string, error) {
	novaSenha := gerarSenhaAleatoria(12)
	hash, err := bcrypt.GenerateFromPassword([]byte(novaSenha), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Erro ao gerar hash da senha: %v", err)
		return "", errors.New("erro ao gerar senha")
	}

	if err := s.repo.AtualizarSenha(usuarioID, string(hash)); err != nil {
		return "", err
	}

	return novaSenha, nil
}

// BuscarPorVendedorID retorna o usuário associado a um vendedor
func (s *UsuarioService) BuscarPorVendedorID(vendedorID int) (*models.Usuario, error) {
	return s.repo.BuscarPorVendedorID(vendedorID)
}

// gerarSenhaAleatoria gera uma senha aleatória segura
func gerarSenhaAleatoria(tam int) string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"
	bytes := make([]byte, tam)
	if _, err := rand.Read(bytes); err != nil {
		log.Printf("Erro ao gerar senha aleatória: %v", err)
		return ""
	}
	for i, b := range bytes {
		bytes[i] = chars[int(b)%len(chars)]
	}
	return string(bytes)
}

// VendedorService lida com a lógica de negócio de vendedores
type VendedorService struct {
	repo repository.VendedorRepository
}

// NewVendedorService cria uma nova instância do serviço de vendedor
func NewVendedorService(repo repository.VendedorRepository) *VendedorService {
	return &VendedorService{repo: repo}
}

// Listar retorna todos os vendedores com filtros
func (s *VendedorService) Listar(filtros models.VendedorFiltros) ([]models.Vendedor, error) {
	return s.repo.ListarTodos(filtros)
}

// BuscarPorID retorna um vendedor pelo ID
func (s *VendedorService) BuscarPorID(id int) (*models.Vendedor, error) {
	return s.repo.BuscarPorID(id)
}

// Criar cria um novo vendedor
func (s *VendedorService) Criar(vendedor *models.Vendedor) error {
	return s.repo.Criar(vendedor)
}

// Atualizar atualiza um vendedor existente
func (s *VendedorService) Atualizar(vendedor *models.Vendedor) error {
	return s.repo.Atualizar(vendedor)
}
