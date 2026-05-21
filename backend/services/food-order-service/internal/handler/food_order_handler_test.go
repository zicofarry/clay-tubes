//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"log/slog"
	"os"

	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/food-order-service/mocks"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/middleware"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.uber.org/mock/gomock"
)

func setupTestLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, nil))
}

func TestCreateOrder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockFoodOrderServiceInterface(ctrl)
	mockSvc.EXPECT().
		CreateOrder(gomock.Any(), "user-123", gomock.Any()).
		Return(&model.FoodOrder{
			ID:            "order-123",
			UserID:        "user-123",
			MerchantID:    "merchant-1",
			Status:        model.StatusPending,
			TotalCents:    25000,
		}, nil)

	h := NewFoodOrderHandler(mockSvc, setupTestLogger())

	body := `{
		"merchant_id": "merchant-1",
		"payment_method": "gopay",
		"delivery_lat": -6.2,
		"delivery_lng": 106.8,
		"delivery_address": "Test Address",
		"items": [
			{"menu_item_id": "item-1", "quantity": 1}
		]
	}`
	req := httptest.NewRequest("POST", "/orders", strings.NewReader(body))
	
	// Add user ID to context
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.CreateOrder(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var resp response.SuccessResp
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestCreateOrder_ActiveOrderExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockFoodOrderServiceInterface(ctrl)
	mockSvc.EXPECT().
		CreateOrder(gomock.Any(), "user-123", gomock.Any()).
		Return(nil, service.ErrActiveOrderExists)

	h := NewFoodOrderHandler(mockSvc, setupTestLogger())

	body := `{"merchant_id": "merchant-1", "items": [{"menu_item_id": "item-1", "quantity": 1}]}`
	req := httptest.NewRequest("POST", "/orders", strings.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.CreateOrder(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestGetOrder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockFoodOrderServiceInterface(ctrl)
	mockSvc.EXPECT().
		GetOrder(gomock.Any(), "order-123", "user-123").
		Return(&model.FoodOrder{ID: "order-123", UserID: "user-123"}, []model.FoodOrderItem{}, nil)

	h := NewFoodOrderHandler(mockSvc, setupTestLogger())

	req := httptest.NewRequest("GET", "/orders/order-123", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	req = req.WithContext(ctx)
	req.SetPathValue("orderId", "order-123")

	w := httptest.NewRecorder()
	h.GetOrder(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCancelOrder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockFoodOrderServiceInterface(ctrl)
	mockSvc.EXPECT().
		CancelOrder(gomock.Any(), "order-123", "user-123", gomock.Any()).
		Return(&model.FoodOrder{ID: "order-123", Status: model.StatusCancelled}, nil)

	h := NewFoodOrderHandler(mockSvc, setupTestLogger())

	body := `{"reason": "Changed my mind"}`
	req := httptest.NewRequest("POST", "/orders/order-123/cancel", strings.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	req = req.WithContext(ctx)
	req.SetPathValue("orderId", "order-123")

	w := httptest.NewRecorder()
	h.CancelOrder(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCancelOrder_GraceExpired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockFoodOrderServiceInterface(ctrl)
	mockSvc.EXPECT().
		CancelOrder(gomock.Any(), "order-123", "user-123", gomock.Any()).
		Return(nil, service.ErrCancelGraceExpired)

	h := NewFoodOrderHandler(mockSvc, setupTestLogger())

	body := `{"reason": "Changed my mind"}`
	req := httptest.NewRequest("POST", "/orders/order-123/cancel", strings.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	req = req.WithContext(ctx)
	req.SetPathValue("orderId", "order-123")

	w := httptest.NewRecorder()
	h.CancelOrder(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp response.ErrorResp
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Code != "CANCEL_GRACE_EXPIRED" {
		t.Errorf("expected CANCEL_GRACE_EXPIRED, got %s", resp.Code)
	}
}

func TestMerchantConfirmOrder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockFoodOrderServiceInterface(ctrl)
	mockSvc.EXPECT().
		MerchantConfirmOrder(gomock.Any(), "order-123", "merchant-1", gomock.Any()).
		Return(&model.FoodOrder{ID: "order-123", Status: model.StatusConfirmed}, nil)

	h := NewFoodOrderHandler(mockSvc, setupTestLogger())

	body := `{"est_prep_time_min": 20}`
	req := httptest.NewRequest("POST", "/merchant/orders/order-123/confirm", strings.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "merchant-1")
	req = req.WithContext(ctx)
	req.SetPathValue("orderId", "order-123")

	w := httptest.NewRecorder()
	h.MerchantConfirmOrder(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMerchantUpdateStatus_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockFoodOrderServiceInterface(ctrl)
	mockSvc.EXPECT().
		MerchantUpdateStatus(gomock.Any(), "order-123", gomock.Any()).
		Return(&model.FoodOrder{ID: "order-123", Status: model.StatusPreparing}, nil)

	h := NewFoodOrderHandler(mockSvc, setupTestLogger())

	body := `{"action": "start_preparing"}`
	req := httptest.NewRequest("PUT", "/merchant/orders/order-123/status", strings.NewReader(body))
	req.SetPathValue("orderId", "order-123")

	w := httptest.NewRecorder()
	h.MerchantUpdateStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetActiveOrder_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockFoodOrderServiceInterface(ctrl)
	mockSvc.EXPECT().
		GetActiveOrder(gomock.Any(), "user-123").
		Return(nil, service.ErrNoActiveOrder)

	h := NewFoodOrderHandler(mockSvc, setupTestLogger())

	req := httptest.NewRequest("GET", "/orders/active", nil)
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.GetActiveOrder(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSubmitRating_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockFoodOrderServiceInterface(ctrl)
	mockSvc.EXPECT().
		SubmitRating(gomock.Any(), "order-123", "user-123", gomock.Any()).
		Return(nil)

	h := NewFoodOrderHandler(mockSvc, setupTestLogger())

	body := `{"driver_rating": 5, "merchant_rating": 4}`
	req := httptest.NewRequest("POST", "/orders/order-123/rate", strings.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	req = req.WithContext(ctx)
	req.SetPathValue("orderId", "order-123")

	w := httptest.NewRecorder()
	h.SubmitRating(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestMerchantRejectOrder_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockFoodOrderServiceInterface(ctrl)
	mockSvc.EXPECT().
		MerchantRejectOrder(gomock.Any(), "order-123", gomock.Any()).
		Return(&model.FoodOrder{ID: "order-123", Status: model.StatusCancelled}, nil)

	h := NewFoodOrderHandler(mockSvc, setupTestLogger())

	body := `{"reason": "out_of_stock"}`
	req := httptest.NewRequest("POST", "/merchant/orders/order-123/reject", strings.NewReader(body))
	req.SetPathValue("orderId", "order-123")

	w := httptest.NewRecorder()
	h.MerchantRejectOrder(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCreateOrder_MenuItemNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockFoodOrderServiceInterface(ctrl)
	mockSvc.EXPECT().
		CreateOrder(gomock.Any(), "user-123", gomock.Any()).
		Return(nil, service.ErrMenuItemNotFound)

	h := NewFoodOrderHandler(mockSvc, setupTestLogger())

	body := `{"merchant_id": "merchant-1", "items": [{"menu_item_id": "item-1", "quantity": 1}]}`
	req := httptest.NewRequest("POST", "/orders", strings.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.CreateOrder(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCreateOrder_MerchantClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockFoodOrderServiceInterface(ctrl)
	mockSvc.EXPECT().
		CreateOrder(gomock.Any(), "user-123", gomock.Any()).
		Return(nil, service.ErrMerchantClosed)

	h := NewFoodOrderHandler(mockSvc, setupTestLogger())

	body := `{"merchant_id": "merchant-1", "items": [{"menu_item_id": "item-1", "quantity": 1}]}`
	req := httptest.NewRequest("POST", "/orders", strings.NewReader(body))
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.CreateOrder(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
}
