package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port      string
	DBURL     string
	JWTSecret string
}

func LoadConfig() (*Config, error) {
	godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		dbURL = "golang:golang@tcp(localhost:3306)/rotaperfumes?parseTime=true"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "rotaperfumes_crm_secret_2024_change_in_production"
	}

	return &Config{
		Port:      port,
		DBURL:     dbURL,
		JWTSecret: jwtSecret,
	}, nil
}
