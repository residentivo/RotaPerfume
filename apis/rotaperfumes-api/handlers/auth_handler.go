// Package handlers contém os handlers HTTP da API de autenticação.
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rotaperfumes/rotaperfumes-api/middleware"
	"github.com/rotaperfumes/rotaperfumes-api/services"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/models"
	"github.com/rotaperfumes/shared/repositories"
	sharedsvc "github.com/rotaperfumes/shared/services"
)

// Parâmetros do rate limiting anti-bruteforce de login/refresh: no máximo
// loginMaxFailures tentativas falhas em loginWindow, bloqueando a chave
// (IP, ou IP+email) por loginBlockFor após exceder o limite.
const (
	loginMaxFailures = 5
	loginWindow      = 1 * time.Minute
	loginBlockFor    = 5 * time.Minute

	refreshMaxFailures = 10
	refreshWindow      = 1 * time.Minute
	refreshBlockFor    = 5 * time.Minute

	// Rate limiting do reset-password: mais permissivo que login (usuário já
	// autenticado), mas ainda protege contra automação de tentativas de
	// adivinhar a senha_atual.
	resetPasswordMaxFailures = 10
	resetPasswordWindow      = 5 * time.Minute
	resetPasswordBlockFor    = 10 * time.Minute
)

// AuthHandler trata as rotas /api/auth/*.
type AuthHandler struct {
	db                   *sql.DB
	repo                 *repositories.UsuarioRepository
	vendedorRepo         *repositories.VendedorRepository
	auth                 *sharedsvc.AuthService
	refreshSvc           *services.RefreshTokenService
	senhaSvc             *services.SenhaHistoricoService
	cfg                  *config.Config
	loginLimiter         *middleware.LoginRateLimiter
	refreshLimiter       *middleware.LoginRateLimiter
	resetPasswordLimiter *middleware.LoginRateLimiter
	captcha              sharedsvc.CaptchaVerifier
}

// NewAuthHandler cria um AuthHandler com pool de conexão injetado.
func NewAuthHandler(db *sql.DB, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		db:                   db,
		repo:                 repositories.NewUsuarioRepository(),
		vendedorRepo:         repositories.NewVendedorRepository(),
		auth:                 sharedsvc.NewAuthService(),
		refreshSvc:           services.NewRefreshTokenService(),
		senhaSvc:             services.NewSenhaHistoricoService(),
		cfg:                  cfg,
		loginLimiter:         middleware.NewLoginRateLimiter(loginMaxFailures, loginWindow, loginBlockFor),
		refreshLimiter:       middleware.NewLoginRateLimiter(refreshMaxFailures, refreshWindow, refreshBlockFor),
		resetPasswordLimiter: middleware.NewLoginRateLimiter(resetPasswordMaxFailures, resetPasswordWindow, resetPasswordBlockFor),
		captcha:              sharedsvc.NewTurnstileService(cfg),
	}
}

// LoginRequest body do POST /api/auth/login.
// CaptchaToken é o token do Cloudflare Turnstile gerado no frontend
// (widget) e enviado no campo JSON "captchaToken" — contrato acordado com o
// FrontBrain. Obrigatório: requisições sem esse campo são recusadas (400)
// antes mesmo de consultar o Cloudflare.
type LoginRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	CaptchaToken string `json:"captchaToken"`
}

// RefreshTokenRequest body do POST /api/auth/refresh.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// ResetPasswordRequest body do POST /api/auth/reset-password.
// O usuario_id é extraído do token JWT — não deve vir no body.
// CaptchaToken é o token do Cloudflare Turnstile gerado no frontend e
// enviado no campo JSON "captchaToken" — mesmo contrato usado em LoginRequest.
type ResetPasswordRequest struct {
	SenhaAtual   string `json:"senha_atual"`
	NovaSenha    string `json:"nova_senha"`
	CaptchaToken string `json:"captchaToken"`
}

