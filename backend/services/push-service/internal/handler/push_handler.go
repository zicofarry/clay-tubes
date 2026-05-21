// Package handler implements HTTP handlers for the Push Service.
// Each method maps 1:1 to an OpenAPI endpoint.
package handler

import (
	"net/http"

	"github.com/zicofarry/clay-app/backend/services/push-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/validator"
)

// PushHandler holds references to the push service layer.
type PushHandler struct {
	svc service.PushServiceInterface
}

// NewPushHandler creates a new PushHandler.
func NewPushHandler(svc service.PushServiceInterface) *PushHandler {
	return &PushHandler{svc: svc}
}

// ── Delivery ─────────────────────────────────────────────────────────────────

// SendPush handles POST /internal/push/send
func (h *PushHandler) SendPush(w http.ResponseWriter, r *http.Request) {
	var req service.SendPushRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.SendPush(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// SendBatchPush handles POST /internal/push/send-batch
func (h *PushHandler) SendBatchPush(w http.ResponseWriter, r *http.Request) {
	var req service.SendBatchPushRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.SendBatchPush(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// ── Topic ────────────────────────────────────────────────────────────────────

// SubscribeTopic handles POST /internal/push/topics/{topicName}/subscribe
func (h *PushHandler) SubscribeTopic(w http.ResponseWriter, r *http.Request) {
	topicName := r.PathValue("topicName")

	var req service.TopicSubscribeRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.SubscribeTopic(r.Context(), topicName, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// UnsubscribeTopic handles POST /internal/push/topics/{topicName}/unsubscribe
func (h *PushHandler) UnsubscribeTopic(w http.ResponseWriter, r *http.Request) {
	topicName := r.PathValue("topicName")

	var req service.TopicSubscribeRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.UnsubscribeTopic(r.Context(), topicName, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// SendTopicPush handles POST /internal/push/topics/{topicName}/send
func (h *PushHandler) SendTopicPush(w http.ResponseWriter, r *http.Request) {
	topicName := r.PathValue("topicName")

	var req service.SendTopicPushRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.SendTopicPush(r.Context(), topicName, &req)
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
