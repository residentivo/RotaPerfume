package handlers_test

// Fixtures de identidade dos testes de handler (ARQ-07: substituem o shim
// helpers_identidade_test.go, que imitava o AuthHandler/AuthService legados).
// Os casos de uso são montados pelo bootstrap sobre o db do sqlmock; captcha
// e publicador de eventos entram como dependências na montagem
// (ExtIdentidade) e são dublês trocáveis do teste, não setters de produção.

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
)

// assinanteReuso é o assinante do evento de reuso (o notifier de produção,
// notificacaoapp.AlertaSegurancaNotifier, tem esta assinatura).
type assinanteReuso interface {
	Notificar(userID, tokenID int64, ip, ua string)
}

// identidadeTeste é o AuthHandler real, os casos de uso que ele usa e os
// dublês injetados na montagem.
type identidadeTeste struct {
	*identidadehttp.AuthHandler
	casos   *identidadeapp.Casos
	captcha *captchaTrocavel
	alerta  *alertaTrocavel
}

// novaIdentidadeTeste monta os casos de uso com captcha (aprova token não
// vazio), publicador trocável e o e-mail fake.
func novaIdentidadeTeste(db *sql.DB, cfg *config.Config) *identidadeTeste {
	c := &captchaTrocavel{v: &fakeCaptchaVerifier{}}
	a := &alertaTrocavel{}
	casos := bootstrap.NovaIdentidade(db, cfg, bootstrap.ExtIdentidade{Captcha: c, Publicador: a, Email: &fakeEmailService{}})
	return &identidadeTeste{AuthHandler: bootstrap.NovoIdentidadeHTTP(casos, cfg).Auth, casos: casos, captcha: c, alerta: a}
}

// captchaTrocavel é o identidadeapp.VerificadorCaptcha do teste.
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

// alertaTrocavel é o publicador do evento de reuso do teste; sem assinante
// (nil) o evento é descartado.
type alertaTrocavel struct {
	mu sync.Mutex
	n  assinanteReuso
}

func (a *alertaTrocavel) trocar(n assinanteReuso) {
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

// gerarHashSenha gera o hash Argon2id de senha com os parâmetros hs.
func gerarHashSenha(hs config.HashSenha, senha string) (string, error) {
	h, err := identidadecrypto.NovoHasher(hs).Gerar(identidade.NovaSenha(senha))
	return h.Revelar(), err
}

// exigirAutenticado põe next atrás do httpx.Autenticador real (política
// Authenticated) para os testes de escopo que montam um handler isolado, sem
// o Router. O status do usuário vem do próprio token (ativo, role do claim):
// esses testes não exercitam o corte por banco (coberto em tests/middleware).
func exigirAutenticado(cfg *config.Config, next http.Handler) http.Handler {
	jwt := identidadecrypto.NovoJWT(cfg)
	interno := httpx.NovoAutenticador(jwt, statusDoToken{jwt}).Exigir(httpx.Authenticated, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		interno.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), chaveToken{}, tokenBruto(r))))
	})
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

// emitirRefresh grava um refresh token direto no banco (testes de
// integração), pelo mesmo caminho do login: gerador opaco + adapter MySQL.
func emitirRefresh(ctx context.Context, db *sql.DB, uid int64, ip, ua string) (string, error) {
	g := identidadecrypto.GeradorRefresh{}
	tok := g.Novo()
	rt := identidade.NovoRefreshToken(uid, g.Hash(tok), time.Now(), ip, ua)
	if err := identidademysql.NovoRefreshTokens(db).Inserir(ctx, rt); err != nil {
		return "", err
	}
	return tok.Revelar(), nil
}
