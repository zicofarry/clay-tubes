// Package database provides helpers for connecting to PostgreSQL and Redis,
// the two primary data stores used across Clay microservices.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ───── PostgreSQL ─────────────────────────────────────────────────────────────

// PostgresConfig holds PostgreSQL connection settings.
type PostgresConfig struct {
	Host     string `json:"host" env:"DB_HOST"`
	Port     int    `json:"port" env:"DB_PORT"`
	User     string `json:"user" env:"DB_USER"`
	Password string `json:"password" env:"DB_PASSWORD"`
	DBName   string `json:"db_name" env:"DB_NAME"`
	SSLMode  string `json:"ssl_mode" env:"DB_SSL_MODE"`

	// Pool settings
	MaxOpenConns    int           `json:"max_open_conns" env:"DB_MAX_OPEN_CONNS"`
	MaxIdleConns    int           `json:"max_idle_conns" env:"DB_MAX_IDLE_CONNS"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME"`
}

// DefaultPostgresConfig returns sensible defaults for local development.
func DefaultPostgresConfig() PostgresConfig {
	return PostgresConfig{
		Host:            "localhost",
		Port:            5432,
		User:            "clay",
		Password:        "clay",
		DBName:          "clay",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}
}

// DSN returns the PostgreSQL connection string.
func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// NewPostgresDB opens a PostgreSQL connection pool with the given config.
// Caller is responsible for calling db.Close() on shutdown.
func NewPostgresDB(cfg PostgresConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

// ───── Redis ──────────────────────────────────────────────────────────────────

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Host     string `json:"host" env:"REDIS_HOST"`
	Port     int    `json:"port" env:"REDIS_PORT"`
	Password string `json:"password" env:"REDIS_PASSWORD"`
	DB       int    `json:"db" env:"REDIS_DB"`
}

// DefaultRedisConfig returns sensible defaults for local development.
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		Host: "localhost",
		Port: 6379,
		DB:   0,
	}
}

// Addr returns the Redis address in host:port format.
func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// ───── Transaction Helper ─────────────────────────────────────────────────────

// TxFunc is a function that runs inside a database transaction.
type TxFunc func(tx *sql.Tx) error

// WithTransaction executes fn inside a transaction. If fn returns an error,
// the transaction is rolled back; otherwise it's committed.
//
// Usage:
//
//	err := database.WithTransaction(ctx, db, func(tx *sql.Tx) error {
//	    _, err := tx.ExecContext(ctx, "UPDATE wallets SET balance = balance - $1 WHERE id = $2", amount, walletID)
//	    return err
//	})
func WithTransaction(ctx context.Context, db *sql.DB, fn TxFunc) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
