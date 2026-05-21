// Package service implements the business logic for the Push Service.
package service

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/zicofarry/clay-app/backend/services/push-service/internal/repository"
)

// ── Service Error ────────────────────────────────────────────────────────────

// ServiceError represents a business logic error with HTTP status mapping.
type ServiceError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *ServiceError) Error() string {
	return e.Message
}

// Common errors
var (
	ErrMissingToken     = &ServiceError{http.StatusBadRequest, "MISSING_TOKEN", "device token is required"}
	ErrMissingPlatform  = &ServiceError{http.StatusBadRequest, "MISSING_PLATFORM", "platform is required"}
	ErrMissingPayload   = &ServiceError{http.StatusBadRequest, "MISSING_PAYLOAD", "payload title and body are required"}
	ErrMissingTokens    = &ServiceError{http.StatusBadRequest, "MISSING_TOKENS", "at least one token is required"}
	ErrMissingTopic     = &ServiceError{http.StatusBadRequest, "MISSING_TOPIC", "topic name is required"}
	ErrInvalidPlatform  = &ServiceError{http.StatusBadRequest, "INVALID_PLATFORM", "platform must be ios, android, or web"}
	ErrTokenExpired     = &ServiceError{http.StatusGone, "TOKEN_EXPIRED", "device token is invalid or expired"}
	ErrProviderError    = &ServiceError{http.StatusBadGateway, "PROVIDER_ERROR", "upstream provider (FCM/APNs) error"}
	ErrBatchTooLarge    = &ServiceError{http.StatusBadRequest, "BATCH_TOO_LARGE", "maximum 500 tokens per batch"}
)

// ── Request/Response DTOs ────────────────────────────────────────────────────

type SendPushRequest struct {
	Token    string                 `json:"token"`
	Platform string                 `json:"platform"` // ios | android | web
	Payload  repository.PushPayload `json:"payload"`
}

type SendPushResponse struct {
	MessageID string `json:"message_id"`
	Provider  string `json:"provider"`
}

type BatchToken struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

type SendBatchPushRequest struct {
	Tokens  []BatchToken           `json:"tokens"`
	Payload repository.PushPayload `json:"payload"`
}

type SendBatchPushResponse struct {
	Total        int                      `json:"total"`
	SuccessCount int                      `json:"success_count"`
	FailureCount int                      `json:"failure_count"`
	Results      []repository.BatchResult `json:"results"`
}

type TopicSubscribeRequest struct {
	Tokens []string `json:"tokens"`
}

type TopicSubscribeResponse struct {
	SuccessCount int `json:"success_count"`
	FailureCount int `json:"failure_count"`
}

type SendTopicPushRequest struct {
	Payload repository.PushPayload `json:"payload"`
}

type SendTopicPushResponse struct {
	MessageID string `json:"message_id"`
}

// ── Interface ────────────────────────────────────────────────────────────────

// PushServiceInterface defines the contract for the push service layer.
// Used by handler layer and for mock generation in tests.
//
//go:generate mockgen -source=push_service.go -destination=../../mocks/mock_push_service.go -package=mocks
type PushServiceInterface interface {
	SendPush(ctx context.Context, req *SendPushRequest) (*SendPushResponse, error)
	SendBatchPush(ctx context.Context, req *SendBatchPushRequest) (*SendBatchPushResponse, error)
	SubscribeTopic(ctx context.Context, topic string, req *TopicSubscribeRequest) (*TopicSubscribeResponse, error)
	UnsubscribeTopic(ctx context.Context, topic string, req *TopicSubscribeRequest) (*TopicSubscribeResponse, error)
	SendTopicPush(ctx context.Context, topic string, req *SendTopicPushRequest) (*SendTopicPushResponse, error)
}

// ── Implementation ───────────────────────────────────────────────────────────

// PushService implements PushServiceInterface.
type PushService struct {
	repo   repository.PushRepositoryInterface
	logger *slog.Logger
}

// NewPushService creates a new PushService.
func NewPushService(repo repository.PushRepositoryInterface, logger *slog.Logger) *PushService {
	return &PushService{repo: repo, logger: logger}
}

