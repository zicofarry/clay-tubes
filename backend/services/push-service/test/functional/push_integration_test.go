//go:build functional

package functional

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zicofarry/clay-app/backend/services/push-service/internal/repository"
)

// redisAddr returns the Redis address for functional tests.
// Uses REDIS_TEST_URL env var if set, otherwise defaults to localhost:6384
// (the host-mapped port from docker-compose).
func redisAddr() string {
	if url := os.Getenv("REDIS_TEST_URL"); url != "" {
		return url
	}
	return "localhost:6384"
}

// setupTestRepo creates a real Redis client connected to the Docker container
// and returns a PushRepository backed by it. It also flushes the database
// to ensure a clean state for each test run.
func setupTestRepo(t *testing.T) (*repository.PushRepository, *redis.Client) {
	t.Helper()

	client := redis.NewClient(&redis.Options{
		Addr: redisAddr(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("failed to connect to Redis at %s: %v (is docker-compose up?)", redisAddr(), err)
	}

	// Flush DB for a clean test state
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("failed to flush Redis: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	repo := repository.NewPushRepository(logger, client)

	t.Cleanup(func() {
		client.Close()
	})

	return repo, client
}

// ── Redis: push:retry:{message_id} HASH ─────────────────────────────────────

func TestRetryMessage_SaveAndGet(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	messageID := "msg_test_001"
	msg := &repository.RetryMessage{
		UserID:   "user_123",
		Token:    "fcm-token-abc",
		Title:    "Driver sedang menuju ke kamu",
		Body:     "Budi Santoso (D 1234 ABC) akan tiba dalam 3 menit",
		Attempts: 1,
		Payload:  `{"order_id":"ord_x9y8z7","action":"open_order_tracking"}`,
	}

	// Save retry message
	err := repo.SaveRetryMessage(ctx, messageID, msg)
	if err != nil {
		t.Fatalf("SaveRetryMessage failed: %v", err)
	}
	t.Logf("✓ SaveRetryMessage succeeded for key push:retry:%s", messageID)

	// Get retry message back
	got, err := repo.GetRetryMessage(ctx, messageID)
	if err != nil {
		t.Fatalf("GetRetryMessage failed: %v", err)
	}

	// Verify all fields match the HASH schema
	if got.UserID != msg.UserID {
		t.Errorf("user_id: expected %q, got %q", msg.UserID, got.UserID)
	}
	if got.Token != msg.Token {
		t.Errorf("token: expected %q, got %q", msg.Token, got.Token)
	}
	if got.Title != msg.Title {
		t.Errorf("title: expected %q, got %q", msg.Title, got.Title)
	}
	if got.Body != msg.Body {
		t.Errorf("body: expected %q, got %q", msg.Body, got.Body)
	}
	if got.Attempts != msg.Attempts {
		t.Errorf("attempts: expected %d, got %d", msg.Attempts, got.Attempts)
	}
	if got.Payload != msg.Payload {
		t.Errorf("payload: expected %q, got %q", msg.Payload, got.Payload)
	}
	t.Log("✓ GetRetryMessage returned correct data from Redis HASH")
}

func TestRetryMessage_TTL(t *testing.T) {
	_, client := setupTestRepo(t)
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	messageID := "msg_ttl_test"
	msg := &repository.RetryMessage{
		UserID:   "user_456",
		Token:    "fcm-token-xyz",
		Title:    "TTL Test",
		Body:     "Testing 24h TTL",
		Attempts: 0,
		Payload:  "{}",
	}

	err := repo.SaveRetryMessage(ctx, messageID, msg)
	if err != nil {
		t.Fatalf("SaveRetryMessage failed: %v", err)
	}

	// Verify TTL is set (should be ~24 hours)
	key := fmt.Sprintf("push:retry:%s", messageID)
	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("failed to get TTL: %v", err)
	}

	// TTL should be between 23h59m and 24h
	if ttl < 23*time.Hour+59*time.Minute || ttl > 24*time.Hour+1*time.Second {
		t.Errorf("expected TTL ~24h, got %v", ttl)
	}
	t.Logf("✓ push:retry:%s TTL = %v (expected ~24h)", messageID, ttl)
}

func TestRetryMessage_NotFound(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	_, err := repo.GetRetryMessage(ctx, "nonexistent_msg_id")
	if err == nil {
		t.Error("expected error for nonexistent message, got nil")
	}
	t.Logf("✓ GetRetryMessage correctly returned error for nonexistent key: %v", err)
}

func TestRetryMessage_UpdateAttempts(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	messageID := "msg_retry_attempts"
	msg := &repository.RetryMessage{
		UserID:   "user_789",
		Token:    "fcm-token-retry",
		Title:    "Retry Test",
		Body:     "Testing retry attempts increment",
		Attempts: 1,
		Payload:  `{"order_id":"ord_retry"}`,
	}

	// Save initial message (attempt 1)
	err := repo.SaveRetryMessage(ctx, messageID, msg)
	if err != nil {
		t.Fatalf("SaveRetryMessage (attempt 1) failed: %v", err)
	}

	// Simulate retry: increment attempts and save again
	msg.Attempts = 2
	err = repo.SaveRetryMessage(ctx, messageID, msg)
	if err != nil {
		t.Fatalf("SaveRetryMessage (attempt 2) failed: %v", err)
	}

	// Third attempt (max 3)
	msg.Attempts = 3
	err = repo.SaveRetryMessage(ctx, messageID, msg)
	if err != nil {
		t.Fatalf("SaveRetryMessage (attempt 3) failed: %v", err)
	}

	// Verify final attempt count
	got, err := repo.GetRetryMessage(ctx, messageID)
	if err != nil {
		t.Fatalf("GetRetryMessage failed: %v", err)
	}
	if got.Attempts != 3 {
		t.Errorf("expected attempts=3, got %d", got.Attempts)
	}
	t.Logf("✓ Retry message updated to %d attempts (max 3)", got.Attempts)
}

// ── Redis: push:invalid_token:{token_hash} STRING ────────────────────────────

func TestInvalidToken_MarkAndCheck(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	tokenHash := "sha256_abc123def456"

	// Token should NOT be invalid initially
	invalid, err := repo.IsTokenInvalid(ctx, tokenHash)
	if err != nil {
		t.Fatalf("IsTokenInvalid failed: %v", err)
	}
	if invalid {
		t.Error("expected token to be valid initially")
	}
	t.Log("✓ Token is valid before marking")

	// Mark token as invalid
	err = repo.MarkTokenInvalid(ctx, tokenHash)
	if err != nil {
		t.Fatalf("MarkTokenInvalid failed: %v", err)
	}
	t.Log("✓ MarkTokenInvalid succeeded")

	// Token should now be invalid
	invalid, err = repo.IsTokenInvalid(ctx, tokenHash)
	if err != nil {
		t.Fatalf("IsTokenInvalid failed: %v", err)
	}
	if !invalid {
		t.Error("expected token to be marked invalid")
	}
	t.Log("✓ IsTokenInvalid correctly returns true after marking")
}

func TestInvalidToken_TTL(t *testing.T) {
	_, client := setupTestRepo(t)
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	tokenHash := "sha256_ttl_test"

	err := repo.MarkTokenInvalid(ctx, tokenHash)
	if err != nil {
		t.Fatalf("MarkTokenInvalid failed: %v", err)
	}

	// Verify TTL is set (should be ~7 days)
	key := fmt.Sprintf("push:invalid_token:%s", tokenHash)
	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("failed to get TTL: %v", err)
	}

	expectedTTL := 7 * 24 * time.Hour
	if ttl < expectedTTL-1*time.Minute || ttl > expectedTTL+1*time.Second {
		t.Errorf("expected TTL ~7 days, got %v", ttl)
	}
	t.Logf("✓ push:invalid_token:%s TTL = %v (expected ~7 days)", tokenHash, ttl)
}

func TestInvalidToken_ValueIsMarker(t *testing.T) {
	_, client := setupTestRepo(t)
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	tokenHash := "sha256_marker_test"

	err := repo.MarkTokenInvalid(ctx, tokenHash)
	if err != nil {
		t.Fatalf("MarkTokenInvalid failed: %v", err)
	}

	// Verify the raw value stored is "1" (marker) as per schema
	key := fmt.Sprintf("push:invalid_token:%s", tokenHash)
	val, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("failed to get raw value: %v", err)
	}
	if val != "1" {
		t.Errorf("expected value '1' (marker), got %q", val)
	}
	t.Log("✓ Invalid token value is '1' (marker) as per schema")
}

