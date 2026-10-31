package bootstrap

import (
	"database/sql"
	"log"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/internal/httpx"
	"github.com/rotaperfumes/rotaperfumes-api/internal/identidadehttp"
	"github.com/rotaperfumes/shared/adapters/ratelimit"
	"github.com/rotaperfumes/shared/adapters/turnstile"
	"github.com/rotaperfumes/shared/comercial/comercialmysql"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/db"
	"github.com/rotaperfumes/shared/identidade/identidadeapp"
	"github.com/rotaperfumes/shared/identidade/identidadecrypto"
	"github.com/rotaperfumes/shared/identidade/identidademysql"
)

// Limites de rate limit (I-14), os mesmos do legado: login por IP e por
// IP+conta, refresh por IP, troca de senha por IP+usuário.
const (
	loginMaxFalhas   = 5
	loginJanela      = time.Minute
	loginBloqueio    = 5 * time.Minute
	refreshMaxFalhas = 10
	refreshJanela    = time.Minute
	refreshBloqueio  = 5 * time.Minute
	resetMaxFalhas   = 10
	resetJanela      = 5 * time.Minute
	resetBloqueio    = 10 * time.Minute
)

// ExtIdentidade são as dependências externas do contexto que variam entre
// produção, testes e CLI.
type ExtIdentidade struct {
	Captcha    identidadeapp.VerificadorCaptcha
	Publicador identidadeapp.Publicador
	Email      identidadeapp.EnviadorSenhaInicial
}

// NovaIdentidade liga o contexto Identidade sobre o pool: adapters MariaDB,
// crypto, os três limitadores e os UoW da rotação e da troca/reset de senha
// (BUG-15). Chamar UMA vez por processo: os limitadores são singletons
// (I-14).
func NovaIdentidade(pool *sql.DB, cfg *config.Config, ext ExtIdentidade) *identidadeapp.Casos {
	return identidadeapp.Novo(identidadeapp.Deps{
		Usuarios:  identidademysql.NovoUsuarios(pool),
		Tokens:    identidademysql.NovoRefreshTokens(pool),
		Historico: identidademysql.NovoHistoricoSenhas(pool),
		Rotacao: db.NewTransactor(pool, func(q db.Querier) identidadeapp.TxRotacao {
			return identidadeapp.TxRotacao{Tokens: identidademysql.NovoRefreshTokens(q)}
		}),
		TrocaSenha: db.NewTransactor(pool, func(q db.Querier) identidadeapp.TxSenha {
			return identidadeapp.TxSenha{
				Usuarios:  identidademysql.NovoUsuarios(q),
				Historico: identidademysql.NovoHistoricoSenhas(q),
				Tokens:    identidademysql.NovoRefreshTokens(q),
			}
		}),
		Hasher:  identidadecrypto.NovoHasher(cfg.HashSenha),
		Emissor: identidadecrypto.NovoJWT(cfg),
		Refresh: identidadecrypto.GeradorRefresh{},
		Senhas:  identidadecrypto.GeradorSenha{},
		Captcha: ext.Captcha,
		Limites: identidadeapp.Limites{
			Login:   ratelimit.Nova(loginMaxFalhas, loginJanela, loginBloqueio),
			Refresh: ratelimit.Nova(refreshMaxFalhas, refreshJanela, refreshBloqueio),
			Reset:   ratelimit.Nova(resetMaxFalhas, resetJanela, resetBloqueio),
		},
		Publicador: ext.Publicador,
		Email:      ext.Email,
		Vendedores: vendedoresParaIdentidade{vendedores: comercialmysql.NovoVendedores(pool)},
		Relogio:    time.Now,
	}, identidadeapp.Opcoes{
		JanelaSupressaoReuso: cfg.RefreshReuseSuppressWindow,
		RetencaoRefresh:      cfg.RefreshTokenRetencao,
	})
}

// extPadrao: Turnstile real, sem assinante de evento.
func extPadrao(cfg *config.Config, email identidadeapp.EnviadorSenhaInicial) ExtIdentidade {
	return ExtIdentidade{Captcha: turnstile.Novo(cfg.TurnstileSecretKey), Publicador: identidadeapp.SemPublicador{}, Email: email}
}

// IdentidadeHTTP são os handlers do contexto.
type IdentidadeHTTP struct {
	Auth           *identidadehttp.AuthHandler
	Usuario        *identidadehttp.UsuarioHandler
	SenhaHistorico *identidadehttp.SenhaHistoricoHandler
}

// NovoIdentidadeHTTP monta os handlers sobre os casos de uso. Com
// COOKIE_SECURE=false loga um aviso (cookies sem Secure, SEC-21).
func NovoIdentidadeHTTP(c *identidadeapp.Casos, cfg *config.Config) IdentidadeHTTP {
	if !cfg.CookieSecure {
		log.Printf("[config] COOKIE_SECURE=false: cookies de sessão sem Secure (somente dev em HTTP)")
	}
	return IdentidadeHTTP{
		Auth:           identidadehttp.NovoAuthHandler(c, cfg.JWTTTL, cfg.TrustProxyHeaders, cfg.CookieSecure),
		Usuario:        identidadehttp.NovoUsuarioHandler(c, cfg.TrustProxyHeaders),
		SenhaHistorico: identidadehttp.NovoSenhaHistoricoHandler(c),
	}
}

// NovoVerificadorStatus é a consulta de status direto no banco (o adapter
// MariaDB satisfaz httpx.ConsultaStatus); em produção o checker é o próprio
// caso de uso StatusParaAutenticacao.
func NovoVerificadorStatus(pool *sql.DB) httpx.ConsultaStatus {
	return identidademysql.NovoUsuarios(pool)
}
