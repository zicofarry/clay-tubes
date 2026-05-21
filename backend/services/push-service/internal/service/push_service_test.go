//go:build unit

package service

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/push-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/push-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

// helper to build a test service with gomock repo
func newTestService(t *testing.T) (*PushService, *repomock.MockPushRepositoryInterface, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockPushRepositoryInterface(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewPushService(mockRepo, logger)
	return svc, mockRepo, ctrl
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestSendPush_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		Send(gomock.Any(), "device-token", "android", gomock.Any()).
		Return(&repository.PushDeliveryResult{
			MessageID: "msg-123",
			Provider:  "fcm",
		}, nil)

	result, err := svc.SendPush(context.Background(), &SendPushRequest{
		Token:    "device-token",
		Platform: "android",
		Payload:  repository.PushPayload{Title: "Test", Body: "Body"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MessageID != "msg-123" {
		t.Errorf("expected msg-123, got %s", result.MessageID)
	}
	if result.Provider != "fcm" {
		t.Errorf("expected fcm, got %s", result.Provider)
	}
}

func TestSendPush_MissingToken(t *testing.T) {
	svc, _, _ := newTestService(t)

	_, err := svc.SendPush(context.Background(), &SendPushRequest{
		Token:    "",
		Platform: "android",
		Payload:  repository.PushPayload{Title: "Test", Body: "Body"},
	})

	if err != ErrMissingToken {
		t.Errorf("expected ErrMissingToken, got %v", err)
	}
}

func TestSendPush_InvalidPlatform(t *testing.T) {
	svc, _, _ := newTestService(t)

	_, err := svc.SendPush(context.Background(), &SendPushRequest{
		Token:    "device-token",
		Platform: "windows",
		Payload:  repository.PushPayload{Title: "Test", Body: "Body"},
	})

	if err != ErrInvalidPlatform {
		t.Errorf("expected ErrInvalidPlatform, got %v", err)
	}
}

func TestSendPush_MissingPayload(t *testing.T) {
	svc, _, _ := newTestService(t)

	_, err := svc.SendPush(context.Background(), &SendPushRequest{
		Token:    "device-token",
		Platform: "android",
		Payload:  repository.PushPayload{Title: "", Body: ""},
	})

	if err != ErrMissingPayload {
		t.Errorf("expected ErrMissingPayload, got %v", err)
	}
}

func TestSendBatchPush_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		BatchSend(gomock.Any(), []string{"t1", "t2"}, "android", gomock.Any()).
		Return(&repository.BatchDeliveryResult{
			Total:        2,
			SuccessCount: 2,
			FailureCount: 0,
			Results: []repository.BatchResult{
				{Token: "t1", Status: "success"},
				{Token: "t2", Status: "success"},
			},
		}, nil)

	result, err := svc.SendBatchPush(context.Background(), &SendBatchPushRequest{
		Tokens: []BatchToken{
			{Token: "t1", Platform: "android"},
			{Token: "t2", Platform: "android"},
		},
		Payload: repository.PushPayload{Title: "Batch", Body: "Body"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Total)
	}
	if result.SuccessCount != 2 {
		t.Errorf("expected success 2, got %d", result.SuccessCount)
	}
}

func TestSendBatchPush_EmptyTokens(t *testing.T) {
	svc, _, _ := newTestService(t)

	_, err := svc.SendBatchPush(context.Background(), &SendBatchPushRequest{
		Tokens:  []BatchToken{},
		Payload: repository.PushPayload{Title: "Batch", Body: "Body"},
	})

	if err != ErrMissingTokens {
		t.Errorf("expected ErrMissingTokens, got %v", err)
	}
}

func TestSubscribeTopic_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		SubscribeToTopic(gomock.Any(), "promo_all", []string{"t1", "t2"}).
		Return(&repository.TopicResult{SuccessCount: 2, FailureCount: 0}, nil)

	result, err := svc.SubscribeTopic(context.Background(), "promo_all", &TopicSubscribeRequest{
		Tokens: []string{"t1", "t2"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SuccessCount != 2 {
		t.Errorf("expected success 2, got %d", result.SuccessCount)
	}
}

func TestSubscribeTopic_MissingTopic(t *testing.T) {
	svc, _, _ := newTestService(t)

	_, err := svc.SubscribeTopic(context.Background(), "", &TopicSubscribeRequest{
		Tokens: []string{"t1"},
	})

	if err != ErrMissingTopic {
		t.Errorf("expected ErrMissingTopic, got %v", err)
	}
}

func TestSendTopicPush_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		SendToTopic(gomock.Any(), "driver_bandung", gomock.Any()).
		Return(&repository.PushDeliveryResult{
			MessageID: "topic-msg-123",
			Provider:  "fcm",
		}, nil)

	result, err := svc.SendTopicPush(context.Background(), "driver_bandung", &SendTopicPushRequest{
		Payload: repository.PushPayload{Title: "Topic", Body: "Body"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MessageID != "topic-msg-123" {
		t.Errorf("expected topic-msg-123, got %s", result.MessageID)
	}
}

func TestSendTopicPush_MissingPayload(t *testing.T) {
	svc, _, _ := newTestService(t)

	_, err := svc.SendTopicPush(context.Background(), "driver_bandung", &SendTopicPushRequest{
		Payload: repository.PushPayload{Title: "", Body: ""},
	})

	if err != ErrMissingPayload {
		t.Errorf("expected ErrMissingPayload, got %v", err)
	}
}
