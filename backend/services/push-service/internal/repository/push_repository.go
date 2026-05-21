// Package repository implements the data access layer for the Push Service.
package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// ── Models ───────────────────────────────────────────────────────────────────

// RetryMessage represents a push message queued for retry.
type RetryMessage struct {
	UserID   string `redis:"user_id" json:"user_id"`
	Token    string `redis:"token" json:"token"`
	Title    string `redis:"title" json:"title"`
	Body     string `redis:"body" json:"body"`
	Attempts int    `redis:"attempts" json:"attempts"`
	Payload  string `redis:"payload" json:"payload"` // JSON string
}

// PushPayload defines the content of the push notification.
type PushPayload struct {
	Title      string                 `json:"title"`
	Body       string                 `json:"body"`
	ImageURL   string                 `json:"image_url,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	TTLSeconds int                    `json:"ttl_seconds,omitempty"`
}

// PushDeliveryResult represents the result for a single push delivery.
type PushDeliveryResult struct {
	MessageID string `json:"message_id"`
	Provider  string `json:"provider"` // fcm | apns
}

// BatchResult represents the result for a single token in a batch.
type BatchResult struct {
	Token  string `json:"token"`
	Status string `json:"status"` // success | error
	Error  string `json:"error,omitempty"`
}

// BatchDeliveryResult represents the aggregated result of a batch send.
type BatchDeliveryResult struct {
	Total        int           `json:"total"`
	SuccessCount int           `json:"success_count"`
	FailureCount int           `json:"failure_count"`
	Results      []BatchResult `json:"results"`
}

// TopicResult represents the result of a topic subscribe/unsubscribe.
type TopicResult struct {
	SuccessCount int `json:"success_count"`
	FailureCount int `json:"failure_count"`
}

// ── Interface ────────────────────────────────────────────────────────────────

// PushRepositoryInterface defines the contract for push delivery providers.
// Used by service layer and for mock generation in tests.
//
//go:generate mockgen -source=push_repository.go -destination=../../mocks/repomock/mock_push_repository.go -package=repomock
type PushRepositoryInterface interface {
	// Send delivers a push notification to a single device token.
	Send(ctx context.Context, token string, platform string, payload PushPayload) (*PushDeliveryResult, error)

	// BatchSend delivers a push notification to multiple device tokens.
	BatchSend(ctx context.Context, tokens []string, platform string, payload PushPayload) (*BatchDeliveryResult, error)

	// SubscribeToTopic subscribes device tokens to an FCM topic.
	SubscribeToTopic(ctx context.Context, topic string, tokens []string) (*TopicResult, error)

	// UnsubscribeFromTopic unsubscribes device tokens from an FCM topic.
	UnsubscribeFromTopic(ctx context.Context, topic string, tokens []string) (*TopicResult, error)

	// SendToTopic sends a push notification to an entire FCM topic.
	SendToTopic(ctx context.Context, topic string, payload PushPayload) (*PushDeliveryResult, error)

	// SaveRetryMessage saves a push message for later retry on FCM/APNs failure.
	SaveRetryMessage(ctx context.Context, messageID string, msg *RetryMessage) error

	// GetRetryMessage retrieves a retry message by its ID.
	GetRetryMessage(ctx context.Context, messageID string) (*RetryMessage, error)

	// MarkTokenInvalid marks an FCM token as known-invalid.
	MarkTokenInvalid(ctx context.Context, tokenHash string) error

	// IsTokenInvalid checks if an FCM token is marked as invalid.
	IsTokenInvalid(ctx context.Context, tokenHash string) (bool, error)
}

// ── Implementation ───────────────────────────────────────────────────────────

// PushRepository implements PushRepositoryInterface using dummy FCM/APNs providers.
type PushRepository struct {
	logger *slog.Logger
	redis  *redis.Client
}

// NewPushRepository creates a new PushRepository.
func NewPushRepository(logger *slog.Logger, redisClient *redis.Client) *PushRepository {
	return &PushRepository{
		logger: logger,
		redis:  redisClient,
	}
}

func (r *PushRepository) Send(ctx context.Context, token string, platform string, payload PushPayload) (*PushDeliveryResult, error) {
	provider := "fcm"
	if platform == "ios" {
		provider = "apns"
	}

	r.logger.Info("push sent",
		slog.String("provider", provider),
		slog.String("token", token),
		slog.String("title", payload.Title),
	)

	return &PushDeliveryResult{
		MessageID: uuid.New().String(),
		Provider:  provider,
	}, nil
}

func (r *PushRepository) BatchSend(ctx context.Context, tokens []string, platform string, payload PushPayload) (*BatchDeliveryResult, error) {
	provider := "fcm"
	if platform == "ios" {
		provider = "apns"
	}

	r.logger.Info("batch push sent",
		slog.String("provider", provider),
		slog.Int("count", len(tokens)),
	)

	results := make([]BatchResult, len(tokens))
	for i, t := range tokens {
		results[i] = BatchResult{
			Token:  t,
			Status: "success",
		}
	}

	return &BatchDeliveryResult{
		Total:        len(tokens),
		SuccessCount: len(tokens),
		FailureCount: 0,
		Results:      results,
	}, nil
}

func (r *PushRepository) SubscribeToTopic(ctx context.Context, topic string, tokens []string) (*TopicResult, error) {
	r.logger.Info("topic subscribe",
		slog.String("topic", topic),
		slog.Int("count", len(tokens)),
	)

	return &TopicResult{
		SuccessCount: len(tokens),
		FailureCount: 0,
	}, nil
}

func (r *PushRepository) UnsubscribeFromTopic(ctx context.Context, topic string, tokens []string) (*TopicResult, error) {
	r.logger.Info("topic unsubscribe",
		slog.String("topic", topic),
		slog.Int("count", len(tokens)),
	)

	return &TopicResult{
		SuccessCount: len(tokens),
		FailureCount: 0,
	}, nil
}

func (r *PushRepository) SendToTopic(ctx context.Context, topic string, payload PushPayload) (*PushDeliveryResult, error) {
	if topic == "" {
		return nil, fmt.Errorf("topic name is required")
	}

	r.logger.Info("topic push sent",
		slog.String("topic", topic),
		slog.String("title", payload.Title),
	)

	return &PushDeliveryResult{
		MessageID: uuid.New().String(),
		Provider:  "fcm",
	}, nil
}

func (r *PushRepository) SaveRetryMessage(ctx context.Context, messageID string, msg *RetryMessage) error {
	key := fmt.Sprintf("push:retry:%s", messageID)
	if err := r.redis.HSet(ctx, key, msg).Err(); err != nil {
		return err
	}
	return r.redis.Expire(ctx, key, 24*time.Hour).Err()
}

func (r *PushRepository) GetRetryMessage(ctx context.Context, messageID string) (*RetryMessage, error) {
	key := fmt.Sprintf("push:retry:%s", messageID)
	
	var msg RetryMessage
	err := r.redis.HGetAll(ctx, key).Scan(&msg)
	if err != nil {
		return nil, err
	}
	
	if msg.Token == "" && msg.UserID == "" {
		return nil, redis.Nil
	}

	return &msg, nil
}

func (r *PushRepository) MarkTokenInvalid(ctx context.Context, tokenHash string) error {
	key := fmt.Sprintf("push:invalid_token:%s", tokenHash)
	return r.redis.Set(ctx, key, "1", 7*24*time.Hour).Err()
}

func (r *PushRepository) IsTokenInvalid(ctx context.Context, tokenHash string) (bool, error) {
	key := fmt.Sprintf("push:invalid_token:%s", tokenHash)
	val, err := r.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return val == "1", nil
}
