// Package identidademysql implementa as portas de identidadeapp sobre MySQL
// (ARQ-03). O SQL é o dos repositórios legados (usuario_repository.go,
// refresh_token_repository.go, senha_historico_repository.go), copiado sem
// alteração; as condições ficam no WHERE (F-07), a ordenação sai de switch
// sobre os enums do domínio (C-02) e todo valor vai por placeholder.
package identidademysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqUsuarios = "usuarios.go"

// usuarioSelectComVendedor: LEFT JOIN em vendedores para o vendedor_nome,
// sem N+1.
const usuarioSelectComVendedor = `
	SELECT u.id, u.nome, u.email, u.password_hash, u.role, u.id_vendedor, u.ativo,
	       u.deve_trocar_senha, u.created_at, u.updated_at, u.ultimo_login_at, v.nome
	FROM usuarios u
	LEFT JOIN vendedores v ON v.id = u.id_vendedor`

// Usuarios é o repositório de usuários (porta identidadeapp.Usuarios). Cada
// método executa a própria consulta: sem cache entre requests (R4).
type Usuarios struct {
	q     db.Querier
	agora func() time.Time
}

// NovoUsuarios liga o repositório ao pool. q nil: panic.
func NovoUsuarios(q db.Querier) *Usuarios {
	if q == nil {
		panic("identidademysql: NovoUsuarios exige Querier")
	}
	return &Usuarios{q: q, agora: time.Now}
}

// PorEmail busca pelo e-mail; identidade.ErrUsuarioNaoEncontrado se não existir.
func (r *Usuarios) PorEmail(ctx context.Context, email string) (*identidade.Usuario, error) {
	q := usuarioSelectComVendedor + `
		WHERE u.email = ?
		LIMIT 1`
	u, err := scanUsuario(r.q.QueryRowContext(ctx, q, email))
	vlog.Printf(arqUsuarios, "Usuarios.PorEmail", "SELECT usuarios encontrado=%t", err == nil)
	return u, err
}

// PorID busca pelo id; identidade.ErrUsuarioNaoEncontrado se não existir.
func (r *Usuarios) PorID(ctx context.Context, id int64) (*identidade.Usuario, error) {
	q := usuarioSelectComVendedor + `
		WHERE u.id = ?
		LIMIT 1`
	u, err := scanUsuario(r.q.QueryRowContext(ctx, q, id))
	vlog.Printf(arqUsuarios, "Usuarios.PorID", "SELECT usuarios encontrado=%t", err == nil)
	return u, err
}

// StatusParaAutenticacao é a consulta enxuta (sem JOIN) de cada request protegido.
func (r *Usuarios) StatusParaAutenticacao(ctx context.Context, id int64) (identidade.StatusAcesso, error) {
	const q = `SELECT ativo, role, tokens_validos_desde FROM usuarios WHERE id = ? LIMIT 1`
	var st identidade.StatusAcesso
	var role string
	var corte sql.NullTime
	if err := r.q.QueryRowContext(ctx, q, id).Scan(&st.Ativo, &role, &corte); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return identidade.StatusAcesso{}, identidade.ErrUsuarioNaoEncontrado
		}
		return identidade.StatusAcesso{}, fmt.Errorf("identidademysql: get status usuario: %w", err)
	}
	st.Role = kernel.RoleDoBanco(role)
	if corte.Valid {
		t := corte.Time
		st.TokensValidosDesde = &t
	}
	return st, nil
}

// Listar pagina os usuários, mais o total.
func (r *Usuarios) Listar(ctx context.Context, o kernel.Ordenacao[identidade.CampoUsuario], p kernel.Pagina) (kernel.Resultado[identidade.Usuario], error) {
	var total int
	if err := r.q.QueryRowContext(ctx, `SELECT COUNT(*) FROM usuarios`).Scan(&total); err != nil {
		return kernel.Resultado[identidade.Usuario]{}, fmt.Errorf("identidademysql: count falhou: %w", err)
	}
	q := usuarioSelectComVendedor + ordemUsuario(o) + `
		LIMIT ? OFFSET ?`
	itens, err := db.Consultar(ctx, r.q, "identidademysql: list falhou", q, db.Paginada(nil, p), scanUsuario)
	if err != nil {
		return kernel.Resultado[identidade.Usuario]{}, err
	}
	vlog.Printf(arqUsuarios, "Usuarios.Listar", "SELECT usuarios linhas=%d total=%d", len(itens), total)
	return kernel.NovoResultado(itens, p, total), nil
}

// ListarAtivos devolve todos os ativos em ordem de id (reset em massa).
func (r *Usuarios) ListarAtivos(ctx context.Context) ([]identidade.Usuario, error) {
	q := usuarioSelectComVendedor + `
		WHERE u.ativo = 1
		ORDER BY u.id ASC`
	itens, err := db.Consultar(ctx, r.q, "identidademysql: list ativos falhou", q, nil, scanUsuario)
	vlog.Printf(arqUsuarios, "Usuarios.ListarAtivos", "SELECT usuarios ativos linhas=%d", len(itens))
	return itens, err
}

