package services_test

// ARQ-03: RefreshTokenService, UsuarioService, SenhaHistoricoService,
// ResetSenhasAtivosService e IniciarLimpezaRefreshTokens saíram de
// rotaperfumes-api/services. Os testes (mesmos nomes e asserções) usam os
// nomes antigos, agora declarados AQUI como repassadores finos para o código
// novo: identidadeapp.Casos montado sobre os adapters reais
// (identidademysql, identidadecrypto) e o db do sqlmock. Nenhuma regra mora
// aqui: expiração/revogação/reuso vêm de identidade.RefreshToken.Avaliar e
// ReusoDeRotacao; janela, retenção e marca de reuso, dos casos de uso.
// Sai no ARQ-07, quando os testes forem reescritos sobre a API nova.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rotaperfumes/shared/adapters/ratelimit"
	"github.com/rotaperfumes/shared/config"
	shareddb "github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"github.com/rotaperfumes/shared/kernel"
	"github.com/rotaperfumes/shared/repositories"
)

// Sentinelas e constantes com os nomes antigos.
var (
	ErrRefreshTokenNotFound = identidade.ErrRefreshInvalido
	ErrRefreshTokenExpired  = identidade.ErrRefreshExpirado
	ErrRefreshTokenRevoked  = identidade.ErrRefreshRevogado
	// ErrRefreshTokenRevokedRecently envolve ErrRefreshTokenRevoked (a
	// resposta HTTP é a mesma; só o rate limit distingue).
	ErrRefreshTokenRevokedRecently = fmt.Errorf("%w recentemente (rotação concorrente)", identidade.ErrRefreshRevogado)

	ErrUsuarioNaoEncontrado  = identidade.ErrUsuarioNaoEncontrado
	ErrEmailDuplicado        = identidade.ErrEmailDuplicado
	ErrRoleInvalido          = identidade.ErrRoleInvalido
	ErrNomeObrigatorio       = identidade.ErrNomeObrigatorio
	ErrEmailInvalido         = identidade.ErrEmailObrigatorio // e-mail vazio
	ErrVendedorNaoEncontrado = identidade.ErrVendedorInexistente
	ErrEmailResetFalhou      = identidadeapp.ErrEmailResetFalhou
)

const (
	RefreshTokenBytes                = identidadecrypto.BytesRefresh
	RefreshRevokeGraceWindow         = identidade.JanelaGracaRevogacao
	RefreshReuseSuppressWindowPadrao = identidadeapp.JanelaSupressaoReusoPadrao
	RefreshTokenRetencaoPadrao       = identidadeapp.RetencaoRefreshPadrao
	RefreshTokenRetencaoMinima       = identidadeapp.RetencaoRefreshMinima
	RefreshCleanupLote               = identidadeapp.LoteLimpezaRefresh

	TipoResetUsuario        = string(identidade.TipoResetUsuario)
	TipoResetAdmin          = string(identidade.TipoResetAdmin)
	TipoResetPrimeiroAcesso = string(identidade.TipoResetPrimeiroAcesso)
	TipoResetEsquecimento   = string(identidade.TipoResetEsquecimento)

	ResetMassaIPOrigem  = identidadeapp.ResetMassaIPOrigem
	ResetMassaUserAgent = identidadeapp.ResetMassaUserAgent
)

// ItemResetMassa é o item do reset em massa.
type ItemResetMassa = identidadeapp.ItemResetMassa

// ---------------------------------------------------------------------------
// Montagem dos casos de uso sobre o db do sqlmock
// ---------------------------------------------------------------------------

var cfgIdentidadeTeste = &config.Config{
	JWTSecret: "segredo-teste-servicos", JWTIssuer: "rotaperfumes-test", JWTTTL: time.Hour,
	HashSenha: config.HashSenha{Pepper: "pepper-de-teste-com-pelo-menos-32-bytes", MemoriaKiB: 64, Iteracoes: 1, Paralelismo: 1},
}

