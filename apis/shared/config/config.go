// Package config carrega configurações do sistema a partir de variáveis de ambiente.
//
// O arquivo .env na raiz do projeto é carregado pelos binários (cmdutil /
// godotenv) ou pelo Makefile ("-include .env") antes de chamar o Load — aqui
// apenas consumimos via os.Getenv. DB_USUARIO/DB_SENHA são obrigatórios e
// não têm default (SEC-11): sem eles, Load devolve ErrCredenciaisDB.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	// Fixa time.Local em -03:00 (sem horário de verão) antes de qualquer
	// sql.Open — ver pacote tz (RISCO-01). Todo binário que usa config.DSN()
	// herda o fuso por este import.
	_ "github.com/rotaperfumes/shared/tz"
)

// Config agrega todas as configurações da aplicação.
type Config struct {
	// Banco de dados
	DBHost    string
	DBPort    string
	DBName    string
	DBUsuario string
	DBSenha   string

	// JWT
	JWTSecret string
	JWTTTL    time.Duration
	JWTIssuer string

	// HashSenha: Argon2id + pepper das senhas (SEC-13).
	HashSenha HashSenha

	// Logging / Debug
	Verbose bool

	// SMTP (envio de emails transacionais, ex: senha inicial/reset)
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string

	// CORSAllowedOrigins é a lista explícita de origens permitidas (comparação
	// exata, sem prefix-match), configurável via CORS_ALLOWED_ORIGINS
	// (separadas por vírgula). Sempre inclui as origens padrão de dev local.
	CORSAllowedOrigins []string

	// TrustProxyHeaders indica se a aplicação está atrás de um proxy/load
	// balancer confiável que popula X-Forwarded-For/X-Real-IP corretamente.
	// Quando false (padrão), esses headers são ignorados e r.RemoteAddr é
	// sempre usado como IP do cliente — evita spoofing de IP.
	TrustProxyHeaders bool

	// TurnstileSecretKey é a chave secreta do Cloudflare Turnstile usada para
	// verificar o token de CAPTCHA enviado pelo frontend em /api/auth/login e
	// /api/auth/reset-password (env TURNSTILE_SECRET_KEY). Nunca é logada nem
	// retornada em resposta JSON.
	TurnstileSecretKey string

	// SecurityAlertEmails são os destinatários administrativos dos alertas de
	// segurança (SEC-09, ex.: reuso de refresh token), via
	// SECURITY_ALERT_EMAILS (separados por vírgula). Entradas vazias, sem "@"
	// ou com caracteres de quebra de linha são descartadas.
	SecurityAlertEmails []string

	// RefreshReuseSuppressWindow é a janela em que um novo reuso do MESMO
	// refresh token já rotacionado não derruba as sessões de novo (SEC-12),
	// via REFRESH_REUSE_SUPPRESS_WINDOW. Padrão 30m, faixa [1m, 24h].
	RefreshReuseSuppressWindow time.Duration

	// RefreshCleanupInterval é o intervalo da limpeza periódica de
	// refresh_tokens (CHORE-02), via REFRESH_CLEANUP_INTERVAL. Padrão 6h,
	// mínimo 1m; "0" desativa a limpeza.
	RefreshCleanupInterval time.Duration

	// RefreshTokenRetencao é quanto tempo um refresh token fica no banco
	// depois de expires_at antes de ser apagado (CHORE-02), via
	// REFRESH_TOKEN_RETENCAO. Padrão 720h (30 dias), faixa [24h, 8760h].
	RefreshTokenRetencao time.Duration
}

// HashSenha são os parâmetros do hash de senha (SEC-13): Argon2id sobre o
// HMAC-SHA256(Pepper, senha). Pepper vem de PASSWORD_PEPPER e nunca é logado;
// trocá-lo invalida todas as senhas Argon2id já gravadas.
type HashSenha struct {
	Pepper      string
	MemoriaKiB  uint32
	Iteracoes   uint32
	Paralelismo uint8
}

