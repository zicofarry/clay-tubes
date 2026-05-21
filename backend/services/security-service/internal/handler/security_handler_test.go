//go:build unit

package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zicofarry/clay-app/backend/services/security-service/internal/handler"
	"github.com/zicofarry/clay-app/backend/services/security-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/security-service/mocks"
	"go.uber.org/mock/gomock"
)

func newTestHandler(t *testing.T) (*handler.SecurityHandler, *mocks.MockSecurityServiceInterface) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	mockSvc := mocks.NewMockSecurityServiceInterface(ctrl)
	h := handler.NewSecurityHandler(mockSvc)
	return h, mockSvc
}

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewBuffer(b)
}

// ── RecordLoginAttempt ────────────────────────────────────────────────────────

func TestHandler_RecordLoginAttempt_Success(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	req := service.RecordLoginAttemptRequest{
		UserID:    "00000000-0000-0000-0000-000000000001",
		IPAddress: "103.28.14.52",
		Success:   false,
	}
	mockSvc.EXPECT().
		RecordLoginAttempt(gomock.Any(), gomock.Any()).
		Return(&service.RecordLoginAttemptResponse{Recorded: true, AutoFlagged: false}, nil)

	r := httptest.NewRequest(http.MethodPost, "/internal/login-attempts", jsonBody(t, req))
	w := httptest.NewRecorder()
	h.RecordLoginAttempt(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d", w.Code)
	}
}

func TestHandler_RecordLoginAttempt_InvalidBody(t *testing.T) {
	h, _ := newTestHandler(t)

	r := httptest.NewRequest(http.MethodPost, "/internal/login-attempts", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()
	h.RecordLoginAttempt(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ── ValidateIP ────────────────────────────────────────────────────────────────

func TestHandler_ValidateIP_Allowed(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		ValidateIP(gomock.Any(), gomock.Any()).
		Return(&service.ValidateIPResponse{Allowed: true}, nil)

	body := jsonBody(t, map[string]string{"ip_address": "1.2.3.4"})
	r := httptest.NewRequest(http.MethodPost, "/internal/validate/ip", body)
	w := httptest.NewRecorder()
	h.ValidateIP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestHandler_ValidateIP_Blocked(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		ValidateIP(gomock.Any(), gomock.Any()).
		Return(&service.ValidateIPResponse{Allowed: false, Reason: "brute-force"}, nil)

	body := jsonBody(t, map[string]string{"ip_address": "192.168.1.1"})
	r := httptest.NewRequest(http.MethodPost, "/internal/validate/ip", body)
	w := httptest.NewRecorder()
	h.ValidateIP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

// ── ValidateUser ──────────────────────────────────────────────────────────────

func TestHandler_ValidateUser_Allowed(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		ValidateUser(gomock.Any(), gomock.Any()).
		Return(&service.ValidateUserResponse{Allowed: true, ActiveFlags: 0}, nil)

	body := jsonBody(t, map[string]string{"user_id": "00000000-0000-0000-0000-000000000001"})
	r := httptest.NewRequest(http.MethodPost, "/internal/validate/user", body)
	w := httptest.NewRecorder()
	h.ValidateUser(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

// ── ListMyLoginAttempts ───────────────────────────────────────────────────────

func TestHandler_ListMyLoginAttempts(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		ListMyLoginAttempts(gomock.Any(), "user-123", gomock.Any()).
		Return(&service.LoginAttemptListResponse{
			Data:  []service.LoginAttemptResponse{},
			Total: 0, Page: 1, Limit: 20,
		}, nil)

	r := httptest.NewRequest(http.MethodGet, "/login-attempts", nil)
	r.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()
	h.ListMyLoginAttempts(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

// ── CreateFraudFlag ───────────────────────────────────────────────────────────

func TestHandler_CreateFraudFlag_Success(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	req := service.CreateFraudFlagRequest{
		UserID:      "00000000-0000-0000-0000-000000000001",
		FlagType:    "suspicious_login",
		Severity:    "medium",
		Description: "Test description",
	}
	now := time.Now()
	mockSvc.EXPECT().
		CreateFraudFlag(gomock.Any(), "admin-1", gomock.Any()).
		Return(&service.FraudFlagResponse{
			ID: "flag-1", UserID: req.UserID, FlagType: req.FlagType,
			Severity: req.Severity, CreatedAt: now,
		}, nil)

	r := httptest.NewRequest(http.MethodPost, "/admin/fraud-flags", jsonBody(t, req))
	r.Header.Set("X-User-ID", "admin-1")
	w := httptest.NewRecorder()
	h.CreateFraudFlag(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d", w.Code)
	}
}

func TestHandler_CreateFraudFlag_InvalidBody(t *testing.T) {
	h, _ := newTestHandler(t)

	r := httptest.NewRequest(http.MethodPost, "/admin/fraud-flags", bytes.NewBufferString("{bad"))
	w := httptest.NewRecorder()
	h.CreateFraudFlag(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ── GetFraudFlag ──────────────────────────────────────────────────────────────

func TestHandler_GetFraudFlag_NotFound(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		GetFraudFlag(gomock.Any(), "nonexistent-id").
		Return(nil, service.ErrFlagNotFound)

	r := httptest.NewRequest(http.MethodGet, "/admin/fraud-flags/nonexistent-id", nil)
	// Simulate path value via a custom request setup
	r = r.WithContext(r.Context())
	w := httptest.NewRecorder()

	// We call through a mux to test PathValue; use a simple wrapper here
	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin/fraud-flags/{flagId}", h.GetFraudFlag)
	r2 := httptest.NewRequest(http.MethodGet, "/admin/fraud-flags/nonexistent-id", nil)
	mux.ServeHTTP(w, r2)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

// ── BlockIP ───────────────────────────────────────────────────────────────────

func TestHandler_BlockIP_Success(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	req := service.BlockIPRequest{
		IPAddress: "10.0.0.1",
		Reason:    "brute-force",
	}
	now := time.Now()
	mockSvc.EXPECT().
		BlockIP(gomock.Any(), "admin-1", gomock.Any()).
		Return(&service.IPBlacklistResponse{
			ID: "block-1", IPAddress: req.IPAddress,
			IsActive: true, CreatedAt: now,
		}, nil)

	r := httptest.NewRequest(http.MethodPost, "/admin/ip-blacklist", jsonBody(t, req))
	r.Header.Set("X-User-ID", "admin-1")
	w := httptest.NewRecorder()
	h.BlockIP(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d", w.Code)
	}
}

func TestHandler_BlockIP_AlreadyBlocked(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		BlockIP(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, service.ErrIPAlreadyBlocked)

	req := service.BlockIPRequest{IPAddress: "10.0.0.1", Reason: "test"}
	r := httptest.NewRequest(http.MethodPost, "/admin/ip-blacklist", jsonBody(t, req))
	w := httptest.NewRecorder()
	h.BlockIP(w, r)

	if w.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", w.Code)
	}
}

// ── UnblockIP ─────────────────────────────────────────────────────────────────

func TestHandler_UnblockIP_Success(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		UnblockIP(gomock.Any(), "block-1").
		Return(nil)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /admin/ip-blacklist/{blockId}", h.UnblockIP)
	r := httptest.NewRequest(http.MethodDelete, "/admin/ip-blacklist/block-1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestHandler_UnblockIP_NotFound(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		UnblockIP(gomock.Any(), "missing").
		Return(service.ErrBlockNotFound)

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /admin/ip-blacklist/{blockId}", h.UnblockIP)
	r := httptest.NewRequest(http.MethodDelete, "/admin/ip-blacklist/missing", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}
