//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/ride-order-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/ride-order-service/mocks"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.uber.org/mock/gomock"
)

func newTestHandler(t *testing.T) (*RideOrderHandler, *mocks.MockRideOrderServiceInterface) {
	t.Helper()
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockRideOrderServiceInterface(ctrl)
	return NewRideOrderHandler(mockSvc), mockSvc
}

// ── EstimateFare ────────────────────────────────────────────────────────────

func TestEstimateFare_Success(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		EstimateFare(gomock.Any(), gomock.Any()).
		Return(&service.FareEstimateResponse{VehicleType: "motor", FareEstimate: 18000}, nil)

	body := `{"origin_lat":-6.91,"origin_lng":107.6,"dest_lat":-6.92,"dest_lng":107.61,"vehicle_type":"motor"}`
	req := httptest.NewRequest(http.MethodPost, "/orders/estimate", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.EstimateFare(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestEstimateFare_BadJSON(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/orders/estimate", strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	h.EstimateFare(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ── CreateOrder ─────────────────────────────────────────────────────────────

func TestCreateOrder_Success(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		CreateOrder(gomock.Any(), "user-1", gomock.Any()).
		Return(&service.RideOrderResponse{ID: "order-1", Status: "finding_driver"}, nil)

	body := `{"service_type":"goride","vehicle_type":"motor","origin_lat":-6.91,"origin_lng":107.6,"dest_lat":-6.92,"dest_lng":107.61,"payment_method":"gopay"}`
	req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.CreateOrder(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d", w.Code)
	}

	var resp response.SuccessResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Error("want success=true")
	}
}

func TestCreateOrder_DuplicateActive(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		CreateOrder(gomock.Any(), "user-1", gomock.Any()).
		Return(nil, service.ErrActiveOrderExists)

	body := `{"service_type":"goride","vehicle_type":"motor","origin_lat":0,"origin_lng":0,"dest_lat":0,"dest_lng":0,"payment_method":"gopay"}`
	req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.CreateOrder(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", w.Code)
	}
}

func TestCreateOrder_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{not-json`))
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.CreateOrder(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

// ── GetActiveOrder ──────────────────────────────────────────────────────────

func TestGetActiveOrder_None(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		GetActiveOrder(gomock.Any(), "user-1").
		Return(nil, service.ErrNoActiveOrder)

	req := httptest.NewRequest(http.MethodGet, "/orders/active", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.GetActiveOrder(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestGetActiveOrder_Success(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		GetActiveOrder(gomock.Any(), "user-1").
		Return(&service.RideOrderResponse{ID: "order-1"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/orders/active", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.GetActiveOrder(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

// ── GetOrderHistory ─────────────────────────────────────────────────────────

func TestGetOrderHistory_Paginated(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		GetOrderHistory(gomock.Any(), "user-1", gomock.Any()).
		Return(&service.RideOrderHistoryResponse{
			Orders: []service.RideOrderResponse{{ID: "o-1"}},
			Total:  1, Page: 1, Limit: 10,
		}, nil)

	req := httptest.NewRequest(http.MethodGet, "/orders/history?page=1&limit=10&status=completed", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	h.GetOrderHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

// ── GetOrder ────────────────────────────────────────────────────────────────

func TestGetOrder_NotFound(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		GetOrder(gomock.Any(), "user-1", "user", "missing").
		Return(nil, service.ErrOrderNotFound)

	req := httptest.NewRequest(http.MethodGet, "/orders/missing", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("orderId", "missing")
	w := httptest.NewRecorder()
	h.GetOrder(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestGetOrder_DriverRole(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		GetOrder(gomock.Any(), "driver-1", "driver", "order-1").
		Return(&service.RideOrderDetailResponse{
			RideOrderResponse: service.RideOrderResponse{ID: "order-1", OTPCode: "847291"},
		}, nil)

	req := httptest.NewRequest(http.MethodGet, "/orders/order-1", nil)
	req.Header.Set("X-User-ID", "driver-1")
	req.Header.Set("X-User-Role", "driver")
	req.SetPathValue("orderId", "order-1")
	w := httptest.NewRecorder()
	h.GetOrder(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

// ── CancelOrder ─────────────────────────────────────────────────────────────

func TestCancelOrder_OnTrip(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		CancelOrder(gomock.Any(), "user-1", "order-1", gomock.Any()).
		Return(nil, service.ErrCannotCancelOnTrip)

	req := httptest.NewRequest(http.MethodPost, "/orders/order-1/cancel", strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("orderId", "order-1")
	w := httptest.NewRecorder()
	h.CancelOrder(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestCancelOrder_NoBody(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		CancelOrder(gomock.Any(), "user-1", "order-1", gomock.Any()).
		Return(&service.RideOrderResponse{ID: "order-1", Status: "cancelled"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/orders/order-1/cancel", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("orderId", "order-1")
	w := httptest.NewRecorder()
	h.CancelOrder(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

// ── SubmitRating ────────────────────────────────────────────────────────────

func TestSubmitRating_Success(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		SubmitRating(gomock.Any(), "user-1", "order-1", gomock.Any()).
		Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/orders/order-1/rate", strings.NewReader(`{"score":5}`))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("orderId", "order-1")
	w := httptest.NewRecorder()
	h.SubmitRating(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d", w.Code)
	}
}

func TestSubmitRating_AlreadyRated(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		SubmitRating(gomock.Any(), "user-1", "order-1", gomock.Any()).
		Return(service.ErrRatingAlreadySubmitted)

	req := httptest.NewRequest(http.MethodPost, "/orders/order-1/rate", strings.NewReader(`{"score":5}`))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("orderId", "order-1")
	w := httptest.NewRecorder()
	h.SubmitRating(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", w.Code)
	}
}

// ── Driver endpoints ────────────────────────────────────────────────────────

func TestDriverAcceptOrder_AlreadyTaken(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		DriverAcceptOrder(gomock.Any(), "driver-1", "order-1").
		Return(nil, service.ErrOrderAlreadyTaken)

	req := httptest.NewRequest(http.MethodPost, "/driver/orders/order-1/accept", nil)
	req.Header.Set("X-User-ID", "driver-1")
	req.SetPathValue("orderId", "order-1")
	w := httptest.NewRecorder()
	h.DriverAcceptOrder(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", w.Code)
	}
}

func TestDriverAcceptOrder_Success(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		DriverAcceptOrder(gomock.Any(), "driver-1", "order-1").
		Return(&service.DriverAcceptResponse{
			OrderID: "order-1", OTPCode: "847291", Status: "assigned",
		}, nil)

	req := httptest.NewRequest(http.MethodPost, "/driver/orders/order-1/accept", nil)
	req.Header.Set("X-User-ID", "driver-1")
	req.SetPathValue("orderId", "order-1")
	w := httptest.NewRecorder()
	h.DriverAcceptOrder(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestDriverUpdateOrderStatus_BadOTP(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		DriverUpdateOrderStatus(gomock.Any(), "driver-1", "order-1", gomock.Any()).
		Return(nil, service.ErrInvalidOTP)

	body := `{"action":"start_trip","otp_code":"000000"}`
	req := httptest.NewRequest(http.MethodPut, "/driver/orders/order-1/status", strings.NewReader(body))
	req.Header.Set("X-User-ID", "driver-1")
	req.SetPathValue("orderId", "order-1")
	w := httptest.NewRecorder()
	h.DriverUpdateOrderStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestDriverRejectOrder_Success(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		DriverRejectOrder(gomock.Any(), "driver-1", "order-1", gomock.Any()).
		Return(nil)

	req := httptest.NewRequest(http.MethodPost, "/driver/orders/order-1/reject", strings.NewReader(`{"reason":"too_far"}`))
	req.Header.Set("X-User-ID", "driver-1")
	req.SetPathValue("orderId", "order-1")
	w := httptest.NewRecorder()
	h.DriverRejectOrder(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

// ── Internal endpoints ──────────────────────────────────────────────────────

func TestInternalAssignDriver_Success(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		InternalAssignDriver(gomock.Any(), "order-1", gomock.Any()).
		Return(&service.InternalAssignDriverResponse{
			OrderID: "order-1", DriverID: "driver-1", OTPCode: "847291", Status: "assigned",
		}, nil)

	req := httptest.NewRequest(http.MethodPut, "/internal/orders/order-1/assign-driver",
		strings.NewReader(`{"driver_id":"driver-1","eta_seconds":180}`))
	req.SetPathValue("orderId", "order-1")
	w := httptest.NewRecorder()
	h.InternalAssignDriver(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestInternalGetOrder_NotFound(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		InternalGetOrder(gomock.Any(), "missing").
		Return(nil, service.ErrOrderNotFound)

	req := httptest.NewRequest(http.MethodGet, "/internal/orders/missing", nil)
	req.SetPathValue("orderId", "missing")
	w := httptest.NewRecorder()
	h.InternalGetOrder(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

// ── Error mapping fallback ──────────────────────────────────────────────────

func TestHandleServiceError_NonServiceError(t *testing.T) {
	w := httptest.NewRecorder()
	handleServiceError(w, http.ErrAbortHandler)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500 for non-ServiceError, got %d", w.Code)
	}
}

// ── GetFareBreakdown ────────────────────────────────────────────────────────

func TestGetFareBreakdown_Success(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		GetFareBreakdown(gomock.Any(), "user-1", "order-1").
		Return(&service.FareBreakdownResponse{Total: 15000.0}, nil)

	req := httptest.NewRequest(http.MethodGet, "/orders/order-1/fare-breakdown", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("orderId", "order-1")
	w := httptest.NewRecorder()
	h.GetFareBreakdown(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestGetFareBreakdown_FareNotFinalized(t *testing.T) {
	h, mockSvc := newTestHandler(t)

	mockSvc.EXPECT().
		GetFareBreakdown(gomock.Any(), "user-1", "order-1").
		Return(nil, service.ErrFareNotFinalized)

	req := httptest.NewRequest(http.MethodGet, "/orders/order-1/fare-breakdown", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("orderId", "order-1")
	w := httptest.NewRecorder()
	h.GetFareBreakdown(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}
