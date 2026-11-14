// Package db provides a reusable connection pool for MSSQL.
package db

import (
	"database/sql"
	"fmt"
	"omniChannelservice/tomlloader"

	_ "github.com/denisenkom/go-mssqldb"
)

var pool *sql.DB

// Init initializes the MSSQL connection pool using the provided config.
func Init(cfg tomlloader.DBSection) error {
	connStr := fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s&encrypt=%v&connection+timeout=%d",
		cfg.User, cfg.Password, cfg.Server, cfg.Port, cfg.Name, cfg.Encrypt, cfg.Timeout)
	db, err := sql.Open("sqlserver", connStr)
	if err != nil {
		return fmt.Errorf("sql.Open: %w", err)
	}
	// Test the connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("db.Ping: %w", err)
	}
	pool = db
	return nil
}

// Get returns the shared *sql.DB pool. Call Init first.
func Get() *sql.DB {
	return pool
}

// Close closes the pool if open.
func Close() error {
	if pool != nil {
		return pool.Close()
	}
	return nil
}
