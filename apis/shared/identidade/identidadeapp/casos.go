package identidadeapp

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/kernel"
)

// Valores padrão das opções (os mesmos do legado).
const (
	// JanelaSupressaoReusoPadrao (SEC-12): um novo reuso do MESMO token
	// rotacionado dentro dela não corta as sessões de novo.
	JanelaSupressaoReusoPadrao = 30 * time.Minute
	// RetencaoRefreshPadrao (CHORE-02): tempo depois de expires_at antes de
	// a limpeza apagar o token.
	RetencaoRefreshPadrao = 720 * time.Hour
	// RetencaoRefreshMinima: piso da retenção (nunca apaga token que ainda
	// serve para detectar reuso, I-10).
	RetencaoRefreshMinima = 24 * time.Hour
	// LoteLimpezaRefresh: linhas por DELETE na limpeza.
	LoteLimpezaRefresh = 1000
	// maxTentativasSenhaGerada: regenerações se a senha aleatória colidir
	// com a atual no reset administrativo (defesa em profundidade).
	maxTentativasSenhaGerada = 3
)

// Deps são as dependências dos casos de uso, todas obrigatórias.
type Deps struct {
	Usuarios   Usuarios
	Tokens     RefreshTokens
	Historico  HistoricoSenhas
	Rotacao    Transactor
	Hasher     Hasher
	Emissor    EmissorAcesso
	Refresh    GeradorRefresh
	Senhas     GeradorSenha
	Captcha    VerificadorCaptcha
	Limites    Limites
	Publicador Publicador
	Email      EnviadorSenhaInicial
	Vendedores Vendedores
	// Relogio é a fonte de "agora" (expiração, janela de graça, cortes).
	Relogio func() time.Time
}

// Opcoes vêm da config, preenchidas pelo bootstrap. Zero ou negativo cai no
// padrão (como os antigos SetReuseSuppressWindow/SetRetencao).
type Opcoes struct {
	JanelaSupressaoReuso time.Duration
	RetencaoRefresh      time.Duration
}

// Casos agrupa os casos de uso do contexto (um método por caso de uso).
type Casos struct {
	d Deps
	o Opcoes
}

// Novo monta os casos de uso. Dependência nil é erro de montagem: panic.
func Novo(d Deps, o Opcoes) *Casos {
	obrigatorias := []any{d.Usuarios, d.Tokens, d.Historico, d.Rotacao, d.Hasher, d.Emissor, d.Refresh,
		d.Senhas, d.Captcha, d.Limites.Login, d.Limites.Refresh, d.Limites.Reset, d.Publicador, d.Email, d.Vendedores}
	for _, dep := range obrigatorias {
		if dep == nil {
			panic("identidadeapp: Novo exige todas as dependências")
		}
	}
	if d.Relogio == nil {
		panic("identidadeapp: Novo exige o relógio")
	}
	if o.JanelaSupressaoReuso <= 0 {
		o.JanelaSupressaoReuso = JanelaSupressaoReusoPadrao
	}
	if o.RetencaoRefresh <= 0 {
		o.RetencaoRefresh = RetencaoRefreshPadrao
	}
	return &Casos{d: d, o: o}
}

func (c *Casos) agora() time.Time { return c.d.Relogio() }

// bloqueio devolve o 429 com Retry-After (I-14).
func bloqueio(retry time.Duration) error {
	return &kernel.Bloqueio{Sentinela: identidade.ErrMuitasTentativas, RetryAfter: retry}
}

// verificarCaptcha roda ANTES do rate limit (I-13): não gasta o budget com
// bots óbvios. Fail-closed; token e secret nunca são logados.
func (c *Casos) verificarCaptcha(ctx context.Context, token, ip, acao string) error {
	if err := c.d.Captcha.Verificar(ctx, token, ip); err != nil {
		if strings.TrimSpace(token) == "" {
			log.Printf("[auth] %s: captchaToken ausente: ip=%s", acao, ip)
		} else {
			log.Printf("[auth] %s: falha na verificação de captcha: ip=%s", acao, ip)
		}
		return identidade.ErrCaptchaInvalido
	}
	return nil
}

// emitirRefresh gera e grava um refresh token (no pool ou na tx da rotação).
func (c *Casos) emitirRefresh(ctx context.Context, tokens RefreshTokens, usuarioID int64, o Origem) (identidade.TokenOpaco, error) {
	novo := c.d.Refresh.Novo()
	rt := identidade.NovoRefreshToken(usuarioID, c.d.Refresh.Hash(novo), c.agora(), o.IP, o.UserAgent)
	if err := tokens.Inserir(ctx, rt); err != nil {
		return identidade.TokenOpaco{}, err
	}
	log.Printf("[refresh] token criado: usuario_id=%d ip=%s", usuarioID, o.IP)
	return novo, nil
}

// revogarTodos revoga as sessões de refresh; falha só é logada (o chamador
// já gravou a mudança principal).
func (c *Casos) revogarTodos(ctx context.Context, usuarioID int64, m identidade.MotivoRevogacao, tagFalha string) {
	if err := c.d.Tokens.RevogarTodosDoUsuario(ctx, usuarioID, m); err != nil {
		log.Printf("%s: falha ao revogar refresh tokens do usuario_id=%d: %v", tagFalha, usuarioID, err)
		return
	}
	log.Printf("[refresh] todos tokens revogados: usuario_id=%d motivo=%s", usuarioID, m)
}

// exigirAdmin é a defesa em profundidade dos casos de uso de gestão (I-05):
// Principal inválido falha fechado (500); não admin, 403.
func exigirAdmin(p kernel.Principal) error {
	if !p.Valido() {
		return kernel.ErrPrincipalInvalido
	}
	if !p.EhAdmin() {
		return identidade.ErrAcessoRestritoAdmin
	}
	return nil
}

// SemPublicador é o Null Object do Publicador (testes e CLIs).
type SemPublicador struct{}

// Publicar não faz nada.
func (SemPublicador) Publicar(context.Context, identidade.ReusoDeRefreshDetectado) {}

// PublicadorLocal despacha o evento, em processo e em ordem, para cada
// assinante (Observer). Os assinantes não podem bloquear (o notificador de
// alerta já é assíncrono, I-08).
type PublicadorLocal struct {
	assinantes []func(context.Context, identidade.ReusoDeRefreshDetectado)
}

// NovoPublicadorLocal registra os assinantes.
func NovoPublicadorLocal(assinantes ...func(context.Context, identidade.ReusoDeRefreshDetectado)) *PublicadorLocal {
	return &PublicadorLocal{assinantes: assinantes}
}

// Publicar entrega o evento a todos os assinantes.
func (p *PublicadorLocal) Publicar(ctx context.Context, e identidade.ReusoDeRefreshDetectado) {
	for _, a := range p.assinantes {
		a(ctx, e)
	}
}