// PepperMinBytes é o tamanho mínimo aceito para PASSWORD_PEPPER.
const PepperMinBytes = 32

// ErrPepperAusente: PASSWORD_PEPPER ausente ou curto demais (SEC-13).
var ErrPepperAusente = fmt.Errorf("config: PASSWORD_PEPPER é obrigatório (mínimo de %d bytes)", PepperMinBytes)

// Faixas e padrões do Argon2id (SEC-13). O mínimo de memória segue a
// recomendação da OWASP (19 MiB).
const (
	argon2MemoriaPadrao     = 64 * 1024
	argon2MemoriaMin        = 19 * 1024
	argon2MemoriaMax        = 1024 * 1024
	argon2IteracoesPadrao   = 3
	argon2IteracoesMax      = 10
	argon2ParalelismoPadrao = 2
	argon2ParalelismoMax    = 16
)

// ErrCredenciaisDB: DB_USUARIO/DB_SENHA ausentes ou vazios (SEC-11).
var ErrCredenciaisDB = errors.New("config: defina DB_USUARIO/DB_SENHA no .env")

// Faixas e padrões das durações de refresh token (SEC-12 / CHORE-02).
const (
	refreshReuseWindowPadrao = 30 * time.Minute
	refreshReuseWindowMin    = time.Minute
	refreshReuseWindowMax    = 24 * time.Hour

	refreshCleanupIntervalPadrao = 6 * time.Hour
	refreshCleanupIntervalMin    = time.Minute

	refreshRetencaoPadrao = 720 * time.Hour
	refreshRetencaoMin    = 24 * time.Hour
	refreshRetencaoMax    = 8760 * time.Hour
)

// Load lê as variáveis de ambiente e retorna uma Config preenchida.
// Retorna erro se variáveis obrigatórias estiverem ausentes — inclusive
// DB_USUARIO/DB_SENHA (ErrCredenciaisDB, SEC-11).
func Load() (*Config, error) {
	return load(true)
}

// LoadSemCredenciaisDB é o Load sem a exigência de DB_USUARIO/DB_SENHA, para
// ferramentas que não abrem conexão (ex.: seedusers -dry-run/-no-exec, SEC-10).
// As demais validações continuam valendo.
func LoadSemCredenciaisDB() (*Config, error) {
	return load(false)
}

func load(exigirDB bool) (*Config, error) {
	cfg := &Config{
		DBHost:    getEnv("DB_HOST", "localhost"),
		DBPort:    getEnv("DB_PORT", "3306"),
		DBName:    getEnv("DB_NAME", "rotaperfumes"),
		DBUsuario: os.Getenv("DB_USUARIO"),
		DBSenha:   os.Getenv("DB_SENHA"),
		JWTSecret: os.Getenv("JWT_SECRET"),
		JWTIssuer: getEnv("JWT_ISSUER", "rotaperfumes"),
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("config: JWT_SECRET é obrigatório")
	}

	ttlStr := getEnv("JWT_TTL", "24h")
	ttl, err := time.ParseDuration(ttlStr)
	if err != nil {
		return nil, fmt.Errorf("config: JWT_TTL inválido (%q): %w", ttlStr, err)
	}
	cfg.JWTTTL = ttl

	hs, err := LoadHashSenha()
	if err != nil {
		return nil, err
	}
	cfg.HashSenha = hs

	cfg.Verbose = getEnv("VERBOSE", "false") == "true" ||
		getEnv("LOG_LEVEL", "info") == "debug"

	// SMTP: sem defaults para user/password/from — quando ausentes, a
	// aplicação usa um EmailService "noop" (log-only). Host/porta têm
	// defaults compatíveis com Gmail.
	cfg.SMTPHost = getEnv("SMTP_HOST", "smtp.gmail.com")
	cfg.SMTPPort = getEnv("SMTP_PORT", "587")
	cfg.SMTPUser = os.Getenv("SMTP_USER")
	cfg.SMTPPassword = os.Getenv("SMTP_PASSWORD")
	cfg.SMTPFrom = os.Getenv("SMTP_FROM")

	cfg.CORSAllowedOrigins = parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	cfg.TrustProxyHeaders = getEnv("TRUST_PROXY_HEADERS", "false") == "true"
	cfg.TurnstileSecretKey = os.Getenv("TURNSTILE_SECRET_KEY")
	cfg.SecurityAlertEmails = ParseSecurityAlertEmails(os.Getenv("SECURITY_ALERT_EMAILS"))

	if err := carregarDuracoesRefresh(cfg); err != nil {
		return nil, err
	}

	// SEC-11: checagem no fim, depois das demais validações, para que os
	// erros já existentes (JWT_SECRET, JWT_TTL, PASSWORD_PEPPER/ARGON2_*) tenham prioridade.
	// A mensagem nunca inclui os valores.
	if exigirDB && (strings.TrimSpace(cfg.DBUsuario) == "" || cfg.DBSenha == "") {
		return nil, ErrCredenciaisDB
	}

	return cfg, nil
}

