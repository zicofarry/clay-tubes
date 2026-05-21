package handler

import (
	"net/http"

	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/validator"
	"github.com/zicofarry/clay-app/backend/services/tracking-service/internal/service"
)

type TrackingHandler struct {
	svc service.TrackingServiceInterface
}

func NewTrackingHandler(svc service.TrackingServiceInterface) *TrackingHandler {
	return &TrackingHandler{svc: svc}
}

// ── Tracking ─────────────────────────────────────────────────────────────────

func (h *TrackingHandler) GetOrderPosition(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	if orderID == "" {
		response.Error(w, http.StatusBadRequest, "INVALID_REQUEST", "orderId path parameter is required")
		return
	}

	pos, err := h.svc.GetOrderPosition(r.Context(), orderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, pos)
}

func (h *TrackingHandler) GetOrderETA(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	
	eta, err := h.svc.GetOrderETA(r.Context(), orderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, eta)
}

func (h *TrackingHandler) GetOrderRoute(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	
	route, err := h.svc.GetOrderRoute(r.Context(), orderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, route)
}

// ── Route History ────────────────────────────────────────────────────────────

func (h *TrackingHandler) GetTripRoute(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	
	route, err := h.svc.GetTripRoute(r.Context(), orderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, route)
}

// ── Internal ─────────────────────────────────────────────────────────────────

func (h *TrackingHandler) StartTracking(w http.ResponseWriter, r *http.Request) {
	var req service.StartTrackingRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	if err := h.svc.StartTracking(r.Context(), &req); err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusCreated, map[string]bool{"success": true})
}

func (h *TrackingHandler) StopTracking(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	
	if err := h.svc.StopTracking(r.Context(), orderID); err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *TrackingHandler) PushLocationUpdate(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	var req service.LocationUpdateEvent
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	if err := h.svc.PushLocationUpdate(r.Context(), orderID, &req); err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, map[string]bool{"success": true})
}

// ── Error Mapping ────────────────────────────────────────────────────────────

func handleServiceError(w http.ResponseWriter, err error) {
	svcErr, ok := err.(*service.ServiceError)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
}
