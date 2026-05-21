//go:build unit

package service

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/zicofarry/clay-app/backend/services/sms-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

func TestSMSService_SendOTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockSMSRepositoryInterface(ctrl)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewSMSService(mockRepo, logger)

	ctx := context.Background()
	req := SendOTPRequest{
		Phone:   "+628123456789",
		Purpose: "login",
	}

	// Expectations — no more CreateSMSLog
	mockRepo.EXPECT().GetRateLimit(ctx, req.Phone).Return(int64(0), nil)
	mockRepo.EXPECT().StoreOTP(ctx, req.Phone, req.Purpose, gomock.Any(), 5*time.Minute).Return(nil)
	mockRepo.EXPECT().IncrementRateLimit(ctx, req.Phone, time.Hour).Return(int64(1), nil)

	resp, err := svc.SendOTP(ctx, req)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if resp == nil {
		t.Errorf("expected response, got nil")
	}
	if resp != nil && resp.Phone != req.Phone {
		t.Errorf("expected phone %v, got %v", req.Phone, resp.Phone)
	}
}

func TestSMSService_SendOTP_RateLimitExceeded(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockSMSRepositoryInterface(ctrl)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewSMSService(mockRepo, logger)

	ctx := context.Background()
	req := SendOTPRequest{
		Phone:   "+628123456789",
		Purpose: "login",
	}

	// Expectations
	mockRepo.EXPECT().GetRateLimit(ctx, req.Phone).Return(int64(10), nil) // Rate limit hit

	resp, err := svc.SendOTP(ctx, req)

	if err != ErrRateLimitExceeded {
		t.Errorf("expected ErrRateLimitExceeded, got %v", err)
	}
	if resp != nil {
		t.Errorf("expected nil response, got %v", resp)
	}
}

func TestSMSService_VerifyOTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := repomock.NewMockSMSRepositoryInterface(ctrl)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	svc := NewSMSService(mockRepo, logger)

	ctx := context.Background()
	req := VerifyOTPRequest{
		Phone:   "+628123456789",
		Purpose: "login",
		OTPCode: "123456",
	}

	// Expectations
	mockRepo.EXPECT().GetOTP(ctx, req.Phone, req.Purpose).Return("123456", nil)
	mockRepo.EXPECT().DeleteOTP(ctx, req.Phone, req.Purpose).Return(nil)

	resp, err := svc.VerifyOTP(ctx, req)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if resp == nil || !resp.Valid {
		t.Errorf("expected valid response")
	}
}
