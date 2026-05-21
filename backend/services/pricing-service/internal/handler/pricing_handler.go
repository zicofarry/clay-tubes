// Package handler implements HTTP handlers for the Pricing Service.
// Each method maps 1:1 to an OpenAPI endpoint.
package handler

import (
	"net/http"
	"strconv"

	"github.com/zicofarry/clay-app/backend/services/pricing-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/validator"
)

// PricingHandler holds references to the pricing service layer.
type PricingHandler struct {
	svc service.PricingServiceInterface
}

// NewPricingHandler creates a new PricingHandler.
func NewPricingHandler(svc service.PricingServiceInterface) *PricingHandler {
	return &PricingHandler{svc: svc}
}

// ── Estimate ─────────────────────────────────────────────────────────────────

// EstimateRideFare handles POST /estimate/ride
func (h *PricingHandler) EstimateRideFare(w http.ResponseWriter, r *http.Request) {
	var req service.RideEstimateRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.EstimateRideFare(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// EstimateDeliveryFare handles POST /estimate/delivery
func (h *PricingHandler) EstimateDeliveryFare(w http.ResponseWriter, r *http.Request) {
	var req service.DeliveryEstimateRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.EstimateDeliveryFare(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// EstimateFoodFare handles POST /estimate/food
func (h *PricingHandler) EstimateFoodFare(w http.ResponseWriter, r *http.Request) {
	var req service.FoodEstimateRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.EstimateFoodFare(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// ── Surge ────────────────────────────────────────────────────────────────────

// GetSurge handles GET /surge
func (h *PricingHandler) GetSurge(w http.ResponseWriter, r *http.Request) {
	lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lng, _ := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)
	serviceType := r.URL.Query().Get("service_type")

	result, err := h.svc.GetSurge(r.Context(), lat, lng, serviceType)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// ── Fare ─────────────────────────────────────────────────────────────────────

// CalculateFinalFare handles POST /fare/calculate
func (h *PricingHandler) CalculateFinalFare(w http.ResponseWriter, r *http.Request) {
	var req service.FinalFareRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	result, err := h.svc.CalculateFinalFare(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	response.Success(w, http.StatusOK, result)
}

// ── Internal ─────────────────────────────────────────────────────────────────

// GetFareRules handles GET /internal/fare-rules
func (h *PricingHandler) GetFareRules(w http.ResponseWriter, r *http.Request) {
	serviceType := r.URL.Query().Get("service_type")
	var zoneID *string
	if z := r.URL.Query().Get("zone_id"); z != "" {
		zoneID = &z
	}

	result, err := h.svc.GetFareRules(r.Context(), serviceType, zoneID)
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
