// Package handler implements HTTP handlers for the Matching Service.
// Each method maps 1:1 to an OpenAPI endpoint.
package handler

import (
	"net/http"

	"github.com/zicofarry/clay-app/backend/services/matching-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/validator"
)

// MatchingHandler holds references to the service layer.
type MatchingHandler struct {
	svc service.MatchingServiceInterface
}

// NewMatchingHandler creates a new MatchingHandler.
func NewMatchingHandler(svc service.MatchingServiceInterface) *MatchingHandler {
	return &MatchingHandler{svc: svc}
}

// ── Driver-facing (Dispatcher) ────────────────────────────────────────────

// GoOnline handles POST /dispatcher/go-online
func (h *MatchingHandler) GoOnline(w http.ResponseWriter, r *http.Request) {
	var req service.GoOnlineRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	driverID := r.Header.Get("X-User-ID")
	result, err := h.svc.GoOnline(r.Context(), driverID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// GoOffline handles POST /dispatcher/go-offline
func (h *MatchingHandler) GoOffline(w http.ResponseWriter, r *http.Request) {
	driverID := r.Header.Get("X-User-ID")
	result, err := h.svc.GoOffline(r.Context(), driverID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// UpdateLocation handles PUT /dispatcher/location
func (h *MatchingHandler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	var req service.LocationUpdateRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	driverID := r.Header.Get("X-User-ID")
	if err := h.svc.UpdateLocation(r.Context(), driverID, &req); err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "location updated",
	})
}

// Heartbeat handles POST /dispatcher/heartbeat
func (h *MatchingHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	driverID := r.Header.Get("X-User-ID")
	result, err := h.svc.Heartbeat(r.Context(), driverID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// Respond handles POST /dispatcher/respond
func (h *MatchingHandler) Respond(w http.ResponseWriter, r *http.Request) {
	var req service.OfferResponseRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	driverID := r.Header.Get("X-User-ID")
	if err := h.svc.Respond(r.Context(), driverID, &req); err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "response recorded",
	})
}

// SetMode handles PUT /dispatcher/mode
func (h *MatchingHandler) SetMode(w http.ResponseWriter, r *http.Request) {
	var req service.SetDispatchModeRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	driverID := r.Header.Get("X-User-ID")
	result, err := h.svc.SetMode(r.Context(), driverID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// GetFullStatus handles GET /dispatcher/status
func (h *MatchingHandler) GetFullStatus(w http.ResponseWriter, r *http.Request) {
	driverID := r.Header.Get("X-User-ID")
	result, err := h.svc.GetFullStatus(r.Context(), driverID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// GetTodayEarnings handles GET /dispatcher/earnings/today
func (h *MatchingHandler) GetTodayEarnings(w http.ResponseWriter, r *http.Request) {
	driverID := r.Header.Get("X-User-ID")
	result, err := h.svc.GetTodayEarnings(r.Context(), driverID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Internal (service-to-service) ─────────────────────────────────────────

// StartDispatch handles POST /internal/dispatcher/dispatch
func (h *MatchingHandler) StartDispatch(w http.ResponseWriter, r *http.Request) {
	var req service.DispatchRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.StartDispatch(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusAccepted, result)
}

// CancelDispatch handles POST /internal/dispatcher/cancel
func (h *MatchingHandler) CancelDispatch(w http.ResponseWriter, r *http.Request) {
	var req service.CancelDispatchRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	if err := h.svc.CancelDispatch(r.Context(), &req); err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "dispatch cancelled",
	})
}

// NearbyActiveDrivers handles GET /internal/dispatcher/nearby-drivers
func (h *MatchingHandler) NearbyActiveDrivers(w http.ResponseWriter, r *http.Request) {
	q := service.NearbyDriversQuery{
		Lat:         validator.QueryFloat64(r, "lat", 0),
		Lng:         validator.QueryFloat64(r, "lng", 0),
		RadiusKm:    validator.QueryFloat64(r, "radius_km", 5.0),
		ServiceType: validator.QueryString(r, "service_type", ""),
		Limit:       validator.QueryInt(r, "limit", 20),
	}
	result, err := h.svc.NearbyActiveDrivers(r.Context(), q)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// GetSession handles GET /internal/dispatcher/order/{orderId}/status
func (h *MatchingHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	result, err := h.svc.GetSession(r.Context(), orderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// GetZoneStats handles GET /internal/dispatcher/zone/{zoneId}/stats
func (h *MatchingHandler) GetZoneStats(w http.ResponseWriter, r *http.Request) {
	zoneID := r.PathValue("zoneId")
	vehicleType := validator.QueryString(r, "vehicle_type", "motor")
	result, err := h.svc.GetZoneStats(r.Context(), vehicleType, zoneID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// FreeDriver handles PUT /internal/drivers/{driverId}/free
func (h *MatchingHandler) FreeDriver(w http.ResponseWriter, r *http.Request) {
	driverID := r.PathValue("driverId")
	var req service.FreeDriverRequest
	if r.ContentLength > 0 {
		if err := validator.DecodeJSON(r, &req); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
	}
	if err := h.svc.FreeDriver(r.Context(), driverID, &req); err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "driver freed",
	})
}

// ── Error Mapping ─────────────────────────────────────────────────────────

func handleServiceError(w http.ResponseWriter, err error) {
	svcErr, ok := err.(*service.ServiceError)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred")
		return
	}
	response.Error(w, svcErr.StatusCode, svcErr.Code, svcErr.Message)
}
