// Package handler implements HTTP handlers for the Geo Service.
package handler

import (
	"net/http"
	"strconv"

	"github.com/zicofarry/clay-app/backend/services/geo-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/validator"
)

type GeoHandler struct {
	svc service.GeoServiceInterface
}

func NewGeoHandler(svc service.GeoServiceInterface) *GeoHandler {
	return &GeoHandler{svc: svc}
}

// ── Location ─────────────────────────────────────────────────────────────────

func (h *GeoHandler) UpdateDriverLocation(w http.ResponseWriter, r *http.Request) {
	var req service.UpdateLocationRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	driverID := r.PathValue("driverId")
	if err := h.svc.UpdateDriverLocation(r.Context(), driverID, &req); err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]bool{"success": true})
}

func (h *GeoHandler) GetDriverLocation(w http.ResponseWriter, r *http.Request) {
	driverID := r.PathValue("driverId")
	result, err := h.svc.GetDriverLocation(r.Context(), driverID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *GeoHandler) FindNearbyDrivers(w http.ResponseWriter, r *http.Request) {
	lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lng, _ := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
	radiusKm, _ := strconv.ParseFloat(r.URL.Query().Get("radius_km"), 64)
	if radiusKm == 0 {
		radiusKm = 5.0
	}
	serviceType := r.URL.Query().Get("service_type")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 20
	}
	result, err := h.svc.FindNearbyDrivers(r.Context(), lat, lng, radiusKm, serviceType, limit)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Maps ─────────────────────────────────────────────────────────────────────

func (h *GeoHandler) EstimateRoute(w http.ResponseWriter, r *http.Request) {
	var req service.RouteEstimateRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.EstimateRoute(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *GeoHandler) GetPolyline(w http.ResponseWriter, r *http.Request) {
	oLat, _ := strconv.ParseFloat(r.URL.Query().Get("origin_lat"), 64)
	oLng, _ := strconv.ParseFloat(r.URL.Query().Get("origin_lng"), 64)
	dLat, _ := strconv.ParseFloat(r.URL.Query().Get("dest_lat"), 64)
	dLng, _ := strconv.ParseFloat(r.URL.Query().Get("dest_lng"), 64)
	result, err := h.svc.GetPolyline(r.Context(), oLat, oLng, dLat, dLng)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *GeoHandler) GetRouting(w http.ResponseWriter, r *http.Request) {
	oLat, _ := strconv.ParseFloat(r.URL.Query().Get("origin_lat"), 64)
	oLng, _ := strconv.ParseFloat(r.URL.Query().Get("origin_lng"), 64)
	dLat, _ := strconv.ParseFloat(r.URL.Query().Get("dest_lat"), 64)
	dLng, _ := strconv.ParseFloat(r.URL.Query().Get("dest_lng"), 64)
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "driving"
	}
	result, err := h.svc.GetRouting(r.Context(), oLat, oLng, dLat, dLng, mode)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *GeoHandler) SnapToRoad(w http.ResponseWriter, r *http.Request) {
	var req service.SnappingRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.SnapToRoad(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *GeoHandler) GetTraffic(w http.ResponseWriter, r *http.Request) {
	oLat, _ := strconv.ParseFloat(r.URL.Query().Get("origin_lat"), 64)
	oLng, _ := strconv.ParseFloat(r.URL.Query().Get("origin_lng"), 64)
	dLat, _ := strconv.ParseFloat(r.URL.Query().Get("dest_lat"), 64)
	dLng, _ := strconv.ParseFloat(r.URL.Query().Get("dest_lng"), 64)
	result, err := h.svc.GetTraffic(r.Context(), oLat, oLng, dLat, dLng)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Geocoding ────────────────────────────────────────────────────────────────

func (h *GeoHandler) ForwardGeocode(w http.ResponseWriter, r *http.Request) {
	var req service.ForwardGeocodeRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.ForwardGeocode(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *GeoHandler) ReverseGeocode(w http.ResponseWriter, r *http.Request) {
	var req service.ReverseGeocodeRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.ReverseGeocode(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *GeoHandler) CalculateDistance(w http.ResponseWriter, r *http.Request) {
	oLat, _ := strconv.ParseFloat(r.URL.Query().Get("origin_lat"), 64)
	oLng, _ := strconv.ParseFloat(r.URL.Query().Get("origin_lng"), 64)
	dLat, _ := strconv.ParseFloat(r.URL.Query().Get("dest_lat"), 64)
	dLng, _ := strconv.ParseFloat(r.URL.Query().Get("dest_lng"), 64)
	result, err := h.svc.CalculateDistance(r.Context(), oLat, oLng, dLat, dLng)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Places ───────────────────────────────────────────────────────────────────

func (h *GeoHandler) PlacesAutocomplete(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	var lat, lng *float64
	if v := r.URL.Query().Get("lat"); v != "" {
		f, _ := strconv.ParseFloat(v, 64)
		lat = &f
	}
	if v := r.URL.Query().Get("lng"); v != "" {
		f, _ := strconv.ParseFloat(v, 64)
		lng = &f
	}
	radiusM, _ := strconv.Atoi(r.URL.Query().Get("radius_m"))
	if radiusM == 0 {
		radiusM = 50000
	}
	result, err := h.svc.PlacesAutocomplete(r.Context(), query, lat, lng, radiusM)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *GeoHandler) GetPlaceDetail(w http.ResponseWriter, r *http.Request) {
	placeID := r.URL.Query().Get("placeId")
	result, err := h.svc.GetPlaceDetail(r.Context(), placeID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Geofence ─────────────────────────────────────────────────────────────────

func (h *GeoHandler) CheckGeofence(w http.ResponseWriter, r *http.Request) {
	var req service.GeofenceCheckRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.CheckGeofence(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Internal ─────────────────────────────────────────────────────────────────

func (h *GeoHandler) BatchGetDriverLocations(w http.ResponseWriter, r *http.Request) {
	var req service.BatchLocationRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.BatchGetDriverLocations(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]interface{}{"locations": result})
}

func (h *GeoHandler) GetDriverETA(w http.ResponseWriter, r *http.Request) {
	driverID := r.PathValue("driverId")
	orderID := r.PathValue("orderId")
	result, err := h.svc.GetDriverETA(r.Context(), driverID, orderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

func (h *GeoHandler) UpdateDriverETA(w http.ResponseWriter, r *http.Request) {
	var req service.UpdateEtaRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	driverID := r.PathValue("driverId")
	orderID := r.PathValue("orderId")
	result, err := h.svc.UpdateDriverETA(r.Context(), driverID, orderID, &req)
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