// LoadHashSenha lê PASSWORD_PEPPER e ARGON2_MEMORIA_KIB / ARGON2_ITERACOES /
// ARGON2_PARALELISMO. Exposto à parte para ferramentas que não carregam a
// Config inteira (ex.: cmd/resetpassword). As mensagens nunca incluem o pepper.
func LoadHashSenha() (HashSenha, error) {
	hs := HashSenha{Pepper: os.Getenv("PASSWORD_PEPPER")}
	if len(hs.Pepper) < PepperMinBytes {
		return HashSenha{}, ErrPepperAusente
	}
	m, err := parseUintNaFaixa("ARGON2_MEMORIA_KIB", argon2MemoriaPadrao, argon2MemoriaMin, argon2MemoriaMax)
	if err != nil {
		return HashSenha{}, err
	}
	t, err := parseUintNaFaixa("ARGON2_ITERACOES", argon2IteracoesPadrao, 1, argon2IteracoesMax)
	if err != nil {
		return HashSenha{}, err
	}
	p, err := parseUintNaFaixa("ARGON2_PARALELISMO", argon2ParalelismoPadrao, 1, argon2ParalelismoMax)
	if err != nil {
		return HashSenha{}, err
	}
	hs.MemoriaKiB, hs.Iteracoes, hs.Paralelismo = uint32(m), uint32(t), uint8(p)
	return hs, nil
}

// parseUintNaFaixa lê a env chave como inteiro (padrão se vazia) e exige
// minimo <= valor <= maximo.
func parseUintNaFaixa(chave string, padrao, minimo, maximo uint64) (uint64, error) {
	raw := strings.TrimSpace(os.Getenv(chave))
	if raw == "" {
		return padrao, nil
	}
	v, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || v < minimo || v > maximo {
		return 0, fmt.Errorf("config: %s inválido (%q): esperado inteiro entre %d e %d", chave, raw, minimo, maximo)
	}
	return v, nil
}

// carregarDuracoesRefresh lê as durações de refresh token (SEC-12 e CHORE-02)
// e valida as faixas; valor inválido vira erro do Load (estilo JWT_TTL).
func carregarDuracoesRefresh(cfg *Config) error {
	janela, err := parseDuracaoNaFaixa("REFRESH_REUSE_SUPPRESS_WINDOW", refreshReuseWindowPadrao, refreshReuseWindowMin, refreshReuseWindowMax)
	if err != nil {
		return err
	}
	cfg.RefreshReuseSuppressWindow = janela

	intervalo, err := parseIntervaloLimpeza()
	if err != nil {
		return err
	}
	cfg.RefreshCleanupInterval = intervalo

	retencao, err := parseDuracaoNaFaixa("REFRESH_TOKEN_RETENCAO", refreshRetencaoPadrao, refreshRetencaoMin, refreshRetencaoMax)
	if err != nil {
		return err
	}
	cfg.RefreshTokenRetencao = retencao
	return nil
}

