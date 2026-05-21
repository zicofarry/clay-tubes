//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"github.com/zicofarry/clay-app/backend/services/tracking-service/internal/cache"
	"github.com/zicofarry/clay-app/backend/services/tracking-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/tracking-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/tracking-service/mocks"
	"go.uber.org/mock/gomock"
)

// ── GetOrderPosition ─────────────────────────────────────────────────────────

func TestGetOrderPosition_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)

	mockSvc.EXPECT().GetOrderPosition(gomock.Any(), "order-123").Return(&cache.OrderPosition{
		OrderID: "order-123", DriverID: "driver-1", Lat: -6.9, Lng: 107.6,
	}, nil)

	h := NewTrackingHandler(mockSvc)
	req := httptest.NewRequest("GET", "/tracking/orders/order-123/position", nil)
	req.SetPathValue("orderId", "order-123")
	w := httptest.NewRecorder()
	h.GetOrderPosition(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp response.SuccessResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestGetOrderPosition_MissingOrderID(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)
	// No EXPECT — service should not be called

	h := NewTrackingHandler(mockSvc)
	req := httptest.NewRequest("GET", "/tracking/orders//position", nil)
	// Not setting path value to simulate missing orderId
	w := httptest.NewRecorder()
	h.GetOrderPosition(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetOrderPosition_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)

	mockSvc.EXPECT().GetOrderPosition(gomock.Any(), "order-missing").Return(nil, service.ErrOrderNotFound)

	h := NewTrackingHandler(mockSvc)
	req := httptest.NewRequest("GET", "/tracking/orders/order-missing/position", nil)
	req.SetPathValue("orderId", "order-missing")
	w := httptest.NewRecorder()
	h.GetOrderPosition(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── GetOrderETA ──────────────────────────────────────────────────────────────

func TestGetOrderETA_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)

	mockSvc.EXPECT().GetOrderETA(gomock.Any(), "order-123").Return(&service.ETAResponse{
		OrderID: "order-123", ETAMinutes: 5, DistanceKm: 1.5,
	}, nil)

	h := NewTrackingHandler(mockSvc)
	req := httptest.NewRequest("GET", "/tracking/orders/order-123/eta", nil)
	req.SetPathValue("orderId", "order-123")
	w := httptest.NewRecorder()
	h.GetOrderETA(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetOrderETA_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)

	mockSvc.EXPECT().GetOrderETA(gomock.Any(), "order-missing").Return(nil, service.ErrOrderNotFound)

	h := NewTrackingHandler(mockSvc)
	req := httptest.NewRequest("GET", "/tracking/orders/order-missing/eta", nil)
	req.SetPathValue("orderId", "order-missing")
	w := httptest.NewRecorder()
	h.GetOrderETA(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── GetOrderRoute ────────────────────────────────────────────────────────────

func TestGetOrderRoute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)

	mockSvc.EXPECT().GetOrderRoute(gomock.Any(), "order-123").Return(&service.RouteResponse{
		OrderID: "order-123", TotalDistanceKm: 3.5,
	}, nil)

	h := NewTrackingHandler(mockSvc)
	req := httptest.NewRequest("GET", "/tracking/orders/order-123/route", nil)
	req.SetPathValue("orderId", "order-123")
	w := httptest.NewRecorder()
	h.GetOrderRoute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetOrderRoute_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)

	mockSvc.EXPECT().GetOrderRoute(gomock.Any(), "order-missing").Return(nil, service.ErrOrderNotFound)

	h := NewTrackingHandler(mockSvc)
	req := httptest.NewRequest("GET", "/tracking/orders/order-missing/route", nil)
	req.SetPathValue("orderId", "order-missing")
	w := httptest.NewRecorder()
	h.GetOrderRoute(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── GetTripRoute ─────────────────────────────────────────────────────────────

func TestGetTripRoute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)

	mockSvc.EXPECT().GetTripRoute(gomock.Any(), "order-123").Return(&repository.TripRoute{
		OrderID: "order-123", TotalDistanceKm: 5.0,
	}, nil)

	h := NewTrackingHandler(mockSvc)
	req := httptest.NewRequest("GET", "/routes/order-123", nil)
	req.SetPathValue("orderId", "order-123")
	w := httptest.NewRecorder()
	h.GetTripRoute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetTripRoute_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)

	mockSvc.EXPECT().GetTripRoute(gomock.Any(), "order-missing").Return(nil, service.ErrRouteNotFound)

	h := NewTrackingHandler(mockSvc)
	req := httptest.NewRequest("GET", "/routes/order-missing", nil)
	req.SetPathValue("orderId", "order-missing")
	w := httptest.NewRecorder()
	h.GetTripRoute(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// ── StartTracking ────────────────────────────────────────────────────────────

func TestStartTracking_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)

	mockSvc.EXPECT().StartTracking(gomock.Any(), gomock.Any()).Return(nil)

	h := NewTrackingHandler(mockSvc)
	body := `{"order_id":"order-123","driver_id":"driver-1","pickup_lat":-6.9,"pickup_lng":107.6,"destination_lat":-6.8,"destination_lng":107.5}`
	req := httptest.NewRequest("POST", "/internal/tracking/start", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.StartTracking(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestStartTracking_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)
	// No EXPECT — service should not be called for invalid JSON

	h := NewTrackingHandler(mockSvc)
	req := httptest.NewRequest("POST", "/internal/tracking/start", strings.NewReader(`{invalid`))
	w := httptest.NewRecorder()
	h.StartTracking(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ── StopTracking ─────────────────────────────────────────────────────────────

func TestStopTracking_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)

	mockSvc.EXPECT().StopTracking(gomock.Any(), "order-123").Return(nil)

	h := NewTrackingHandler(mockSvc)
	req := httptest.NewRequest("POST", "/internal/tracking/order-123/stop", nil)
	req.SetPathValue("orderId", "order-123")
	w := httptest.NewRecorder()
	h.StopTracking(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// ── PushLocationUpdate ───────────────────────────────────────────────────────

func TestPushLocationUpdate_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)

	mockSvc.EXPECT().PushLocationUpdate(gomock.Any(), "order-123", gomock.Any()).Return(nil)

	h := NewTrackingHandler(mockSvc)
	body := `{"driver_id":"driver-1","lat":-6.91,"lng":107.61,"timestamp":"2026-04-26T10:00:00Z"}`
	req := httptest.NewRequest("PUT", "/internal/tracking/order-123/update", strings.NewReader(body))
	req.SetPathValue("orderId", "order-123")
	w := httptest.NewRecorder()
	h.PushLocationUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestPushLocationUpdate_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockTrackingServiceInterface(ctrl)
	// No EXPECT — service should not be called for invalid JSON

	h := NewTrackingHandler(mockSvc)
	req := httptest.NewRequest("PUT", "/internal/tracking/order-123/update", strings.NewReader(`{bad`))
	req.SetPathValue("orderId", "order-123")
	w := httptest.NewRecorder()
	h.PushLocationUpdate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
