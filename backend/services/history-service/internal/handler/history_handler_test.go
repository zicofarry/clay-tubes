//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/history-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/history-service/mocks"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.uber.org/mock/gomock"
)

func TestGetOrderHistoryDetail_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := uuid.New().String()
	orderID := uuid.New().String()

	mockSvc := mocks.NewMockHistoryServiceInterface(ctrl)
	mockSvc.EXPECT().
		GetOrderHistoryDetail(gomock.Any(), orderID, userID).
		Return(&service.OrderHistoryDTO{
			OrderID:   orderID,
			OrderType: "ride",
		}, nil)

	h := NewHistoryHandler(mockSvc)

	req := httptest.NewRequest("GET", "/history/orders/"+orderID, nil)
	req.SetPathValue("orderId", orderID)
	req.Header.Set("X-User-ID", userID)
	w := httptest.NewRecorder()

	h.GetOrderHistoryDetail(w, req)

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

func TestGetOrderHistoryDetail_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := uuid.New().String()
	orderID := uuid.New().String()

	mockSvc := mocks.NewMockHistoryServiceInterface(ctrl)
	mockSvc.EXPECT().
		GetOrderHistoryDetail(gomock.Any(), orderID, userID).
		Return(nil, service.ErrNotFound)

	h := NewHistoryHandler(mockSvc)

	req := httptest.NewRequest("GET", "/history/orders/"+orderID, nil)
	req.SetPathValue("orderId", orderID)
	req.Header.Set("X-User-ID", userID)
	w := httptest.NewRecorder()

	h.GetOrderHistoryDetail(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestInternalSyncOrderHistory_InvalidPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockHistoryServiceInterface(ctrl)
	// No expectations, should fail early

	h := NewHistoryHandler(mockSvc)

	req := httptest.NewRequest("POST", "/internal/history/orders/sync", strings.NewReader(`{invalid`))
	w := httptest.NewRecorder()

	h.InternalSyncOrderHistory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListMyOrderHistory_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := uuid.New().String()

	mockSvc := mocks.NewMockHistoryServiceInterface(ctrl)
	mockSvc.EXPECT().
		ListMyOrderHistory(gomock.Any(), userID, "ride", "completed", 1, 20).
		Return([]service.OrderHistoryDTO{
			{OrderID: uuid.New().String(), OrderType: "ride"},
		}, &service.PaginationMeta{Page: 1, Limit: 20, TotalItems: 1}, nil)

	h := NewHistoryHandler(mockSvc)

	req := httptest.NewRequest("GET", "/history/orders?order_type=ride&status=completed&page=1&limit=20", nil)
	req.Header.Set("X-User-ID", userID)
	w := httptest.NewRecorder()

	h.ListMyOrderHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestListMyOrderHistory_Unauthorized(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockHistoryServiceInterface(ctrl)
	mockSvc.EXPECT().
		ListMyOrderHistory(gomock.Any(), "dummy-user-id", "", "", 0, 0).
		Return(nil, nil, service.ErrInvalidRequest)

	h := NewHistoryHandler(mockSvc)

	req := httptest.NewRequest("GET", "/history/orders", nil)
	// No X-User-ID header set
	w := httptest.NewRecorder()

	h.ListMyOrderHistory(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
