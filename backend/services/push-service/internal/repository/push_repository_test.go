//go:build unit

package repository

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestSend_FCM(t *testing.T) {
	repo := NewPushRepository(testLogger(), nil)

	result, err := repo.Send(context.Background(), "device-token-android", "android", PushPayload{
		Title: "Test Title",
		Body:  "Test Body",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Provider != "fcm" {
		t.Errorf("expected provider 'fcm', got '%s'", result.Provider)
	}
	if result.MessageID == "" {
		t.Error("expected non-empty message_id")
	}
}

func TestSend_APNs(t *testing.T) {
	repo := NewPushRepository(testLogger(), nil)

	result, err := repo.Send(context.Background(), "device-token-ios", "ios", PushPayload{
		Title: "Test Title",
		Body:  "Test Body",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Provider != "apns" {
		t.Errorf("expected provider 'apns', got '%s'", result.Provider)
	}
}

func TestBatchSend_Success(t *testing.T) {
	repo := NewPushRepository(testLogger(), nil)

	tokens := []string{"token-1", "token-2", "token-3"}
	result, err := repo.BatchSend(context.Background(), tokens, "android", PushPayload{
		Title: "Batch Test",
		Body:  "Batch Body",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
}

func TestSubscribeToTopic_Success(t *testing.T) {
	repo := NewPushRepository(testLogger(), nil)

	result, err := repo.SubscribeToTopic(context.Background(), "promo_all", []string{"token-1", "token-2"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SuccessCount != 2 {
		t.Errorf("expected success_count 2, got %d", result.SuccessCount)
	}
}

func TestSendToTopic_EmptyTopic(t *testing.T) {
	repo := NewPushRepository(testLogger(), nil)

	_, err := repo.SendToTopic(context.Background(), "", PushPayload{
		Title: "Topic Test",
		Body:  "Topic Body",
	})

	if err == nil {
		t.Error("expected error for empty topic, got nil")
	}
}

func TestSendToTopic_Success(t *testing.T) {
	repo := NewPushRepository(testLogger(), nil)

	result, err := repo.SendToTopic(context.Background(), "driver_bandung", PushPayload{
		Title: "Topic Test",
		Body:  "Topic Body",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Provider != "fcm" {
		t.Errorf("expected provider 'fcm', got '%s'", result.Provider)
	}
}
