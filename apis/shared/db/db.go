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

	"github.com/rotaperfumes/shared/vlog"
)

// Open abre a conexão com o MySQL e valida com Ping.
// Retorna *sql.DB pronto para uso e fecha qualquer erro na própria inicialização.
func Open(dsn string) (*sql.DB, error) {
	vlog.Printf("db.go", "Open", "abrindo handle MySQL via sql.Open")
	conn, err := sql.Open("mysql", dsn)
	vlog.Printf("db.go", "Open", "verificando se err != nil após sql.Open")
	if err != nil {
		return nil, fmt.Errorf("db: falha ao abrir conexão: %w", err)
	}

	// Pool conservador: cada requisição usa sua conexão, sem hot cache.
	vlog.Printf("db.go", "Open", "configurando MaxOpenConns=25")
	conn.SetMaxOpenConns(25)
	vlog.Printf("db.go", "Open", "configurando MaxIdleConns=5")
	conn.SetMaxIdleConns(5)
	vlog.Printf("db.go", "Open", "configurando ConnMaxLifetime=5m")
	conn.SetConnMaxLifetime(5 * time.Minute)
	vlog.Printf("db.go", "Open", "configurando ConnMaxIdleTime=2m")
	conn.SetConnMaxIdleTime(2 * time.Minute)

	vlog.Printf("db.go", "Open", "criando contexto com timeout de 5s para o ping")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	vlog.Printf("db.go", "Open", "agendando cancel do contexto do ping")
	defer cancel()
	vlog.Printf("db.go", "Open", "executando PingContext e verificando erro")
	if err := conn.PingContext(ctx); err != nil {
		vlog.Printf("db.go", "Open", "ping falhou; fechando handle MySQL")
		_ = conn.Close()
		return nil, fmt.Errorf("db: ping falhou: %w", err)
	}

	log.Printf("[db] conexão MySQL estabelecida (host=%s)", maskDSN(dsn))
	return conn, nil
}

// maskDSN remove a senha do DSN para logging seguro.
func maskDSN(dsn string) string {
	// Formato: user:pass@tcp(host:port)/db?...
	vlog.Printf("db.go", "maskDSN", "inicializando posição do '@' em -1")
	at := -1
	vlog.Printf("db.go", "maskDSN", "procurando o primeiro '@' no DSN")
	for i, r := range dsn {
		if r == '@' {
			at = i
			break
		}
	}
	vlog.Printf("db.go", "maskDSN", "verificando se o '@' foi encontrado (encontrou=%t)", at >= 0)
	if at < 0 {
		return dsn
	}
	vlog.Printf("db.go", "maskDSN", "inicializando posição do ':' em -1")
	colon := -1
	vlog.Printf("db.go", "maskDSN", "procurando o último ':' antes do '@'")
	for i, r := range dsn[:at] {
		if r == ':' {
			colon = i
		}
	}
	vlog.Printf("db.go", "maskDSN", "verificando se o ':' foi encontrado (encontrou=%t)", colon >= 0)
	if colon < 0 {
		return dsn
	}
	return dsn[:colon+1] + "****" + dsn[at:]
}
