// Package repositories contém funções puras de acesso a dados (stateless).
//
// Recebem *sql.DB explicitamente para evitar ciclo de dependência com
// os services da API. Cada função executa sua própria query (sem cache L1).
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/vlog"
)

// ErrNotFound é retornado quando um registro não é encontrado.
var ErrNotFound = errors.New("repositories: registro não encontrado")

// UsuarioRepository agrupa queries da tabela usuarios.
type UsuarioRepository struct{}

// NewUsuarioRepository cria um repositório stateless.
func NewUsuarioRepository() *UsuarioRepository {
	return &UsuarioRepository{}
}

// usuarioSelectComVendedor é a base do SELECT com LEFT JOIN em vendedores,
// usada por GetByEmail, GetByID e List para evitar N+1 queries ao expor o
// nome do vendedor vinculado.
const usuarioSelectComVendedor = `
	SELECT u.id, u.nome, u.email, u.password_hash, u.role, u.id_vendedor, u.ativo,
	       u.deve_trocar_senha, u.created_at, u.updated_at, u.ultimo_login_at, v.nome
	FROM usuarios u
	LEFT JOIN vendedores v ON v.id = u.id_vendedor`

// GetByEmail busca um usuário pelo email. Retorna ErrNotFound se não existir.
func (r *UsuarioRepository) GetByEmail(ctx context.Context, db *sql.DB, email string) (*models.Usuario, error) {
	vlog.Printf("usuario_repository.go", "UsuarioRepository.GetByEmail", "definindo q = usuarioSelectComVendedor + ` WHERE u.email = ? LIMIT 1`")
	q := usuarioSelectComVendedor + `
		WHERE u.email = ?
		LIMIT 1`
	vlog.Printf("usuario_repository.go", "UsuarioRepository.GetByEmail", "definindo row com resultado de execução SQL via db.QueryRowContext (query q, args omitidos)")
	row := db.QueryRowContext(ctx, q, email)
	return scanUsuario(row)
}

// GetByID busca um usuário pelo ID. Retorna ErrNotFound se não existir.
func (r *UsuarioRepository) GetByID(ctx context.Context, db *sql.DB, id int64) (*models.Usuario, error) {
	vlog.Printf("usuario_repository.go", "UsuarioRepository.GetByID", "definindo q = usuarioSelectComVendedor + ` WHERE u.id = ? LIMIT 1`")
	q := usuarioSelectComVendedor + `
		WHERE u.id = ?
		LIMIT 1`
	vlog.Printf("usuario_repository.go", "UsuarioRepository.GetByID", "definindo row com resultado de execução SQL via db.QueryRowContext (query q, args omitidos)")
	row := db.QueryRowContext(ctx, q, id)
	return scanUsuario(row)
}

// UsuarioStatus é o recorte mínimo de um usuário usado na autorização de
// cada requisição autenticada (SEC-06): se ainda está ativo e qual o role
// vigente no banco (que prevalece sobre o role gravado no JWT).
//
// TokensValidosDesde (SEC-08) é o corte de sessão: access tokens emitidos
// (iat) até esse instante, inclusive, não valem mais. nil = sem corte.
type UsuarioStatus struct {
	Ativo              bool
	Role               string
	TokensValidosDesde *time.Time
}

// GetStatusByID retorna ativo, role e o corte de sessão do usuário. Retorna
// ErrNotFound se o usuário não existir. Consulta enxuta (sem JOIN) executada
// a cada request protegido — sem cache, para que inativação/rebaixamento/
// corte de sessão valham na hora.
func (r *UsuarioRepository) GetStatusByID(ctx context.Context, db *sql.DB, id int64) (*UsuarioStatus, error) {
	const q = `SELECT ativo, role, tokens_validos_desde FROM usuarios WHERE id = ? LIMIT 1`
	vlog.Printf("usuario_repository.go", "UsuarioRepository.GetStatusByID", "declarando variável st")
	var st UsuarioStatus
	vlog.Printf("usuario_repository.go", "UsuarioRepository.GetStatusByID", "declarando variável corte")
	var corte sql.NullTime
	vlog.Printf("usuario_repository.go", "UsuarioRepository.GetStatusByID", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query q, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, q, id).Scan(&st.Ativo, &st.Role, &corte); err != nil {
		vlog.Printf("usuario_repository.go", "UsuarioRepository.GetStatusByID", "verificando se errors.Is(err, sql.ErrNoRows)")
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: get status usuario: %w", err)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.GetStatusByID", "verificando se corte.Valid")
	if corte.Valid {
		vlog.Printf("usuario_repository.go", "UsuarioRepository.GetStatusByID", "definindo t = corte.Time")
		t := corte.Time
		vlog.Printf("usuario_repository.go", "UsuarioRepository.GetStatusByID", "atribuindo st.TokensValidosDesde = &t")
		st.TokensValidosDesde = &t
	}
	return &st, nil
}

