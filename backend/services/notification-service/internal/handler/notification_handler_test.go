//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/notification-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/notification-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/notification-service/mocks"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.uber.org/mock/gomock"
)

func TestRegisterDeviceToken_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockNotificationServiceInterface(ctrl)
	mockSvc.EXPECT().
		RegisterDeviceToken(gomock.Any(), "user-123", gomock.Any()).
		Return(&repository.DeviceToken{
			ID:       "tok-uuid-123",
			UserID:   "user-123",
			Token:    "fcm-abc",
			Platform: "android",
			IsActive: true,
		}, nil)

	h := NewNotificationHandler(mockSvc)

	body := `{"token":"fcm-abc","platform":"android","app_version":"2.4.1"}`
	req := httptest.NewRequest("POST", "/device-tokens", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.RegisterDeviceToken(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp response.SuccessResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestRegisterDeviceToken_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockNotificationServiceInterface(ctrl)

	h := NewNotificationHandler(mockSvc)

	req := httptest.NewRequest("POST", "/device-tokens", strings.NewReader(`{invalid`))
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.RegisterDeviceToken(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetPreferences_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockNotificationServiceInterface(ctrl)
	mockSvc.EXPECT().
		GetPreferences(gomock.Any(), "user-123").
		Return(&repository.NotificationPreference{
			UserID:       "user-123",
			PushEnabled:  true,
			EmailEnabled: true,
			SMSEnabled:   false,
		}, nil)

	h := NewNotificationHandler(mockSvc)

	req := httptest.NewRequest("GET", "/preferences", nil)
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.GetPreferences(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCreateTemplate_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockNotificationServiceInterface(ctrl)
	mockSvc.EXPECT().
		CreateTemplate(gomock.Any(), gomock.Any()).
		Return(&repository.NotificationTemplate{
			ID:           "tmpl-uuid-123",
			EventType:    "Driver_Found",
			Channel:      "push",
			BodyTemplate: "{{driver_name}} akan tiba",
			IsActive:     true,
		}, nil)

	h := NewNotificationHandler(mockSvc)

	body := `{"event_type":"Driver_Found","channel":"push","body_template":"{{driver_name}} akan tiba"}`
	req := httptest.NewRequest("POST", "/admin/templates", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.CreateTemplate(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestCreateTemplate_Conflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockNotificationServiceInterface(ctrl)
	mockSvc.EXPECT().
		CreateTemplate(gomock.Any(), gomock.Any()).
		Return(nil, service.ErrTemplateConflict)

	h := NewNotificationHandler(mockSvc)

	body := `{"event_type":"Driver_Found","channel":"push","body_template":"duplicate"}`
	req := httptest.NewRequest("POST", "/admin/templates", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.CreateTemplate(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestListNotifications_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockNotificationServiceInterface(ctrl)
	mockSvc.EXPECT().
		ListNotifications(gomock.Any(), "user-123", 1, 20).
		Return([]repository.NotificationLog{}, 0, nil)

	h := NewNotificationHandler(mockSvc)

	req := httptest.NewRequest("GET", "/notifications", nil)
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.ListNotifications(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDeactivateDeviceToken_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockNotificationServiceInterface(ctrl)
	mockSvc.EXPECT().
		DeactivateDeviceToken(gomock.Any(), "user-123", "tok-unknown").
		Return(service.ErrTokenNotFound)

	h := NewNotificationHandler(mockSvc)

	req := httptest.NewRequest("DELETE", "/device-tokens/tok-unknown", nil)
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("tokenId", "tok-unknown")
	w := httptest.NewRecorder()

	h.DeactivateDeviceToken(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
