// Package db encapsula a abertura e configuração da conexão MySQL.
//
// Seguindo o padrão Clean Code, este package expõe apenas Open() e Close().
// Cada handler/service abre conexões curtas via db.Conn() conforme necessário,
// evitando cache L1 compartilhado entre requisições.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql" // driver MySQL
)

// Open abre a conexão com o MySQL e valida com Ping.
// Retorna *sql.DB pronto para uso e fecha qualquer erro na própria inicialização.
func Open(dsn string) (*sql.DB, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: falha ao abrir conexão: %w", err)
	}

	// Pool conservador: cada requisição usa sua conexão, sem hot cache.
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)
	conn.SetConnMaxIdleTime(2 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("db: ping falhou: %w", err)
	}

	log.Printf("[db] conexão MySQL estabelecida (host=%s)", maskDSN(dsn))
	return conn, nil
}

// maskDSN remove a senha do DSN para logging seguro.
func maskDSN(dsn string) string {
	// Formato: user:pass@tcp(host:port)/db?...
	at := -1
	for i, r := range dsn {
		if r == '@' {
			at = i
			break
		}
	}
	if at < 0 {
		return dsn
	}
	colon := -1
	for i, r := range dsn[:at] {
		if r == ':' {
			colon = i
		}
	}
	if colon < 0 {
		return dsn
	}
	return dsn[:colon+1] + "****" + dsn[at:]
}
