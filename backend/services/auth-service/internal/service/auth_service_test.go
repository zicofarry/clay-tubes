//go:build unit

package service

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/auth-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/auth-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

// helper to build a test service with gomock repo
func newTestService(t *testing.T) (*AuthService, *repomock.MockAuthRepositoryInterface, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockAuthRepositoryInterface(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewAuthService(mockRepo, logger)
	return svc, mockRepo, ctrl
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestRegister_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		ExistsByEmailOrPhone(gomock.Any(), "test@example.com", "+6281234567890").
		Return(false, nil)

	mockRepo.EXPECT().
		CreateCredential(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cred *repository.Credential) (*repository.Credential, error) {
			cred.ID = "generated-uuid"
			return cred, nil
		})

	result, err := svc.Register(context.Background(), &RegisterRequest{
		Email:    "test@example.com",
		Phone:    "+6281234567890",
		Password: "Str0ngP4ss",
		Role:     "user",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UserID == "" {
		t.Error("expected a user ID")
	}
	if result.Email != "test@example.com" {
		t.Errorf("expected test@example.com, got %s", result.Email)
	}
	if result.PhoneVerified {
		t.Error("expected phone_verified to be false")
	}
}

func TestRegister_DuplicateAccount(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		ExistsByEmailOrPhone(gomock.Any(), "dup@example.com", "+6281234567890").
		Return(true, nil)

	_, err := svc.Register(context.Background(), &RegisterRequest{
		Email:    "dup@example.com",
		Phone:    "+6281234567890",
		Password: "Str0ngP4ss",
		Role:     "user",
	})

	if err != ErrDuplicateAccount {
		t.Errorf("expected ErrDuplicateAccount, got %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindByIdentifier(gomock.Any(), "test@example.com").
		Return(&repository.Credential{
			ID:            "user-123",
			Email:         "test@example.com",
			PasswordHash:  "hashed:correctpassword",
			Role:          "user",
			Status:        "active",
			PhoneVerified: true,
		}, nil)

	result, err := svc.Login(context.Background(), &LoginRequest{
		Identifier: "test@example.com",
		Password:   "correctpassword",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.UserID != "user-123" {
		t.Errorf("expected user-123, got %s", result.UserID)
	}
	if result.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %s", result.TokenType)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindByIdentifier(gomock.Any(), "test@example.com").
		Return(&repository.Credential{
			ID:            "user-123",
			PasswordHash:  "hashed:correctpassword",
			Status:        "active",
			PhoneVerified: true,
		}, nil)

	_, err := svc.Login(context.Background(), &LoginRequest{
		Identifier: "test@example.com",
		Password:   "wrongpassword",
	})

	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLogin_AccountNotVerified(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindByIdentifier(gomock.Any(), "test@example.com").
		Return(&repository.Credential{
			ID:            "user-123",
			PasswordHash:  "hashed:pass",
			Status:        "active",
			PhoneVerified: false,
		}, nil)

	_, err := svc.Login(context.Background(), &LoginRequest{
		Identifier: "test@example.com",
		Password:   "pass",
	})

	if err != ErrAccountNotVerified {
		t.Errorf("expected ErrAccountNotVerified, got %v", err)
	}
}

func TestLogin_AccountSuspended(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindByIdentifier(gomock.Any(), "test@example.com").
		Return(&repository.Credential{
			ID:            "user-123",
			PasswordHash:  "hashed:pass",
			Status:        "suspended",
			PhoneVerified: true,
		}, nil)

	_, err := svc.Login(context.Background(), &LoginRequest{
		Identifier: "test@example.com",
		Password:   "pass",
	})

	if err != ErrAccountSuspended {
		t.Errorf("expected ErrAccountSuspended, got %v", err)
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindByID(gomock.Any(), "user-123").
		Return(&repository.Credential{
			ID:           "user-123",
			PasswordHash: "hashed:currentpass",
		}, nil)

	err := svc.ChangePassword(context.Background(), "user-123", &ChangePasswordRequest{
		CurrentPassword: "wrongcurrent",
		NewPassword:     "NewStr0ng",
	})

	if err != ErrWrongPassword {
		t.Errorf("expected ErrWrongPassword, got %v", err)
	}
}

func TestChangePassword_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindByID(gomock.Any(), "user-123").
		Return(&repository.Credential{
			ID:           "user-123",
			PasswordHash: "hashed:currentpass",
		}, nil)

	mockRepo.EXPECT().
		UpdatePassword(gomock.Any(), "user-123", gomock.Any()).
		Return(nil)

	err := svc.ChangePassword(context.Background(), "user-123", &ChangePasswordRequest{
		CurrentPassword: "currentpass",
		NewPassword:     "NewStr0ng",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestForgotPassword_PhoneNotFound(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		ExistsByEmailOrPhone(gomock.Any(), "", "+6281234567890").
		Return(false, nil)

	_, err := svc.ForgotPassword(context.Background(), &ForgotPasswordRequest{
		Phone: "+6281234567890",
	})

	if err != ErrPhoneNotFound {
		t.Errorf("expected ErrPhoneNotFound, got %v", err)
	}
}

func TestRequestOTP_Success(t *testing.T) {
	svc, _, _ := newTestService(t)

	result, err := svc.RequestOTP(context.Background(), &OTPRequest{
		Phone: "+6281234567890",
		Type:  "login",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Phone != "+6281234567890" {
		t.Errorf("expected +6281234567890, got %s", result.Phone)
	}
	if result.Cooldown != 60 {
		t.Errorf("expected cooldown 60, got %d", result.Cooldown)
	}
}

func TestVerifyOTP_Success(t *testing.T) {
	svc, _, _ := newTestService(t)

	result, err := svc.VerifyOTP(context.Background(), &VerifyOTPRequest{
		Phone:   "+6281234567890",
		OTPCode: "123456",
		Type:    "registration",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Verified {
		t.Error("expected verified to be true")
	}
}

func TestLoginWithOTP_Success(t *testing.T) {
	svc, _, _ := newTestService(t)

	result, err := svc.LoginWithOTP(context.Background(), &OTPLoginRequest{
		Phone:   "+6281234567890",
		OTPCode: "123456",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %s", result.TokenType)
	}
}

func TestRefreshToken_Success(t *testing.T) {
	svc, _, _ := newTestService(t)

	result, err := svc.RefreshToken(context.Background(), &RefreshTokenRequest{
		RefreshToken: "some-refresh-token",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %s", result.TokenType)
	}
}

func TestLogout_Success(t *testing.T) {
	svc, _, _ := newTestService(t)

	err := svc.Logout(context.Background(), "user-123", &LogoutRequest{
		RefreshToken: "some-token",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLogoutAll_Success(t *testing.T) {
	svc, _, _ := newTestService(t)

	err := svc.LogoutAll(context.Background(), "user-123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListSessions_Success(t *testing.T) {
	svc, _, _ := newTestService(t)

	sessions, err := svc.ListSessions(context.Background(), "user-123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessions == nil {
		t.Error("expected non-nil sessions slice")
	}
}

func TestRevokeSession_Success(t *testing.T) {
	svc, _, _ := newTestService(t)

	err := svc.RevokeSession(context.Background(), "user-123", "session-456")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestForgotPassword_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		ExistsByEmailOrPhone(gomock.Any(), "", "+6281234567890").
		Return(true, nil)

	result, err := svc.ForgotPassword(context.Background(), &ForgotPasswordRequest{
		Phone: "+6281234567890",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Phone != "+6281234567890" {
		t.Errorf("expected +6281234567890, got %s", result.Phone)
	}
}

func TestResetPassword_Success(t *testing.T) {
	svc, _, _ := newTestService(t)

	err := svc.ResetPassword(context.Background(), &ResetPasswordRequest{
		Phone:       "+6281234567890",
		ResetToken:  "reset-token",
		NewPassword: "NewStr0ng",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
