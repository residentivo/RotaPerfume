package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"backend/crm/internal/models"
	"backend/crm/internal/repository"
)

// DB encapsula a conexão com o MySQL
type DB struct {
	*sql.DB
}

// NewDB cria uma nova conexão com o MySQL
func NewDB(dbURL string) (*DB, error) {
	db, err := sql.Open("mysql", dbURL)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir conexão: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("erro ao pingar banco: %w", err)
	}

	return &DB{db}, nil
}

// ========== ClienteRepository ==========

// ClienteRepo implementa repository.ClienteRepository
type ClienteRepo struct {
	db *DB
}

// NewClienteRepository cria um novo repositório de clientes
func NewClienteRepository(db *DB) *ClienteRepo {
	return &ClienteRepo{db: db}
}

// Compile-time check
var _ repository.ClienteRepository = (*ClienteRepo)(nil)

func (r *ClienteRepo) ListarTodos(ctx context.Context, filtro models.ClienteFiltro) ([]models.Cliente, error) {
	query := "SELECT id, cnpj, razao_social, COALESCE(segmento, ''), COALESCE(cidade, ''), COALESCE(uf, ''), COALESCE(bairro, ''), data_cadastro, ativo FROM clientes WHERE 1=1"
	args := []interface{}{}

	if filtro.UF != "" {
		query += " AND uf = ?"
		args = append(args, filtro.UF)
	}
	if filtro.Segmento != "" {
		query += " AND segmento = ?"
		args = append(args, filtro.Segmento)
	}
	if filtro.Ativo != nil {
		query += " AND ativo = ?"
		args = append(args, *filtro.Ativo)
	}
	if filtro.Cidade != "" {
		query += " AND cidade = ?"
		args = append(args, filtro.Cidade)
	}

	query += " ORDER BY razao_social ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clientes []models.Cliente
	for rows.Next() {
		var c models.Cliente
		if err := rows.Scan(&c.ID, &c.CNPJ, &c.RazaoSocial, &c.Segmento, &c.Cidade, &c.Uf, &c.Bairro, &c.DataCadastro, &c.Ativo); err != nil {
			return nil, err
		}
		clientes = append(clientes, c)
	}

	return clientes, nil
}

func (r *ClienteRepo) BuscarPorID(ctx context.Context, id int64) (*models.Cliente, error) {
	query := "SELECT id, cnpj, razao_social, COALESCE(segmento, ''), COALESCE(cidade, ''), COALESCE(uf, ''), COALESCE(bairro, ''), data_cadastro, ativo FROM clientes WHERE id = ?"
	var c models.Cliente
	err := r.db.QueryRowContext(ctx, query, id).Scan(&c.ID, &c.CNPJ, &c.RazaoSocial, &c.Segmento, &c.Cidade, &c.Uf, &c.Bairro, &c.DataCadastro, &c.Ativo)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ClienteRepo) Criar(ctx context.Context, input models.ClienteInput) (*models.Cliente, error) {
	query := `INSERT INTO clientes (cnpj, razao_social, segmento, cidade, uf, bairro, data_cadastro, ativo) VALUES (?, ?, ?, ?, ?, ?, NOW(), ?)`
	res, err := r.db.ExecContext(ctx, query, input.CNPJ, input.RazaoSocial, input.Segmento, input.Cidade, input.Uf, input.Bairro, input.Ativo)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.BuscarPorID(ctx, id)
}

func (r *ClienteRepo) Atualizar(ctx context.Context, id int64, input models.ClienteInput) (*models.Cliente, error) {
	query := `UPDATE clientes SET cnpj=?, razao_social=?, segmento=?, cidade=?, uf=?, bairro=?, ativo=? WHERE id=?`
	_, err := r.db.ExecContext(ctx, query, input.CNPJ, input.RazaoSocial, input.Segmento, input.Cidade, input.Uf, input.Bairro, input.Ativo, id)
	if err != nil {
		return nil, err
	}
	return r.BuscarPorID(ctx, id)
}

// ========== VendedorRepository ==========

// VendedorRepo implementa repository.VendedorRepository
type VendedorRepo struct {
	db *DB
}

// NewVendedorRepository cria um novo repositório de vendedores
func NewVendedorRepository(db *DB) *VendedorRepo {
	return &VendedorRepo{db: db}
}

// Compile-time check
var _ repository.VendedorRepository = (*VendedorRepo)(nil)

