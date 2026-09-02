package mysql

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"backend/rh/internal/models"
	_ "github.com/go-sql-driver/mysql"
)

// UsuarioRepositoryMySQL é a implementação MySQL do UsuarioRepository
type UsuarioRepositoryMySQL struct {
	db *sql.DB
}

// NewUsuarioRepository cria uma nova instância do repositório de usuários
func NewUsuarioRepository(db *sql.DB) *UsuarioRepositoryMySQL {
	return &UsuarioRepositoryMySQL{db: db}
}

// Login busca um usuário pelo login
func (r *UsuarioRepositoryMySQL) Login(login string) (*models.Usuario, error) {
	return r.BuscarPorLogin(login)
}

// BuscarPorLogin busca um usuário pelo login
func (r *UsuarioRepositoryMySQL) BuscarPorLogin(login string) (*models.Usuario, error) {
	query := `SELECT id, login, email, senha_hash, tipo_usuario, vendedor_id, gerenciado_por, ativo, data_criacao, data_atualizacao
	          FROM usuarios WHERE login = ? LIMIT 1`

	usuario := &models.Usuario{}
	var dataAtualizacao sql.NullTime
	var vendedorID, gerenciadoPor sql.NullInt64

	err := r.db.QueryRow(query, login).Scan(
		&usuario.ID,
		&usuario.Login,
		&usuario.Email,
		&usuario.SenhaHash,
		&usuario.TipoUsuario,
		&vendedorID,
		&gerenciadoPor,
		&usuario.Ativo,
		&usuario.DataCriacao,
		&dataAtualizacao,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("usuário não encontrado")
		}
		log.Printf("Erro ao buscar usuário por login: %v", err)
		return nil, err
	}

	if vendedorID.Valid {
		vID := int(vendedorID.Int64)
		usuario.VendedorID = &vID
	}
	if gerenciadoPor.Valid {
		gID := int(gerenciadoPor.Int64)
		usuario.GerenciadoPor = &gID
	}
	if dataAtualizacao.Valid {
		usuario.DataAtualizacao = &dataAtualizacao.Time
	}

	return usuario, nil
}

// BuscarPorID busca um usuário pelo ID
func (r *UsuarioRepositoryMySQL) BuscarPorID(id int) (*models.Usuario, error) {
	query := `SELECT id, login, email, senha_hash, tipo_usuario, vendedor_id, gerenciado_por, ativo, data_criacao, data_atualizacao
	          FROM usuarios WHERE id = ? LIMIT 1`

	usuario := &models.Usuario{}
	var dataAtualizacao sql.NullTime
	var vendedorID, gerenciadoPor sql.NullInt64

	err := r.db.QueryRow(query, id).Scan(
		&usuario.ID,
		&usuario.Login,
		&usuario.Email,
		&usuario.SenhaHash,
		&usuario.TipoUsuario,
		&vendedorID,
		&gerenciadoPor,
		&usuario.Ativo,
		&usuario.DataCriacao,
		&dataAtualizacao,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("usuário não encontrado")
		}
		log.Printf("Erro ao buscar usuário por ID: %v", err)
		return nil, err
	}

	if vendedorID.Valid {
		vID := int(vendedorID.Int64)
		usuario.VendedorID = &vID
	}
	if gerenciadoPor.Valid {
		gID := int(gerenciadoPor.Int64)
		usuario.GerenciadoPor = &gID
	}
	if dataAtualizacao.Valid {
		usuario.DataAtualizacao = &dataAtualizacao.Time
	}

	return usuario, nil
}

// ListarTodos lista todos os usuários
func (r *UsuarioRepositoryMySQL) ListarTodos() ([]models.Usuario, error) {
	query := `SELECT id, login, email, senha_hash, tipo_usuario, vendedor_id, gerenciado_por, ativo, data_criacao, data_atualizacao
	          FROM usuarios ORDER BY id ASC`

	rows, err := r.db.Query(query)
	if err != nil {
		log.Printf("Erro ao listar usuários: %v", err)
		return nil, err
	}
	defer rows.Close()

	usuarios := make([]models.Usuario, 0)
	for rows.Next() {
		var usuario models.Usuario
		var dataAtualizacao sql.NullTime
		var vendedorID, gerenciadoPor sql.NullInt64

		if err := rows.Scan(
			&usuario.ID,
			&usuario.Login,
			&usuario.Email,
			&usuario.SenhaHash,
			&usuario.TipoUsuario,
			&vendedorID,
			&gerenciadoPor,
			&usuario.Ativo,
			&usuario.DataCriacao,
			&dataAtualizacao,
		); err != nil {
			log.Printf("Erro ao escanear usuário: %v", err)
			return nil, err
		}

		if vendedorID.Valid {
			vID := int(vendedorID.Int64)
			usuario.VendedorID = &vID
		}
		if gerenciadoPor.Valid {
			gID := int(gerenciadoPor.Int64)
			usuario.GerenciadoPor = &gID
		}
		if dataAtualizacao.Valid {
			usuario.DataAtualizacao = &dataAtualizacao.Time
		}

		usuarios = append(usuarios, usuario)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Erro ao iterar usuários: %v", err)
		return nil, err
	}

	return usuarios, nil
}