func TestInvalidToken_NotMarked(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	invalid, err := repo.IsTokenInvalid(ctx, "unknown_token_hash")
	if err != nil {
		t.Fatalf("IsTokenInvalid failed: %v", err)
	}
	if invalid {
		t.Error("expected unknown token to be valid")
	}
	t.Log("✓ Unknown token correctly returns valid (not invalid)")
}

// ── Provider Methods (Send, BatchSend, Topics) ──────────────────────────────

func TestSend_FCM(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	result, err := repo.Send(ctx, "test-device-token-android", "android", repository.PushPayload{
		Title: "Driver sedang menuju ke kamu",
		Body:  "Budi Santoso (D 1234 ABC) akan tiba dalam 3 menit",
		Data: map[string]interface{}{
			"order_id": "ord_x9y8z7",
			"action":   "open_order_tracking",
		},
		TTLSeconds: 3600,
	})

	if err != nil {
		t.Fatalf("failed to send push: %v", err)
	}
	if result.MessageID == "" {
		t.Error("expected non-empty message_id")
	}
	if result.Provider != "fcm" {
		t.Errorf("expected provider 'fcm', got '%s'", result.Provider)
	}
	t.Logf("✓ Sent FCM push with MessageID: %s", result.MessageID)
}

func TestSend_APNs(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	result, err := repo.Send(ctx, "test-device-token-ios", "ios", repository.PushPayload{
		Title: "Pesanan kamu sudah siap",
		Body:  "Silakan ambil pesanan kamu di loket 3",
	})

	if err != nil {
		t.Fatalf("failed to send push: %v", err)
	}
	if result.Provider != "apns" {
		t.Errorf("expected provider 'apns', got '%s'", result.Provider)
	}
	t.Log("✓ Sent iOS push via APNs")
}