// Inserir grava o usuário e preenche só u.ID (a releitura é do caso de uso,
// BUG-11). Chave duplicada vira identidade.ErrEmailDuplicado sem embrulhar
// o erro do driver, que traz o e-mail (O6-iv).
func (r *Usuarios) Inserir(ctx context.Context, u *identidade.Usuario) error {
	const q = `
		INSERT INTO usuarios (nome, email, password_hash, role, id_vendedor, ativo, deve_trocar_senha)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := r.q.ExecContext(ctx, q, u.Nome, u.Email, u.Hash.Revelar(), u.Role.String(), u.IDVendedor, u.Ativo, u.DeveTrocarSenha)
	if err != nil {
		if chaveDuplicada(err) {
			return identidade.ErrEmailDuplicado
		}
		return fmt.Errorf("identidademysql: insert usuario: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("identidademysql: last insert id: %w", err)
	}
	u.ID = id
	vlog.Printf(arqUsuarios, "Usuarios.Inserir", "INSERT usuarios id=%d", id)
	return nil
}

// chaveDuplicada: MySQL 1062. O texto do legado ("Error 1062"/"Duplicate
// entry") continua aceito, com o mesmo resultado.
func chaveDuplicada(err error) bool {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1062
	}
	msg := err.Error()
	return strings.Contains(msg, "Error 1062") || strings.Contains(msg, "Duplicate entry")
}

// Atualizar grava nome, role e vendedor (nil grava NULL).
func (r *Usuarios) Atualizar(ctx context.Context, id int64, v identidade.UsuarioValidado) error {
	const q = `UPDATE usuarios SET nome = ?, role = ?, id_vendedor = ? WHERE id = ?`
	return r.exec(ctx, "Usuarios.Atualizar", "update usuario", q, v.Nome, v.Role.String(), v.IDVendedor, id)
}

// DefinirAtivo ativa/inativa. Ao inativar grava também o corte de sessão,
// para uma reativação não ressuscitar access tokens antigos (SEC-08).
func (r *Usuarios) DefinirAtivo(ctx context.Context, id int64, ativo bool) error {
	if ativo {
		const q = `UPDATE usuarios SET ativo = ? WHERE id = ?`
		return r.exec(ctx, "Usuarios.DefinirAtivo", "set ativo", q, ativo, id)
	}
	const q = `UPDATE usuarios SET ativo = ?, tokens_validos_desde = ? WHERE id = ?`
	return r.exec(ctx, "Usuarios.DefinirAtivo", "set ativo", q, ativo, identidade.CorteDeSessao(r.agora()), id)
}

// AtualizarSenha grava hash e deve_trocar_senha na mesma query, com o corte
// de sessão (I-04).
func (r *Usuarios) AtualizarSenha(ctx context.Context, id int64, h identidade.HashSenha, deveTrocarSenha bool) error {
	const q = `UPDATE usuarios SET password_hash = ?, deve_trocar_senha = ?, tokens_validos_desde = ? WHERE id = ?`
	return r.exec(ctx, "Usuarios.AtualizarSenha", "update password", q, h.Revelar(), deveTrocarSenha, identidade.CorteDeSessao(r.agora()), id)
}

// InvalidarSessoes grava o corte de sessão (SEC-08), truncado ao segundo
// aqui: o MySQL arredondaria a fração e derrubaria um token emitido no
// segundo seguinte.
func (r *Usuarios) InvalidarSessoes(ctx context.Context, id int64, corte time.Time) error {
	const q = `UPDATE usuarios SET tokens_validos_desde = ? WHERE id = ?`
	return r.exec(ctx, "Usuarios.InvalidarSessoes", "invalidar sessoes", q, identidade.CorteDeSessao(corte), id)
}

// RegravarHash troca o hash só se ainda for o antigo (condição no WHERE);
// não mexe em deve_trocar_senha nem no corte (SEC-13). Sem linha afetada
// não é erro (a senha mudou no meio do caminho).
func (r *Usuarios) RegravarHash(ctx context.Context, id int64, antigo, novo identidade.HashSenha) error {
	const q = `UPDATE usuarios SET password_hash = ? WHERE id = ? AND password_hash = ?`
	if _, err := r.q.ExecContext(ctx, q, novo.Revelar(), id, antigo.Revelar()); err != nil {
		return fmt.Errorf("identidademysql: rehash password: %w", err)
	}
	return nil
}

// RegistrarLogin marca ultimo_login_at.
func (r *Usuarios) RegistrarLogin(ctx context.Context, id int64, quando time.Time) error {
	const q = `UPDATE usuarios SET ultimo_login_at = ? WHERE id = ?`
	if _, err := r.q.ExecContext(ctx, q, quando, id); err != nil {
		return fmt.Errorf("identidademysql: update ultimo_login: %w", err)
	}
	return nil
}

// exec roda um UPDATE por id; sem linha afetada é ErrUsuarioNaoEncontrado
// (o DSN usa clientFoundRows, então "mesmo valor" conta como afetada).
func (r *Usuarios) exec(ctx context.Context, rotulo, op, q string, args ...any) error {
	res, err := r.q.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("identidademysql: %s: %w", op, err)
	}
	n, _ := res.RowsAffected()
	vlog.Printf(arqUsuarios, rotulo, "UPDATE usuarios afetadas=%d", n)
	if n == 0 {
		return identidade.ErrUsuarioNaoEncontrado
	}
	return nil
}

func scanUsuario(s db.Linha) (*identidade.Usuario, error) {
	var (
		u           identidade.Usuario
		role, hash  string
		idVendedor  sql.NullInt64
		ultimoLogin sql.NullTime
		vendNome    sql.NullString
		deveTrocar  sql.NullBool
	)
	err := s.Scan(&u.ID, &u.Nome, &u.Email, &hash, &role, &idVendedor, &u.Ativo,
		&deveTrocar, &u.CriadoEm, &u.AtualizadoEm, &ultimoLogin, &vendNome)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, identidade.ErrUsuarioNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("identidademysql: scan usuario: %w", err)
	}
	u.Role = kernel.RoleDoBanco(role)
	u.Hash = identidade.NovoHashSenha(hash)
	u.DeveTrocarSenha = deveTrocar.Valid && deveTrocar.Bool
	if idVendedor.Valid {
		v := idVendedor.Int64
		u.IDVendedor = &v
	}
	if ultimoLogin.Valid {
		t := ultimoLogin.Time
		u.UltimoLoginEm = &t
	}
	if vendNome.Valid {
		n := vendNome.String
		u.VendedorNome = &n
	}
	return &u, nil
}

// ordemUsuario: " ORDER BY <coluna> <dir>" (padrão "u.id ASC").
func ordemUsuario(o kernel.Ordenacao[identidade.CampoUsuario]) string {
	return " ORDER BY " + colunaUsuario(o.Campo()) + " " + db.DirecaoSQL(o.Direcao(), "ASC")
}

// colunaUsuario é a whitelist enum → coluna; fora do enum, "u.id".
func colunaUsuario(c identidade.CampoUsuario) string {
	switch c {
	case identidade.OrdemUsuarioNome:
		return "u.nome"
	case identidade.OrdemUsuarioEmail:
		return "u.email"
	case identidade.OrdemUsuarioRole:
		return "u.role"
	case identidade.OrdemUsuarioAtivo:
		return "u.ativo"
	case identidade.OrdemUsuarioCriadoEm:
		return "u.created_at"
	case identidade.OrdemUsuarioAtualizadoEm:
		return "u.updated_at"
	case identidade.OrdemUsuarioUltimoLogin:
		return "u.ultimo_login_at"
	default:
		return "u.id"
	}
}

// VendedorDoUsuario devolve usuarios.id_vendedor (nil sem vínculo);
// identidade.ErrUsuarioNaoEncontrado se o usuário não existir. Consulta
// própria a cada chamada: base do escopo de carteira (E-01, sem cache).
func (r *Usuarios) VendedorDoUsuario(ctx context.Context, usuarioID int64) (*int64, error) {
	const q = `SELECT id_vendedor FROM usuarios WHERE id = ? LIMIT 1`
	var idVendedor sql.NullInt64
	err := r.q.QueryRowContext(ctx, q, usuarioID).Scan(&idVendedor)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, identidade.ErrUsuarioNaoEncontrado
	}
	if err != nil {
		return nil, fmt.Errorf("identidademysql: get id_vendedor usuario: %w", err)
	}
	vlog.Printf(arqUsuarios, "Usuarios.VendedorDoUsuario", "SELECT usuarios vinculado=%t", idVendedor.Valid)
	if !idVendedor.Valid {
		return nil, nil
	}
	return &idVendedor.Int64, nil
}

// InativarDoVendedor inativa os usuários ATIVOS do vendedor gravando o corte
// de sessão (SEC-08) e devolve quantos foram afetados (0 não é erro). Roda
// na tx do desligamento do vendedor (I-09).
func (r *Usuarios) InativarDoVendedor(ctx context.Context, vendedorID int64) (int64, error) {
	const q = `UPDATE usuarios SET ativo = 0, tokens_validos_desde = ? WHERE id_vendedor = ? AND ativo = 1`
	res, err := r.q.ExecContext(ctx, q, identidade.CorteDeSessao(r.agora()), vendedorID)
	if err != nil {
		return 0, fmt.Errorf("identidademysql: inativar usuarios do vendedor: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("identidademysql: inativar usuarios do vendedor rowsAffected: %w", err)
	}
	vlog.Printf(arqUsuarios, "Usuarios.InativarDoVendedor", "UPDATE usuarios afetadas=%d", n)
	return n, nil
}
