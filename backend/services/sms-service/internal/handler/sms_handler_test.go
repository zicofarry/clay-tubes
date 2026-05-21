//go:build unit

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zicofarry/clay-app/backend/services/sms-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/sms-service/mocks"
	"go.uber.org/mock/gomock"
)

func TestSMSHandler_SendOTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockSMSServiceInterface(ctrl)
	handler := NewSMSHandler(mockSvc)

	reqBody := service.SendOTPRequest{
		Phone:   "+628123456789",
		Purpose: "login",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/internal/sms/otp/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	respBody := &service.SendOTPResponse{
		Phone:                 reqBody.Phone,
		ExpiresAt:             time.Now().Add(5 * time.Minute),
		ResendCooldownSeconds: 60,
	}

	mockSvc.EXPECT().SendOTP(gomock.Any(), reqBody).Return(respBody, nil)

	handler.SendOTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", res.StatusCode)
	}
}

func TestSMSHandler_VerifyOTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockSMSServiceInterface(ctrl)
	handler := NewSMSHandler(mockSvc)

	reqBody := service.VerifyOTPRequest{
		Phone:   "+628123456789",
		OTPCode: "123456",
		Purpose: "login",
	}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/internal/sms/otp/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	respBody := &service.VerifyOTPResponse{
		Valid: true,
		Phone: reqBody.Phone,
	}

	mockSvc.EXPECT().VerifyOTP(gomock.Any(), reqBody).Return(respBody, nil)

	handler.VerifyOTP(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", res.StatusCode)
	}
}
