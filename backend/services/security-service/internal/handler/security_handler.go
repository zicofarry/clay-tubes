// Package handler implements HTTP handlers for the Security Service.
// Each method maps 1:1 to an OpenAPI endpoint.
package handler

import (
	"net/http"

	"github.com/zicofarry/clay-app/backend/services/security-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/validator"
)

// SecurityHandler holds references to the service layer.
type SecurityHandler struct {
	svc service.SecurityServiceInterface
}

// NewSecurityHandler creates a new SecurityHandler.
func NewSecurityHandler(svc service.SecurityServiceInterface) *SecurityHandler {
	return &SecurityHandler{svc: svc}
}

// ── Login Attempts ────────────────────────────────────────────────────────────

// ListMyLoginAttempts handles GET /login-attempts
func (h *SecurityHandler) ListMyLoginAttempts(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	q := service.LoginAttemptQuery{
		From:  validator.QueryString(r, "from", ""),
		To:    validator.QueryString(r, "to", ""),
		Page:  validator.QueryInt(r, "page", 1),
		Limit: validator.QueryInt(r, "limit", 20),
	}
	if s := r.URL.Query().Get("success"); s != "" {
		b := s == "true"
		q.Success = &b
	}

	result, err := h.svc.ListMyLoginAttempts(r.Context(), userID, q)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// AdminListLoginAttempts handles GET /admin/login-attempts
func (h *SecurityHandler) AdminListLoginAttempts(w http.ResponseWriter, r *http.Request) {
	q := service.LoginAttemptQuery{
		UserID:    validator.QueryString(r, "user_id", ""),
		IPAddress: validator.QueryString(r, "ip_address", ""),
		From:      validator.QueryString(r, "from", ""),
		To:        validator.QueryString(r, "to", ""),
		Page:      validator.QueryInt(r, "page", 1),
		Limit:     validator.QueryInt(r, "limit", 20),
	}
	if s := r.URL.Query().Get("success"); s != "" {
		b := s == "true"
		q.Success = &b
	}

	result, err := h.svc.AdminListLoginAttempts(r.Context(), q)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Fraud Flags ───────────────────────────────────────────────────────────────

// ListFraudFlags handles GET /admin/fraud-flags
func (h *SecurityHandler) ListFraudFlags(w http.ResponseWriter, r *http.Request) {
	q := service.FraudFlagQuery{
		UserID:   validator.QueryString(r, "user_id", ""),
		Severity: validator.QueryString(r, "severity", ""),
		FlagType: validator.QueryString(r, "flag_type", ""),
		Page:     validator.QueryInt(r, "page", 1),
		Limit:    validator.QueryInt(r, "limit", 20),
	}
	if s := r.URL.Query().Get("resolved"); s != "" {
		b := s == "true"
		q.Resolved = &b
	}

	result, err := h.svc.ListFraudFlags(r.Context(), q)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// CreateFraudFlag handles POST /admin/fraud-flags
func (h *SecurityHandler) CreateFraudFlag(w http.ResponseWriter, r *http.Request) {
	var req service.CreateFraudFlagRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	adminID := r.Header.Get("X-User-ID")
	result, err := h.svc.CreateFraudFlag(r.Context(), adminID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, result)
}

// GetFraudFlag handles GET /admin/fraud-flags/{flagId}
func (h *SecurityHandler) GetFraudFlag(w http.ResponseWriter, r *http.Request) {
	flagID := r.PathValue("flagId")
	result, err := h.svc.GetFraudFlag(r.Context(), flagID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ResolveFraudFlag handles POST /admin/fraud-flags/{flagId}/resolve
func (h *SecurityHandler) ResolveFraudFlag(w http.ResponseWriter, r *http.Request) {
	var req service.ResolveFraudFlagRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	flagID := r.PathValue("flagId")
	adminID := r.Header.Get("X-User-ID")
	result, err := h.svc.ResolveFraudFlag(r.Context(), flagID, adminID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// GetUserFraudSummary handles GET /admin/users/{userId}/fraud-summary
func (h *SecurityHandler) GetUserFraudSummary(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	result, err := h.svc.GetUserFraudSummary(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── IP Blacklist ──────────────────────────────────────────────────────────────

// ListBlockedIPs handles GET /admin/ip-blacklist
func (h *SecurityHandler) ListBlockedIPs(w http.ResponseWriter, r *http.Request) {
	activeOnly := true
	if s := r.URL.Query().Get("active_only"); s == "false" {
		activeOnly = false
	}
	q := service.IPBlacklistQuery{
		Query:      validator.QueryString(r, "q", ""),
		ActiveOnly: activeOnly,
		Page:       validator.QueryInt(r, "page", 1),
		Limit:      validator.QueryInt(r, "limit", 20),
	}

	result, err := h.svc.ListBlockedIPs(r.Context(), q)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// BlockIP handles POST /admin/ip-blacklist
func (h *SecurityHandler) BlockIP(w http.ResponseWriter, r *http.Request) {
	var req service.BlockIPRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	adminID := r.Header.Get("X-User-ID")
	result, err := h.svc.BlockIP(r.Context(), adminID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, result)
}

// UnblockIP handles DELETE /admin/ip-blacklist/{blockId}
func (h *SecurityHandler) UnblockIP(w http.ResponseWriter, r *http.Request) {
	blockID := r.PathValue("blockId")
	if err := h.svc.UnblockIP(r.Context(), blockID); err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]bool{"success": true})
}

// ── Internal / Validation ─────────────────────────────────────────────────────

// ValidateIP handles POST /internal/validate/ip
func (h *SecurityHandler) ValidateIP(w http.ResponseWriter, r *http.Request) {
	var req service.ValidateIPRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.ValidateIP(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ValidateUser handles POST /internal/validate/user
func (h *SecurityHandler) ValidateUser(w http.ResponseWriter, r *http.Request) {
	var req service.ValidateUserRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.ValidateUser(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// RecordLoginAttempt handles POST /internal/login-attempts
func (h *SecurityHandler) RecordLoginAttempt(w http.ResponseWriter, r *http.Request) {
	var req service.RecordLoginAttemptRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.RecordLoginAttempt(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, result)
}

// ── Error Mapping ─────────────────────────────────────────────────────────────

func handleServiceError(w http.ResponseWriter, err error) {
	svcErr, ok := err.(*service.ServiceError)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
		return
	}
	response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
}