func (r *VendedorRepo) ListarTodos(ctx context.Context, filtro models.VendedorFiltro) ([]models.Vendedor, error) {
	query := "SELECT id, nome, COALESCE(regiao, ''), COALESCE(uf, ''), data_admissao, data_desligamento, COALESCE(meta_mensal, 0) FROM vendedores WHERE 1=1"
	args := []interface{}{}

	if filtro.UF != "" {
		query += " AND uf = ?"
		args = append(args, filtro.UF)
	}
	if filtro.Regiao != "" {
		query += " AND regiao = ?"
		args = append(args, filtro.Regiao)
	}
	if filtro.Ativo {
		query += " AND data_desligamento IS NULL"
	}

	query += " ORDER BY nome ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vendedores []models.Vendedor
	for rows.Next() {
		var v models.Vendedor
		if err := rows.Scan(&v.ID, &v.Nome, &v.Regiao, &v.Uf, &v.DataAdmissao, &v.DataDesligamento, &v.MetaMensal); err != nil {
			return nil, err
		}
		vendedores = append(vendedores, v)
	}

	return vendedores, nil
}

func (r *VendedorRepo) BuscarPorID(ctx context.Context, id int64) (*models.Vendedor, error) {
	query := "SELECT id, nome, COALESCE(regiao, ''), COALESCE(uf, ''), data_admissao, data_desligamento, COALESCE(meta_mensal, 0) FROM vendedores WHERE id = ?"
	var v models.Vendedor
	err := r.db.QueryRowContext(ctx, query, id).Scan(&v.ID, &v.Nome, &v.Regiao, &v.Uf, &v.DataAdmissao, &v.DataDesligamento, &v.MetaMensal)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ========== CarteiraRepository ==========

// CarteiraRepo implementa repository.CarteiraRepository
type CarteiraRepo struct {
	db *DB
}

// NewCarteiraRepository cria um novo repositório de carteira
func NewCarteiraRepository(db *DB) *CarteiraRepo {
	return &CarteiraRepo{db: db}
}

// Compile-time check
var _ repository.CarteiraRepository = (*CarteiraRepo)(nil)

func (r *CarteiraRepo) ListarPorVendedor(ctx context.Context, vendedorID int64) ([]models.Carteira, error) {
	query := `SELECT c.id, c.cliente_id, c.vendedor_id, c.data_inicio, c.data_fim,
		COALESCE(cl.razao_social, ''), COALESCE(v.nome, '')
		FROM carteira c
		LEFT JOIN clientes cl ON cl.id = c.cliente_id
		LEFT JOIN vendedores v ON v.id = c.vendedor_id
		WHERE c.vendedor_id = ? AND c.data_fim IS NULL
		ORDER BY cl.razao_social`
	rows, err := r.db.QueryContext(ctx, query, vendedorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var carteiras []models.Carteira
	for rows.Next() {
		var c models.Carteira
		if err := rows.Scan(&c.ID, &c.ClienteID, &c.VendedorID, &c.DataInicio, &c.DataFim, &c.ClienteNome, &c.VendedorNome); err != nil {
			return nil, err
		}
		carteiras = append(carteiras, c)
	}
	return carteiras, nil
}

func (r *CarteiraRepo) ListarTodos(ctx context.Context) ([]models.Carteira, error) {
	query := `SELECT c.id, c.cliente_id, c.vendedor_id, c.data_inicio, c.data_fim,
		COALESCE(cl.razao_social, ''), COALESCE(v.nome, '')
		FROM carteira c
		LEFT JOIN clientes cl ON cl.id = c.cliente_id
		LEFT JOIN vendedores v ON v.id = c.vendedor_id
		ORDER BY v.nome, cl.razao_social`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var carteiras []models.Carteira
	for rows.Next() {
		var c models.Carteira
		if err := rows.Scan(&c.ID, &c.ClienteID, &c.VendedorID, &c.DataInicio, &c.DataFim, &c.ClienteNome, &c.VendedorNome); err != nil {
			return nil, err
		}
		carteiras = append(carteiras, c)
	}
	return carteiras, nil
}

func (r *CarteiraRepo) Criar(ctx context.Context, input models.CarteiraInput) (*models.Carteira, error) {
	// Encerra carteiras anteriores ativas
	_, err := r.db.ExecContext(ctx, "UPDATE carteira SET data_fim = NOW() WHERE cliente_id = ? AND data_fim IS NULL", input.ClienteID)
	if err != nil {
		return nil, err
	}

	// Insere nova carteira
	query := "INSERT INTO carteira (cliente_id, vendedor_id, data_inicio) VALUES (?, ?, NOW())"
	res, err := r.db.ExecContext(ctx, query, input.ClienteID, input.VendedorID)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	// Busca a carteira criada
	var c models.Carteira
	query2 := `SELECT c.id, c.cliente_id, c.vendedor_id, c.data_inicio, c.data_fim,
		COALESCE(cl.razao_social, ''), COALESCE(v.nome, '')
		FROM carteira c
		LEFT JOIN clientes cl ON cl.id = c.cliente_id
		LEFT JOIN vendedores v ON v.id = c.vendedor_id
		WHERE c.id = ?`
	err = r.db.QueryRowContext(ctx, query2, id).Scan(&c.ID, &c.ClienteID, &c.VendedorID, &c.DataInicio, &c.DataFim, &c.ClienteNome, &c.VendedorNome)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ========== VisitaRepository ==========

// VisitaRepo implementa repository.VisitaRepository
type VisitaRepo struct {
	db *DB
}

// NewVisitaRepository cria um novo repositório de visitas
func NewVisitaRepository(db *DB) *VisitaRepo {
	return &VisitaRepo{db: db}
}

// Compile-time check
var _ repository.VisitaRepository = (*VisitaRepo)(nil)

func (r *VisitaRepo) ListarPorVendedor(ctx context.Context, vendedorID int64) ([]models.Visita, error) {
	query := `SELECT vi.id, vi.cliente_id, vi.vendedor_id, vi.data_visita, COALESCE(vi.resultado, ''), COALESCE(vi.duracao_min, 0),
		COALESCE(cl.razao_social, ''), COALESCE(v.nome, '')
		FROM visitas vi
		LEFT JOIN clientes cl ON cl.id = vi.cliente_id
		LEFT JOIN vendedores v ON v.id = vi.vendedor_id
		WHERE vi.vendedor_id = ?
		ORDER BY vi.data_visita DESC`
	rows, err := r.db.QueryContext(ctx, query, vendedorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var visitas []models.Visita
	for rows.Next() {
		var v models.Visita
		if err := rows.Scan(&v.ID, &v.ClienteID, &v.VendedorID, &v.DataVisita, &v.Resultado, &v.DuracaoMin, &v.ClienteNome, &v.VendedorNome); err != nil {
			return nil, err
		}
		visitas = append(visitas, v)
	}
	return visitas, nil
}

func (r *VisitaRepo) ListarPorCliente(ctx context.Context, clienteID int64) ([]models.Visita, error) {
	query := `SELECT vi.id, vi.cliente_id, vi.vendedor_id, vi.data_visita, COALESCE(vi.resultado, ''), COALESCE(vi.duracao_min, 0),
		COALESCE(cl.razao_social, ''), COALESCE(v.nome, '')
		FROM visitas vi
		LEFT JOIN clientes cl ON cl.id = vi.cliente_id
		LEFT JOIN vendedores v ON v.id = vi.vendedor_id
		WHERE vi.cliente_id = ?
		ORDER BY vi.data_visita DESC`
	rows, err := r.db.QueryContext(ctx, query, clienteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var visitas []models.Visita
	for rows.Next() {
		var v models.Visita
		if err := rows.Scan(&v.ID, &v.ClienteID, &v.VendedorID, &v.DataVisita, &v.Resultado, &v.DuracaoMin, &v.ClienteNome, &v.VendedorNome); err != nil {
			return nil, err
		}
		visitas = append(visitas, v)
	}
	return visitas, nil
}

func (r *VisitaRepo) Criar(ctx context.Context, input models.VisitaInput) (*models.Visita, error) {
	dataVisita := input.DataVisita
	if dataVisita == "" {
		dataVisita = time.Now().Format("2006-01-02 15:04:05")
	}
	if !strings.Contains(dataVisita, " ") {
		dataVisita = dataVisita + " " + time.Now().Format("15:04:05")
	}

	query := "INSERT INTO visitas (cliente_id, vendedor_id, data_visita, resultado, duracao_min) VALUES (?, ?, ?, ?, ?)"
	res, err := r.db.ExecContext(ctx, query, input.ClienteID, input.VendedorID, dataVisita, input.Resultado, input.DuracaoMin)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	var v models.Visita
	query2 := `SELECT vi.id, vi.cliente_id, vi.vendedor_id, vi.data_visita, COALESCE(vi.resultado, ''), COALESCE(vi.duracao_min, 0),
		COALESCE(cl.razao_social, ''), COALESCE(v.nome, '')
		FROM visitas vi
		LEFT JOIN clientes cl ON cl.id = vi.cliente_id
		LEFT JOIN vendedores v ON v.id = vi.vendedor_id
		WHERE vi.id = ?`
	err = r.db.QueryRowContext(ctx, query2, id).Scan(&v.ID, &v.ClienteID, &v.VendedorID, &v.DataVisita, &v.Resultado, &v.DuracaoMin, &v.ClienteNome, &v.VendedorNome)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// ========== OportunidadeRepository ==========

// OportunidadeRepo implementa repository.OportunidadeRepository
type OportunidadeRepo struct {
	db *DB
}

// NewOportunidadeRepository cria um novo repositório de oportunidades
func NewOportunidadeRepository(db *DB) *OportunidadeRepo {
	return &OportunidadeRepo{db: db}
}

// Compile-time check
var _ repository.OportunidadeRepository = (*OportunidadeRepo)(nil)

func (r *OportunidadeRepo) ListarTodos(ctx context.Context, filtro models.OportunidadeFiltro) ([]models.Oportunidade, error) {
	query := `SELECT o.id, o.cliente_id, o.vendedor_id, COALESCE(o.origem, ''), o.data_abertura, COALESCE(o.etapa, ''),
		COALESCE(o.probabilidade_pct, 0), COALESCE(o.valor_estimado, 0), o.data_fechamento, COALESCE(o.ciclo_dias, 0), o.motivo_perda,
		COALESCE(cl.razao_social, ''), COALESCE(v.nome, '')
		FROM oportunidades o
		LEFT JOIN clientes cl ON cl.id = o.cliente_id
		LEFT JOIN vendedores v ON v.id = o.vendedor_id
		WHERE 1=1`
	args := []interface{}{}

	if filtro.VendedorID > 0 {
		query += " AND o.vendedor_id = ?"
		args = append(args, filtro.VendedorID)
	}
	if filtro.Etapa != "" {
		query += " AND o.etapa = ?"
		args = append(args, filtro.Etapa)
	}

	query += " ORDER BY o.data_abertura DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var oportunidades []models.Oportunidade
	for rows.Next() {
		var o models.Oportunidade
		if err := rows.Scan(&o.ID, &o.ClienteID, &o.VendedorID, &o.Origem, &o.DataAbertura, &o.Etapa,
			&o.ProbabilidadePct, &o.ValorEstimado, &o.DataFechamento, &o.CicloDias, &o.MotivoPerda, &o.ClienteNome, &o.VendedorNome); err != nil {
			return nil, err
		}
		oportunidades = append(oportunidades, o)
	}
	return oportunidades, nil
}

func (r *OportunidadeRepo) BuscarPorID(ctx context.Context, id int64) (*models.Oportunidade, error) {
	query := `SELECT o.id, o.cliente_id, o.vendedor_id, COALESCE(o.origem, ''), o.data_abertura, COALESCE(o.etapa, ''),
		COALESCE(o.probabilidade_pct, 0), COALESCE(o.valor_estimado, 0), o.data_fechamento, COALESCE(o.ciclo_dias, 0), o.motivo_perda,
		COALESCE(cl.razao_social, ''), COALESCE(v.nome, '')
		FROM oportunidades o
		LEFT JOIN clientes cl ON cl.id = o.cliente_id
		LEFT JOIN vendedores v ON v.id = o.vendedor_id
		WHERE o.id = ?`
	var o models.Oportunidade
	err := r.db.QueryRowContext(ctx, query, id).Scan(&o.ID, &o.ClienteID, &o.VendedorID, &o.Origem, &o.DataAbertura, &o.Etapa,
		&o.ProbabilidadePct, &o.ValorEstimado, &o.DataFechamento, &o.CicloDias, &o.MotivoPerda, &o.ClienteNome, &o.VendedorNome)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *OportunidadeRepo) Criar(ctx context.Context, input models.OportunidadeInput) (*models.Oportunidade, error) {
	etapa := input.Etapa
	if etapa == "" {
		etapa = "Prospecção"
	}
	origem := input.Origem
	if origem == "" {
		origem = "Carteira"
	}

	query := `INSERT INTO oportunidades (cliente_id, vendedor_id, origem, data_abertura, etapa, probabilidade_pct, valor_estimado)
		VALUES (?, ?, ?, NOW(), ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, input.ClienteID, input.VendedorID, origem, etapa, input.ProbabilidadePct, input.ValorEstimado)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.BuscarPorID(ctx, id)
}

func (r *OportunidadeRepo) Atualizar(ctx context.Context, id int64, input models.OportunidadeInput) (*models.Oportunidade, error) {
	// Busca oportunidade atual
	atual, err := r.BuscarPorID(ctx, id)
	if err != nil {
		return nil, err
	}
	if atual == nil {
		return nil, sql.ErrNoRows
	}

	etapa := input.Etapa
	if etapa == "" {
		etapa = atual.Etapa
	}
	motivoPerda := input.MotivoPerda
	if motivoPerda == "" && etapa == "Perdida" {
		motivoPerda = "Não informado"
	}

	// Calcula ciclo em dias
	cicloDias := atual.CicloDias
	dataFechamento := atual.DataFechamento
	if etapa == "Fechada" || etapa == "Perdida" {
		if !atual.DataFechamento.Valid {
			cicloDias = int(time.Since(atual.DataAbertura).Hours() / 24)
		}
		if input.DataFechamento != "" {
			t, err := time.Parse("2006-01-02 15:04:05", input.DataFechamento)
			if err == nil {
				dataFechamento = sql.NullTime{Time: t, Valid: true}
			}
		} else if !dataFechamento.Valid {
			dataFechamento = sql.NullTime{Time: time.Now(), Valid: true}
		}
	}

	query := `UPDATE oportunidades SET origem=?, etapa=?, probabilidade_pct=?, valor_estimado=?, data_fechamento=?, ciclo_dias=?, motivo_perda=? WHERE id=?`
	_, err = r.db.ExecContext(ctx, query, input.Origem, etapa, input.ProbabilidadePct, input.ValorEstimado, dataFechamento, cicloDias, motivoPerda, id)
	if err != nil {
		return nil, err
	}
	return r.BuscarPorID(ctx, id)
}

// ========== UsuarioRepository ==========

// UsuarioRepo implementa repository.UsuarioRepository
type UsuarioRepo struct {
	db *DB
}

// NewUsuarioRepository cria um novo repositório de usuários
func NewUsuarioRepository(db *DB) *UsuarioRepo {
	return &UsuarioRepo{db: db}
}

// Compile-time check
var _ repository.UsuarioRepository = (*UsuarioRepo)(nil)

func (r *UsuarioRepo) BuscarPorLogin(ctx context.Context, login string) (*models.Usuario, error) {
	query := "SELECT id, login, email, senha_hash, tipo_usuario, vendedor_id, gerenciado_por, ativo FROM usuarios WHERE login = ? LIMIT 1"
	var u models.Usuario
	err := r.db.QueryRowContext(ctx, query, login).Scan(&u.ID, &u.Login, &u.Email, &u.SenhaHash, &u.TipoUsuario, &u.VendedorID, &u.GerenciadoPor, &u.Ativo)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ========== DashboardRepository ==========

// DashboardRepo implementa repository.DashboardRepository
type DashboardRepo struct {
	db *DB
}

// NewDashboardRepository cria um novo repositório de dashboard
func NewDashboardRepository(db *DB) *DashboardRepo {
	return &DashboardRepo{db: db}
}

// Compile-time check
var _ repository.DashboardRepository = (*DashboardRepo)(nil)

func (r *DashboardRepo) RankingVendedores(ctx context.Context) ([]models.RankingVendedor, error) {
	query := `SELECT v.id, v.nome, COALESCE(SUM(o.valor_estimado), 0) AS total, COUNT(o.id) AS quantidade
		FROM vendedores v
		LEFT JOIN oportunidades o ON o.vendedor_id = v.id AND o.etapa = 'Fechada'
		GROUP BY v.id, v.nome
		HAVING total > 0
		ORDER BY total DESC
		LIMIT 20`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ranking []models.RankingVendedor
	for rows.Next() {
		var rk models.RankingVendedor
		if err := rows.Scan(&rk.VendedorID, &rk.VendedorNome, &rk.TotalFechado, &rk.Quantidade); err != nil {
			return nil, err
		}
		ranking = append(ranking, rk)
	}
	return ranking, nil
}

func (r *DashboardRepo) DistribuicaoCarteira(ctx context.Context) ([]models.DistribuicaoCarteira, error) {
	query := `SELECT v.id, v.nome, COUNT(c.id) AS quantidade
		FROM vendedores v
		LEFT JOIN carteira c ON c.vendedor_id = v.id AND c.data_fim IS NULL
		GROUP BY v.id, v.nome
		ORDER BY quantidade DESC, v.nome`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dist []models.DistribuicaoCarteira
	for rows.Next() {
		var d models.DistribuicaoCarteira
		if err := rows.Scan(&d.VendedorID, &d.VendedorNome, &d.Quantidade); err != nil {
			return nil, err
		}
		dist = append(dist, d)
	}
	return dist, nil
}
