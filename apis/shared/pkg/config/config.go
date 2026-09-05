// Package config loads environment variables from .env file and application configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds all configuration for the application.
type Config struct {
	// Database
	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string

	// JWT
	JWTSecret     string
	JWTTTL        string
	JWTIssuer     string

	// Bcrypt
	BcryptCost int

	// Log
	LogLevel  string
	LogFormat string

	// Server
	Port string
}

var globalConfig *Config

// Load reads configuration from environment variables.
// It also loads from .env file if DOTENV_PATH is set.
func Load() (*Config, error) {
	if globalConfig != nil {
		return globalConfig, nil
	}

	// Try to load .env file if it exists
	loadDotEnv()

	cfg := &Config{
		// Database
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBName:     getEnv("DB_NAME", "rotaperfumes"),
		DBUser:     getEnv("DB_USUARIO", "golang"),
		DBPassword: getEnv("DB_SENHA", "golang"),

		// JWT
		JWTSecret:     getEnv("JWT_SECRET", "troque-esta-chave-secreta-em-producao-use-32-chars-min"),
		JWTTTL:        getEnv("JWT_TTL", "24h"),
		JWTIssuer:     getEnv("JWT_ISSUER", "rotaperfumes"),

		// Bcrypt
		BcryptCost: getEnvInt("BCRYPT_COST", 12),

		// Log
		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", "text"),

		// Server
		Port: getEnv("PORT_API", "8080"),
	}

	globalConfig = cfg
	return cfg, nil
}

// Get returns the global config instance.
func Get() *Config {
	if globalConfig == nil {
		cfg, _ := Load()
		return cfg
	}
	return globalConfig
}

// DSN returns the MySQL DSN connection string.
func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

// loadDotEnv loads .env file if it exists.
func loadDotEnv() {
	// Try common locations for .env file
	paths := []string{
		".env",
		"../.env",
		"../../.env",
		"../../../.env",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			loadFile(path)
			return
		}
	}
}

// loadFile reads a .env file and sets environment variables.
func loadFile(path string) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Find the first = sign
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		// Remove surrounding quotes if present
		value = strings.Trim(value, "\"")
		value = strings.Trim(value, "'")

		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		if _, err := fmt.Sscanf(value, "%d", &result); err == nil {
			return result
		}
	}
	return defaultValue
}
