// Package database provides MySQL database connection management.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"rotaperfumes/shared/pkg/config"
	"rotaperfumes/shared/pkg/logger"
)

var db *sql.DB

// Connect establishes a connection to the MySQL database.
func Connect(cfg *config.Config) (*sql.DB, error) {
	log := logger.Get()

	log.Info("Connecting to MySQL database",
		"host", cfg.DBHost,
		"port", cfg.DBPort,
		"database", cfg.DBName,
	)

	dsn := cfg.DSN()

	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Database connection established successfully")
	return db, nil
}

// Get returns the global database connection.
func Get() *sql.DB {
	return db
}

// Close closes the database connection.
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// HealthCheck verifies the database connection is alive.
func HealthCheck(ctx context.Context) error {
	if db == nil {
		return fmt.Errorf("database not connected")
	}
	return db.PingContext(ctx)
}