// corteDeSessaoAgora é o instante gravado em tokens_validos_desde (SEC-08).
// Sempre gerado pelo Go (nunca NOW() do MySQL; DSN usa loc=Local) e truncado
// em segundos, mesma precisão do iat do JWT.
func corteDeSessaoAgora() time.Time {
	return time.Now().Truncate(time.Second)
}

// InvalidarSessoes grava o corte de sessão do usuário (SEC-08): todo access
// token com iat <= t passa a ser recusado pelo middleware. Usado na
// revogação em massa por reuso de refresh token (SEC-07). Retorna
// ErrNotFound se o usuário não existir. Aceita *sql.DB ou *sql.Tx.
func (r *UsuarioRepository) InvalidarSessoes(ctx context.Context, db Execer, id int64, t time.Time) error {
	const q = `UPDATE usuarios SET tokens_validos_desde = ? WHERE id = ?`
	// Trunca em segundos (mesma regra de corteDeSessaoAgora): o iat do JWT tem
	// precisão de segundo e o middleware recusa iat <= corte. Um DATETIME sem
	// fração no MySQL ARREDONDA a fração (10:00:00.7 vira 10:00:01), o que
	// derrubaria também um token legítimo emitido no segundo seguinte (ex.: o
	// novo login logo após a revogação). Truncar no Go torna o valor gravado
	// determinístico e independente do chamador passar t com fração.
	vlog.Printf("usuario_repository.go", "UsuarioRepository.InvalidarSessoes", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q, t.Truncate(time.Second), id)
	vlog.Printf("usuario_repository.go", "UsuarioRepository.InvalidarSessoes", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: invalidar sessoes: %w", err)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.InvalidarSessoes", "definindo n, _ com resultado de chamada a res.RowsAffected")
	n, _ := res.RowsAffected()
	vlog.Printf("usuario_repository.go", "UsuarioRepository.InvalidarSessoes", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetIDVendedorByUsuarioID retorna o id_vendedor vinculado ao usuário
// informado (nil quando o usuário não tem vendedor vinculado). Retorna
// ErrNotFound se o usuário não existir. Consulta enxuta (sem JOIN), usada
// pela API para restringir consultas de negócio (clientes/pedidos/
// pagamentos) à carteira do usuário autenticado com role=normal.
func (r *UsuarioRepository) GetIDVendedorByUsuarioID(ctx context.Context, db *sql.DB, usuarioID int64) (*int64, error) {
	const q = `SELECT id_vendedor FROM usuarios WHERE id = ? LIMIT 1`
	vlog.Printf("usuario_repository.go", "UsuarioRepository.GetIDVendedorByUsuarioID", "declarando variável idVendedor")
	var idVendedor sql.NullInt64
	vlog.Printf("usuario_repository.go", "UsuarioRepository.GetIDVendedorByUsuarioID", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query q, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, q, usuarioID).Scan(&idVendedor); err != nil {
		vlog.Printf("usuario_repository.go", "UsuarioRepository.GetIDVendedorByUsuarioID", "verificando se errors.Is(err, sql.ErrNoRows)")
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: get id_vendedor usuario: %w", err)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.GetIDVendedorByUsuarioID", "verificando se !idVendedor.Valid")
	if !idVendedor.Valid {
		return nil, nil
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.GetIDVendedorByUsuarioID", "definindo v = idVendedor.Int64")
	v := idVendedor.Int64
	return &v, nil
}

// usuarioOrderWhitelist mapeia os campos de ordenação aceitos pela API para
// as colunas SQL reais (com alias) da query de listagem de usuários.
var usuarioOrderWhitelist = map[string]string{
	"id":              "u.id",
	"nome":            "u.nome",
	"email":           "u.email",
	"role":            "u.role",
	"ativo":           "u.ativo",
	"created_at":      "u.created_at",
	"updated_at":      "u.updated_at",
	"ultimo_login_at": "u.ultimo_login_at",
}

// List retorna usuários paginados, mais o total para meta-dados de paginação.
// orderBy/orderDir controlam a ordenação (whitelist: ver
// usuarioOrderWhitelist); default "u.id ASC" (comportamento atual).
func (r *UsuarioRepository) List(ctx context.Context, db *sql.DB, page, limit int, orderBy, orderDir string) ([]models.Usuario, int, error) {
	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "verificando se page < 1")
	if page < 1 {
		vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "atribuindo page = 1")
		page = 1
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "verificando se limit < 1")
	if limit < 1 {
		vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "atribuindo limit = 20")
		limit = 20
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "verificando se limit > 100")
	if limit > 100 {
		vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "atribuindo limit = 100")
		limit = 100
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "definindo offset = (page - 1) * limit")
	offset := (page - 1) * limit

	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "declarando variável total")
	var total int
	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query SELECT em usuarios, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM usuarios`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count falhou: %w", err)
	}

	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "definindo orderClause com resultado de chamada a buildOrderByClause")
	orderClause := buildOrderByClause(usuarioOrderWhitelist, orderBy, orderDir, "u.id", "ASC")
	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "definindo q = usuarioSelectComVendedor + orderClause + ` LIMIT ? OFFSET ?`")
	q := usuarioSelectComVendedor + orderClause + `
		LIMIT ? OFFSET ?`
	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, limit, offset)
	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "verificando se err != nil")
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list falhou: %w", err)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "declarando variável out")
	var out []models.Usuario
	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		u, err := scanUsuario(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *u)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "loop concluído; itens acumulados em out: %d", len(out))
	vlog.Printf("usuario_repository.go", "UsuarioRepository.List", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("repositories: list iteração: %w", err)
	}
	return out, total, nil
}

// ListAtivos devolve todos os usuários ativos, em ordem de id (sem
// paginação: uso administrativo, ex.: reset de senha em massa, OPS-01).
func (r *UsuarioRepository) ListAtivos(ctx context.Context, db *sql.DB) ([]models.Usuario, error) {
	vlog.Printf("usuario_repository.go", "UsuarioRepository.ListAtivos", "definindo q = usuarioSelectComVendedor + ` WHERE u.ativo = 1 ORDER BY u.id ASC`")
	q := usuarioSelectComVendedor + `
		WHERE u.ativo = 1
		ORDER BY u.id ASC`
	vlog.Printf("usuario_repository.go", "UsuarioRepository.ListAtivos", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q)
	vlog.Printf("usuario_repository.go", "UsuarioRepository.ListAtivos", "verificando se err != nil")
	if err != nil {
		return nil, fmt.Errorf("repositories: list ativos falhou: %w", err)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.ListAtivos", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("usuario_repository.go", "UsuarioRepository.ListAtivos", "declarando variável out")
	var out []models.Usuario
	vlog.Printf("usuario_repository.go", "UsuarioRepository.ListAtivos", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		u, err := scanUsuario(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.ListAtivos", "loop concluído; itens acumulados em out: %d", len(out))
	vlog.Printf("usuario_repository.go", "UsuarioRepository.ListAtivos", "definindo err com resultado de chamada a rows.Err e verificando se err != nil")
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repositories: list ativos iteração: %w", err)
	}
	return out, nil
}

// UpdatePasswordHash atualiza o password_hash e a flag deve_trocar_senha de
// um usuário na mesma query, evitando estado inconsistente entre as duas
// colunas caso uma segunda escrita separada falhe.
//
// SEC-08: grava também o corte de sessão (tokens_validos_desde), derrubando
// os access tokens emitidos antes da troca/reset de senha.
func (r *UsuarioRepository) UpdatePasswordHash(ctx context.Context, db *sql.DB, id int64, newHash string, deveTrocarSenha bool) error {
	const q = `UPDATE usuarios SET password_hash = ?, deve_trocar_senha = ?, tokens_validos_desde = ? WHERE id = ?`
	vlog.Printf("usuario_repository.go", "UsuarioRepository.UpdatePasswordHash", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q, newHash, deveTrocarSenha, corteDeSessaoAgora(), id)
	vlog.Printf("usuario_repository.go", "UsuarioRepository.UpdatePasswordHash", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update password: %w", err)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.UpdatePasswordHash", "definindo n, _ com resultado de chamada a res.RowsAffected")
	n, _ := res.RowsAffected()
	vlog.Printf("usuario_repository.go", "UsuarioRepository.UpdatePasswordHash", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RehashPassword regrava password_hash com newHash apenas se o valor atual
// ainda for oldHash (condição no WHERE, ver config.DSN). Não altera
// deve_trocar_senha nem tokens_validos_desde: a senha é a mesma, só muda o
// algoritmo (SEC-13). Se a senha mudou no meio do caminho, não faz nada.
func (r *UsuarioRepository) RehashPassword(ctx context.Context, db *sql.DB, id int64, oldHash, newHash string) error {
	const q = `UPDATE usuarios SET password_hash = ? WHERE id = ? AND password_hash = ?`
	vlog.Printf("usuario_repository.go", "UsuarioRepository.RehashPassword", "definindo _, err com resultado de execução SQL via db.ExecContext (query q, args omitidos) e verificando se err != nil")
	if _, err := db.ExecContext(ctx, q, newHash, id, oldHash); err != nil {
		return fmt.Errorf("repositories: rehash password: %w", err)
	}
	return nil
}

// UpdateUltimoLogin marca o timestamp de último login.
func (r *UsuarioRepository) UpdateUltimoLogin(ctx context.Context, db *sql.DB, id int64, t time.Time) error {
	const q = `UPDATE usuarios SET ultimo_login_at = ? WHERE id = ?`
	vlog.Printf("usuario_repository.go", "UsuarioRepository.UpdateUltimoLogin", "definindo _, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	_, err := db.ExecContext(ctx, q, t, id)
	vlog.Printf("usuario_repository.go", "UsuarioRepository.UpdateUltimoLogin", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update ultimo_login: %w", err)
	}
	return nil
}

// ErrEmailDuplicado é retornado quando o email já existe (UNIQUE constraint).
var ErrEmailDuplicado = errors.New("repositories: email já cadastrado")

// Create insere um novo usuário e preenche apenas u.ID (LastInsertId).
//
// BUG-11: não relê o registro. Campos preenchidos pelo MySQL (created_at,
// updated_at) e o nome do vendedor vinculado ficam a cargo do chamador, que
// deve reler com GetByID depois do INSERT (ver relerUsuarioCriado no
// UsuarioService). Assim uma falha só na releitura não é confundida com
// falha na gravação.
func (r *UsuarioRepository) Create(ctx context.Context, db *sql.DB, u *models.Usuario) error {
	const q = `
		INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	vlog.Printf("usuario_repository.go", "UsuarioRepository.Create", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q, u.Nome, u.Email, u.PasswordHash, u.Role, u.IDVendedor, u.Ativo, u.DeveTrocarSenha)
	vlog.Printf("usuario_repository.go", "UsuarioRepository.Create", "verificando se err != nil")
	if err != nil {
		// MySQL duplicate-key error code = 1062.
		vlog.Printf("usuario_repository.go", "UsuarioRepository.Create", "verificando se strings.Contains(err.Error(), \"Error 1062\") || strings.Contains(err.Error(), \"Duplicate entry\")")
		if strings.Contains(err.Error(), "Error 1062") || strings.Contains(err.Error(), "Duplicate entry") {
			return ErrEmailDuplicado
		}
		return fmt.Errorf("repositories: insert usuario: %w", err)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.Create", "definindo id, err com resultado de chamada a res.LastInsertId")
	id, err := res.LastInsertId()
	vlog.Printf("usuario_repository.go", "UsuarioRepository.Create", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: last insert id: %w", err)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.Create", "atribuindo u.ID = id")
	u.ID = id
	return nil
}

