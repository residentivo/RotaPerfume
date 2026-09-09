// Package config carrega configurações do sistema a partir de variáveis de ambiente.
//
// O arquivo .env na raiz do projeto é lido uma vez (carregado por outras ferramentas
// como o Makefile antes de invocar os binários Go) — aqui apenas consumimos via os.Getenv.
package config

import (
	"fmt"
	"os"
	"strconv"
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

	return cfg, nil
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
