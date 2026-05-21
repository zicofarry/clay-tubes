package handler

import (
	"encoding/json"
	"net/http"

	"github.com/zicofarry/clay-app/backend/services/email-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/email-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
)

type EmailHandler struct {
	svc service.EmailService
}

func NewEmailHandler(svc service.EmailService) *EmailHandler {
	return &EmailHandler{svc: svc}
}

func (h *EmailHandler) SendEmail(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		response.Error(w, http.StatusBadRequest, "invalid_request", "Idempotency-Key header is required")
		return
	}

	var req model.SendEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	// Validate
	if req.To == "" || req.TemplateId == "" {
		response.Error(w, http.StatusBadRequest, "invalid_request", "to and template_id are required")
		return
	}

	res, err := h.svc.SendEmail(r.Context(), idempotencyKey, req)
	if err != nil {
		if err == service.ErrTemplateNotFound {
			response.Error(w, http.StatusBadRequest, "invalid_request", "template not found")
			return
		}
		if err == service.ErrRateLimitExceeded {
			response.Error(w, http.StatusTooManyRequests, "rate_limit_exceeded", "rate limit exceeded for recipient")
			return
		}
		response.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(res)
}

func (h *EmailHandler) GetEmailStatus(w http.ResponseWriter, r *http.Request) {
	emailId := r.PathValue("emailId")
	if emailId == "" {
		response.Error(w, http.StatusBadRequest, "invalid_request", "emailId is required")
		return
	}

	res, err := h.svc.GetEmailStatus(r.Context(), emailId)
	if err != nil {
		response.Error(w, http.StatusNotFound, "not_found", "email log not found")
		return
	}

	response.Success(w, http.StatusOK, res)
}

func (h *EmailHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_request", "invalid webhook payload")
		return
	}

	err := h.svc.HandleWebhook(r.Context(), payload)
	if err != nil {
		// Even if error, return 200 for webhook to prevent retries if it's our fault
		response.Success(w, http.StatusOK, model.SuccessResponse{Success: false})
		return
	}

	response.Success(w, http.StatusOK, model.SuccessResponse{Success: true})
}

func (h *EmailHandler) GetTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.svc.GetTemplates(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	response.Success(w, http.StatusOK, model.TemplateListResponse{Templates: templates})
}

func (h *EmailHandler) UpsertTemplate(w http.ResponseWriter, r *http.Request) {
	var req model.UpsertTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	if req.TemplateId == "" || req.Subject == "" || req.BodyHtml == "" {
		response.Error(w, http.StatusBadRequest, "invalid_request", "template_id, subject, and body_html are required")
		return
	}

	res, err := h.svc.UpsertTemplate(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	response.Success(w, http.StatusOK, res)
}