// Update atualiza nome, role e o vendedor vinculado de um usuário.
// idVendedor nil grava NULL na coluna id_vendedor.
// Retorna ErrNotFound se não existir.
func (r *UsuarioRepository) Update(ctx context.Context, db *sql.DB, id int64, nome, role string, idVendedor *int64) error {
	const q = `UPDATE usuarios SET nome = ?, role = ?, id_vendedor = ? WHERE id = ?`
	vlog.Printf("usuario_repository.go", "UsuarioRepository.Update", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q, nome, role, idVendedor, id)
	vlog.Printf("usuario_repository.go", "UsuarioRepository.Update", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: update usuario: %w", err)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.Update", "definindo n, _ com resultado de chamada a res.RowsAffected")
	n, _ := res.RowsAffected()
	vlog.Printf("usuario_repository.go", "UsuarioRepository.Update", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetAtivo ativa/inativa um usuário (toggle). Retorna ErrNotFound se não existir.
//
// SEC-08: ao inativar grava também o corte de sessão (tokens_validos_desde),
// para que uma reativação posterior não ressuscite access tokens antigos.
// Ao ativar a coluna não é alterada.
func (r *UsuarioRepository) SetAtivo(ctx context.Context, db *sql.DB, id int64, ativo bool) error {
	vlog.Printf("usuario_repository.go", "UsuarioRepository.SetAtivo", "declarando variável res, err")
	var (
		res sql.Result
		err error
	)
	vlog.Printf("usuario_repository.go", "UsuarioRepository.SetAtivo", "verificando se ativo")
	if ativo {
		const q = `UPDATE usuarios SET ativo = ? WHERE id = ?`
		vlog.Printf("usuario_repository.go", "UsuarioRepository.SetAtivo", "atribuindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
		res, err = db.ExecContext(ctx, q, ativo, id)
	} else {
		const q = `UPDATE usuarios SET ativo = ?, tokens_validos_desde = ? WHERE id = ?`
		vlog.Printf("usuario_repository.go", "UsuarioRepository.SetAtivo", "atribuindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
		res, err = db.ExecContext(ctx, q, ativo, corteDeSessaoAgora(), id)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.SetAtivo", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: set ativo: %w", err)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.SetAtivo", "definindo n, _ com resultado de chamada a res.RowsAffected")
	n, _ := res.RowsAffected()
	vlog.Printf("usuario_repository.go", "UsuarioRepository.SetAtivo", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// InativarByVendedorID inativa (ativo = 0) todos os usuários ativos
// vinculados ao vendedor informado e retorna quantos foram afetados.
// Zero linhas afetadas NÃO é erro: o vendedor pode não ter usuário.
// Aceita *sql.DB ou *sql.Tx (ver Execer). SEC-08: grava também o corte de
// sessão (tokens_validos_desde) dos usuários inativados.
func (r *UsuarioRepository) InativarByVendedorID(ctx context.Context, db Execer, vendedorID int64) (int64, error) {
	const q = `UPDATE usuarios SET ativo = 0, tokens_validos_desde = ? WHERE id_vendedor = ? AND ativo = 1`
	vlog.Printf("usuario_repository.go", "UsuarioRepository.InativarByVendedorID", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q, corteDeSessaoAgora(), vendedorID)
	vlog.Printf("usuario_repository.go", "UsuarioRepository.InativarByVendedorID", "verificando se err != nil")
	if err != nil {
		return 0, fmt.Errorf("repositories: inativar usuarios do vendedor: %w", err)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.InativarByVendedorID", "definindo n, err com resultado de chamada a res.RowsAffected")
	n, err := res.RowsAffected()
	vlog.Printf("usuario_repository.go", "UsuarioRepository.InativarByVendedorID", "verificando se err != nil")
	if err != nil {
		return 0, fmt.Errorf("repositories: inativar usuarios do vendedor rowsAffected: %w", err)
	}
	return n, nil
}

// SetDeveTrocarSenha marca/desmarca a flag de primeiro acesso (troca de
// senha obrigatória). Retorna ErrNotFound se não existir.
func (r *UsuarioRepository) SetDeveTrocarSenha(ctx context.Context, db *sql.DB, id int64, valor bool) error {
	const q = `UPDATE usuarios SET deve_trocar_senha = ? WHERE id = ?`
	vlog.Printf("usuario_repository.go", "UsuarioRepository.SetDeveTrocarSenha", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q, valor, id)
	vlog.Printf("usuario_repository.go", "UsuarioRepository.SetDeveTrocarSenha", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: set deve_trocar_senha: %w", err)
	}
	vlog.Printf("usuario_repository.go", "UsuarioRepository.SetDeveTrocarSenha", "definindo n, _ com resultado de chamada a res.RowsAffected")
	n, _ := res.RowsAffected()
	vlog.Printf("usuario_repository.go", "UsuarioRepository.SetDeveTrocarSenha", "verificando se n == 0")
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// rowScanner é a interface comum entre *sql.Row e *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanUsuario(s rowScanner) (*models.Usuario, error) {
	vlog.Printf("usuario_repository.go", "scanUsuario", "declarando variável u")
	var u models.Usuario
	vlog.Printf("usuario_repository.go", "scanUsuario", "declarando variável idVendedor")
	var idVendedor sql.NullInt64
	vlog.Printf("usuario_repository.go", "scanUsuario", "declarando variável ultimoLogin")
	var ultimoLogin sql.NullTime
	vlog.Printf("usuario_repository.go", "scanUsuario", "declarando variável vendedorNome")
	var vendedorNome sql.NullString
	vlog.Printf("usuario_repository.go", "scanUsuario", "declarando variável deveTrocarSenha")
	var deveTrocarSenha sql.NullBool
	vlog.Printf("usuario_repository.go", "scanUsuario", "definindo err com resultado de leitura das colunas via s.Scan e verificando se err != nil")
	if err := s.Scan(
		&u.ID,
		&u.Nome,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&idVendedor,
		&u.Ativo,
		&deveTrocarSenha,
		&u.CreatedAt,
		&u.UpdatedAt,
		&ultimoLogin,
		&vendedorNome,
	); err != nil {
		vlog.Printf("usuario_repository.go", "scanUsuario", "verificando se errors.Is(err, sql.ErrNoRows)")
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan usuario: %w", err)
	}
	vlog.Printf("usuario_repository.go", "scanUsuario", "verificando se idVendedor.Valid")
	if idVendedor.Valid {
		vlog.Printf("usuario_repository.go", "scanUsuario", "definindo v = idVendedor.Int64")
		v := idVendedor.Int64
		vlog.Printf("usuario_repository.go", "scanUsuario", "atribuindo u.IDVendedor = &v")
		u.IDVendedor = &v
	}
	vlog.Printf("usuario_repository.go", "scanUsuario", "atribuindo u.DeveTrocarSenha = deveTrocarSenha.Valid && deveTrocarSenha.Bool")
	u.DeveTrocarSenha = deveTrocarSenha.Valid && deveTrocarSenha.Bool
	vlog.Printf("usuario_repository.go", "scanUsuario", "verificando se ultimoLogin.Valid")
	if ultimoLogin.Valid {
		vlog.Printf("usuario_repository.go", "scanUsuario", "definindo t = ultimoLogin.Time")
		t := ultimoLogin.Time
		vlog.Printf("usuario_repository.go", "scanUsuario", "atribuindo u.UltimoLoginAt = &t")
		u.UltimoLoginAt = &t
	}
	vlog.Printf("usuario_repository.go", "scanUsuario", "verificando se vendedorNome.Valid")
	if vendedorNome.Valid {
		vlog.Printf("usuario_repository.go", "scanUsuario", "definindo n = vendedorNome.String")
		n := vendedorNome.String
		vlog.Printf("usuario_repository.go", "scanUsuario", "atribuindo u.VendedorNome = &n")
		u.VendedorNome = &n
	}
	return &u, nil
}
