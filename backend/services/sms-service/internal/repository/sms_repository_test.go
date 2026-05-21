//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func setupTestRedis(t *testing.T) (*SMSRepository, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	repo := NewSMSRepository(rdb)
	return repo, mr
}

func TestSMSRepository_StoreAndGetRetryMessage(t *testing.T) {
	repo, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	err := repo.StoreRetryMessage(ctx, "msg-123", "+628123456789", "Hello", 0, 24*time.Hour)
	if err != nil {
		t.Fatalf("failed to store retry message: %v", err)
	}

	phone, message, attempts, err := repo.GetRetryMessage(ctx, "msg-123")
	if err != nil {
		t.Fatalf("failed to get retry message: %v", err)
	}
	if phone != "+628123456789" {
		t.Errorf("expected phone +628123456789, got %s", phone)
	}
	if message != "Hello" {
		t.Errorf("expected message Hello, got %s", message)
	}
	if attempts != 0 {
		t.Errorf("expected attempts 0, got %d", attempts)
	}
}

func TestSMSRepository_IncrementRetryAttempt(t *testing.T) {
	repo, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	_ = repo.StoreRetryMessage(ctx, "msg-456", "+628123456789", "Test", 0, 24*time.Hour)

	newCount, err := repo.IncrementRetryAttempt(ctx, "msg-456")
	if err != nil {
		t.Fatalf("failed to increment retry attempt: %v", err)
	}
	if newCount != 1 {
		t.Errorf("expected 1, got %d", newCount)
	}
}

func TestSMSRepository_RateLimit(t *testing.T) {
	repo, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	count, err := repo.IncrementRateLimit(ctx, "+628123456789", time.Hour)
	if err != nil {
		t.Fatalf("failed to increment rate limit: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	got, err := repo.GetRateLimit(ctx, "+628123456789")
	if err != nil {
		t.Fatalf("failed to get rate limit: %v", err)
	}
	if got != 1 {
		t.Errorf("expected rate limit 1, got %d", got)
	}
}

func TestSMSRepository_GetRetryMessage_NotFound(t *testing.T) {
	repo, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	_, _, _, err := repo.GetRetryMessage(ctx, "nonexistent")
	if err != ErrSMSNotFound {
		t.Errorf("expected ErrSMSNotFound, got %v", err)
	}
}