type captchaAprova struct{}

func (captchaAprova) Verificar(context.Context, string, string) error { return nil }

// vendedoresLegado: a porta identidadeapp.Vendedores sobre o repositório de
// vendedores ainda legado (como o bootstrap).
type vendedoresLegado struct{ db *sql.DB }

func (v vendedoresLegado) Existe(ctx context.Context, id int64) (bool, error) {
	return repositories.NewVendedorRepository().ExistsByID(ctx, v.db, id)
}

func (v vendedoresLegado) Desligado(ctx context.Context, id int64) (bool, error) {
	d, err := repositories.NewVendedorRepository().IsDesligado(ctx, v.db, id)
	if errors.Is(err, repositories.ErrNotFound) {
		return false, nil
	}
	return d, err
}

func novosCasos(db *sql.DB, cfg *config.Config, email identidadeapp.EnviadorSenhaInicial, agora func() time.Time, o identidadeapp.Opcoes) *identidadeapp.Casos {
	if cfg == nil {
		cfg = cfgIdentidadeTeste
	}
	if email == nil {
		email = &fakeEmailService{}
	}
	return identidadeapp.Novo(identidadeapp.Deps{
		Usuarios:  identidademysql.NovoUsuarios(db),
		Tokens:    identidademysql.NovoRefreshTokens(db),
		Historico: identidademysql.NovoHistoricoSenhas(db),
		Rotacao: shareddb.NewTransactor(db, func(q shareddb.Querier) identidadeapp.TxRotacao {
			return identidadeapp.TxRotacao{Tokens: identidademysql.NovoRefreshTokens(q)}
		}),
		Hasher:     identidadecrypto.NovoHasher(cfg.HashSenha),
		Emissor:    identidadecrypto.NovoJWT(cfg),
		Refresh:    identidadecrypto.GeradorRefresh{},
		Senhas:     identidadecrypto.GeradorSenha{},
		Captcha:    captchaAprova{},
		Limites:    identidadeapp.Limites{Login: ratelimit.Nova(5, time.Minute, time.Minute), Refresh: ratelimit.Nova(10, time.Minute, time.Minute), Reset: ratelimit.Nova(10, time.Minute, time.Minute)},
		Publicador: identidadeapp.SemPublicador{},
		Email:      email,
		Vendedores: vendedoresLegado{db: db},
		Relogio:    agora,
	}, o)
}

// adminTeste é o Principal admin dos casos de uso de gestão (I-05).
func adminTeste() kernel.Principal {
	p, _ := kernel.NovoPrincipal(1, kernel.RoleAdmin)
	return p
}

// ---------------------------------------------------------------------------
// RefreshTokenService
// ---------------------------------------------------------------------------

// RefreshTokenService guarda o relógio e as opções; cada chamada monta os
// casos de uso sobre o db recebido.
type RefreshTokenService struct {
	agora  func() time.Time
	opcoes identidadeapp.Opcoes
}

func NewRefreshTokenService() *RefreshTokenService { return NewRefreshTokenServiceWithClock(nil) }

func NewRefreshTokenServiceWithClock(now func() time.Time) *RefreshTokenService {
	if now == nil {
		now = time.Now
	}
	return &RefreshTokenService{agora: now}
}

// SetReuseSuppressWindow e SetRetencao: <= 0 cai no padrão (identidadeapp.Novo).
func (s *RefreshTokenService) SetReuseSuppressWindow(d time.Duration) {
	s.opcoes.JanelaSupressaoReuso = d
}
func (s *RefreshTokenService) SetRetencao(d time.Duration) { s.opcoes.RetencaoRefresh = d }

func (s *RefreshTokenService) casos(db *sql.DB) *identidadeapp.Casos {
	return novosCasos(db, nil, nil, s.agora, s.opcoes)
}

