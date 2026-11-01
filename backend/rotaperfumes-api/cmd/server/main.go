// Command server inicia a API rotaperfumes-api na porta 8080.
//
// Só carrega o ambiente e a config; a montagem (composition root), o
// servidor e o shutdown ficam em internal/bootstrap.
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"github.com/rotaperfumes/rotaperfumes-api/internal/bootstrap"
	"github.com/rotaperfumes/shared/config"
	"github.com/rotaperfumes/shared/vlog"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("[server] iniciando rotaperfumes-api")
	carregarEnv()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[server] config: %v", err)
	}
	vlog.SetEnabled(cfg.Verbose)
	if vlog.Enabled() {
		log.Printf("[server] WARN: VERBOSE ativo — logs de rastreamento ligados; não use em produção")
	}
	log.Printf("[server] config OK: db=%s@%s jwt_issuer=%s verbose=%v",
		cfg.DBName, cfg.DBHost, cfg.JWTIssuer, cfg.Verbose)

	bootstrap.Run(cfg)
}

// carregarEnv procura o .env da raiz do projeto subindo até 6 diretórios a
// partir do working directory (funciona em `go run` e no binário). Sem .env,
// segue só com o ambiente.
func carregarEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for i := 0; i < 6; i++ {
		arquivo := filepath.Join(dir, ".env")
		if _, err := os.Stat(arquivo); err == nil {
			_ = godotenv.Load(arquivo)
			return
		}
		pai := filepath.Dir(dir)
		if pai == dir {
			return
		}
		dir = pai
	}
}
