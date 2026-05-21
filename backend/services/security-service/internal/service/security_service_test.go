//go:build unit

package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/zicofarry/clay-app/backend/services/security-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/security-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/security-service/mocks/repomock"
	"go.uber.org/mock/gomock"
	"log/slog"
	"os"
)

func newTestService(t *testing.T) (*service.SecurityService, *repomock.MockSecurityRepositoryInterface) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	mockRepo := repomock.NewMockSecurityRepositoryInterface(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := service.NewSecurityService(mockRepo, logger)
	return svc, mockRepo
}

// ── RecordLoginAttempt ────────────────────────────────────────────────────────

func TestService_RecordLoginAttempt_Success(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	req := &service.RecordLoginAttemptRequest{
		UserID:    "user-1",
		IPAddress: "1.2.3.4",
		Success:   true,
	}
	mockRepo.EXPECT().
		InsertLoginAttempt(ctx, gomock.Any()).
		Return(&repository.LoginAttempt{ID: "attempt-1"}, nil)

	resp, err := svc.RecordLoginAttempt(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Recorded {
		t.Error("expected Recorded=true")
	}
	if resp.AutoFlagged {
		t.Error("expected AutoFlagged=false for successful login")
	}
}

func TestService_RecordLoginAttempt_AutoFlag(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	req := &service.RecordLoginAttemptRequest{
		UserID:        "user-1",
		IPAddress:     "1.2.3.4",
		Success:       false,
		FailureReason: "wrong_password",
	}
	mockRepo.EXPECT().
		InsertLoginAttempt(ctx, gomock.Any()).
		Return(&repository.LoginAttempt{ID: "attempt-1"}, nil)
	mockRepo.EXPECT().
		CountFailedLoginsInWindow(ctx, "user-1", gomock.Any()).
		Return(5, nil)
	mockRepo.EXPECT().
		InsertFraudFlag(ctx, gomock.Any()).
		Return(&repository.FraudFlag{ID: "flag-1", UserID: "user-1"}, nil)

	resp, err := svc.RecordLoginAttempt(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.AutoFlagged {
		t.Error("expected AutoFlagged=true when threshold reached")
	}
}

func TestService_RecordLoginAttempt_MissingFields(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, err := svc.RecordLoginAttempt(ctx, &service.RecordLoginAttemptRequest{})
	if err != service.ErrValidation {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

// ── CreateFraudFlag ───────────────────────────────────────────────────────────

func TestService_CreateFraudFlag_Success(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	now := time.Now()
	mockRepo.EXPECT().
		InsertFraudFlag(ctx, gomock.Any()).
		Return(&repository.FraudFlag{
			ID: "flag-1", UserID: "user-1",
			FlagType: "suspicious_login", Severity: "medium",
			Resolved: false, CreatedAt: now,
		}, nil)

	req := &service.CreateFraudFlagRequest{
		UserID:      "user-1",
		FlagType:    "suspicious_login",
		Severity:    "medium",
		Description: "Test description",
	}
	resp, err := svc.CreateFraudFlag(ctx, "admin-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "flag-1" {
		t.Errorf("want flag ID=flag-1, got %s", resp.ID)
	}
}

func TestService_CreateFraudFlag_InvalidSeverity(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	req := &service.CreateFraudFlagRequest{
		UserID:      "user-1",
		FlagType:    "test",
		Severity:    "extreme",
		Description: "test",
	}
	_, err := svc.CreateFraudFlag(ctx, "admin-1", req)
	if err == nil {
		t.Fatal("expected error for invalid severity")
	}
}

// ── GetFraudFlag ──────────────────────────────────────────────────────────────

func TestService_GetFraudFlag_Found(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	mockRepo.EXPECT().
		GetFraudFlagByID(ctx, "flag-1").
		Return(&repository.FraudFlag{
			ID: "flag-1", UserID: "user-1",
			FlagType: "chargeback", Severity: "critical",
			Resolved: false, CreatedAt: time.Now(),
		}, nil)

	resp, err := svc.GetFraudFlag(ctx, "flag-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "flag-1" {
		t.Errorf("want flag-1, got %s", resp.ID)
	}
}

func TestService_GetFraudFlag_NotFound(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	mockRepo.EXPECT().
		GetFraudFlagByID(ctx, "missing").
		Return(nil, sql.ErrNoRows)

	_, err := svc.GetFraudFlag(ctx, "missing")
	if err != service.ErrFlagNotFound {
		t.Errorf("want ErrFlagNotFound, got %v", err)
	}
}

// ── ResolveFraudFlag ──────────────────────────────────────────────────────────

func TestService_ResolveFraudFlag_Success(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	now := time.Now()
	mockRepo.EXPECT().
		GetFraudFlagByID(ctx, "flag-1").
		Return(&repository.FraudFlag{
			ID: "flag-1", UserID: "user-1", Severity: "medium",
			Resolved: false, CreatedAt: now,
		}, nil)
	mockRepo.EXPECT().
		ResolveFraudFlag(ctx, "flag-1", "admin-1", "false positive", gomock.Any()).
		Return(&repository.FraudFlag{
			ID: "flag-1", UserID: "user-1", Severity: "medium",
			Resolved: true,
			ResolvedBy: sql.NullString{String: "admin-1", Valid: true},
			ResolvedAt: sql.NullTime{Time: now, Valid: true},
			ResolutionNote: sql.NullString{String: "false positive", Valid: true},
			CreatedAt: now,
		}, nil)

	req := &service.ResolveFraudFlagRequest{ResolutionNote: "false positive"}
	resp, err := svc.ResolveFraudFlag(ctx, "flag-1", "admin-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Resolved {
		t.Error("expected resolved=true")
	}
}

func TestService_ResolveFraudFlag_AlreadyResolved(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	mockRepo.EXPECT().
		GetFraudFlagByID(ctx, "flag-1").
		Return(&repository.FraudFlag{
			ID: "flag-1", Resolved: true, CreatedAt: time.Now(),
		}, nil)

	req := &service.ResolveFraudFlagRequest{ResolutionNote: "note"}
	_, err := svc.ResolveFraudFlag(ctx, "flag-1", "admin-1", req)
	if err != service.ErrFlagAlreadyResolved {
		t.Errorf("want ErrFlagAlreadyResolved, got %v", err)
	}
}

// ── ValidateIP ────────────────────────────────────────────────────────────────

func TestService_ValidateIP_Allowed(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	mockRepo.EXPECT().
		GetActiveIPBlock(ctx, "1.2.3.4").
		Return(nil, sql.ErrNoRows)

	resp, err := svc.ValidateIP(ctx, &service.ValidateIPRequest{IPAddress: "1.2.3.4"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed {
		t.Error("expected allowed=true")
	}
}

func TestService_ValidateIP_Blocked(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	mockRepo.EXPECT().
		GetActiveIPBlock(ctx, "10.0.0.1").
		Return(&repository.IPBlacklistEntry{
			ID: "block-1", IPAddress: "10.0.0.1",
			Reason:   sql.NullString{String: "brute-force", Valid: true},
			IsActive: true,
		}, nil)

	resp, err := svc.ValidateIP(ctx, &service.ValidateIPRequest{IPAddress: "10.0.0.1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Allowed {
		t.Error("expected allowed=false")
	}
	if resp.Reason != "brute-force" {
		t.Errorf("want reason=brute-force, got %s", resp.Reason)
	}
}

// ── ValidateUser ──────────────────────────────────────────────────────────────

func TestService_ValidateUser_NoFlags(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	mockRepo.EXPECT().CountActiveFlagsForUser(ctx, "user-1").Return(0, nil)
	mockRepo.EXPECT().GetMaxSeverityForUser(ctx, "user-1").Return("", nil)

	resp, err := svc.ValidateUser(ctx, &service.ValidateUserRequest{UserID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Allowed {
		t.Error("expected allowed=true when no flags")
	}
}

func TestService_ValidateUser_CriticalFlag(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	mockRepo.EXPECT().CountActiveFlagsForUser(ctx, "user-1").Return(1, nil)
	mockRepo.EXPECT().GetMaxSeverityForUser(ctx, "user-1").Return("critical", nil)

	resp, err := svc.ValidateUser(ctx, &service.ValidateUserRequest{UserID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Allowed {
		t.Error("expected allowed=false for critical flag")
	}
}

// ── BlockIP ───────────────────────────────────────────────────────────────────

func TestService_BlockIP_Success(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	mockRepo.EXPECT().GetActiveIPBlock(ctx, "5.6.7.8").Return(nil, sql.ErrNoRows)
	mockRepo.EXPECT().
		InsertIPBlock(ctx, gomock.Any()).
		Return(&repository.IPBlacklistEntry{
			ID: "block-1", IPAddress: "5.6.7.8", IsActive: true, CreatedAt: time.Now(),
		}, nil)

	req := &service.BlockIPRequest{IPAddress: "5.6.7.8", Reason: "spam"}
	resp, err := svc.BlockIP(ctx, "admin-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "block-1" {
		t.Errorf("want block-1, got %s", resp.ID)
	}
}

func TestService_BlockIP_AlreadyBlocked(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	mockRepo.EXPECT().
		GetActiveIPBlock(ctx, "5.6.7.8").
		Return(&repository.IPBlacklistEntry{ID: "existing", IPAddress: "5.6.7.8", IsActive: true}, nil)

	req := &service.BlockIPRequest{IPAddress: "5.6.7.8", Reason: "spam"}
	_, err := svc.BlockIP(ctx, "admin-1", req)
	if err != service.ErrIPAlreadyBlocked {
		t.Errorf("want ErrIPAlreadyBlocked, got %v", err)
	}
}

// ── UnblockIP ─────────────────────────────────────────────────────────────────

func TestService_UnblockIP_Success(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	mockRepo.EXPECT().DeactivateIPBlock(ctx, "block-1").Return(nil)

	if err := svc.UnblockIP(ctx, "block-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_UnblockIP_NotFound(t *testing.T) {
	svc, mockRepo := newTestService(t)
	ctx := context.Background()

	mockRepo.EXPECT().DeactivateIPBlock(ctx, "missing").Return(sql.ErrNoRows)

	err := svc.UnblockIP(ctx, "missing")
	if err != service.ErrBlockNotFound {
		t.Errorf("want ErrBlockNotFound, got %v", err)
	}
}
