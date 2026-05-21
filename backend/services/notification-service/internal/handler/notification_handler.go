// Package handler implements HTTP handlers for the Notification Service.
package handler

import (
	"net/http"
	"strconv"

	"github.com/zicofarry/clay-app/backend/services/notification-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/validator"
)

type NotificationHandler struct {
	svc service.NotificationServiceInterface
}

func NewNotificationHandler(svc service.NotificationServiceInterface) *NotificationHandler {
	return &NotificationHandler{svc: svc}
}

// ── Device Token ─────────────────────────────────────────────────────────────

func (h *NotificationHandler) RegisterDeviceToken(w http.ResponseWriter, r *http.Request) {
	var req service.RegisterDeviceTokenRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	userID := r.Header.Get("X-User-ID")
	result, err := h.svc.RegisterDeviceToken(r.Context(), userID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *NotificationHandler) ListDeviceTokens(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	activeOnly := r.URL.Query().Get("is_active") == "true"
	result, err := h.svc.ListDeviceTokens(r.Context(), userID, activeOnly)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *NotificationHandler) DeactivateDeviceToken(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	tokenID := r.PathValue("tokenId")
	if err := h.svc.DeactivateDeviceToken(r.Context(), userID, tokenID); err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]string{"message": "device token deactivated"})
}

// ── Preference ───────────────────────────────────────────────────────────────

func (h *NotificationHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	result, err := h.svc.GetPreferences(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *NotificationHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	var req service.UpdatePreferenceRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	userID := r.Header.Get("X-User-ID")
	result, err := h.svc.UpdatePreferences(r.Context(), userID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Notification ─────────────────────────────────────────────────────────────

func (h *NotificationHandler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 50 {
		limit = 20
	}
	result, total, err := h.svc.ListNotifications(r.Context(), userID, page, limit)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	totalPages := (total + limit - 1) / limit
	response.Success(w, http.StatusOK, map[string]interface{}{
		"data": result,
		"meta": map[string]int{"page": page, "limit": limit, "total": total, "total_pages": totalPages},
	})
}

func (h *NotificationHandler) GetNotification(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	notificationID := r.PathValue("notificationId")
	result, err := h.svc.GetNotification(r.Context(), userID, notificationID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Template (Admin) ─────────────────────────────────────────────────────────

func (h *NotificationHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	result, err := h.svc.ListTemplates(r.Context())
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *NotificationHandler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req service.CreateTemplateRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.CreateTemplate(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, result)
}

func (h *NotificationHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	templateID := r.PathValue("templateId")
	result, err := h.svc.GetTemplate(r.Context(), templateID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *NotificationHandler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	var req service.UpdateTemplateRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	templateID := r.PathValue("templateId")
	result, err := h.svc.UpdateTemplate(r.Context(), templateID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *NotificationHandler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	templateID := r.PathValue("templateId")
	if err := h.svc.DeleteTemplate(r.Context(), templateID); err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]string{"message": "template deactivated"})
}

func (h *NotificationHandler) PreviewTemplate(w http.ResponseWriter, r *http.Request) {
	var req service.PreviewTemplateRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	templateID := r.PathValue("templateId")
	result, err := h.svc.PreviewTemplate(r.Context(), templateID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Internal ─────────────────────────────────────────────────────────────────

func (h *NotificationHandler) InternalSendNotification(w http.ResponseWriter, r *http.Request) {
	var req service.InternalSendRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.SendNotification(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *NotificationHandler) InternalSendBatch(w http.ResponseWriter, r *http.Request) {
	var req service.InternalSendBatchRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.SendBatchNotification(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *NotificationHandler) InternalGetDeviceTokens(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	result, err := h.svc.ListDeviceTokens(r.Context(), userID, true) // Only active tokens
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
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