// GenerateRefreshToken emite um token (gerador opaco + adapter MySQL).
func (s *RefreshTokenService) GenerateRefreshToken(ctx context.Context, db *sql.DB, uid int64, ip, ua string) (string, error) {
	return emitirRefresh(ctx, identidademysql.NovoRefreshTokens(db), uid, ip, ua)
}

func emitirRefresh(ctx context.Context, tokens *identidademysql.RefreshTokens, uid int64, ip, ua string) (string, error) {
	g := identidadecrypto.GeradorRefresh{}
	tok := g.Novo()
	if err := tokens.Inserir(ctx, identidade.NovoRefreshToken(uid, g.Hash(tok), time.Now(), ip, ua)); err != nil {
		return "", err
	}
	return tok.Revelar(), nil
}

// RevokedTokenError é o retrato do token revogado (estado do domínio).
type RevokedTokenError struct {
	TokenID   int64
	UsuarioID int64
	Reason    repositories.RevokeReason
	RevokedAt time.Time
	Recent    bool
	reuso     bool
}

func (e *RevokedTokenError) Error() string { return e.Unwrap().Error() }

func (e *RevokedTokenError) Unwrap() error {
	if e.Recent {
		return ErrRefreshTokenRevokedRecently
	}
	return ErrRefreshTokenRevoked
}

// IsRotationReuse é identidade.RefreshToken.ReusoDeRotacao.
func (e *RevokedTokenError) IsRotationReuse() bool { return e.reuso }

// ValidateRefreshToken busca pelo hash e classifica com Avaliar.
func (s *RefreshTokenService) ValidateRefreshToken(ctx context.Context, db *sql.DB, token string) (*identidade.RefreshToken, error) {
	g := identidadecrypto.GeradorRefresh{}
	rt, err := identidademysql.NovoRefreshTokens(db).PorHash(ctx, g.Hash(identidade.NovoTokenOpaco(token)))
	if err != nil {
		return nil, err
	}
	switch estado := rt.Avaliar(s.agora()); estado {
	case identidade.RefreshExpirado:
		return nil, ErrRefreshTokenExpired
	case identidade.RefreshRevogado, identidade.RefreshRevogadoRecente:
		return nil, &RevokedTokenError{TokenID: rt.ID, UsuarioID: rt.UsuarioID, Reason: repositories.RevokeReason(rt.Motivo),
			RevokedAt: *rt.RevogadoEm, Recent: estado == identidade.RefreshRevogadoRecente, reuso: rt.ReusoDeRotacao(estado)}
	default:
		return rt, nil
	}
}

// RevokeToken: busca + revogação condicional (o passo do Logout).
func (s *RefreshTokenService) RevokeToken(ctx context.Context, db *sql.DB, token string, motivo repositories.RevokeReason) error {
	tokens := identidademysql.NovoRefreshTokens(db)
	rt, err := tokens.PorHash(ctx, identidadecrypto.GeradorRefresh{}.Hash(identidade.NovoTokenOpaco(token)))
	if err != nil {
		return err
	}
	return tokens.Revogar(ctx, rt.ID, identidade.MotivoRevogacao(motivo))
}

func (s *RefreshTokenService) RevokeAllUserTokens(ctx context.Context, db *sql.DB, uid int64, motivo repositories.RevokeReason) error {
	return identidademysql.NovoRefreshTokens(db).RevogarTodosDoUsuario(ctx, uid, identidade.MotivoRevogacao(motivo))
}

func (s *RefreshTokenService) RegistrarReuso(ctx context.Context, db *sql.DB, tokenID int64) (bool, time.Time, error) {
	return s.casos(db).RegistrarReuso(ctx, tokenID)
}

func (s *RefreshTokenService) DesfazerReuso(ctx context.Context, db *sql.DB, tokenID int64, marca time.Time) error {
	return s.casos(db).DesfazerReuso(ctx, tokenID, marca)
}

func (s *RefreshTokenService) CleanupExpired(ctx context.Context, db *sql.DB) (int64, error) {
	return s.casos(db).LimparRefreshExpirados(ctx)
}