// Criar cria um novo usuário
func (r *UsuarioRepositoryMySQL) Criar(usuario *models.Usuario) error {
	query := `INSERT INTO usuarios (login, email, senha_hash, tipo_usuario, vendedor_id, gerenciado_por, ativo, data_criacao)
	          VALUES (?, ?, ?, ?, ?, ?, ?, NOW())`

	result, err := r.db.Exec(query,
		usuario.Login,
		usuario.Email,
		usuario.SenhaHash,
		usuario.TipoUsuario,
		usuario.VendedorID,
		usuario.GerenciadoPor,
		usuario.Ativo,
	)
	if err != nil {
		log.Printf("Erro ao criar usuário: %v", err)
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Erro ao obter ID do usuário criado: %v", err)
		return err
	}
	usuario.ID = int(id)

	return nil
}

// Atualizar atualiza um usuário existente
func (r *UsuarioRepositoryMySQL) Atualizar(usuario *models.Usuario) error {
	query := `UPDATE usuarios
	          SET email = ?, tipo_usuario = ?, vendedor_id = ?, gerenciado_por = ?, ativo = ?, data_atualizacao = NOW()
	          WHERE id = ?`

	_, err := r.db.Exec(query,
		usuario.Email,
		usuario.TipoUsuario,
		usuario.VendedorID,
		usuario.GerenciadoPor,
		usuario.Ativo,
		usuario.ID,
	)
	if err != nil {
		log.Printf("Erro ao atualizar usuário: %v", err)
		return err
	}

	return nil
}

// AtualizarSenha atualiza a senha de um usuário
func (r *UsuarioRepositoryMySQL) AtualizarSenha(id int, novaSenhaHash string) error {
	query := `UPDATE usuarios SET senha_hash = ?, data_atualizacao = NOW() WHERE id = ?`
	_, err := r.db.Exec(query, novaSenhaHash, id)
	if err != nil {
		log.Printf("Erro ao atualizar senha: %v", err)
		return err
	}
	return nil
}

// Deletar remove um usuário (soft delete)
func (r *UsuarioRepositoryMySQL) Deletar(id int) error {
	query := `UPDATE usuarios SET ativo = 'N', data_atualizacao = NOW() WHERE id = ?`
	_, err := r.db.Exec(query, id)
	if err != nil {
		log.Printf("Erro ao deletar usuário: %v", err)
		return err
	}
	return nil
}

// BuscarPorVendedorID busca um usuário pelo ID do vendedor
func (r *UsuarioRepositoryMySQL) BuscarPorVendedorID(vendedorID int) (*models.Usuario, error) {
	query := `SELECT id, login, email, senha_hash, tipo_usuario, vendedor_id, gerenciado_por, ativo, data_criacao, data_atualizacao
	          FROM usuarios WHERE vendedor_id = ? LIMIT 1`

	usuario := &models.Usuario{}
	var dataAtualizacao sql.NullTime
	var vID, gerenciadoPor sql.NullInt64

	err := r.db.QueryRow(query, vendedorID).Scan(
		&usuario.ID,
		&usuario.Login,
		&usuario.Email,
		&usuario.SenhaHash,
		&usuario.TipoUsuario,
		&vID,
		&gerenciadoPor,
		&usuario.Ativo,
		&usuario.DataCriacao,
		&dataAtualizacao,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("usuário não encontrado")
		}
		log.Printf("Erro ao buscar usuário por vendedor_id: %v", err)
		return nil, err
	}

	if vID.Valid {
		vid := int(vID.Int64)
		usuario.VendedorID = &vid
	}
	if gerenciadoPor.Valid {
		gID := int(gerenciadoPor.Int64)
		usuario.GerenciadoPor = &gID
	}
	if dataAtualizacao.Valid {
		usuario.DataAtualizacao = &dataAtualizacao.Time
	}

	return usuario, nil
}

// VendedorRepositoryMySQL é a implementação MySQL do VendedorRepository
type VendedorRepositoryMySQL struct {
	db *sql.DB
}

// NewVendedorRepository cria uma nova instância do repositório de vendedores
func NewVendedorRepository(db *sql.DB) *VendedorRepositoryMySQL {
	return &VendedorRepositoryMySQL{db: db}
}

