package config

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

type Config struct {
	Port     string
	DBURL    string
	JWTSecret string
}

var AppConfig Config
var DB *sql.DB

func LoadConfig() error {
	err := godotenv.Load()
	if err != nil {
		log.Println("Aviso: arquivo .env nao encontrado, usando valores padrao")
	}

	AppConfig.Port = getEnv("PORT", "8083")
	AppConfig.DBURL = getEnv("DB_URL", "golang:golang@tcp(localhost:3306)/rotaperfumes?parseTime=true")
	AppConfig.JWTSecret = getEnv("JWT_SECRET", "rotaperfumes_erp_secret_2024_change_in_production")

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func ConnectDB() error {
	var err error
	DB, err = sql.Open("mysql", AppConfig.DBURL)
	if err != nil {
		return fmt.Errorf("erro ao conectar ao banco: %v", err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("erro ao pingar banco: %v", err)
	}

	log.Println("Conexao com banco de dados estabelecida")
	return nil
}

func CloseDB() {
	if DB != nil {
		DB.Close()
	}
}
