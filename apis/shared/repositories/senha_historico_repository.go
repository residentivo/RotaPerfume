// Package repositories contém funções puras de acesso a dados (stateless).
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/vlog"
)

// SenhaHistorico representa um registro de auditoria de alteração de senha.
type SenhaHistorico struct {
	ID                int64
	UsuarioID         int64
	UsuarioNome       string
	ResetadoPorID     sql.NullInt64
	ResetadoPorNome   sql.NullString
	SenhaHashAnterior string
	IPOrigem          string
	UserAgent         string
	TipoReset         string // "usuario" | "admin" | "primeiro_acesso" | "esquecimento"
	CreatedAt         time.Time
}

// NewSenhaHistoricoRepository cria um repositório stateless.
func NewSenhaHistoricoRepository() *SenhaHistoricoRepository {
	return &SenhaHistoricoRepository{}
}

// SenhaHistoricoRepository agrupa queries da tabela senha_historico.
type SenhaHistoricoRepository struct{}

// Create insere um novo registro de histórico de senha.
func (r *SenhaHistoricoRepository) Create(ctx context.Context, db *sql.DB, h *SenhaHistorico) error {
	const q = `
		INSERT INTO senha_historico (usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset)
		VALUES (?, ?, ?, ?, ?, ?)`
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.Create", "declarando variável resetadoPorID")
	var resetadoPorID interface{}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.Create", "verificando se h.ResetadoPorID.Valid")
	if h.ResetadoPorID.Valid {
		vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.Create", "atribuindo resetadoPorID = h.ResetadoPorID.Int64")
		resetadoPorID = h.ResetadoPorID.Int64
	} else {
		vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.Create", "atribuindo resetadoPorID = nil")
		resetadoPorID = nil
	}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.Create", "definindo res, err com resultado de execução SQL via db.ExecContext (query q, args omitidos)")
	res, err := db.ExecContext(ctx, q, h.UsuarioID, resetadoPorID, h.SenhaHashAnterior, h.IPOrigem, h.UserAgent, h.TipoReset)
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.Create", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: insert senha_historico: %w", err)
	}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.Create", "definindo id, err com resultado de chamada a res.LastInsertId")
	id, err := res.LastInsertId()
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.Create", "verificando se err != nil")
	if err != nil {
		return fmt.Errorf("repositories: last insert id: %w", err)
	}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.Create", "atribuindo h.ID = id")
	h.ID = id
	return nil
}

// senhaHistoricoOrderWhitelist mapeia os campos de ordenação aceitos pela
// API para as colunas SQL reais da tabela senha_historico.
var senhaHistoricoOrderWhitelist = map[string]string{
	"id":         "sh.id",
	"usuario_id": "sh.usuario_id",
	"tipo_reset": "sh.tipo_reset",
	"created_at": "sh.created_at",
	// FE-13: colunas exibidas na tela admin/senha-historico. Os nomes vêm dos
	// LEFT JOINs já existentes (u1 = usuário, u2 = quem resetou; u2.nome é
	// NULL em reset feito pelo próprio usuário e o MySQL ordena NULL primeiro
	// no ASC).
	"usuario_nome":      "u1.nome",
	"resetado_por_nome": "u2.nome",
	"ip_origem":         "sh.ip_origem",
}

// senhaHistoricoDesempate é o critério fixo de desempate da listagem: as
// colunas ordenáveis (nome, IP, tipo) se repetem entre linhas, e sem ele a
// paginação por LIMIT/OFFSET pode repetir ou pular registros entre páginas.
const senhaHistoricoDesempate = "sh.id DESC"

// senhaHistoricoOrderClause monta o ORDER BY da listagem (whitelist:
// senhaHistoricoOrderWhitelist; default "sh.id DESC") acrescentando o
// desempate estável sh.id DESC quando a coluna escolhida não é o próprio id.
func senhaHistoricoOrderClause(orderBy, orderDir string) string {
	vlog.Printf("senha_historico_repository.go", "senhaHistoricoOrderClause", "definindo col com resultado de chamada a resolveOrderColumn")
	col := resolveOrderColumn(senhaHistoricoOrderWhitelist, orderBy, "sh.id")
	vlog.Printf("senha_historico_repository.go", "senhaHistoricoOrderClause", "definindo clause com resultado de chamada a buildOrderByClause")
	clause := buildOrderByClause(senhaHistoricoOrderWhitelist, orderBy, orderDir, "sh.id", "DESC")
	vlog.Printf("senha_historico_repository.go", "senhaHistoricoOrderClause", "verificando se col == \"sh.id\"")
	if col == "sh.id" {
		return clause
	}
	return clause + ", " + senhaHistoricoDesempate
}

