//go:build unit

package service

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/notification-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/notification-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

func newTestService(t *testing.T) (*NotificationService, *repomock.MockNotificationRepositoryInterface, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockNotificationRepositoryInterface(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewNotificationService(mockRepo, logger)
	return svc, mockRepo, ctrl
}

func TestRegisterDeviceToken_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		CreateDeviceToken(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, token *repository.DeviceToken) (*repository.DeviceToken, error) {
			token.ID = "tok-generated"
			token.IsActive = true
			return token, nil
		})

	result, err := svc.RegisterDeviceToken(context.Background(), "user-123", &RegisterDeviceTokenRequest{
		Token:    "fcm-token-abc",
		Platform: "android",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID == "" {
		t.Error("expected a token ID")
	}
	if result.UserID != "user-123" {
		t.Errorf("expected user-123, got %s", result.UserID)
	}
}

func TestDeactivateDeviceToken_NotFound(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		DeactivateDeviceToken(gomock.Any(), "tok-unknown", "user-123").
		Return(sql.ErrNoRows)

	err := svc.DeactivateDeviceToken(context.Background(), "user-123", "tok-unknown")

	if err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestUpdatePreferences_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		GetPreferences(gomock.Any(), "user-123").
		Return(nil, sql.ErrNoRows)

	mockRepo.EXPECT().
		UpsertPreferences(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, pref *repository.NotificationPreference) (*repository.NotificationPreference, error) {
			return pref, nil
		})

	pushEnabled := true
	smsEnabled := false
	result, err := svc.UpdatePreferences(context.Background(), "user-123", &UpdatePreferenceRequest{
		PushEnabled: &pushEnabled,
		SMSEnabled:  &smsEnabled,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.PushEnabled {
		t.Error("expected push_enabled to be true")
	}
	if result.SMSEnabled {
		t.Error("expected sms_enabled to be false")
	}
}

func TestCreateTemplate_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindTemplateByEventAndChannel(gomock.Any(), "Driver_Found", "push").
		Return(nil, sql.ErrNoRows)

	mockRepo.EXPECT().
		CreateTemplate(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tmpl *repository.NotificationTemplate) (*repository.NotificationTemplate, error) {
			tmpl.ID = "tmpl-generated"
			tmpl.IsActive = true
			return tmpl, nil
		})

	result, err := svc.CreateTemplate(context.Background(), &CreateTemplateRequest{
		EventType:    "Driver_Found",
		Channel:      "push",
		BodyTemplate: "{{driver_name}} akan tiba",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "tmpl-generated" {
		t.Errorf("expected tmpl-generated, got %s", result.ID)
	}
}

func TestCreateTemplate_Conflict(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindTemplateByEventAndChannel(gomock.Any(), "Driver_Found", "push").
		Return(&repository.NotificationTemplate{ID: "existing"}, nil)

	_, err := svc.CreateTemplate(context.Background(), &CreateTemplateRequest{
		EventType:    "Driver_Found",
		Channel:      "push",
		BodyTemplate: "duplicate",
	})

	if err != ErrTemplateConflict {
		t.Errorf("expected ErrTemplateConflict, got %v", err)
	}
}

func TestSendNotification_ChannelDisabled(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		GetPreferences(gomock.Any(), "user-123").
		Return(&repository.NotificationPreference{
			UserID:      "user-123",
			PushEnabled: false,
		}, nil)

	result, err := svc.SendNotification(context.Background(), &InternalSendRequest{
		RecipientID: "user-123",
		EventType:   "Order_Created",
		Channel:     "push",
		Priority:    "normal",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DeliveryStatus != "skipped" {
		t.Errorf("expected skipped, got %s", result.DeliveryStatus)
	}
}

func TestSendNotification_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		GetPreferences(gomock.Any(), "user-123").
		Return(&repository.NotificationPreference{
			UserID:      "user-123",
			PushEnabled: true,
		}, nil)

	title := "Driver ditemukan"
	mockRepo.EXPECT().
		FindTemplateByEventAndChannel(gomock.Any(), "Driver_Found", "push").
		Return(&repository.NotificationTemplate{
			ID:            "tmpl-1",
			TitleTemplate: &title,
			BodyTemplate:  "Driver {{name}} sedang menuju",
		}, nil)

	mockRepo.EXPECT().
		CreateNotificationLog(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, log *repository.NotificationLog) (*repository.NotificationLog, error) {
			log.ID = "notif-generated"
			return log, nil
		})

	result, err := svc.SendNotification(context.Background(), &InternalSendRequest{
		RecipientID: "user-123",
		EventType:   "Driver_Found",
		Channel:     "push",
		Priority:    "normal",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.DeliveryStatus != "sent" {
		t.Errorf("expected sent, got %s", result.DeliveryStatus)
	}
	if result.NotificationID != "notif-generated" {
		t.Errorf("expected notif-generated, got %s", result.NotificationID)
	}
}

func TestGetPreferences_DefaultsWhenNotFound(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		GetPreferences(gomock.Any(), "new-user").
		Return(nil, sql.ErrNoRows)

	result, err := svc.GetPreferences(context.Background(), "new-user")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.PushEnabled {
		t.Error("expected push_enabled default true")
	}
	if !result.EmailEnabled {
		t.Error("expected email_enabled default true")
	}
	if !result.SMSEnabled {
		t.Error("expected sms_enabled default true")
	}
}
