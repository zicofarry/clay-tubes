// Package handler implements HTTP handlers for the Auth Service.
// Each method maps 1:1 to an OpenAPI endpoint.
package handler

import (
	"net/http"

	"github.com/zicofarry/clay-app/backend/services/auth-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/validator"
)

// AuthHandler holds references to the auth service layer.
type AuthHandler struct {
	svc service.AuthServiceInterface
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(svc service.AuthServiceInterface) *AuthHandler {
	return &AuthHandler{svc: svc}
}

// ── Registration ─────────────────────────────────────────────────────────────

// Register handles POST /auth/register
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req service.RegisterRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.Register(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusCreated, result)
}

// ── OTP ──────────────────────────────────────────────────────────────────────

// RequestOTP handles POST /auth/request-otp
func (h *AuthHandler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	var req service.OTPRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.RequestOTP(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// VerifyOTP handles POST /auth/verify-otp
func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req service.VerifyOTPRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.VerifyOTP(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// ── Login ────────────────────────────────────────────────────────────────────

// Login handles POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req service.LoginRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.Login(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// LoginWithOTP handles POST /auth/login/otp
func (h *AuthHandler) LoginWithOTP(w http.ResponseWriter, r *http.Request) {
	var req service.OTPLoginRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.LoginWithOTP(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// ── Token ────────────────────────────────────────────────────────────────────

// RefreshToken handles POST /auth/refresh-token
func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var req service.RefreshTokenRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.RefreshToken(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// ── Logout ───────────────────────────────────────────────────────────────────

// Logout handles POST /auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req service.LogoutRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	userID := r.Header.Get("X-User-ID")
	if err := h.svc.Logout(r.Context(), userID, &req); err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "logged out successfully"})
}

// LogoutAll handles POST /auth/logout-all
func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if err := h.svc.LogoutAll(r.Context(), userID); err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "all sessions revoked"})
}

// ── Sessions ─────────────────────────────────────────────────────────────────

// ListSessions handles GET /auth/sessions
func (h *AuthHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")

	sessions, err := h.svc.ListSessions(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, sessions)
}

// RevokeSession handles DELETE /auth/sessions/{sessionId}
func (h *AuthHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	sessionID := r.PathValue("sessionId")

	if err := h.svc.RevokeSession(r.Context(), userID, sessionID); err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "session revoked"})
}

// ── Password ─────────────────────────────────────────────────────────────────

// ForgotPassword handles POST /auth/password/forgot
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req service.ForgotPasswordRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.ForgotPassword(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// ResetPassword handles POST /auth/password/reset
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req service.ResetPasswordRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	if err := h.svc.ResetPassword(r.Context(), &req); err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "password reset successful"})
}

// ChangePassword handles PUT /auth/password/change
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req service.ChangePasswordRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	userID := r.Header.Get("X-User-ID")
	if err := h.svc.ChangePassword(r.Context(), userID, &req); err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "password changed"})
}

// ── Error Mapping ────────────────────────────────────────────────────────────

func handleServiceError(w http.ResponseWriter, err error) {
	svcErr, ok := err.(*service.ServiceError)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
		return
	}
	response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
}