// setTokenCookies define os cookies HttpOnly com os tokens de autenticacao.
// Secure eh ativado apenas em producao (false em dev localhost para evitar erro de HTTPS).
func setTokenCookies(w http.ResponseWriter, r *http.Request, accessToken, refreshToken string, maxAge int) {
	isLocalhost := strings.HasPrefix(r.Host, "localhost") ||
		strings.HasPrefix(r.Host, "127.0.0.1") ||
		strings.HasPrefix(r.Host, "0.0.0.0")

	// access_token: SameSite=Strict para protecao CSRF maxima.
	accessCookie := &http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   !isLocalhost, // true em producao, false em dev localhost
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, accessCookie)

	// refresh_token: SameSite=Lax para permitir refresh em navegacao top-level.
	refreshCookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 7, // 7 dias
		HttpOnly: true,
		Secure:   !isLocalhost,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, refreshCookie)
}

// clearTokenCookies remove os cookies de tokens (logout).
func clearTokenCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   "access_token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:   "refresh_token",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

// extractRefreshToken extrai o refresh token do body OU do cookie (Set-Cookie flow).
func extractRefreshToken(r *http.Request, bodyRefresh string) string {
	if bodyRefresh != "" {
		return bodyRefresh
	}
	if c, err := r.Cookie("refresh_token"); err == nil {
		return c.Value
	}
	return ""
}

