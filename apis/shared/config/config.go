// Package config carrega configurações do sistema a partir de variáveis de ambiente.
//
// O arquivo .env na raiz do projeto é lido uma vez (carregado por outras ferramentas
// como o Makefile antes de invocar os binários Go) — aqui apenas consumimos via os.Getenv.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config agrega todas as configurações da aplicação.
type Config struct {
	// Banco de dados
	DBHost     string
	DBPort     string
	DBName     string
	DBUsuario  string
	DBSenha    string

	// JWT
	JWTSecret string
	JWTTTL    time.Duration
	JWTIssuer string

	// Bcrypt
	BCryptCost int

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
}

// Load lê as variáveis de ambiente e retorna uma Config preenchida.
// Retorna erro se variáveis obrigatórias estiverem ausentes.
func Load() (*Config, error) {
	cfg := &Config{
		DBHost:    getEnv("DB_HOST", "localhost"),
		DBPort:    getEnv("DB_PORT", "3306"),
		DBName:    getEnv("DB_NAME", "rotaperfumes"),
		DBUsuario: getEnv("DB_USUARIO", "golang"),
		DBSenha:   getEnv("DB_SENHA", "golang"),
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

	costStr := getEnv("BCRYPT_COST", "12")
	cost, err := strconv.Atoi(costStr)
	if err != nil || cost < 4 || cost > 31 {
		return nil, fmt.Errorf("config: BCRYPT_COST inválido (%q): esperado inteiro entre 4 e 31", costStr)
	}
	cfg.BCryptCost = cost

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

	return cfg, nil
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
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local",
		c.DBUsuario, c.DBSenha, c.DBHost, c.DBPort, c.DBName,
	)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
