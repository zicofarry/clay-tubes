//go:build unit

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/promotion-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/promotion-service/mocks"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.uber.org/mock/gomock"
)

func TestValidatePromo_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockPromotionServiceInterface(ctrl)
	
	reqPayload := service.ValidatePromoRequest{
		Code:        "DISCOUNT10",
		UserID:      uuid.New().String(),
		ServiceType: "ride",
		OrderAmount: 50000.0,
	}
	
	mockSvc.EXPECT().
		ValidatePromo(gomock.Any(), reqPayload).
		Return(&service.PromoValidationResponse{
			PromoID:        uuid.New().String(),
			Code:           "DISCOUNT10",
			Type:           "percentage_off",
			DiscountAmount: 10000.0,
			Description:    "Valid promo code",
		}, nil)

	h := NewPromotionHandler(mockSvc)

	body, _ := json.Marshal(reqPayload)
	req := httptest.NewRequest("POST", "/promotion/promos/validate", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	h.ValidatePromo(w, req)

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

func TestValidatePromo_InvalidPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSvc := mocks.NewMockPromotionServiceInterface(ctrl)
	h := NewPromotionHandler(mockSvc)

	req := httptest.NewRequest("POST", "/promotion/promos/validate", bytes.NewBufferString(`{invalid`))
	w := httptest.NewRecorder()

	h.ValidatePromo(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestListMyVouchers_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := uuid.New().String()

	mockSvc := mocks.NewMockPromotionServiceInterface(ctrl)
	mockSvc.EXPECT().
		ListMyVouchers(gomock.Any(), userID, "all", "available").
		Return([]service.VoucherDTO{
			{PromoID: uuid.New().String(), Code: "DISCOUNT10"},
		}, nil)

	h := NewPromotionHandler(mockSvc)

	req := httptest.NewRequest("GET", "/promotion/vouchers?service_type=all&status=available", nil)
	req.Header.Set("X-User-ID", userID)
	w := httptest.NewRecorder()

	h.ListMyVouchers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestClaimVoucher_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	userID := uuid.New().String()
	
	mockSvc := mocks.NewMockPromotionServiceInterface(ctrl)
	mockSvc.EXPECT().
		ClaimVoucher(gomock.Any(), userID, "NEWUSER20").
		Return(&service.VoucherDTO{
			PromoID: uuid.New().String(),
			Code:    "NEWUSER20",
			Status:  "available",
		}, nil)

	h := NewPromotionHandler(mockSvc)

	body := `{"code":"NEWUSER20"}`
	req := httptest.NewRequest("POST", "/promotion/vouchers/claim", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", userID)
	w := httptest.NewRecorder()

	h.ClaimVoucher(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}
