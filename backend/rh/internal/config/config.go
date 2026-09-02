package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config armazena todas as configurações da aplicação
type Config struct {
	Port      string
	DBURL     string
	JWTSecret string
}

// Load carrega as configurações do arquivo .env e variáveis de ambiente
func Load() *Config {
	// Carrega .env (não falha se o arquivo não existir)
	if err := godotenv.Load(); err != nil {
		log.Printf("Aviso: arquivo .env não encontrado, usando variáveis de ambiente: %v", err)
	}

	cfg := &Config{
		Port:      getEnv("PORT", "8081"),
		DBURL:     getEnv("DB_URL", "golang:golang@tcp(localhost:3306)/rotaperfumes?parseTime=true"),
		JWTSecret: getEnv("JWT_SECRET", "rotaperfumes_rh_secret_2024_change_in_production"),
	}

	return cfg
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}
