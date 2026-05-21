package handler

import (
	"log/slog"
	"net/http"

	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/service"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/validator"
)

// FoodOrderHandler handles HTTP requests for food orders.
type FoodOrderHandler struct {
	svc    service.FoodOrderServiceInterface
	logger *slog.Logger
}

// NewFoodOrderHandler creates a new FoodOrderHandler.
func NewFoodOrderHandler(svc service.FoodOrderServiceInterface, logger *slog.Logger) *FoodOrderHandler {
	return &FoodOrderHandler{svc: svc, logger: logger}
}

// EstimateFare handles POST /orders/estimate.
func (h *FoodOrderHandler) EstimateFare(w http.ResponseWriter, r *http.Request) {
	var req model.FareEstimateRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	result, err := h.svc.EstimateFare(r.Context(), req)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Success(w, http.StatusOK, result)
}

// CreateOrder handles POST /orders.
func (h *FoodOrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	var req model.CreateFoodOrderRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}
	if len(req.Items) == 0 {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", "items cannot be empty")
		return
	}

	order, err := h.svc.CreateOrder(r.Context(), userID, req)
	if err != nil {
		switch err {
		case service.ErrActiveOrderExists:
			response.Error(w, http.StatusConflict, "ACTIVE_ORDER_EXISTS", err.Error())
		case service.ErrMenuItemNotFound:
			response.Error(w, http.StatusNotFound, "MENU_ITEM_NOT_FOUND", err.Error())
		case service.ErrMerchantClosed:
			response.Error(w, http.StatusUnprocessableEntity, "MERCHANT_CLOSED", err.Error())
		case service.ErrPromoInvalid:
			response.Error(w, http.StatusUnprocessableEntity, "PROMO_INVALID", err.Error())
		default:
			h.logger.Error("create order failed", slog.Any("error", err))
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}
	response.Success(w, http.StatusCreated, order)
}

