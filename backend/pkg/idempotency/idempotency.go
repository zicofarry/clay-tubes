// Package idempotency provides Redis-backed idempotency key checking
// for operations that must not be executed twice (payments, refunds, credits).
//
// Flow:
//  1. Client sends request with `Idempotency-Key` header
//  2. Service calls Acquire() — if key already exists, returns the cached response
//  3. If key is new, service processes the request and calls Complete() to store result
//  4. Key auto-expires after TTL (default 24h)
package idempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DefaultTTL is how long idempotency keys are remembered.
const DefaultTTL = 24 * time.Hour

// ───── Store Interface ────────────────────────────────────────────────────────

// Store is the interface for idempotency key persistence.
// The default implementation uses Redis, but it's abstracted for testing.
type Store interface {
	// Get returns the cached result for the key, or nil if not found.
	Get(ctx context.Context, key string) (*CachedResult, error)
	// Set stores the result for the key with the given TTL.
	Set(ctx context.Context, key string, result *CachedResult, ttl time.Duration) error
	// SetNX attempts to set the key only if it doesn't exist (lock acquisition).
	// Returns true if the key was set (lock acquired), false if already exists.
	SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// CachedResult holds the response that was cached for a previously-processed
// idempotent request.
type CachedResult struct {
	StatusCode int             `json:"status_code"`
	Body       json.RawMessage `json:"body"`
}

// ───── Checker ────────────────────────────────────────────────────────────────

// Checker handles idempotency key checking logic.
type Checker struct {
	store Store
	ttl   time.Duration
}

// NewChecker creates a new idempotency Checker.
func NewChecker(store Store, ttl time.Duration) *Checker {
	if ttl == 0 {
		ttl = DefaultTTL
	}
	return &Checker{store: store, ttl: ttl}
}

// ExtractKey reads the Idempotency-Key header from the request.
// Returns empty string if not present.
func ExtractKey(r *http.Request) string {
	return r.Header.Get("Idempotency-Key")
}

// Acquire attempts to acquire the idempotency lock for the given key.
// Returns:
//   - (nil, nil) if the key is new — caller should proceed with processing
//   - (result, nil) if the key was already processed — caller should return cached result
//   - (nil, err) on store errors
func (c *Checker) Acquire(ctx context.Context, key string) (*CachedResult, error) {
	if key == "" {
		return nil, fmt.Errorf("idempotency key is empty")
	}

	// Try to set the lock
	acquired, err := c.store.SetNX(ctx, "idem:lock:"+key, c.ttl)
	if err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}

	if acquired {
		// New key — caller should process the request
		return nil, nil
	}

	// Key exists — check for cached result
	result, err := c.store.Get(ctx, "idem:result:"+key)
	if err != nil {
		return nil, fmt.Errorf("get cached result: %w", err)
	}

	// If result is nil, the previous request is still in-flight
	// Return a "processing" indicator
	if result == nil {
		return &CachedResult{
			StatusCode: http.StatusConflict,
			Body:       json.RawMessage(`{"code":"REQUEST_IN_FLIGHT","message":"a request with this idempotency key is already being processed"}`),
		}, nil
	}

	return result, nil
}

// Complete stores the result of a successfully-processed request so that
// subsequent requests with the same key return the cached response.
func (c *Checker) Complete(ctx context.Context, key string, statusCode int, body interface{}) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	result := &CachedResult{
		StatusCode: statusCode,
		Body:       raw,
	}

	if err := c.store.Set(ctx, "idem:result:"+key, result, c.ttl); err != nil {
		return fmt.Errorf("store result: %w", err)
	}
	return nil
}
