package repositories_test

// ARQ-03: os repositórios de usuarios, refresh_tokens e senha_historico
// migraram para identidade/identidademysql (SQL idêntico). Os testes mantêm
// os nomes e exercitam os adapters novos por estes repassadores finos, que
// só traduzem a assinatura antiga (db por parâmetro, page/limit/order_by em
// string) para a nova e convertem o retorno para a forma que as asserções
// já liam. Erros não são traduzidos: as asserções usam os sentinelas de
// identidade. As consultas que o legado de outros contextos ainda usa
// (GetByID, FindByUsuario) continuam no pacote repositories. ARQ-04:
// GetIDVendedorByUsuarioID, InativarByVendedorID e RevokeAllByVendedorID
// passaram a repassar a identidademysql (VendedorDoUsuario,
// InativarDoVendedor, RevogarDoVendedor), usados pelo escopo e pela tx do
// desligamento de vendedor.

import (
	"context"
	"database/sql"
	"time"

	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
)

// ---------------------------------------------------------------------------
// usuarios
// ---------------------------------------------------------------------------

type usuarioRepo struct {
	legado *repositories.UsuarioRepository
}

func novoUsuarioRepo() usuarioRepo { return usuarioRepo{legado: repositories.NewUsuarioRepository()} }

func (usuarioRepo) GetByEmail(ctx context.Context, q db.Querier, email string) (*identidade.Usuario, error) {
	return identidademysql.NovoUsuarios(q).PorEmail(ctx, email)
}