func (s *PushService) SendPush(ctx context.Context, req *SendPushRequest) (*SendPushResponse, error) {
	// Validate
	if req.Token == "" {
		return nil, ErrMissingToken
	}
	if req.Platform == "" {
		return nil, ErrMissingPlatform
	}
	if !isValidPlatform(req.Platform) {
		return nil, ErrInvalidPlatform
	}
	if req.Payload.Title == "" || req.Payload.Body == "" {
		return nil, ErrMissingPayload
	}

	result, err := s.repo.Send(ctx, req.Token, req.Platform, req.Payload)
	if err != nil {
		return nil, err
	}

	s.logger.Info("push delivered",
		slog.String("message_id", result.MessageID),
		slog.String("provider", result.Provider),
	)

	return &SendPushResponse{
		MessageID: result.MessageID,
		Provider:  result.Provider,
	}, nil
}

func (s *PushService) SendBatchPush(ctx context.Context, req *SendBatchPushRequest) (*SendBatchPushResponse, error) {
	if len(req.Tokens) == 0 {
		return nil, ErrMissingTokens
	}
	if len(req.Tokens) > 500 {
		return nil, ErrBatchTooLarge
	}
	if req.Payload.Title == "" || req.Payload.Body == "" {
		return nil, ErrMissingPayload
	}

	// Extract token strings
	tokenStrs := make([]string, len(req.Tokens))
	for i, t := range req.Tokens {
		tokenStrs[i] = t.Token
	}

	// Use first token's platform for batch (simplified)
	platform := req.Tokens[0].Platform
	result, err := s.repo.BatchSend(ctx, tokenStrs, platform, req.Payload)
	if err != nil {
		return nil, err
	}

	s.logger.Info("batch push delivered",
		slog.Int("total", result.Total),
		slog.Int("success", result.SuccessCount),
	)

	return &SendBatchPushResponse{
		Total:        result.Total,
		SuccessCount: result.SuccessCount,
		FailureCount: result.FailureCount,
		Results:      result.Results,
	}, nil
}

func (s *PushService) SubscribeTopic(ctx context.Context, topic string, req *TopicSubscribeRequest) (*TopicSubscribeResponse, error) {
	if topic == "" {
		return nil, ErrMissingTopic
	}
	if len(req.Tokens) == 0 {
		return nil, ErrMissingTokens
	}

	result, err := s.repo.SubscribeToTopic(ctx, topic, req.Tokens)
	if err != nil {
		return nil, err
	}

	s.logger.Info("topic subscribed",
		slog.String("topic", topic),
		slog.Int("success", result.SuccessCount),
	)

	return &TopicSubscribeResponse{
		SuccessCount: result.SuccessCount,
		FailureCount: result.FailureCount,
	}, nil
}

func (s *PushService) UnsubscribeTopic(ctx context.Context, topic string, req *TopicSubscribeRequest) (*TopicSubscribeResponse, error) {
	if topic == "" {
		return nil, ErrMissingTopic
	}
	if len(req.Tokens) == 0 {
		return nil, ErrMissingTokens
	}

	result, err := s.repo.UnsubscribeFromTopic(ctx, topic, req.Tokens)
	if err != nil {
		return nil, err
	}

	s.logger.Info("topic unsubscribed",
		slog.String("topic", topic),
		slog.Int("success", result.SuccessCount),
	)

	return &TopicSubscribeResponse{
		SuccessCount: result.SuccessCount,
		FailureCount: result.FailureCount,
	}, nil
}

func (s *PushService) SendTopicPush(ctx context.Context, topic string, req *SendTopicPushRequest) (*SendTopicPushResponse, error) {
	if topic == "" {
		return nil, ErrMissingTopic
	}
	if req.Payload.Title == "" || req.Payload.Body == "" {
		return nil, ErrMissingPayload
	}

	result, err := s.repo.SendToTopic(ctx, topic, req.Payload)
	if err != nil {
		return nil, err
	}

	s.logger.Info("topic push delivered",
		slog.String("topic", topic),
		slog.String("message_id", result.MessageID),
	)

	return &SendTopicPushResponse{
		MessageID: result.MessageID,
	}, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func isValidPlatform(platform string) bool {
	return platform == "ios" || platform == "android" || platform == "web"
}
