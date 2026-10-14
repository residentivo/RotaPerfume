package handlers_test

// ARQ-03: o AuthHandler legado (com SetCaptchaVerifier/SetAlertaSeguranca)
// e o AuthService saíram. Os handlers são os de internal/identidadehttp,
// montados pelo bootstrap sobre o db do sqlmock (adapters reais, mesmo SQL).
// authHandlerTeste devolve aos testes os dois pontos de troca que eles
// usavam, agora como dependências do caso de uso (captcha e publicador de
// eventos), sem setter na produção.

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/rotaperfumes-api/internal/identidadehttp"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/identidade"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
	"github.com/rotaperfumes/shared/kernel"
)

// alertaSegurancaNotificador é o assinante do evento de reuso (o notifier
// de produção tem esta assinatura).
type alertaSegurancaNotificador interface {
	Notificar(userID, tokenID int64, ip, ua string)
}

// authHandlerTeste é o AuthHandler real mais os pontos de troca do teste.
type authHandlerTeste struct {
	*identidadehttp.AuthHandler
	casos   *identidadeapp.Casos
	captcha *captchaTrocavel
	alerta  *alertaTrocavel
}

// SetCaptchaVerifier troca o verificador usado pelo caso de uso.
func (h *authHandlerTeste) SetCaptchaVerifier(v identidadeapp.VerificadorCaptcha) {
	h.captcha.trocar(v)
}

// SetAlertaSeguranca liga (ou desliga, com nil) o assinante do evento.
func (h *authHandlerTeste) SetAlertaSeguranca(n alertaSegurancaNotificador) { h.alerta.trocar(n) }

// novaIdentidadeTeste monta os casos de uso com captcha (aprova token não
// vazio) e publicador trocáveis e o e-mail fake.
func novaIdentidadeTeste(db *sql.DB, cfg *config.Config) *authHandlerTeste {
	c := &captchaTrocavel{v: &fakeCaptchaVerifier{}}
	a := &alertaTrocavel{}
	casos := bootstrap.NovaIdentidade(db, cfg, bootstrap.ExtIdentidade{Captcha: c, Publicador: a, Email: &fakeEmailService{}})
	return &authHandlerTeste{AuthHandler: bootstrap.NovoIdentidadeHTTP(casos, cfg).Auth, casos: casos, captcha: c, alerta: a}
}

// novoAuthHandler substitui novoAuthHandler(db, cfg).
func novoAuthHandler(db *sql.DB, cfg *config.Config) *authHandlerTeste {
	return novaIdentidadeTeste(db, cfg)
}

type captchaTrocavel struct {
	mu sync.Mutex
	v  identidadeapp.VerificadorCaptcha
}

func (c *captchaTrocavel) trocar(v identidadeapp.VerificadorCaptcha) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.v = v
}

func (c *captchaTrocavel) Verificar(ctx context.Context, token, ip string) error {
	c.mu.Lock()
	v := c.v
	c.mu.Unlock()
	return v.Verificar(ctx, token, ip)
}

type alertaTrocavel struct {
	mu sync.Mutex
	n  alertaSegurancaNotificador
}

func (a *alertaTrocavel) trocar(n alertaSegurancaNotificador) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.n = n
}

func (a *alertaTrocavel) Publicar(_ context.Context, e identidade.ReusoDeRefreshDetectado) {
	a.mu.Lock()
	n := a.n
	a.mu.Unlock()
	if n != nil {
		n.Notificar(e.UsuarioID, e.TokenID, e.IP, e.UserAgent)
	}
}

// authCrypto repassa as chamadas do antigo AuthService ao identidadecrypto.
type authCrypto struct{}

func novoAuthCrypto() authCrypto { return authCrypto{} }

func (authCrypto) HashPassword(cfg *config.Config, senha string) (string, error) {
	return gerarHashSenha(cfg.HashSenha, senha)
}

func (authCrypto) VerifyPassword(cfg *config.Config, hash, senha string) bool {
	return identidadecrypto.NovoHasher(cfg.HashSenha).Verificar(identidade.NovoHashSenha(hash), identidade.NovaSenha(senha))
}

func (authCrypto) NeedsRehash(cfg *config.Config, hash string) bool {
	return identidadecrypto.NovoHasher(cfg.HashSenha).PrecisaRehash(identidade.NovoHashSenha(hash))
}

func (authCrypto) GenerateJWT(cfg *config.Config, uid int64, role string) (string, error) {
	return identidadecrypto.NovoJWT(cfg).Emitir(uid, kernel.RoleDoBanco(role))
}

func gerarHashSenha(hs config.HashSenha, senha string) (string, error) {
	h, err := identidadecrypto.NovoHasher(hs).Gerar(identidade.NovaSenha(senha))
	return h.Revelar(), err
}

// jwtMiddleware substitui o middleware.JWTMiddleware legado (C11, removido
// no ARQ-03) nos testes de escopo que montam um handler legado isolado: o
// httpx.Autenticador real com um status "sem banco" (ativo, role do claim),
// como a variante sem checker fazia. protected=false não autentica.
func jwtMiddleware(cfg *config.Config, protected, requireAdmin bool) func(http.Handler) http.Handler {
	jwt := identidadecrypto.NovoJWT(cfg)
	a := httpx.NovoAutenticador(jwt, statusDoToken{jwt})
	return func(next http.Handler) http.Handler {
		if !protected {
			return next
		}
		politica := httpx.Authenticated
		if requireAdmin {
			politica = httpx.AdminOnly
		}
		interno := a.Exigir(politica, next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			interno.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), chaveToken{}, tokenBruto(r))))
		})
	}
}

type chaveToken struct{}

// statusDoToken devolve ativo e o role do claim do token do request.
type statusDoToken struct{ jwt *identidadecrypto.JWT }

func (s statusDoToken) StatusParaAutenticacao(ctx context.Context, _ int64) (identidade.StatusAcesso, error) {
	tok, _ := ctx.Value(chaveToken{}).(string)
	c, err := s.jwt.ValidarAcesso(tok)
	if err != nil {
		return identidade.StatusAcesso{}, err
	}
	return identidade.StatusAcesso{Ativo: true, Role: c.Role}, nil
}

func tokenBruto(r *http.Request) string {
	if _, tok, ok := strings.Cut(r.Header.Get("Authorization"), " "); ok {
		return tok
	}
	if c, err := r.Cookie(httpx.CookieAcesso); err == nil {
		return c.Value
	}
	return ""
}

// refreshSvcTeste emite um refresh token direto no banco (testes de
// integração), pelo mesmo caminho do login: gerador opaco + adapter MySQL.
type refreshSvcTeste struct{}

func novoRefreshSvc() refreshSvcTeste { return refreshSvcTeste{} }

func (refreshSvcTeste) GenerateRefreshToken(ctx context.Context, db *sql.DB, uid int64, ip, ua string) (string, error) {
	g := identidadecrypto.GeradorRefresh{}
	tok := g.Novo()
	rt := identidade.NovoRefreshToken(uid, g.Hash(tok), time.Now(), ip, ua)
	if err := identidademysql.NovoRefreshTokens(db).Inserir(ctx, rt); err != nil {
		return "", err
	}
	return tok.Revelar(), nil
}
