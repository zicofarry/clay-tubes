package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/sms-service/internal/repository"
)

var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded (max 10 SMS/hour)")
	ErrInvalidOTP        = errors.New("invalid or expired OTP")
	ErrTooManyAttempts   = errors.New("too many failed attempts")
)

// --- Domain Models ---

type SendOTPRequest struct {
	Phone   string `json:"phone"`
	Purpose string `json:"purpose"`
}

type SendOTPResponse struct {
	Phone                 string    `json:"phone"`
	ExpiresAt             time.Time `json:"expires_at"`
	ResendCooldownSeconds int       `json:"resend_cooldown_seconds"`
}

type VerifyOTPRequest struct {
	Phone   string `json:"phone"`
	OTPCode string `json:"otp_code"`
	Purpose string `json:"purpose"`
}

type VerifyOTPResponse struct {
	Valid bool   `json:"valid"`
	Phone string `json:"phone"`
}

type SendSMSRequest struct {
	To      string `json:"to"`
	Message string `json:"message"`
}

type SendSMSResponse struct {
	SMSID  string `json:"sms_id"`
	Status string `json:"status"`
}

type WebhookPayload map[string]interface{} // Simplified for generic providers

type SmsStatusResponse struct {
	SMSID    string `json:"sms_id"`
	To       string `json:"to"`
	Status   string `json:"status"`
	Attempts int    `json:"attempts"`
}

// --- Service Interface ---

//go:generate mockgen -source=sms_service.go -destination=../../mocks/mock_sms_service.go -package=mocks
type SMSServiceInterface interface {
	SendOTP(ctx context.Context, req SendOTPRequest) (*SendOTPResponse, error)
	VerifyOTP(ctx context.Context, req VerifyOTPRequest) (*VerifyOTPResponse, error)
	SendSMS(ctx context.Context, req SendSMSRequest, idempotencyKey string) (*SendSMSResponse, error)
	ProcessWebhook(ctx context.Context, payload WebhookPayload) error
	GetSMSStatus(ctx context.Context, smsID string) (*SmsStatusResponse, error)
}

// --- Service Implementation ---

type SMSService struct {
	repo   repository.SMSRepositoryInterface
	logger *slog.Logger
}

func NewSMSService(repo repository.SMSRepositoryInterface, logger *slog.Logger) *SMSService {
	return &SMSService{
		repo:   repo,
		logger: logger,
	}
}

func generateOTP() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func (s *SMSService) SendOTP(ctx context.Context, req SendOTPRequest) (*SendOTPResponse, error) {
	// 1. Check Rate Limit (max 10 per hour)
	count, err := s.repo.GetRateLimit(ctx, req.Phone)
	if err != nil {
		s.logger.Error("failed to get rate limit", slog.Any("error", err))
		return nil, err
	}
	if count >= 10 {
		return nil, ErrRateLimitExceeded
	}

	// 2. Generate OTP
	code, err := generateOTP()
	if err != nil {
		s.logger.Error("failed to generate otp", slog.Any("error", err))
		return nil, err
	}

	// 3. Store OTP in Redis (TTL 5 min)
	ttl := 5 * time.Minute
	err = s.repo.StoreOTP(ctx, req.Phone, req.Purpose, code, ttl)
	if err != nil {
		s.logger.Error("failed to store otp", slog.Any("error", err))
		return nil, err
	}

	// 4. Increment Rate Limit (TTL 1 hour)
	_, err = s.repo.IncrementRateLimit(ctx, req.Phone, time.Hour)
	if err != nil {
		s.logger.Error("failed to increment rate limit", slog.Any("error", err))
	}

	// 5. Send actual SMS (Mocked for now)
	message := fmt.Sprintf("[%s] Your verification code is: %s. Valid for 5 minutes.", req.Purpose, code)
	s.logger.Info("Mock sending SMS", slog.String("phone", req.Phone), slog.String("message", message))

	expiresAt := time.Now().Add(ttl)
	return &SendOTPResponse{
		Phone:                 req.Phone,
		ExpiresAt:             expiresAt,
		ResendCooldownSeconds: 60, // Arbitrary cooldown rule for response
	}, nil
}

func (s *SMSService) VerifyOTP(ctx context.Context, req VerifyOTPRequest) (*VerifyOTPResponse, error) {
	code, err := s.repo.GetOTP(ctx, req.Phone, req.Purpose)
	if err != nil {
		if errors.Is(err, repository.ErrOTPNotFound) {
			return nil, ErrInvalidOTP
		}
		s.logger.Error("failed to get otp", slog.Any("error", err))
		return nil, err
	}

	if code != req.OTPCode {
		return nil, ErrInvalidOTP
	}

	// Correct OTP, delete it
	err = s.repo.DeleteOTP(ctx, req.Phone, req.Purpose)
	if err != nil {
		s.logger.Error("failed to delete otp after verify", slog.Any("error", err))
	}

	return &VerifyOTPResponse{
		Valid: true,
		Phone: req.Phone,
	}, nil
}

func (s *SMSService) SendSMS(ctx context.Context, req SendSMSRequest, idempotencyKey string) (*SendSMSResponse, error) {
	smsID := uuid.New().String()

	// Store in retry queue (Redis HASH) with TTL 24h
	err := s.repo.StoreRetryMessage(ctx, smsID, req.To, req.Message, 0, 24*time.Hour)
	if err != nil {
		s.logger.Error("failed to store retry message", slog.Any("error", err))
		return nil, err
	}

	// Check rate limit
	count, err := s.repo.GetRateLimit(ctx, req.To)
	if err != nil {
		s.logger.Error("failed to get rate limit", slog.Any("error", err))
		return nil, err
	}
	if count >= 10 {
		return nil, ErrRateLimitExceeded
	}

	// Increment rate limit
	_, err = s.repo.IncrementRateLimit(ctx, req.To, time.Hour)
	if err != nil {
		s.logger.Error("failed to increment rate limit", slog.Any("error", err))
	}

	// Trigger async send (mocked)
	s.logger.Info("Queued SMS", slog.String("sms_id", smsID))

	return &SendSMSResponse{
		SMSID:  smsID,
		Status: "queued",
	}, nil
}

func (s *SMSService) ProcessWebhook(ctx context.Context, payload WebhookPayload) error {
	// Simplified processing
	s.logger.Info("Received webhook", slog.Any("payload", payload))
	return nil
}

func (s *SMSService) GetSMSStatus(ctx context.Context, smsID string) (*SmsStatusResponse, error) {
	phone, _, attempts, err := s.repo.GetRetryMessage(ctx, smsID)
	if err != nil {
		return nil, err
	}

	return &SmsStatusResponse{
		SMSID:    smsID,
		To:       phone,
		Status:   "queued",
		Attempts: attempts,
	}, nil
}
