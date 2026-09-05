// Command seedusers generates bcrypt password hashes for the seed SQL file.
package main

import (
	"fmt"
	"os"

	"rotaperfumes/shared/pkg/config"
	"rotaperfumes/shared/pkg/logger"
	"rotaperfumes/shared/pkg/password"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao carregar config: %v\n", err)
		os.Exit(1)
	}

	logger.Init(cfg.LogLevel, cfg.LogFormat)
	log := logger.Get()

	defaultPassword := os.Getenv("SEED_DEFAULT_PASSWORD")
	if defaultPassword == "" {
		defaultPassword = "Temp@2026!"
	}

	log.Info("Generating bcrypt hash", "password", defaultPassword, "cost", cfg.BcryptCost)

	hash, err := password.HashPassword(defaultPassword, cfg.BcryptCost)
	if err != nil {
		log.Error("Failed to hash password", "error", err)
		os.Exit(1)
	}

	fmt.Println("=== Hash bcrypt gerado ===")
	fmt.Println("SET @SEED_PASSWORD_HASH = '" + hash + "';")
	fmt.Println()
	fmt.Println("=== Usar no SQL seed ===")
	fmt.Println("INSERT INTO usuarios (login, email, password_hash, role, ativo, gerente_id)")
	fmt.Println("VALUES ('admin', 'admin@rotaperfumes.com.br', '" + hash + "', 'RH', TRUE, NULL);")
}
