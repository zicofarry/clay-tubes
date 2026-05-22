//go:build unit

package database

import (
	"testing"
	"time"
)

func TestDefaultPostgresConfig(t *testing.T) {
	cfg := DefaultPostgresConfig()

	if cfg.Host != "localhost" {
		t.Errorf("expected localhost, got %s", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("expected 5432, got %d", cfg.Port)
	}
	if cfg.MaxOpenConns != 25 {
		t.Errorf("expected 25, got %d", cfg.MaxOpenConns)
	}
	if cfg.ConnMaxLifetime != 5*time.Minute {
		t.Errorf("expected 5m, got %v", cfg.ConnMaxLifetime)
	}
}

func TestPostgresConfig_DSN(t *testing.T) {
	cfg := DefaultPostgresConfig()
	dsn := cfg.DSN()

	expected := "host=localhost port=5432 user=clay password=clay dbname=clay sslmode=disable"
	if dsn != expected {
		t.Errorf("expected DSN:\n  %s\ngot:\n  %s", expected, dsn)
	}
}

func TestDefaultRedisConfig(t *testing.T) {
	cfg := DefaultRedisConfig()

	if cfg.Host != "localhost" {
		t.Errorf("expected localhost, got %s", cfg.Host)
	}
	if cfg.Port != 6379 {
		t.Errorf("expected 6379, got %d", cfg.Port)
	}
}

func TestRedisConfig_Addr(t *testing.T) {
	cfg := DefaultRedisConfig()
	addr := cfg.Addr()

	if addr != "localhost:6379" {
		t.Errorf("expected localhost:6379, got %s", addr)
	}
}
