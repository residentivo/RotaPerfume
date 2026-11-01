package identidademysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/vlog"
)

const arqHistorico = "historico.go"

const historicoSelect = `
		SELECT sh.id, sh.usuario_id, sh.resetado_por_id, sh.senha_hash_anterior, sh.ip_origem, sh.user_agent, sh.tipo_reset, sh.created_at,
			u1.nome AS usuario_nome, u2.nome AS resetado_por_nome
		FROM senha_historico sh
		LEFT JOIN usuarios u1 ON u1.id = sh.usuario_id
		LEFT JOIN usuarios u2 ON u2.id = sh.resetado_por_id`

// HistoricoSenhas é o repositório de senha_historico (porta
// identidadeapp.HistoricoSenhas).
type HistoricoSenhas struct{ q db.Querier }

// NovoHistoricoSenhas liga o repositório ao pool. q nil: panic.
func NovoHistoricoSenhas(q db.Querier) *HistoricoSenhas {
	if q == nil {
		panic("identidademysql: NovoHistoricoSenhas exige Querier")
	}
	return &HistoricoSenhas{q: q}
}

// Registrar grava a senha anterior (auditoria e bloqueio de reuso).
func (r *HistoricoSenhas) Registrar(ctx context.Context, h *identidade.RegistroSenha) error {
	const q = `
		INSERT INTO senha_historico (usuario_id, resetado_por_id, senha_hash_anterior, ip_origem, user_agent, tipo_reset)
		VALUES (?, ?, ?, ?, ?, ?)`
	var resetadoPor any // nil grava NULL
	if h.ResetadoPorID != nil {
		resetadoPor = *h.ResetadoPorID
	}
	res, err := r.q.ExecContext(ctx, q, h.UsuarioID, resetadoPor, h.HashAnterior.Revelar(), h.IP, h.UserAgent, string(h.Tipo))
	if err != nil {
		return fmt.Errorf("identidademysql: insert senha_historico: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("identidademysql: last insert id: %w", err)
	}
	h.ID = id
	vlog.Printf(arqHistorico, "HistoricoSenhas.Registrar", "INSERT senha_historico id=%d", id)
	return nil
}

// Todos pagina o histórico inteiro (admin).
func (r *HistoricoSenhas) Todos(ctx context.Context, o kernel.Ordenacao[identidade.CampoHistorico], p kernel.Pagina) (kernel.Resultado[identidade.RegistroSenha], error) {
	return r.listar(ctx, "HistoricoSenhas.Todos", `SELECT COUNT(*) FROM senha_historico`, historicoSelect, nil, o, p)
}

// DoUsuario pagina o histórico de um usuário (admin e bloqueio de reuso).
func (r *HistoricoSenhas) DoUsuario(ctx context.Context, usuarioID int64, o kernel.Ordenacao[identidade.CampoHistorico], p kernel.Pagina) (kernel.Resultado[identidade.RegistroSenha], error) {
	return r.listar(ctx, "HistoricoSenhas.DoUsuario", `SELECT COUNT(*) FROM senha_historico WHERE usuario_id = ?`,
		historicoSelect+`
		WHERE sh.usuario_id = ?`, []any{usuarioID}, o, p)
}

func (r *HistoricoSenhas) listar(ctx context.Context, rotulo, contagem, selecao string, args []any, o kernel.Ordenacao[identidade.CampoHistorico], p kernel.Pagina) (kernel.Resultado[identidade.RegistroSenha], error) {
	var total int
	if err := r.q.QueryRowContext(ctx, contagem, args...).Scan(&total); err != nil {
		return kernel.Resultado[identidade.RegistroSenha]{}, fmt.Errorf("identidademysql: count senha_historico: %w", err)
	}
	q := selecao + ordemHistorico(o) + `
		LIMIT ? OFFSET ?`
	itens, err := db.Consultar(ctx, r.q, "identidademysql: list senha_historico", q, db.Paginada(args, p), scanRegistroSenha)
	if err != nil {
		return kernel.Resultado[identidade.RegistroSenha]{}, err
	}
	vlog.Printf(arqHistorico, rotulo, "SELECT senha_historico linhas=%d total=%d", len(itens), total)
	return kernel.NovoResultado(itens, p, total), nil
}

// scanRegistroSenha: colunas anuláveis (ip_origem, user_agent, created_at e
// u1.nome do LEFT JOIN) viram o valor zero (BUG-12: um NULL não derruba a
// listagem).
func scanRegistroSenha(s db.Linha) (*identidade.RegistroSenha, error) {
	var (
		h               identidade.RegistroSenha
		hash, tipo      string
		resetadoPorID   sql.NullInt64
		resetadoPorNome sql.NullString
		ip, agent, nome sql.NullString
		criado          sql.NullTime
	)
	err := s.Scan(&h.ID, &h.UsuarioID, &resetadoPorID, &hash, &ip, &agent, &tipo, &criado, &nome, &resetadoPorNome)
	if err != nil {
		return nil, fmt.Errorf("identidademysql: scan senha_historico: %w", err)
	}
	h.HashAnterior = identidade.NovoHashSenha(hash)
	h.Tipo = identidade.TipoReset(tipo)
	h.IP, h.UserAgent, h.UsuarioNome, h.CriadoEm = ip.String, agent.String, nome.String, criado.Time
	if resetadoPorID.Valid {
		v := resetadoPorID.Int64
		h.ResetadoPorID = &v
	}
	if resetadoPorNome.Valid {
		v := resetadoPorNome.String
		h.ResetadoPorNome = &v
	}
	return &h, nil
}

// ordemHistorico monta o ORDER BY (padrão "sh.id DESC"), com o desempate
// estável sh.id DESC quando a coluna não é o id (FE-13): sem ele, LIMIT/
// OFFSET repete ou pula linhas entre páginas.
func ordemHistorico(o kernel.Ordenacao[identidade.CampoHistorico]) string {
	col := colunaHistorico(o.Campo())
	clausula := " ORDER BY " + col + " " + db.DirecaoSQL(o.Direcao(), "DESC")
	if col == "sh.id" {
		return clausula
	}
	return clausula + ", sh.id DESC"
}

// colunaHistorico é a whitelist enum → coluna; fora do enum, "sh.id".
func colunaHistorico(c identidade.CampoHistorico) string {
	switch c {
	case identidade.OrdemHistoricoUsuarioID:
		return "sh.usuario_id"
	case identidade.OrdemHistoricoTipo:
		return "sh.tipo_reset"
	case identidade.OrdemHistoricoCriadoEm:
		return "sh.created_at"
	case identidade.OrdemHistoricoUsuarioNome:
		return "u1.nome"
	case identidade.OrdemHistoricoResetadoPorNome:
		return "u2.nome"
	case identidade.OrdemHistoricoIP:
		return "sh.ip_origem"
	default:
		return "sh.id"
	}
}