// IniciarLimpezaRefreshTokens: svc nil faz cada execução entrar em panic
// (recuperado pelo agendador).
func IniciarLimpezaRefreshTokens(ctx context.Context, db *sql.DB, svc *RefreshTokenService, intervalo time.Duration) <-chan struct{} {
	var c *identidadeapp.Casos
	if svc != nil {
		c = svc.casos(db)
	}
	return identidadeapp.IniciarLimpezaPeriodica(ctx, c, intervalo)
}

// RefreshRotation é a revogação condicional (motivo rotacao) e a emissão do
// novo token numa mesma tx, sobre o adapter ligado à tx.
type RefreshRotation struct {
	tx   *sql.Tx
	done bool
}

func (s *RefreshTokenService) BeginRotation(ctx context.Context, db *sql.DB, oldID int64) (*RefreshRotation, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if err := identidademysql.NovoRefreshTokens(tx).Revogar(ctx, oldID, identidade.MotivoRotacao); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return &RefreshRotation{tx: tx}, nil
}

func (r *RefreshRotation) Issue(ctx context.Context, uid int64, ip, ua string) (string, error) {
	if r.done {
		return "", errors.New("rotação já finalizada")
	}
	tok, err := emitirRefresh(ctx, identidademysql.NovoRefreshTokens(r.tx), uid, ip, ua)
	if err != nil {
		r.Rollback()
		return "", err
	}
	r.done = true
	if err := r.tx.Commit(); err != nil {
		return "", err
	}
	return tok, nil
}

func (r *RefreshRotation) Rollback() {
	if r.done {
		return
	}
	r.done = true
	_ = r.tx.Rollback()
}

// ---------------------------------------------------------------------------
// UsuarioService, SenhaHistoricoService, ResetSenhasAtivosService
// ---------------------------------------------------------------------------

type UsuarioService struct {
	db    *sql.DB
	cfg   *config.Config
	casos *identidadeapp.Casos
}

func NewUsuarioService(db *sql.DB, cfg *config.Config, email identidadeapp.EnviadorSenhaInicial) *UsuarioService {
	return &UsuarioService{db: db, cfg: cfg, casos: novosCasos(db, cfg, email, time.Now, identidadeapp.Opcoes{})}
}

func (s *UsuarioService) GetUsuarioByEmail(ctx context.Context, db *sql.DB, email string) (*identidade.Usuario, error) {
	return identidademysql.NovoUsuarios(db).PorEmail(ctx, email)
}

func (s *UsuarioService) GetUsuarioByID(ctx context.Context, db *sql.DB, id int64) (*identidade.Usuario, error) {
	return identidademysql.NovoUsuarios(db).PorID(ctx, id)
}

