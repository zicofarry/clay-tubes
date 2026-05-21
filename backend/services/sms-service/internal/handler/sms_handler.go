package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/zicofarry/clay-app/backend/services/sms-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/sms-service/internal/service"
)

type SMSHandler struct {
	svc service.SMSServiceInterface
}

func NewSMSHandler(svc service.SMSServiceInterface) *SMSHandler {
	return &SMSHandler{
		svc: svc,
	}
}

// writeError writes a JSON error response
func writeError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"code":    code,
		"message": message,
	})
}

func (h *SMSHandler) SendOTP(w http.ResponseWriter, r *http.Request) {
	var req service.SendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	if req.Phone == "" || req.Purpose == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "phone and purpose are required")
		return
	}

	resp, err := h.svc.SendOTP(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrRateLimitExceeded) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to send OTP")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *SMSHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req service.VerifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	resp, err := h.svc.VerifyOTP(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidOTP) {
			writeError(w, http.StatusBadRequest, "INVALID_OTP", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to verify OTP")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (h *SMSHandler) SendSMS(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "MISSING_HEADER", "Idempotency-Key is required")
		return
	}

	var req service.SendSMSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	resp, err := h.svc.SendSMS(r.Context(), req, idempotencyKey)
	if err != nil {
		if errors.Is(err, service.ErrRateLimitExceeded) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to send SMS")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}

func (h *SMSHandler) ProcessWebhook(w http.ResponseWriter, r *http.Request) {
	var payload service.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid payload")
		return
	}

	err := h.svc.ProcessWebhook(r.Context(), payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to process webhook")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *SMSHandler) GetSMSStatus(w http.ResponseWriter, r *http.Request) {
	smsID := r.PathValue("smsId")
	if smsID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "smsId path parameter is required")
		return
	}

	resp, err := h.svc.GetSMSStatus(r.Context(), smsID)
	if err != nil {
		if errors.Is(err, repository.ErrSMSNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "SMS not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get SMS status")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
