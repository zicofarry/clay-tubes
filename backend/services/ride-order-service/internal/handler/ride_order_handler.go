// Package handler implements HTTP handlers for the Ride Order Service.
// Each method maps 1:1 to an OpenAPI endpoint.
package handler

import (
	"net/http"

	"github.com/zicofarry/clay-app/backend/services/ride-order-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/validator"
)

// RideOrderHandler holds references to the service layer.
type RideOrderHandler struct {
	svc service.RideOrderServiceInterface
}

// NewRideOrderHandler creates a new RideOrderHandler.
func NewRideOrderHandler(svc service.RideOrderServiceInterface) *RideOrderHandler {
	return &RideOrderHandler{svc: svc}
}

// ── Fare ─────────────────────────────────────────────────────────────────────

// EstimateFare handles POST /orders/estimate
func (h *RideOrderHandler) EstimateFare(w http.ResponseWriter, r *http.Request) {
	var req service.FareEstimateRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.EstimateFare(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Order ────────────────────────────────────────────────────────────────────

// CreateOrder handles POST /orders
func (h *RideOrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req service.CreateRideOrderRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	userID := r.Header.Get("X-User-ID")
	result, err := h.svc.CreateOrder(r.Context(), userID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, result)
}

// GetActiveOrder handles GET /orders/active
func (h *RideOrderHandler) GetActiveOrder(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	result, err := h.svc.GetActiveOrder(r.Context(), userID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// GetOrderHistory handles GET /orders/history
func (h *RideOrderHandler) GetOrderHistory(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	q := service.HistoryQuery{
		Status:      validator.QueryString(r, "status", ""),
		ServiceType: validator.QueryString(r, "service_type", ""),
		From:        validator.QueryString(r, "from", ""),
		To:          validator.QueryString(r, "to", ""),
		Page:        validator.QueryInt(r, "page", 1),
		Limit:       validator.QueryInt(r, "limit", 10),
	}
	result, err := h.svc.GetOrderHistory(r.Context(), userID, q)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Paginated(w, http.StatusOK, result.Orders, result.Total, result.Page, result.Limit)
}

// GetOrder handles GET /orders/{orderId}
func (h *RideOrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	role := r.Header.Get("X-User-Role")
	if role == "" {
		role = "user"
	}
	orderID := r.PathValue("orderId")
	result, err := h.svc.GetOrder(r.Context(), userID, role, orderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// CancelOrder handles POST /orders/{orderId}/cancel
func (h *RideOrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	var req service.CancelOrderRequest
	// Body is optional; only decode when present.
	if r.ContentLength > 0 {
		if err := validator.DecodeJSON(r, &req); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
	}
	userID := r.Header.Get("X-User-ID")
	orderID := r.PathValue("orderId")
	result, err := h.svc.CancelOrder(r.Context(), userID, orderID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// SubmitRating handles POST /orders/{orderId}/rate
func (h *RideOrderHandler) SubmitRating(w http.ResponseWriter, r *http.Request) {
	var req service.SubmitRatingRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	userID := r.Header.Get("X-User-ID")
	orderID := r.PathValue("orderId")
	if err := h.svc.SubmitRating(r.Context(), userID, orderID, &req); err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, map[string]string{"message": "rating submitted"})
}

// GetFareBreakdown handles GET /orders/{orderId}/fare-breakdown
func (h *RideOrderHandler) GetFareBreakdown(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	orderID := r.PathValue("orderId")
	result, err := h.svc.GetFareBreakdown(r.Context(), userID, orderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Driver ───────────────────────────────────────────────────────────────────

// DriverAcceptOrder handles POST /driver/orders/{orderId}/accept
func (h *RideOrderHandler) DriverAcceptOrder(w http.ResponseWriter, r *http.Request) {
	driverID := r.Header.Get("X-User-ID")
	orderID := r.PathValue("orderId")
	result, err := h.svc.DriverAcceptOrder(r.Context(), driverID, orderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// DriverRejectOrder handles POST /driver/orders/{orderId}/reject
func (h *RideOrderHandler) DriverRejectOrder(w http.ResponseWriter, r *http.Request) {
	var req service.DriverRejectRequest
	if r.ContentLength > 0 {
		if err := validator.DecodeJSON(r, &req); err != nil {
			response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
			return
		}
	}
	driverID := r.Header.Get("X-User-ID")
	orderID := r.PathValue("orderId")
	if err := h.svc.DriverRejectOrder(r.Context(), driverID, orderID, &req); err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, map[string]string{"message": "order rejected"})
}

// DriverUpdateOrderStatus handles PUT /driver/orders/{orderId}/status
func (h *RideOrderHandler) DriverUpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	var req service.DriverUpdateStatusRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	driverID := r.Header.Get("X-User-ID")
	orderID := r.PathValue("orderId")
	result, err := h.svc.DriverUpdateOrderStatus(r.Context(), driverID, orderID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// ── Internal ────────────────────────────────────────────────────────────────

// InternalCreateOrder handles POST /internal/orders
func (h *RideOrderHandler) InternalCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req service.InternalCreateOrderRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.InternalCreateOrder(r.Context(), &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusCreated, result)
}

// InternalGetOrder handles GET /internal/orders/{orderId}
func (h *RideOrderHandler) InternalGetOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	result, err := h.svc.InternalGetOrder(r.Context(), orderID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// InternalUpdateStatus handles PUT /internal/orders/{orderId}/status
func (h *RideOrderHandler) InternalUpdateStatus(w http.ResponseWriter, r *http.Request) {
	var req service.InternalUpdateStatusRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	orderID := r.PathValue("orderId")
	result, err := h.svc.InternalUpdateStatus(r.Context(), orderID, &req)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	response.Success(w, http.StatusOK, result)
}

// InternalAssignDriver handles PUT /internal/orders/{orderId}/assign-driver
func (h *RideOrderHandler) InternalAssignDriver(w http.ResponseWriter, r *http.Request) {
	var req service.InternalAssignDriverRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	orderID := r.PathValue("orderId")
	result, err := h.svc.InternalAssignDriver(r.Context(), orderID, &req)
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
