// Package cache provides Redis-backed implementations for idempotency key
// checking and rate limiting in the Payment Service.
//
// Implements the idempotency.Store interface from clay-shared.
//
// Storage keys:
//   - payment:idempotency:{key}   → cached charge/refund response (TTL 24h)
//   - payment:ratelimit:{scope}:{id} → sliding window counter
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/zicofarry/clay-app/backend/pkg/pkg/idempotency"
)

// ───── Redis Client Interface ────────────────────────────────────────────────

// RedisClient abstracts the Redis operations needed by this package.
// In production, inject a real *redis.Client from go-redis/v9.
// For testing, use the InMemoryRedis implementation below.
type RedisClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)
}

// ───── Idempotency Store (implements idempotency.Store) ──────────────────────

// IdempotencyStore implements the idempotency.Store interface from clay-shared
// using Redis as the backing store.
type IdempotencyStore struct {
	client RedisClient
	logger *slog.Logger
}

// NewIdempotencyStore creates a new Redis-backed idempotency store.
func NewIdempotencyStore(client RedisClient, logger *slog.Logger) *IdempotencyStore {
	return &IdempotencyStore{client: client, logger: logger}
}

// Get returns the cached result for the given idempotency key.
func (s *IdempotencyStore) Get(ctx context.Context, key string) (*idempotency.CachedResult, error) {
	val, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, nil // key not found → treated as cache miss
	}

	var result idempotency.CachedResult
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		s.logger.Error("failed to unmarshal cached result", slog.String("key", key), slog.Any("error", err))
		return nil, fmt.Errorf("unmarshal cached result: %w", err)
	}
	return &result, nil
}

// Set stores the result for the given idempotency key with the specified TTL.
func (s *IdempotencyStore) Set(ctx context.Context, key string, result *idempotency.CachedResult, ttl time.Duration) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	return s.client.Set(ctx, key, string(data), ttl)
}

// SetNX attempts to set the key only if it doesn't exist (lock acquisition).
// Returns true if the lock was acquired, false if the key already exists.
func (s *IdempotencyStore) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, key, "locked", ttl)
}

// ───── Rate Limiter ──────────────────────────────────────────────────────────

// RateLimiter provides sliding-window rate limiting using Redis.
// Used for COD verification (max 3 per hour) and other rate-limited operations.
type RateLimiter struct {
	client RedisClient
	logger *slog.Logger
}

// NewRateLimiter creates a new Redis-backed rate limiter.
func NewRateLimiter(client RedisClient, logger *slog.Logger) *RateLimiter {
	return &RateLimiter{client: client, logger: logger}
}

// Allow checks if the action is allowed under the rate limit.
// Returns (allowed bool, remaining int, error).
//
// Parameters:
//   - scope: the rate limit scope, e.g. "cod_verify"
//   - identifier: the unique key, e.g. user_id or phone number
//   - maxAttempts: maximum attempts allowed in the window
//   - window: time window for the rate limit
func (rl *RateLimiter) Allow(ctx context.Context, scope, identifier string, maxAttempts int, window time.Duration) (bool, int, error) {
	key := fmt.Sprintf("payment:ratelimit:%s:%s", scope, identifier)

	count, err := rl.client.Incr(ctx, key)
	if err != nil {
		return false, 0, fmt.Errorf("incr rate limit key: %w", err)
	}

	// Set expiry on first increment
	if count == 1 {
		if err := rl.client.Expire(ctx, key, window); err != nil {
			rl.logger.Error("failed to set rate limit expiry", slog.String("key", key), slog.Any("error", err))
		}
	}

	remaining := maxAttempts - int(count)
	if remaining < 0 {
		remaining = 0
	}

	allowed := int(count) <= maxAttempts
	if !allowed {
		rl.logger.Warn("rate limit exceeded",
			slog.String("scope", scope),
			slog.String("identifier", identifier),
			slog.Int64("count", count),
		)
	}

	return allowed, remaining, nil
}

// ───── In-Memory Redis (for testing / development) ───────────────────────────

// InMemoryRedis is a simple in-memory implementation of RedisClient for
// local development and unit testing when Redis is not available.
type InMemoryRedis struct {
	data    map[string]string
	expiry  map[string]time.Time
	counter map[string]int64
}

// NewInMemoryRedis creates a new in-memory Redis substitute.
func NewInMemoryRedis() *InMemoryRedis {
	return &InMemoryRedis{
		data:    make(map[string]string),
		expiry:  make(map[string]time.Time),
		counter: make(map[string]int64),
	}
}

func (m *InMemoryRedis) Get(_ context.Context, key string) (string, error) {
	if exp, ok := m.expiry[key]; ok && time.Now().After(exp) {
		delete(m.data, key)
		delete(m.expiry, key)
		return "", fmt.Errorf("key not found")
	}
	val, ok := m.data[key]
	if !ok {
		return "", fmt.Errorf("key not found")
	}
	return val, nil
}

func (m *InMemoryRedis) Set(_ context.Context, key string, value interface{}, expiration time.Duration) error {
	m.data[key] = fmt.Sprintf("%v", value)
	if expiration > 0 {
		m.expiry[key] = time.Now().Add(expiration)
	}
	return nil
}

func (m *InMemoryRedis) SetNX(_ context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	if exp, ok := m.expiry[key]; ok && time.Now().After(exp) {
		delete(m.data, key)
		delete(m.expiry, key)
	}
	if _, exists := m.data[key]; exists {
		return false, nil
	}
	m.data[key] = fmt.Sprintf("%v", value)
	if expiration > 0 {
		m.expiry[key] = time.Now().Add(expiration)
	}
	return true, nil
}

func (m *InMemoryRedis) Incr(_ context.Context, key string) (int64, error) {
	m.counter[key]++
	return m.counter[key], nil
}

func (m *InMemoryRedis) Expire(_ context.Context, key string, expiration time.Duration) error {
	m.expiry[key] = time.Now().Add(expiration)
	return nil
}

func (m *InMemoryRedis) TTL(_ context.Context, key string) (time.Duration, error) {
	exp, ok := m.expiry[key]
	if !ok {
		return -1, nil
	}
	remaining := time.Until(exp)
	if remaining < 0 {
		return -1, nil
	}
	return remaining, nil
}