func (s *UsuarioService) ListUsuarios(ctx context.Context, _ *sql.DB, page, limit int, orderBy, orderDir string) ([]identidade.Usuario, int, error) {
	o := kernel.ParseOrdenacao(orderBy, orderDir, identidade.CamposOrdenacaoUsuario(), identidade.OrdenacaoUsuarioPadrao())
	r, err := s.casos.ListarUsuarios(ctx, adminTeste(), o, kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

// ResetSenha (sem chamador de produção no legado) grava uma senha escolhida:
// hash Argon2id + AtualizarSenha sem obrigar troca.
func (s *UsuarioService) ResetSenha(ctx context.Context, db *sql.DB, id int64, senha string) error {
	h, err := identidadecrypto.NovoHasher(s.cfg.HashSenha).Gerar(identidade.NovaSenha(senha))
	if err != nil {
		return err
	}
	return identidademysql.NovoUsuarios(db).AtualizarSenha(ctx, id, h, false)
}

// AdminResetPassword é o caso de uso AdminResetarSenha (relê o alvo, gera,
// grava, envia, registra o histórico e revoga as sessões).
func (s *UsuarioService) AdminResetPassword(ctx context.Context, _ *sql.DB, id int64, _, _ string) (bool, error) {
	return s.casos.AdminResetarSenha(ctx, adminTeste(), identidadeapp.ComandoAdminResetarSenha{UsuarioID: id})
}

func (s *UsuarioService) CreateUsuario(ctx context.Context, _ *sql.DB, in struct {
	Nome       string
	Email      string
	Role       string
	IDVendedor *int64
}) (*identidade.Usuario, bool, error) {
	return s.casos.CriarUsuario(ctx, adminTeste(), identidade.DadosUsuario{Nome: in.Nome, Email: in.Email, Role: in.Role, IDVendedor: in.IDVendedor})
}

func (s *UsuarioService) UpdateUsuario(ctx context.Context, _ *sql.DB, id int64, nome, role string, idVendedor *int64) (*identidade.Usuario, error) {
	return s.casos.AtualizarUsuario(ctx, adminTeste(), id, identidade.DadosUsuario{Nome: nome, Role: role, IDVendedor: idVendedor})
}

func (s *UsuarioService) ToggleAtivoUsuario(ctx context.Context, _ *sql.DB, id int64, ativo *bool) (*identidade.Usuario, error) {
	return s.casos.AlternarAtivoUsuario(ctx, adminTeste(), id, ativo)
}

type SenhaHistoricoService struct{}

func NewSenhaHistoricoService() SenhaHistoricoService { return SenhaHistoricoService{} }

func (SenhaHistoricoService) Registrar(ctx context.Context, db *sql.DB, uid int64, por *int64, hash, ip, ua, tipo string) error {
	return identidademysql.NovoHistoricoSenhas(db).Registrar(ctx, &identidade.RegistroSenha{UsuarioID: uid, ResetadoPorID: por,
		HashAnterior: identidade.NovoHashSenha(hash), IP: ip, UserAgent: ua, Tipo: identidade.TipoReset(tipo)})
}

func (SenhaHistoricoService) ListarPorUsuario(ctx context.Context, db *sql.DB, uid int64, page, limit int, orderBy, orderDir string) ([]identidade.RegistroSenha, int, error) {
	r, err := identidademysql.NovoHistoricoSenhas(db).DoUsuario(ctx, uid, ordenacaoHistorico(orderBy, orderDir), kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

func (SenhaHistoricoService) ListarTodos(ctx context.Context, db *sql.DB, page, limit int, orderBy, orderDir string) ([]identidade.RegistroSenha, int, error) {
	r, err := identidademysql.NovoHistoricoSenhas(db).Todos(ctx, ordenacaoHistorico(orderBy, orderDir), kernel.NovaPagina(page, limit))
	return r.Itens, r.Total, err
}

func ordenacaoHistorico(orderBy, orderDir string) kernel.Ordenacao[identidade.CampoHistorico] {
	return kernel.ParseOrdenacao(orderBy, orderDir, identidade.CamposOrdenacaoHistorico(), identidade.OrdenacaoHistoricoPadrao())
}

type ResetSenhasAtivosService struct{ u *UsuarioService }

func NewResetSenhasAtivosService(u *UsuarioService) *ResetSenhasAtivosService {
	return &ResetSenhasAtivosService{u: u}
}

func (s *ResetSenhasAtivosService) Executar(ctx context.Context, _ *sql.DB, simular bool) ([]ItemResetMassa, error) {
	return s.u.casos.ResetarSenhasAtivos(ctx, simular)
}

// verificarSenha / precisaRehash: o hasher de produção (ops01).
func verificarSenha(hs config.HashSenha, hash, senha string) bool {
	return identidadecrypto.NovoHasher(hs).Verificar(identidade.NovoHashSenha(hash), identidade.NovaSenha(senha))
}

func precisaRehash(hs config.HashSenha, hash string) bool {
	return identidadecrypto.NovoHasher(hs).PrecisaRehash(identidade.NovoHashSenha(hash))
}