// Login POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, nil, "email e senha são obrigatórios")
		return
	}

	maskedEmail := maskEmail(req.Email)
	log.Printf("[auth] Login attempt: email=%s", maskedEmail)

	ipOrigemPreCheck := getClientIP(r, h.cfg.TrustProxyHeaders)

	// Validação do CAPTCHA (Cloudflare Turnstile) roda ANTES do rate limiting
	// para não gastar o "budget" de rate limit contra bots óbvios. Fail-closed:
	// token ausente ou inválido/irverificável => recusa imediatamente.
	if !h.verifyCaptcha(w, r, req.CaptchaToken, ipOrigemPreCheck, "login") {
		return
	}

	// Rate limiting anti-bruteforce: bloqueia por IP e por conta (IP+email)
	// após várias falhas consecutivas em janela curta.
	ipKey := "ip:" + ipOrigemPreCheck
	acctKey := "acct:" + ipOrigemPreCheck + "|" + req.Email
	if blocked, retryAfter := h.loginLimiter.Blocked(ipKey); blocked {
		log.Printf("[auth] login: IP bloqueado por rate limit: ip=%s retry_after=%s", ipOrigemPreCheck, retryAfter)
		writeRateLimited(w, retryAfter)
		return
	}
	if blocked, retryAfter := h.loginLimiter.Blocked(acctKey); blocked {
		log.Printf("[auth] login: conta bloqueada por rate limit: email=%s retry_after=%s", maskedEmail, retryAfter)
		writeRateLimited(w, retryAfter)
		return
	}

	ctx := r.Context()
	u, err := h.repo.GetByEmail(ctx, h.db, req.Email)
	if err != nil {
		if err == repositories.ErrNotFound {
			log.Printf("[auth] login: usuário não encontrado: %s", maskedEmail)
			h.loginLimiter.RegisterFailure(ipKey)
			h.loginLimiter.RegisterFailure(acctKey)
			writeJSON(w, http.StatusUnauthorized, nil, "credenciais inválidas")
			return
		}
		log.Printf("[auth] login: repo.GetByEmail: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	if !u.IsActive() {
		log.Printf("[auth] login: usuário inativo: %s", maskedEmail)
		h.loginLimiter.RegisterFailure(ipKey)
		h.loginLimiter.RegisterFailure(acctKey)
		writeJSON(w, http.StatusUnauthorized, nil, "usuário inativo")
		return
	}

	if !h.auth.VerifyPassword(u.PasswordHash, req.Password) {
		log.Printf("[auth] login: senha incorreta para: %s", maskedEmail)
		h.loginLimiter.RegisterFailure(ipKey)
		h.loginLimiter.RegisterFailure(acctKey)
		writeJSON(w, http.StatusUnauthorized, nil, "credenciais inválidas")
		return
	}

	// Login bem-sucedido: limpa os contadores de falhas.
	h.loginLimiter.RegisterSuccess(ipKey)
	h.loginLimiter.RegisterSuccess(acctKey)

	// Gera access token JWT.
	token, err := h.auth.GenerateJWT(h.cfg, u.ID, u.Role)
	if err != nil {
		log.Printf("[auth] login: GenerateJWT: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro ao gerar token")
		return
	}

	// Gera refresh token.
	userAgent := r.UserAgent()
	refreshToken, err := h.refreshSvc.GenerateRefreshToken(ctx, h.db, u.ID, ipOrigemPreCheck, userAgent)
	if err != nil {
		log.Printf("[auth] login: GenerateRefreshToken: %v", err)
		// Não falha o login, apenas não retorna refresh token.
	}

	// Atualiza ultimo_login_at (não falha o login se falhar).
	_ = h.repo.UpdateUltimoLogin(ctx, h.db, u.ID, time.Now())

	// Flag de primeiro acesso / senha gerada pelo sistema, persistida na coluna
	// deve_trocar_senha e populada pelo repositório em GetByEmail.
	trocarSenha := u.DeveTrocarSenha

	log.Printf("[auth] login OK: user_id=%d role=%s trocar_senha=%t", u.ID, u.Role, trocarSenha)

	// Define tokens via Set-Cookie HttpOnly (NÃO mais no body JSON — vulnerabilidade #1 mitigada).
	setTokenCookies(w, r, token, refreshToken, int(h.cfg.JWTTTL.Seconds()))

	// Retorna apenas dados do usuário (tokens via cookie).
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]any{
		"success": true,
		"data": map[string]any{
			"token_type": "Bearer",
			"expires_in": int64(h.cfg.JWTTTL.Seconds()),
			"user": map[string]any{
				"id":    u.ID,
				"email": u.Email,
				"role":  u.Role,
				"nome":  u.Nome,
			},
			"trocar_senha": trocarSenha,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// Refresh POST /api/auth/refresh
// Recebe refresh_token (body OU cookie), valida, revoga o antigo e retorna novo
// access_token + refresh_token via Set-Cookie HttpOnly.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // body opcional, usa cookie se vazio

	refreshTokenInput := extractRefreshToken(r, req.RefreshToken)
	if refreshTokenInput == "" {
		writeJSON(w, http.StatusBadRequest, nil, "refresh_token é obrigatório (body ou cookie)")
		return
	}

	ctx := r.Context()
	ipOrigem := getClientIP(r, h.cfg.TrustProxyHeaders)
	userAgent := r.UserAgent()

	// Rate limiting anti-bruteforce por IP: bloqueia tentativas repetidas de
	// forjar/adivinhar refresh tokens. Contam como falha: token não encontrado,
	// expirado e revogado fora da janela de graça (possível roubo). NÃO contam
	// (SEC-04): token revogado dentro de services.RefreshRevokeGraceWindow e a
	// corrida no BeginRotation — ambos são rotação concorrente legítima (abas,
	// usuários atrás de NAT) e continuam recebendo 401 sem emitir tokens.
	refreshIPKey := "refresh-ip:" + ipOrigem
	if blocked, retryAfter := h.refreshLimiter.Blocked(refreshIPKey); blocked {
		log.Printf("[auth] refresh: IP bloqueado por rate limit: ip=%s retry_after=%s", ipOrigem, retryAfter)
		writeRateLimited(w, retryAfter)
		return
	}

	// Valida o refresh token.
	rt, err := h.refreshSvc.ValidateRefreshToken(ctx, h.db, refreshTokenInput)
	if err != nil {
		if errors.Is(err, services.ErrRefreshTokenNotFound) {
			log.Printf("[auth] refresh: token não encontrado")
			h.refreshLimiter.RegisterFailure(refreshIPKey)
			writeJSON(w, http.StatusUnauthorized, nil, "refresh token inválido")
			return
		}
		if errors.Is(err, services.ErrRefreshTokenExpired) {
			log.Printf("[auth] refresh: token expirado")
			h.refreshLimiter.RegisterFailure(refreshIPKey)
			writeJSON(w, http.StatusUnauthorized, nil, "refresh token expirado")
			return
		}
		// Deve vir antes de ErrRefreshTokenRevoked, que ele envolve.
		if errors.Is(err, services.ErrRefreshTokenRevokedRecently) {
			log.Printf("[auth] refresh: token revogado recentemente (corrida entre abas), não conta no rate limit: ip=%s", ipOrigem)
			writeJSON(w, http.StatusUnauthorized, nil, "refresh token revogado")
			return
		}
		if errors.Is(err, services.ErrRefreshTokenRevoked) {
			log.Printf("[auth][seguranca] refresh: reuso de refresh token revogado fora da janela de graça (possível roubo de token): ip=%s ua=%s", ipOrigem, userAgent)
			h.refreshLimiter.RegisterFailure(refreshIPKey)
			writeJSON(w, http.StatusUnauthorized, nil, "refresh token revogado")
			return
		}
		log.Printf("[auth] refresh: ValidateRefreshToken: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	// Busca o usuário para gerar novo access token.
	u, err := h.repo.GetByID(ctx, h.db, rt.UsuarioID)
	if err != nil {
		log.Printf("[auth] refresh: GetByID: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	if !u.IsActive() {
		log.Printf("[auth] refresh: usuário inativo: user_id=%d", u.ID)
		writeJSON(w, http.StatusUnauthorized, nil, "usuário inativo")
		return
	}

	// Revoga o refresh token antigo (single-use) de forma atômica, reutilizando
	// o ID obtido na validação. Em uma corrida entre duas requisições com o
	// mesmo token, só uma revoga; a outra recebe 401 sem emitir tokens. Essa
	// corrida é rotação concorrente legítima e não conta no rate limit (SEC-04).
	rotation, err := h.refreshSvc.BeginRotation(ctx, h.db, rt.ID)
	if err != nil {
		if errors.Is(err, services.ErrRefreshTokenRevoked) {
			log.Printf("[auth] refresh: token já revogado por requisição concorrente (corrida no BeginRotation), não conta no rate limit: user_id=%d ip=%s", u.ID, ipOrigem)
			writeJSON(w, http.StatusUnauthorized, nil, "refresh token revogado")
			return
		}
		log.Printf("[auth] refresh: BeginRotation: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	defer rotation.Rollback() // no-op após Issue bem-sucedido

	// Gera novo access token.
	newAccessToken, err := h.auth.GenerateJWT(h.cfg, u.ID, u.Role)
	if err != nil {
		log.Printf("[auth] refresh: GenerateJWT: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro ao gerar access token")
		return
	}

	// Gera novo refresh token na mesma transação da revogação e confirma.
	newRefreshToken, err := rotation.Issue(ctx, u.ID, ipOrigem, userAgent)
	if err != nil {
		log.Printf("[auth] refresh: GenerateRefreshToken: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	// Refresh concluído: limpa o contador de falhas do IP.
	h.refreshLimiter.RegisterSuccess(refreshIPKey)

	log.Printf("[auth] refresh OK: user_id=%d", u.ID)

	// Define novos tokens via Set-Cookie HttpOnly.
	setTokenCookies(w, r, newAccessToken, newRefreshToken, int(h.cfg.JWTTTL.Seconds()))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]any{
		"success": true,
		"data": map[string]any{
			"token_type": "Bearer",
			"expires_in": int64(h.cfg.JWTTTL.Seconds()),
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// ResetPassword POST /api/auth/reset-password
// O usuário autenticado troca a própria senha informando a senha atual.
// O usuario_id é extraído do token JWT — não deve vir no body.
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, nil, "não autenticado")
		return
	}

	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, nil, "body JSON inválido")
		return
	}
	if req.SenhaAtual == "" {
		writeJSON(w, http.StatusBadRequest, nil, "senha_atual é obrigatória")
		return
	}
	if req.NovaSenha == "" {
		writeJSON(w, http.StatusBadRequest, nil, "nova_senha é obrigatória")
		return
	}
	ipOrigemPreCheck := getClientIP(r, h.cfg.TrustProxyHeaders)

	// Validação do CAPTCHA (Cloudflare Turnstile) roda ANTES do rate limiting,
	// mesmo padrão do Login. Fail-closed: token ausente/inválido => recusa.
	if !h.verifyCaptcha(w, r, req.CaptchaToken, ipOrigemPreCheck, "reset-password") {
		return
	}

	// Rate limiting anti-bruteforce por IP+usuário: protege contra automação
	// de tentativas de adivinhar a senha_atual (endpoint antes não tinha
	// nenhum rate limit).
	resetKey := "reset:" + ipOrigemPreCheck + "|" + strconv.FormatInt(uid, 10)
	if blocked, retryAfter := h.resetPasswordLimiter.Blocked(resetKey); blocked {
		log.Printf("[auth] reset-password: bloqueado por rate limit: user_id=%d retry_after=%s", uid, retryAfter)
		writeRateLimited(w, retryAfter)
		return
	}

	ctx := r.Context()
	u, err := h.repo.GetByID(ctx, h.db, uid)
	if err != nil {
		if err == repositories.ErrNotFound {
			writeJSON(w, http.StatusNotFound, nil, "usuário não encontrado")
			return
		}
		log.Printf("[auth] reset-password: GetByID: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	if !h.auth.VerifyPassword(u.PasswordHash, req.SenhaAtual) {
		log.Printf("[auth] reset-password: senha atual incorreta: user_id=%d", uid)
		h.resetPasswordLimiter.RegisterFailure(resetKey)
		// 400 (e não 401): o usuário está autenticado. 401 fica reservado para
		// sessão/JWT inválido — o frontend trata 401 como token expirado, faz
		// refresh e reenvia o body com o mesmo captchaToken (uso único),
		// mascarando este erro como "captcha inválido".
		writeJSON(w, http.StatusBadRequest, nil, "senha atual incorreta")
		return
	}

	// Senha atual confirmada: limpa o contador de falhas do rate limiter.
	h.resetPasswordLimiter.RegisterSuccess(resetKey)

	// Nova senha igual à atual: checado ANTES da força, senão uma senha atual
	// fraca retornaria o erro de força em vez do problema real. Não conta no
	// rate limit: a senha atual já foi confirmada, então não há vazamento.
	if req.NovaSenha == req.SenhaAtual {
		log.Printf("[auth] reset-password: rejeitada: user_id=%d motivo=igual_senha_atual", uid)
		writeJSON(w, http.StatusBadRequest, nil, "a nova senha não pode ser igual à senha atual")
		return
	}

	// Força da nova senha: erro de usabilidade normal — não conta no rate
	// limit de reset-password (só tentativas de adivinhar segredo contam).
	if err := sharedsvc.ValidarForcaSenha(req.NovaSenha); err != nil {
		log.Printf("[auth] reset-password: rejeitada: user_id=%d motivo=senha_fraca", uid)
		writeJSON(w, http.StatusBadRequest, nil, err.Error())
		return
	}

	// Bloqueio de reuso das últimas 3 senhas: hash atual + os 2 registros
	// mais recentes do histórico. Rodamos as 3 comparações bcrypt até o
	// fim, sem short-circuit, para não criar um side-channel de timing que
	// revele qual das 3 senhas anteriores foi reutilizada.
	historico, _, err := h.senhaSvc.ListarPorUsuario(ctx, h.db, uid, 1, 2, "id", "desc")
	if err != nil {
		log.Printf("[auth] reset-password: ListarPorUsuario: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}
	hashesCandidatos := make([]string, 0, 3)
	hashesCandidatos = append(hashesCandidatos, u.PasswordHash)
	for _, registro := range historico {
		hashesCandidatos = append(hashesCandidatos, registro.SenhaHashAnterior)
	}
	reutilizada := false
	for _, hashCandidato := range hashesCandidatos {
		if h.auth.VerifyPassword(hashCandidato, req.NovaSenha) {
			reutilizada = true
		}
	}
	if reutilizada {
		log.Printf("[auth] reset-password: rejeitada: user_id=%d motivo=senha_reutilizada", uid)
		h.resetPasswordLimiter.RegisterFailure(resetKey)
		writeJSON(w, http.StatusBadRequest, nil, "a nova senha não pode ser igual a uma das últimas senhas utilizadas")
		return
	}

	newHash, err := h.auth.HashPassword(h.cfg, req.NovaSenha)
	if err != nil {
		log.Printf("[auth] reset-password: HashPassword: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	// Registrar no histórico de senhas (tipo "usuario" = auto-troca).
	ipOrigem := getClientIP(r, h.cfg.TrustProxyHeaders)
	userAgent := r.UserAgent()
	_ = h.senhaSvc.Registrar(ctx, h.db, uid, nil, u.PasswordHash, ipOrigem, userAgent, "usuario")

	// Troca voluntária pelo próprio usuário — não é mais primeiro acesso.
	if err := h.repo.UpdatePasswordHash(ctx, h.db, uid, newHash, false); err != nil {
		log.Printf("[auth] reset-password: UpdatePasswordHash: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	// Revoga todos os refresh tokens após troca de senha (security best practice).
	_ = h.refreshSvc.RevokeAllUserTokens(ctx, h.db, uid)

	// Limpa cookies (forçar novo login).
	clearTokenCookies(w)

	log.Printf("[auth] reset-password: user_id=%d", uid)
	writeJSON(w, http.StatusOK, map[string]any{
		"mensagem": "senha alterada com sucesso",
	}, "")
}

// Me GET /api/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, nil, "não autenticado")
		return
	}

	ctx := r.Context()
	u, err := h.repo.GetByID(ctx, h.db, uid)
	if err != nil {
		if err == repositories.ErrNotFound {
			writeJSON(w, http.StatusNotFound, nil, "usuário não encontrado")
			return
		}
		log.Printf("[auth] me: GetByID: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	vendedorDesligado, err := h.vendedorDesligadoDoUsuario(ctx, u)
	if err != nil {
		log.Printf("[auth] me: vendedor desligado: %v", err)
		writeJSON(w, http.StatusInternalServerError, nil, "erro interno")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":                 u.ID,
		"nome":               u.Nome,
		"email":              u.Email,
		"role":               u.Role,
		"ativo":              u.Ativo,
		"id_vendedor":        u.IDVendedor,
		"created_at":         u.CreatedAt,
		"ultimo_login_at":    u.UltimoLoginAt,
		"vendedor_desligado": vendedorDesligado,
	}, "")
}

// vendedorDesligadoDoUsuario reporta se o usuário (não admin) está vinculado
// a um vendedor desligado. Admin e usuário sem vínculo (ou com vínculo órfão)
// retornam false sem erro. Consulta própria ao banco a cada chamada (sem
// cache L1).
func (h *AuthHandler) vendedorDesligadoDoUsuario(ctx context.Context, u *models.Usuario) (bool, error) {
	if u.IsAdmin() || u.IDVendedor == nil || *u.IDVendedor <= 0 {
		return false, nil
	}
	desligado, err := h.vendedorRepo.IsDesligado(ctx, h.db, *u.IDVendedor)
	if errors.Is(err, repositories.ErrNotFound) {
		return false, nil
	}
	return desligado, err
}

// Logout POST /api/auth/logout
// Revoga o refresh token atual e limpa os cookies.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Pode receber refresh_token no body OU no cookie.
	var req RefreshTokenRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // body opcional

	ctx := r.Context()
	refreshInput := extractRefreshToken(r, req.RefreshToken)
	if refreshInput != "" {
		if err := h.refreshSvc.RevokeToken(ctx, h.db, refreshInput); err != nil {
			if !errors.Is(err, services.ErrRefreshTokenNotFound) && !errors.Is(err, services.ErrRefreshTokenRevoked) {
				log.Printf("[auth] logout: RevokeToken: %v", err)
			}
		}
	}

	// Limpa cookies.
	clearTokenCookies(w)

	log.Printf("[auth] logout OK")
	writeJSON(w, http.StatusOK, map[string]any{
		"mensagem": "logout realizado",
	}, "")
}

// getClientIP extrai o IP do cliente. Os headers X-Forwarded-For/X-Real-IP só
// são considerados quando trustProxyHeaders=true (ou seja, quando a aplicação
// roda atrás de um proxy/load balancer confiável que os popula corretamente).
// Caso contrário — e por padrão — esses headers são ignorados (são facilmente
// forjáveis pelo próprio cliente) e r.RemoteAddr é sempre usado.
func getClientIP(r *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			parts := strings.Split(fwd, ",")
			return strings.TrimSpace(parts[0])
		}
		if fwd := r.Header.Get("X-Real-IP"); fwd != "" {
			return fwd
		}
	}
	return stripPort(r.RemoteAddr)
}

// stripPort remove a porta de um endereço "IP:porta" (ou "[IPv6]:porta"),
// retornando apenas o IP. Se o valor não puder ser interpretado como
// host:porta (ex.: já é só um IP), o valor original é devolvido sem mudanças.
func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return host
}

// maskEmail mascara parcialmente um e-mail para uso em logs, preservando
// apenas o primeiro caractere do usuário e o domínio completo
// (ex.: "ana.silva@empresa.com" -> "a***@empresa.com").
func maskEmail(email string) string {
	at := strings.Index(email, "@")
	if at <= 0 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}

// SetCaptchaVerifier substitui o CaptchaVerifier usado pelo handler. Existe
// apenas para permitir injeção de mocks/fakes em testes (produção sempre usa
// o TurnstileService criado por NewAuthHandler) — não redesenha a
// arquitetura, apenas expõe um setter mínimo para testabilidade.
func (h *AuthHandler) SetCaptchaVerifier(v sharedsvc.CaptchaVerifier) {
	h.captcha = v
}

// verifyCaptcha valida o token do Cloudflare Turnstile (campo JSON
// "captchaToken") enviado pelo frontend. Roda ANTES das checagens de rate
// limiting para economizar esse "budget" contra bots óbvios. Comportamento
// fail-closed: token ausente ou inválido/irverificável (erro de rede, timeout,
// resposta inesperada, success=false) faz a requisição ser recusada com 400,
// sem nunca logar o token ou a secret key. Retorna true se a validação
// passou (chamador pode prosseguir).
func (h *AuthHandler) verifyCaptcha(w http.ResponseWriter, r *http.Request, token, remoteIP, acao string) bool {
	if err := h.captcha.Verify(r.Context(), token, remoteIP); err != nil {
		if errors.Is(err, sharedsvc.ErrCaptchaTokenAusente) {
			log.Printf("[auth] %s: captchaToken ausente: ip=%s", acao, remoteIP)
		} else {
			log.Printf("[auth] %s: falha na verificação de captcha: ip=%s", acao, remoteIP)
		}
		writeJSON(w, http.StatusBadRequest, nil, "captcha inválido ou ausente")
		return false
	}
	return true
}

// writeRateLimited escreve uma resposta 429 padronizada, incluindo o header
// Retry-After (em segundos) para orientar o cliente sobre quando tentar de novo.
func writeRateLimited(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeJSON(w, http.StatusTooManyRequests, nil, "muitas tentativas — tente novamente mais tarde")
}
