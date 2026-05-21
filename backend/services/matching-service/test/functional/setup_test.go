//go:build functional

// Package functional contains end-to-end tests that hit a real Redis instance.
//
// These tests are designed to FAIL if the dependent infrastructure is not
// running. To run them locally:
//
//	docker compose up -d redis-matching
//	go test -tags=functional -v ./test/functional/...
//
// CI runs the same flow via the Jenkinsfile "5. Functional Tests" stage.
package functional

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/zicofarry/clay-app/backend/services/matching-service/internal/geo"
	"github.com/zicofarry/clay-app/backend/services/matching-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/matching-service/internal/service"
)

// redisAddr resolves the test Redis address. CI sets TEST_REDIS_ADDR from the
// Jenkinsfile; locally we fall back to the docker-compose mapping (localhost:6380).
func redisAddr() string {
	if v := os.Getenv("TEST_REDIS_ADDR"); v != "" {
		return v
	}
	return "localhost:6380"
}

// newRedisClient returns a real Redis client. If Redis is not reachable
// (e.g. docker compose is down), the test FAILS — it does not skip — so the
// build catches missing infrastructure.
func newRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := redisAddr()
	rdb := redis.NewClient(&redis.Options{
		Addr:        addr,
		DialTimeout: 2 * time.Second,
		ReadTimeout: 2 * time.Second,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf(
			"functional tests require Redis at %s — got %v\n"+
				"Did you forget to run `docker compose up -d redis-matching`?",
			addr, err,
		)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// resetRedis flushes the test database so each test starts clean.
func resetRedis(t *testing.T, rdb *redis.Client) {
	t.Helper()
	if err := rdb.FlushDB(context.Background()).Err(); err != nil {
		t.Fatalf("FLUSHDB: %v", err)
	}
}

// newRepo wires a real repository against the test Redis.
func newRepo(t *testing.T) (*repository.MatchingRepository, *redis.Client) {
	t.Helper()
	rdb := newRedisClient(t)
	resetRedis(t, rdb)
	return repository.NewMatchingRepository(rdb), rdb
}

// newService wires the full service stack (repo + Noop geo client) against
// real Redis. Returns the service and the underlying redis client so tests
// can poke at state directly.
func newService(t *testing.T) (*service.MatchingService, *redis.Client) {
	t.Helper()
	rdb := newRedisClient(t)
	resetRedis(t, rdb)
	repo := repository.NewMatchingRepository(rdb)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := service.NewMatchingService(repo, geo.NewNoopClient(), logger)
	return svc, rdb
}