func TestBatchSend(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	tokens := []string{"token-1", "token-2", "token-3"}
	result, err := repo.BatchSend(ctx, tokens, "android", repository.PushPayload{
		Title: "Promo Akhir Pekan!",
		Body:  "Diskon 50% untuk semua perjalanan",
	})

	if err != nil {
		t.Fatalf("failed to batch send: %v", err)
	}
	if result.Total != 3 {
		t.Errorf("expected total 3, got %d", result.Total)
	}
	if result.SuccessCount != 3 {
		t.Errorf("expected success_count 3, got %d", result.SuccessCount)
	}
	if result.FailureCount != 0 {
		t.Errorf("expected failure_count 0, got %d", result.FailureCount)
	}
	t.Logf("✓ Batch sent: %d total, %d success, %d failed", result.Total, result.SuccessCount, result.FailureCount)
}

func TestSubscribeAndUnsubscribeTopic(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	tokens := []string{"token-a", "token-b"}

	// Subscribe
	subResult, err := repo.SubscribeToTopic(ctx, "promo_bandung", tokens)
	if err != nil {
		t.Fatalf("failed to subscribe to topic: %v", err)
	}
	if subResult.SuccessCount != 2 {
		t.Errorf("expected subscribe success_count 2, got %d", subResult.SuccessCount)
	}
	t.Logf("✓ Subscribed %d tokens to topic 'promo_bandung'", subResult.SuccessCount)

	// Unsubscribe
	unsubResult, err := repo.UnsubscribeFromTopic(ctx, "promo_bandung", tokens)
	if err != nil {
		t.Fatalf("failed to unsubscribe from topic: %v", err)
	}
	if unsubResult.SuccessCount != 2 {
		t.Errorf("expected unsubscribe success_count 2, got %d", unsubResult.SuccessCount)
	}
	t.Logf("✓ Unsubscribed %d tokens from topic 'promo_bandung'", unsubResult.SuccessCount)
}

func TestSendToTopic(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	result, err := repo.SendToTopic(ctx, "driver_all", repository.PushPayload{
		Title: "Update Sistem",
		Body:  "Silakan update aplikasi ke versi terbaru",
	})

	if err != nil {
		t.Fatalf("failed to send topic push: %v", err)
	}
	if result.MessageID == "" {
		t.Error("expected non-empty message_id")
	}
	if result.Provider != "fcm" {
		t.Errorf("expected provider 'fcm', got '%s'", result.Provider)
	}
	t.Logf("✓ Topic push sent with MessageID: %s", result.MessageID)
}

func TestSendToTopic_EmptyTopicError(t *testing.T) {
	repo, _ := setupTestRepo(t)
	ctx := context.Background()

	_, err := repo.SendToTopic(ctx, "", repository.PushPayload{
		Title: "Test",
		Body:  "Body",
	})

	if err == nil {
		t.Error("expected error for empty topic, got nil")
	}
	t.Log("✓ Correctly rejected empty topic name")
}