// ListarTodos lista todos os vendedores aplicando filtros
func (r *VendedorRepositoryMySQL) ListarTodos(filtros models.VendedorFiltros) ([]models.Vendedor, error) {
	conditions := []string{"1=1"}
	args := []interface{}{}

	if filtros.Nome != "" {
		conditions = append(conditions, "nome LIKE ?")
		args = append(args, "%"+filtros.Nome+"%")
	}
	if filtros.Uf != "" {
		conditions = append(conditions, "uf = ?")
		args = append(args, filtros.Uf)
	}
	if filtros.Ativo != nil {
		if *filtros.Ativo {
			conditions = append(conditions, "(data_desligamento IS NULL OR data_desligamento = '0000-00-00' OR data_desligamento = '')")
		} else {
			conditions = append(conditions, "(data_desligamento IS NOT NULL AND data_desligamento <> '0000-00-00' AND data_desligamento <> '')")
		}
	}

	query := fmt.Sprintf(`SELECT id, nome, regiao, uf, data_admissao, data_desligamento, meta_mensal
	                     FROM vendedores WHERE %s ORDER BY nome ASC`, strings.Join(conditions, " AND "))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		log.Printf("Erro ao listar vendedores: %v", err)
		return nil, err
	}
	defer rows.Close()

	vendedores := make([]models.Vendedor, 0)
	for rows.Next() {
		var v models.Vendedor
		var dataAdmissao, dataDesligamento sql.NullTime

		if err := rows.Scan(
			&v.ID,
			&v.Nome,
			&v.Regiao,
			&v.Uf,
			&dataAdmissao,
			&dataDesligamento,
			&v.MetaMensal,
		); err != nil {
			log.Printf("Erro ao escanear vendedor: %v", err)
			return nil, err
		}

		if dataAdmissao.Valid {
			t := dataAdmissao.Time
			v.DataAdmissao = &t
		}
		if dataDesligamento.Valid {
			t := dataDesligamento.Time
			v.DataDesligamento = &t
		}

		vendedores = append(vendedores, v)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Erro ao iterar vendedores: %v", err)
		return nil, err
	}

	return vendedores, nil
}

// BuscarPorID busca um vendedor pelo ID
func (r *VendedorRepositoryMySQL) BuscarPorID(id int) (*models.Vendedor, error) {
	query := `SELECT id, nome, regiao, uf, data_admissao, data_desligamento, meta_mensal
	          FROM vendedores WHERE id = ? LIMIT 1`

	var v models.Vendedor
	var dataAdmissao, dataDesligamento sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
		&v.ID,
		&v.Nome,
		&v.Regiao,
		&v.Uf,
		&dataAdmissao,
		&dataDesligamento,
		&v.MetaMensal,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("vendedor não encontrado")
		}
		log.Printf("Erro ao buscar vendedor por ID: %v", err)
		return nil, err
	}

	if dataAdmissao.Valid {
		t := dataAdmissao.Time
		v.DataAdmissao = &t
	}
	if dataDesligamento.Valid {
		t := dataDesligamento.Time
		v.DataDesligamento = &t
	}

	return &v, nil
}

// Criar cria um novo vendedor
func (r *VendedorRepositoryMySQL) Criar(vendedor *models.Vendedor) error {
	query := `INSERT INTO vendedores (nome, regiao, uf, data_admissao, data_desligamento, meta_mensal)
	          VALUES (?, ?, ?, ?, ?, ?)`

	result, err := r.db.Exec(query,
		vendedor.Nome,
		vendedor.Regiao,
		vendedor.Uf,
		vendedor.DataAdmissao,
		vendedor.DataDesligamento,
		vendedor.MetaMensal,
	)
	if err != nil {
		log.Printf("Erro ao criar vendedor: %v", err)
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		log.Printf("Erro ao obter ID do vendedor criado: %v", err)
		return err
	}
	vendedor.ID = int(id)

	return nil
}

// Atualizar atualiza um vendedor existente
func (r *VendedorRepositoryMySQL) Atualizar(vendedor *models.Vendedor) error {
	query := `UPDATE vendedores
	          SET nome = ?, regiao = ?, uf = ?, data_admissao = ?, data_desligamento = ?, meta_mensal = ?
	          WHERE id = ?`

	_, err := r.db.Exec(query,
		vendedor.Nome,
		vendedor.Regiao,
		vendedor.Uf,
		vendedor.DataAdmissao,
		vendedor.DataDesligamento,
		vendedor.MetaMensal,
		vendedor.ID,
	)
	if err != nil {
		log.Printf("Erro ao atualizar vendedor: %v", err)
		return err
	}

	return nil
}

// NewConnection cria e retorna uma conexão com o banco MySQL
func NewConnection(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir conexão: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("erro ao pingar banco: %w", err)
	}

	return db, nil
}