// GetActiveOrder handles GET /orders/active.
func (h *FoodOrderHandler) GetActiveOrder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	order, err := h.svc.GetActiveOrder(r.Context(), userID)
	if err != nil {
		if err == service.ErrNoActiveOrder {
			response.Error(w, http.StatusNotFound, "NO_ACTIVE_ORDER", "no active food order found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Success(w, http.StatusOK, order)
}

// GetOrderHistory handles GET /orders/history.
func (h *FoodOrderHandler) GetOrderHistory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	pg := validator.ParsePagination(r, 50)
	status := validator.QueryString(r, "status", "")

	orders, total, err := h.svc.GetHistory(r.Context(), userID, status, pg.Page, pg.Limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Paginated(w, http.StatusOK, orders, total, pg.Page, pg.Limit)
}

// GetOrder handles GET /orders/{orderId}.
func (h *FoodOrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	orderID := r.PathValue("orderId")

	order, items, err := h.svc.GetOrder(r.Context(), orderID, userID)
	if err != nil {
		if err == service.ErrOrderNotFound {
			response.Error(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"order": order,
		"items": items,
	})
}

// CancelOrder handles POST /orders/{orderId}/cancel.
func (h *FoodOrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	orderID := r.PathValue("orderId")

	var req model.CancelOrderRequest
	_ = validator.DecodeJSON(r, &req)

	order, err := h.svc.CancelOrder(r.Context(), orderID, userID, req)
	if err != nil {
		switch err {
		case service.ErrOrderNotFound:
			response.Error(w, http.StatusNotFound, "ORDER_NOT_FOUND", err.Error())
		case service.ErrForbidden:
			response.Error(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		case service.ErrCancelGraceExpired:
			response.Error(w, http.StatusBadRequest, "CANCEL_GRACE_EXPIRED", err.Error())
		case service.ErrCannotCancelState:
			response.Error(w, http.StatusBadRequest, "CANNOT_CANCEL_PREPARING", err.Error())
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}
	response.Success(w, http.StatusOK, order)
}

// SubmitRating handles POST /orders/{orderId}/rate.
func (h *FoodOrderHandler) SubmitRating(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	orderID := r.PathValue("orderId")

	var req model.SubmitFoodRatingRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	if err := h.svc.SubmitRating(r.Context(), orderID, userID, req); err != nil {
		switch err {
		case service.ErrOrderNotFound:
			response.Error(w, http.StatusNotFound, "ORDER_NOT_FOUND", err.Error())
		case service.ErrForbidden:
			response.Error(w, http.StatusForbidden, "FORBIDDEN", err.Error())
		case service.ErrRatingAlreadySubmitted:
			response.Error(w, http.StatusConflict, "RATING_ALREADY_SUBMITTED", err.Error())
		case service.ErrInvalidStateTransition:
			response.Error(w, http.StatusUnprocessableEntity, "ORDER_NOT_DELIVERED", "order is not delivered yet")
		default:
			response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}
	response.Success(w, http.StatusCreated, map[string]string{"message": "rating submitted"})
}

// GetFareBreakdown handles GET /orders/{orderId}/fare-breakdown.
func (h *FoodOrderHandler) GetFareBreakdown(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	orderID := r.PathValue("orderId")

	fb, err := h.svc.GetFareBreakdown(r.Context(), orderID, userID)
	if err != nil {
		if err == service.ErrOrderNotFound {
			response.Error(w, http.StatusNotFound, "ORDER_NOT_FOUND", err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if fb == nil {
		response.Error(w, http.StatusNotFound, "FARE_NOT_FOUND", "fare breakdown not available")
		return
	}
	response.Success(w, http.StatusOK, fb)
}

// ── Merchant-facing handlers ──────────────────────────────────────────────────

// MerchantListOrders handles GET /merchant/orders.
func (h *FoodOrderHandler) MerchantListOrders(w http.ResponseWriter, r *http.Request) {
	merchantID := r.Header.Get("X-Merchant-ID")
	if merchantID == "" {
		merchantID = middleware.GetUserID(r.Context())
	}
	pg := validator.ParsePagination(r, 50)
	status := validator.QueryString(r, "status", "")

	orders, total, err := h.svc.MerchantListOrders(r.Context(), merchantID, status, pg.Page, pg.Limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Paginated(w, http.StatusOK, orders, total, pg.Page, pg.Limit)
}

// MerchantGetOrder handles GET /merchant/orders/{orderId}.
func (h *FoodOrderHandler) MerchantGetOrder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	orderID := r.PathValue("orderId")
	order, items, err := h.svc.GetOrder(r.Context(), orderID, userID)
	if err != nil {
		response.Error(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
		return
	}
	response.Success(w, http.StatusOK, map[string]interface{}{"order": order, "items": items})
}

// MerchantConfirmOrder handles POST /merchant/orders/{orderId}/confirm.
func (h *FoodOrderHandler) MerchantConfirmOrder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	orderID := r.PathValue("orderId")

	var req model.MerchantConfirmRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	order, err := h.svc.MerchantConfirmOrder(r.Context(), orderID, userID, req)
	if err != nil {
		h.handleOrderError(w, err)
		return
	}
	response.Success(w, http.StatusOK, order)
}

// MerchantRejectOrder handles POST /merchant/orders/{orderId}/reject.
func (h *FoodOrderHandler) MerchantRejectOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")

	var req model.MerchantRejectRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	order, err := h.svc.MerchantRejectOrder(r.Context(), orderID, req)
	if err != nil {
		h.handleOrderError(w, err)
		return
	}
	response.Success(w, http.StatusOK, order)
}

// MerchantUpdateStatus handles PUT /merchant/orders/{orderId}/status.
func (h *FoodOrderHandler) MerchantUpdateStatus(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")

	var req model.MerchantUpdateStatusRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	order, err := h.svc.MerchantUpdateStatus(r.Context(), orderID, req)
	if err != nil {
		h.handleOrderError(w, err)
		return
	}
	response.Success(w, http.StatusOK, order)
}

// ── Driver-facing handlers ────────────────────────────────────────────────────

// DriverPickup handles POST /driver/orders/{orderId}/pickup.
func (h *FoodOrderHandler) DriverPickup(w http.ResponseWriter, r *http.Request) {
	driverID := middleware.GetUserID(r.Context())
	orderID := r.PathValue("orderId")

	order, err := h.svc.DriverPickup(r.Context(), orderID, driverID)
	if err != nil {
		h.handleOrderError(w, err)
		return
	}
	response.Success(w, http.StatusOK, order)
}

// DriverDeliver handles POST /driver/orders/{orderId}/deliver.
func (h *FoodOrderHandler) DriverDeliver(w http.ResponseWriter, r *http.Request) {
	driverID := middleware.GetUserID(r.Context())
	orderID := r.PathValue("orderId")

	order, err := h.svc.DriverDeliver(r.Context(), orderID, driverID)
	if err != nil {
		h.handleOrderError(w, err)
		return
	}
	response.Success(w, http.StatusOK, order)
}

// ── Internal handlers (service-to-service) ───────────────────────────────────

// InternalGetOrder handles GET /internal/orders/{orderId}.
func (h *FoodOrderHandler) InternalGetOrder(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")
	order, items, err := h.svc.GetOrder(r.Context(), orderID, "")
	if err != nil {
		response.Error(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
		return
	}
	response.Success(w, http.StatusOK, map[string]interface{}{"order": order, "items": items})
}

// InternalAssignDriver handles POST /internal/orders/{orderId}/assign-driver.
func (h *FoodOrderHandler) InternalAssignDriver(w http.ResponseWriter, r *http.Request) {
	orderID := r.PathValue("orderId")

	var req model.AssignDriverRequest
	if err := validator.DecodeJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "INVALID_INPUT", err.Error())
		return
	}

	order, err := h.svc.AssignDriver(r.Context(), orderID, req.DriverID)
	if err != nil {
		h.handleOrderError(w, err)
		return
	}
	response.Success(w, http.StatusOK, order)
}

// InternalGetUserActiveOrder handles GET /internal/users/{userId}/active-order.
func (h *FoodOrderHandler) InternalGetUserActiveOrder(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	order, err := h.svc.GetActiveOrder(r.Context(), userID)
	if err != nil {
		if err == service.ErrNoActiveOrder {
			response.Success(w, http.StatusOK, map[string]interface{}{"active": false})
			return
		}
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Success(w, http.StatusOK, map[string]interface{}{"active": true, "order_id": order.ID})
}

// ── Error helper ──────────────────────────────────────────────────────────────

func (h *FoodOrderHandler) handleOrderError(w http.ResponseWriter, err error) {
	switch err {
	case service.ErrOrderNotFound:
		response.Error(w, http.StatusNotFound, "ORDER_NOT_FOUND", err.Error())
	case service.ErrForbidden:
		response.Error(w, http.StatusForbidden, "FORBIDDEN", err.Error())
	case service.ErrInvalidStateTransition:
		response.Error(w, http.StatusBadRequest, "INVALID_STATE_TRANSITION", err.Error())
	case service.ErrCannotCancelState:
		response.Error(w, http.StatusBadRequest, "CANNOT_CANCEL", err.Error())
	default:
		response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}
