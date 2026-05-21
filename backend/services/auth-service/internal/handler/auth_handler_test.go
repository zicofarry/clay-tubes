//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/auth-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/auth-service/mocks"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.uber.org/mock/gomock"
)

func TestRegister_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		Register(gomock.Any(), gomock.Any()).
		Return(&service.RegisterResponse{
			UserID: "uuid-123",
			Email:  "test@example.com",
			Phone:  "+6281234567890",
			Role:   "user",
		}, nil)

	h := NewAuthHandler(mockSvc)

	body := `{"email":"test@example.com","phone":"+6281234567890","password":"Str0ngP4ss","role":"user"}`
	req := httptest.NewRequest("POST", "/auth/register", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var resp response.SuccessResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestRegister_DuplicateAccount(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		Register(gomock.Any(), gomock.Any()).
		Return(nil, service.ErrDuplicateAccount)

	h := NewAuthHandler(mockSvc)

	body := `{"email":"dup@example.com","phone":"+6281234567890","password":"Str0ngP4ss","role":"user"}`
	req := httptest.NewRequest("POST", "/auth/register", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestRegister_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	// No EXPECT — service should never be called for invalid JSON

	h := NewAuthHandler(mockSvc)

	req := httptest.NewRequest("POST", "/auth/register", strings.NewReader(`{invalid`))
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestLogin_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		Login(gomock.Any(), gomock.Any()).
		Return(&service.AuthTokenResponse{
			AccessToken: "jwt-token",
			TokenType:   "Bearer",
			ExpiresIn:   900,
			UserID:      "user-123",
			Role:        "user",
		}, nil)

	h := NewAuthHandler(mockSvc)

	body := `{"identifier":"test@example.com","password":"Str0ngP4ss"}`
	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		Login(gomock.Any(), gomock.Any()).
		Return(nil, service.ErrInvalidCredentials)

	h := NewAuthHandler(mockSvc)

	body := `{"identifier":"test@example.com","password":"wrong"}`
	req := httptest.NewRequest("POST", "/auth/login", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestRequestOTP_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		RequestOTP(gomock.Any(), gomock.Any()).
		Return(&service.OTPResponse{Phone: "+6281234567890", Cooldown: 60}, nil)

	h := NewAuthHandler(mockSvc)

	body := `{"phone":"+6281234567890","type":"login"}`
	req := httptest.NewRequest("POST", "/auth/request-otp", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.RequestOTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestLogout_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		Logout(gomock.Any(), "user-123", gomock.Any()).
		Return(nil)

	h := NewAuthHandler(mockSvc)

	body := `{"refresh_token":"some-token"}`
	req := httptest.NewRequest("POST", "/auth/logout", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestListSessions_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		ListSessions(gomock.Any(), "user-123").
		Return([]service.Session{}, nil)

	h := NewAuthHandler(mockSvc)

	req := httptest.NewRequest("GET", "/auth/sessions", nil)
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.ListSessions(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		ChangePassword(gomock.Any(), "user-123", gomock.Any()).
		Return(service.ErrWrongPassword)

	h := NewAuthHandler(mockSvc)

	body := `{"current_password":"wrong","new_password":"NewStr0ng"}`
	req := httptest.NewRequest("PUT", "/auth/password/change", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}

func TestVerifyOTP_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		VerifyOTP(gomock.Any(), gomock.Any()).
		Return(&service.VerifyOTPResponse{Verified: true}, nil)

	h := NewAuthHandler(mockSvc)

	body := `{"phone":"+6281234567890","otp_code":"123456","type":"registration"}`
	req := httptest.NewRequest("POST", "/auth/verify-otp", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.VerifyOTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestLoginWithOTP_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		LoginWithOTP(gomock.Any(), gomock.Any()).
		Return(&service.AuthTokenResponse{
			AccessToken: "jwt-token",
			TokenType:   "Bearer",
			ExpiresIn:   900,
		}, nil)

	h := NewAuthHandler(mockSvc)

	body := `{"phone":"+6281234567890","otp_code":"123456"}`
	req := httptest.NewRequest("POST", "/auth/login/otp", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.LoginWithOTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRefreshToken_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		RefreshToken(gomock.Any(), gomock.Any()).
		Return(&service.AuthTokenResponse{
			AccessToken: "new-jwt-token",
			TokenType:   "Bearer",
			ExpiresIn:   900,
		}, nil)

	h := NewAuthHandler(mockSvc)

	body := `{"refresh_token":"some-refresh-token"}`
	req := httptest.NewRequest("POST", "/auth/refresh-token", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.RefreshToken(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestLogoutAll_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		LogoutAll(gomock.Any(), "user-123").
		Return(nil)

	h := NewAuthHandler(mockSvc)

	req := httptest.NewRequest("POST", "/auth/logout-all", nil)
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.LogoutAll(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRevokeSession_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		RevokeSession(gomock.Any(), "user-123", "session-456").
		Return(nil)

	h := NewAuthHandler(mockSvc)

	req := httptest.NewRequest("DELETE", "/auth/sessions/session-456", nil)
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("sessionId", "session-456")
	w := httptest.NewRecorder()

	h.RevokeSession(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestForgotPassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		ForgotPassword(gomock.Any(), gomock.Any()).
		Return(&service.OTPResponse{Phone: "+6281234567890", Cooldown: 60}, nil)

	h := NewAuthHandler(mockSvc)

	body := `{"phone":"+6281234567890"}`
	req := httptest.NewRequest("POST", "/auth/password/forgot", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.ForgotPassword(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestResetPassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		ResetPassword(gomock.Any(), gomock.Any()).
		Return(nil)

	h := NewAuthHandler(mockSvc)

	body := `{"phone":"+6281234567890","reset_token":"token","new_password":"NewStr0ng"}`
	req := httptest.NewRequest("POST", "/auth/password/reset", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.ResetPassword(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestChangePassword_Success(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockSvc := mocks.NewMockAuthServiceInterface(ctrl)
	mockSvc.EXPECT().
		ChangePassword(gomock.Any(), "user-123", gomock.Any()).
		Return(nil)

	h := NewAuthHandler(mockSvc)

	body := `{"current_password":"current","new_password":"NewStr0ng"}`
	req := httptest.NewRequest("PUT", "/auth/password/change", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.ChangePassword(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
