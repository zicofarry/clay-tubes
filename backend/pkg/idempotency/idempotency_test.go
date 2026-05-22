//go:build unit

package idempotency

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ───── In-Memory Store (for testing) ─────────────────────────────────────────

type memoryStore struct {
	locks   map[string]bool
	results map[string]*CachedResult
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		locks:   make(map[string]bool),
		results: make(map[string]*CachedResult),
	}
}

func (m *memoryStore) Get(_ context.Context, key string) (*CachedResult, error) {
	r, ok := m.results[key]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (m *memoryStore) Set(_ context.Context, key string, result *CachedResult, _ time.Duration) error {
	m.results[key] = result
	return nil
}

func (m *memoryStore) SetNX(_ context.Context, key string, _ time.Duration) (bool, error) {
	if m.locks[key] {
		return false, nil
	}
	m.locks[key] = true
	return true, nil
}

// ───── Tests ─────────────────────────────────────────────────────────────────

func TestExtractKey(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Idempotency-Key", "abc-123")

	key := ExtractKey(req)
	if key != "abc-123" {
		t.Errorf("expected abc-123, got %s", key)
	}
}

func TestExtractKey_Missing(t *testing.T) {
	req := httptest.NewRequest("POST", "/test", nil)

	key := ExtractKey(req)
	if key != "" {
		t.Errorf("expected empty, got %s", key)
	}
}

func TestAcquire_NewKey(t *testing.T) {
	store := newMemoryStore()
	checker := NewChecker(store, DefaultTTL)
	ctx := context.Background()

	result, err := checker.Acquire(ctx, "new-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for new key")
	}
}

func TestAcquire_DuplicateKey_InFlight(t *testing.T) {
	store := newMemoryStore()
	checker := NewChecker(store, DefaultTTL)
	ctx := context.Background()

	// First call — acquires the lock
	_, _ = checker.Acquire(ctx, "dup-key")

	// Second call — lock exists but no result yet (in-flight)
	result, err := checker.Acquire(ctx, "dup-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected a cached result")
	}
	if result.StatusCode != http.StatusConflict {
		t.Errorf("expected 409, got %d", result.StatusCode)
	}
}

func TestAcquire_DuplicateKey_Completed(t *testing.T) {
	store := newMemoryStore()
	checker := NewChecker(store, DefaultTTL)
	ctx := context.Background()

	// First call — acquire and complete
	_, _ = checker.Acquire(ctx, "done-key")
	_ = checker.Complete(ctx, "done-key", http.StatusOK, map[string]string{"tx_id": "abc"})

	// Second call — should return cached result
	result, err := checker.Acquire(ctx, "done-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected a cached result")
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", result.StatusCode)
	}

	var body map[string]string
	if err := json.Unmarshal(result.Body, &body); err != nil {
		t.Fatalf("failed to parse body: %v", err)
	}
	if body["tx_id"] != "abc" {
		t.Errorf("expected tx_id abc, got %s", body["tx_id"])
	}
}

func TestAcquire_EmptyKey(t *testing.T) {
	store := newMemoryStore()
	checker := NewChecker(store, DefaultTTL)

	_, err := checker.Acquire(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty key")
	}
}
