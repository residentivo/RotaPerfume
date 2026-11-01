// Package db encapsula a abertura e configuração da conexão MariaDB.
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
	"strings"
	"time"

	"github.com/go-sql-driver/mysql" // driver MySQL/MariaDB (go-sql-driver/mysql)

	"github.com/rotaperfumes/shared/vlog"
)

// Open abre a conexão com o MariaDB e valida com Ping.
// Retorna *sql.DB pronto para uso e fecha qualquer erro na própria inicialização.
func Open(dsn string) (*sql.DB, error) {
	vlog.Printf("db.go", "Open", "abrindo handle MariaDB via sql.Open")
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
		vlog.Printf("db.go", "Open", "ping falhou; fechando handle MariaDB")
		_ = conn.Close()
		return nil, fmt.Errorf("db: ping falhou: %w", err)
	}

	log.Printf("[db] conexão MariaDB estabelecida (host=%s)", maskDSN(dsn))
	return conn, nil
}

// maskDSN remove a senha do DSN para logging seguro, com o mesmo corte do
// go-sql-driver/mysql (SEC-15): a senha pode conter '@', ':' e '/'.
//
// Formato: user:pass@tcp(host:port)/db?... O banco começa no último '/'; as
// credenciais terminam no último '@' antes dele; o usuário termina no
// primeiro ':'. Sem '@' ou sem ':', não há senha e o DSN sai como está.
func maskDSN(dsn string) string {
	slash := strings.LastIndexByte(dsn, '/')
	if slash < 0 {
		slash = len(dsn)
	}
	at := strings.LastIndexByte(dsn[:slash], '@')
	if at < 0 {
		return dsn
	}
	colon := strings.IndexByte(dsn[:at], ':')
	if colon < 0 {
		return dsn
	}
	mascarado := dsn[:colon+1] + "****" + dsn[at:]
	if senhaVazou(dsn, mascarado) {
		vlog.Printf("db.go", "maskDSN", "decisão senha_no_dsn_mascarado: log só com ****")
		return "****"
	}
	return mascarado
}

// senhaVazou é a proteção extra do maskDSN: se o parse do driver achar uma
// senha que ainda aparece no texto mascarado, o corte divergiu do driver.
func senhaVazou(dsn, mascarado string) bool {
	cfg, err := mysql.ParseDSN(dsn)
	return err == nil && cfg.Passwd != "" && strings.Contains(mascarado, cfg.Passwd)
}