// FindByUsuario lista histórico de senhas de um usuário (paginado).
// orderBy/orderDir controlam a ordenação (whitelist: ver
// senhaHistoricoOrderWhitelist); default "sh.id DESC", com desempate
// sh.id DESC nas demais colunas (ver senhaHistoricoOrderClause).
func (r *SenhaHistoricoRepository) FindByUsuario(ctx context.Context, db *sql.DB, usuarioID int64, page, limit int, orderBy, orderDir string) ([]SenhaHistorico, int, error) {
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "verificando se page < 1")
	if page < 1 {
		vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "atribuindo page = 1")
		page = 1
	}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "verificando se limit < 1")
	if limit < 1 {
		vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "atribuindo limit = 20")
		limit = 20
	}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "verificando se limit > 100")
	if limit > 100 {
		vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "atribuindo limit = 100")
		limit = 100
	}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "definindo offset = (page - 1) * limit")
	offset := (page - 1) * limit

	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "declarando variável total")
	var total int
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query SELECT em senha_historico, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM senha_historico WHERE usuario_id = ?`, usuarioID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count senha_historico: %w", err)
	}

	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "definindo orderClause com resultado de chamada a senhaHistoricoOrderClause")
	orderClause := senhaHistoricoOrderClause(orderBy, orderDir)
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "montando texto da query SQL SELECT em senha_historico em q")
	q := `
		SELECT sh.id, sh.usuario_id, sh.resetado_por_id, sh.senha_hash_anterior, sh.ip_origem, sh.user_agent, sh.tipo_reset, sh.created_at,
			u1.nome AS usuario_nome, u2.nome AS resetado_por_nome
		FROM senha_historico sh
		LEFT JOIN usuarios u1 ON u1.id = sh.usuario_id
		LEFT JOIN usuarios u2 ON u2.id = sh.resetado_por_id
		WHERE sh.usuario_id = ?` + orderClause + `
		LIMIT ? OFFSET ?`
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, usuarioID, limit, offset)
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "verificando se err != nil")
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list senha_historico: %w", err)
	}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "declarando variável out")
	var out []SenhaHistorico
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		h, err := scanSenhaHistorico(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *h)
	}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindByUsuario", "loop concluído; itens acumulados em out: %d", len(out))
	return out, total, rows.Err()
}

// FindAll lista todos os históricos de senhas (paginado) — admin only.
// orderBy/orderDir controlam a ordenação (whitelist: ver
// senhaHistoricoOrderWhitelist); default "sh.id DESC", com desempate
// sh.id DESC nas demais colunas (ver senhaHistoricoOrderClause).
func (r *SenhaHistoricoRepository) FindAll(ctx context.Context, db *sql.DB, page, limit int, orderBy, orderDir string) ([]SenhaHistorico, int, error) {
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "verificando se page < 1")
	if page < 1 {
		vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "atribuindo page = 1")
		page = 1
	}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "verificando se limit < 1")
	if limit < 1 {
		vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "atribuindo limit = 20")
		limit = 20
	}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "verificando se limit > 100")
	if limit > 100 {
		vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "atribuindo limit = 100")
		limit = 100
	}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "definindo offset = (page - 1) * limit")
	offset := (page - 1) * limit

	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "declarando variável total")
	var total int
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "definindo err com resultado de execução SQL via db.QueryRowContext(...).Scan (query SELECT em senha_historico, args omitidos) com leitura do resultado e verificando se err != nil")
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM senha_historico`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("repositories: count senha_historico: %w", err)
	}

	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "definindo orderClause com resultado de chamada a senhaHistoricoOrderClause")
	orderClause := senhaHistoricoOrderClause(orderBy, orderDir)
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "montando texto da query SQL SELECT em senha_historico em q")
	q := `
		SELECT sh.id, sh.usuario_id, sh.resetado_por_id, sh.senha_hash_anterior, sh.ip_origem, sh.user_agent, sh.tipo_reset, sh.created_at,
			u1.nome AS usuario_nome, u2.nome AS resetado_por_nome
		FROM senha_historico sh
		LEFT JOIN usuarios u1 ON u1.id = sh.usuario_id
		LEFT JOIN usuarios u2 ON u2.id = sh.resetado_por_id` + orderClause + `
		LIMIT ? OFFSET ?`
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "definindo rows, err com resultado de execução SQL via db.QueryContext (query q, args omitidos)")
	rows, err := db.QueryContext(ctx, q, limit, offset)
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "verificando se err != nil")
	if err != nil {
		return nil, 0, fmt.Errorf("repositories: list senha_historico: %w", err)
	}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "agendando defer de chamada a rows.Close")
	defer rows.Close()

	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "declarando variável out")
	var out []SenhaHistorico
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "iniciando loop enquanto rows.Next() (sem log por iteração)")
	for rows.Next() {
		h, err := scanSenhaHistorico(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *h)
	}
	vlog.Printf("senha_historico_repository.go", "SenhaHistoricoRepository.FindAll", "loop concluído; itens acumulados em out: %d", len(out))
	return out, total, rows.Err()
}