func (usuarioRepo) List(ctx context.Context, q db.Querier, page, limit int, orderBy, orderDir string) ([]identidade.Usuario, int, error) {
	o := kernel.ParseOrdenacao(orderBy, orderDir, identidade.CamposOrdenacaoUsuario(), identidade.OrdenacaoUsuarioPadrao())
	r, err := identidademysql.NovoUsuarios(q).Listar(ctx, o, kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

func (usuarioRepo) ListAtivos(ctx context.Context, q db.Querier) ([]identidade.Usuario, error) {
	return identidademysql.NovoUsuarios(q).ListarAtivos(ctx)
}

func (usuarioRepo) GetStatusByID(ctx context.Context, q db.Querier, id int64) (*identidade.StatusAcesso, error) {
	st, err := identidademysql.NovoUsuarios(q).StatusParaAutenticacao(ctx, id)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (usuarioRepo) UpdatePasswordHash(ctx context.Context, q db.Querier, id int64, hash string, deveTrocar bool) error {
	return identidademysql.NovoUsuarios(q).AtualizarSenha(ctx, id, identidade.NovoHashSenha(hash), deveTrocar)
}

func (usuarioRepo) RehashPassword(ctx context.Context, q db.Querier, id int64, antigo, novo string) error {
	return identidademysql.NovoUsuarios(q).RegravarHash(ctx, id, identidade.NovoHashSenha(antigo), identidade.NovoHashSenha(novo))
}

func (usuarioRepo) UpdateUltimoLogin(ctx context.Context, q db.Querier, id int64, t time.Time) error {
	return identidademysql.NovoUsuarios(q).RegistrarLogin(ctx, id, t)
}

func (usuarioRepo) SetAtivo(ctx context.Context, q db.Querier, id int64, ativo bool) error {
	return identidademysql.NovoUsuarios(q).DefinirAtivo(ctx, id, ativo)
}

func (usuarioRepo) InvalidarSessoes(ctx context.Context, q db.Querier, id int64, t time.Time) error {
	return identidademysql.NovoUsuarios(q).InvalidarSessoes(ctx, id, t)
}

func (usuarioRepo) Update(ctx context.Context, q db.Querier, id int64, nome, role string, idVendedor *int64) error {
	return identidademysql.NovoUsuarios(q).Atualizar(ctx, id, identidade.UsuarioValidado{Nome: nome, Role: kernel.RoleDoBanco(role), IDVendedor: idVendedor})
}

// Create recebe o model legado (os literais dos testes) e devolve o ID nele.
func (usuarioRepo) Create(ctx context.Context, q db.Querier, u *models.Usuario) error {
	novo := &identidade.Usuario{Nome: u.Nome, Email: u.Email, Hash: identidade.NovoHashSenha(u.PasswordHash),
		Role: kernel.RoleDoBanco(u.Role), IDVendedor: u.IDVendedor, Ativo: u.Ativo, DeveTrocarSenha: u.DeveTrocarSenha}
	if err := identidademysql.NovoUsuarios(q).Inserir(ctx, novo); err != nil {
		return err
	}
	u.ID = novo.ID
	return nil
}

// Consultas que continuam legadas (GetByID) e as do ARQ-04 (escopo e I-09).
func (r usuarioRepo) GetByID(ctx context.Context, d *sql.DB, id int64) (*models.Usuario, error) {
	return r.legado.GetByID(ctx, d, id)
}

func (usuarioRepo) GetIDVendedorByUsuarioID(ctx context.Context, q db.Querier, id int64) (*int64, error) {
	return identidademysql.NovoUsuarios(q).VendedorDoUsuario(ctx, id)
}

func (usuarioRepo) InativarByVendedorID(ctx context.Context, e repositories.Execer, id int64) (int64, error) {
	return identidademysql.NovoUsuarios(e.(db.Querier)).InativarDoVendedor(ctx, id)
}

// ---------------------------------------------------------------------------
// refresh_tokens
// ---------------------------------------------------------------------------

type refreshRepo struct {
	legado *repositories.RefreshTokenRepository
}

func novoRefreshRepo() refreshRepo {
	return refreshRepo{legado: repositories.NewRefreshTokenRepository()}
}

func (refreshRepo) Create(ctx context.Context, q db.Querier, rt *repositories.RefreshToken) error {
	novo := &identidade.RefreshToken{UsuarioID: rt.UsuarioID, HashToken: rt.TokenHash, ExpiraEm: rt.ExpiresAt, IP: rt.IPOrigem, UserAgent: rt.UserAgent}
	if err := identidademysql.NovoRefreshTokens(q).Inserir(ctx, novo); err != nil {
		return err
	}
	rt.ID = novo.ID
	return nil
}

func (refreshRepo) FindByTokenHash(ctx context.Context, q db.Querier, hash string) (*repositories.RefreshToken, error) {
	rt, err := identidademysql.NovoRefreshTokens(q).PorHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	out := &repositories.RefreshToken{ID: rt.ID, UsuarioID: rt.UsuarioID, TokenHash: rt.HashToken, ExpiresAt: rt.ExpiraEm,
		RevokedReason: repositories.RevokeReason(rt.Motivo), IPOrigem: rt.IP, UserAgent: rt.UserAgent}
	if rt.RevogadoEm != nil {
		out.RevokedAt = sql.NullTime{Time: *rt.RevogadoEm, Valid: true}
	}
	return out, nil
}

func (refreshRepo) Revoke(ctx context.Context, q db.Querier, id int64, motivo repositories.RevokeReason) error {
	return identidademysql.NovoRefreshTokens(q).Revogar(ctx, id, identidade.MotivoRevogacao(motivo))
}

func (refreshRepo) RevokeAllByUser(ctx context.Context, q db.Querier, uid int64, motivo repositories.RevokeReason) error {
	return identidademysql.NovoRefreshTokens(q).RevogarTodosDoUsuario(ctx, uid, identidade.MotivoRevogacao(motivo))
}

func (refreshRepo) MarcarReusoDetectado(ctx context.Context, q db.Querier, id int64, agora, limite time.Time) (bool, error) {
	return identidademysql.NovoRefreshTokens(q).MarcarReuso(ctx, id, agora, limite)
}

func (refreshRepo) DesfazerReusoDetectado(ctx context.Context, q db.Querier, id int64, marca time.Time) error {
	return identidademysql.NovoRefreshTokens(q).DesfazerReuso(ctx, id, marca)
}

func (refreshRepo) DeleteExpired(ctx context.Context, q db.Querier, corte time.Time, lote int) (int64, error) {
	return identidademysql.NovoRefreshTokens(q).ApagarExpirados(ctx, corte, lote)
}

// Consultas que continuam legadas (GetByID) e as do ARQ-04 (escopo e I-09).
func (refreshRepo) RevokeAllByVendedorID(ctx context.Context, e repositories.Execer, id int64, motivo repositories.RevokeReason) (int64, error) {
	return identidademysql.NovoRefreshTokens(e.(db.Querier)).RevogarDoVendedor(ctx, id, identidade.MotivoRevogacao(motivo))
}

func (r refreshRepo) FindByUsuario(ctx context.Context, d *sql.DB, uid int64, page, limit int) ([]repositories.RefreshToken, int, error) {
	return r.legado.FindByUsuario(ctx, d, uid, page, limit)
}

// ---------------------------------------------------------------------------
// senha_historico
// ---------------------------------------------------------------------------

// senhaHistorico é a forma de linha que as asserções já liam.
type senhaHistorico struct {
	ID                int64
	UsuarioID         int64
	UsuarioNome       string
	ResetadoPorID     sql.NullInt64
	ResetadoPorNome   sql.NullString
	SenhaHashAnterior string
	IPOrigem          string
	UserAgent         string
	TipoReset         string
	CreatedAt         time.Time
}

type historicoRepo struct{}

func novoHistoricoRepo() historicoRepo { return historicoRepo{} }

func (historicoRepo) Create(ctx context.Context, q db.Querier, h *senhaHistorico) error {
	r := &identidade.RegistroSenha{UsuarioID: h.UsuarioID, HashAnterior: identidade.NovoHashSenha(h.SenhaHashAnterior),
		IP: h.IPOrigem, UserAgent: h.UserAgent, Tipo: identidade.TipoReset(h.TipoReset)}
	if h.ResetadoPorID.Valid {
		v := h.ResetadoPorID.Int64
		r.ResetadoPorID = &v
	}
	if err := identidademysql.NovoHistoricoSenhas(q).Registrar(ctx, r); err != nil {
		return err
	}
	h.ID = r.ID
	return nil
}

func (historicoRepo) FindByUsuario(ctx context.Context, q db.Querier, uid int64, page, limit int, orderBy, orderDir string) ([]senhaHistorico, int, error) {
	r, err := identidademysql.NovoHistoricoSenhas(q).DoUsuario(ctx, uid, ordenacaoHistorico(orderBy, orderDir), kernel.NovaPagina(page, limit))
	return paraSenhaHistorico(r.Itens), r.Total, err
}

func (historicoRepo) FindAll(ctx context.Context, q db.Querier, page, limit int, orderBy, orderDir string) ([]senhaHistorico, int, error) {
	r, err := identidademysql.NovoHistoricoSenhas(q).Todos(ctx, ordenacaoHistorico(orderBy, orderDir), kernel.NovaPagina(page, limit))
	return paraSenhaHistorico(r.Itens), r.Total, err
}

// ordenacaoHistorico faz o parse de order_by/order_dir como o identidadehttp.
func ordenacaoHistorico(orderBy, orderDir string) kernel.Ordenacao[identidade.CampoHistorico] {
	return kernel.ParseOrdenacao(orderBy, orderDir, identidade.CamposOrdenacaoHistorico(), identidade.OrdenacaoHistoricoPadrao())
}

func paraSenhaHistorico(itens []identidade.RegistroSenha) []senhaHistorico {
	var out []senhaHistorico
	for _, h := range itens {
		s := senhaHistorico{ID: h.ID, UsuarioID: h.UsuarioID, UsuarioNome: h.UsuarioNome, SenhaHashAnterior: h.HashAnterior.Revelar(),
			IPOrigem: h.IP, UserAgent: h.UserAgent, TipoReset: string(h.Tipo), CreatedAt: h.CriadoEm}
		if h.ResetadoPorID != nil {
			s.ResetadoPorID = sql.NullInt64{Int64: *h.ResetadoPorID, Valid: true}
		}
		if h.ResetadoPorNome != nil {
			s.ResetadoPorNome = sql.NullString{String: *h.ResetadoPorNome, Valid: true}
		}
		out = append(out, s)
	}
	return out
}