// parseIntervaloLimpeza lê REFRESH_CLEANUP_INTERVAL: "0" (ou "0s") desativa a
// limpeza; qualquer outro valor precisa ser >= 1m.
func parseIntervaloLimpeza() (time.Duration, error) {
	const chave = "REFRESH_CLEANUP_INTERVAL"
	raw := strings.TrimSpace(os.Getenv(chave))
	if raw == "" {
		return refreshCleanupIntervalPadrao, nil
	}
	if raw == "0" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s inválido (%q): %w", chave, raw, err)
	}
	if d == 0 {
		return 0, nil
	}
	if d < refreshCleanupIntervalMin {
		return 0, fmt.Errorf("config: %s inválido (%q): esperado 0 (desativa) ou >= %s", chave, raw, refreshCleanupIntervalMin)
	}
	return d, nil
}

// parseDuracaoNaFaixa lê a env chave como time.Duration (padrão se vazia) e
// exige minimo <= valor <= maximo.
func parseDuracaoNaFaixa(chave string, padrao, minimo, maximo time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(chave))
	if raw == "" {
		return padrao, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: %s inválido (%q): %w", chave, raw, err)
	}
	if d < minimo || d > maximo {
		return 0, fmt.Errorf("config: %s inválido (%q): esperado entre %s e %s", chave, raw, minimo, maximo)
	}
	return d, nil
}

// ParseSecurityAlertEmails interpreta SECURITY_ALERT_EMAILS: separa por
// vírgula, aplica trim e descarta entradas vazias, sem "@" ou contendo CR/LF
// (evita injeção de cabeçalho no envio SMTP).
func ParseSecurityAlertEmails(raw string) []string {
	var emails []string
	for _, e := range strings.Split(raw, ",") {
		e = strings.TrimSpace(e)
		if e == "" || !strings.Contains(e, "@") || strings.ContainsAny(e, "\r\n") {
			continue
		}
		emails = append(emails, e)
	}
	return emails
}

// defaultDevOrigins são as origens de desenvolvimento local sempre permitidas,
// independente de CORS_ALLOWED_ORIGINS — cobre o frontend Next.js em dev.
var defaultDevOrigins = []string{
	"http://localhost:3000",
	"http://127.0.0.1:3000",
}

// parseAllowedOrigins monta a lista final de origens permitidas: as origens de
// dev local padrão mais o que vier em CORS_ALLOWED_ORIGINS (separadas por
// vírgula). Comparação é sempre exata — sem prefix-match.
func parseAllowedOrigins(raw string) []string {
	origins := make([]string, 0, len(defaultDevOrigins)+2)
	origins = append(origins, defaultDevOrigins...)

	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		origins = append(origins, o)
	}
	return origins
}

// DSN monta a connection string para o driver go-sql-driver/mysql.
// Inclui parseTime=true para que colunas TIMESTAMP sejam tipadas como time.Time.
//
// loc=Local: time.Local é fixado em -03:00 pelo pacote tz (import em branco
// acima). Não adicionar time_zone ao DSN.
//
// clientFoundRows=true (BUG-04): RowsAffected passa a contar as linhas
// ENCONTRADAS pelo WHERE, não só as alteradas. Assim, um UPDATE que não muda
// nenhum valor devolve 1 (sucesso) em vez de 0 (que os repositórios traduzem
// para ErrNotFound/404).
//
// REGRA DO PROJETO: operações do tipo "só se ainda não usado/revogado/
// desligado" DEVEM colocar a condição no WHERE (ex.: "... WHERE id = ? AND
// revogado_em IS NULL") ou usar COALESCE no SET. Nunca deduzir "não mudou"
// a partir de RowsAffected=0 — com clientFoundRows=true isso não acontece
// mais quando a linha existe. Manter este DSN idêntico ao de
// cmd/resetpassword.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local&clientFoundRows=true",
		c.DBUsuario, c.DBSenha, c.DBHost, c.DBPort, c.DBName,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