// scanSenhaHistorico lê uma linha da listagem. As colunas anuláveis no banco
// (ip_origem, user_agent, created_at) e u1.nome (vem de LEFT JOIN) são lidas
// em tipos sql.Null* e convertidas para o valor zero do campo do model
// ("" / time.Time{}) quando NULL — BUG-12: sem isso um único registro com
// NULL derrubava a listagem inteira com 500. O contrato JSON não muda.
func scanSenhaHistorico(s rowScanner) (*SenhaHistorico, error) {
	vlog.Printf("senha_historico_repository.go", "scanSenhaHistorico", "declarando variável h")
	var h SenhaHistorico
	vlog.Printf("senha_historico_repository.go", "scanSenhaHistorico", "declarando variável resetadoPorID, resetadoPorNome, ipOrigem, userAgent, createdAt, usuarioNome")
	var (
		resetadoPorID   sql.NullInt64
		resetadoPorNome sql.NullString
		ipOrigem        sql.NullString
		userAgent       sql.NullString
		createdAt       sql.NullTime
		usuarioNome     sql.NullString
	)
	vlog.Printf("senha_historico_repository.go", "scanSenhaHistorico", "definindo err com resultado de leitura das colunas via s.Scan e verificando se err != nil")
	if err := s.Scan(
		&h.ID,
		&h.UsuarioID,
		&resetadoPorID,
		&h.SenhaHashAnterior,
		&ipOrigem,
		&userAgent,
		&h.TipoReset,
		&createdAt,
		&usuarioNome,
		&resetadoPorNome,
	); err != nil {
		vlog.Printf("senha_historico_repository.go", "scanSenhaHistorico", "verificando se errors.Is(err, sql.ErrNoRows)")
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("repositories: scan senha_historico: %w", err)
	}
	vlog.Printf("senha_historico_repository.go", "scanSenhaHistorico", "atribuindo h.ResetadoPorID = resetadoPorID")
	h.ResetadoPorID = resetadoPorID
	vlog.Printf("senha_historico_repository.go", "scanSenhaHistorico", "atribuindo h.ResetadoPorNome = resetadoPorNome")
	h.ResetadoPorNome = resetadoPorNome
	vlog.Printf("senha_historico_repository.go", "scanSenhaHistorico", "atribuindo h.IPOrigem = ipOrigem.String")
	h.IPOrigem = ipOrigem.String
	vlog.Printf("senha_historico_repository.go", "scanSenhaHistorico", "atribuindo h.UserAgent = userAgent.String")
	h.UserAgent = userAgent.String
	vlog.Printf("senha_historico_repository.go", "scanSenhaHistorico", "atribuindo h.CreatedAt = createdAt.Time")
	h.CreatedAt = createdAt.Time
	vlog.Printf("senha_historico_repository.go", "scanSenhaHistorico", "atribuindo h.UsuarioNome = usuarioNome.String")
	h.UsuarioNome = usuarioNome.String
	return &h, nil
}
