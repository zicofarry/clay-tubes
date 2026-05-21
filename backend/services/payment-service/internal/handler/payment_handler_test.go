//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zicofarry/clay-app/backend/services/payment-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/payment-service/mocks"
	"github.com/zicofarry/clay-app/backend/pkg/pkg/response"
	"go.uber.org/mock/gomock"
)

func TestListPaymentMethods_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPaymentServiceInterface(ctrl)
	mockSvc.EXPECT().
		ListPaymentMethods(gomock.Any(), "user-123").
		Return(&service.PaymentMethodsListResponse{
			Methods: []service.PaymentMethodResponse{
				{MethodID: "pm-1", Type: "credit_card", DisplayName: "Visa •••• 1234", IsDefault: true},
			},
		}, nil)

	h := NewPaymentHandler(mockSvc)
	req := httptest.NewRequest("GET", "/payment-methods", nil)
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.ListPaymentMethods(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAddPaymentMethod_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPaymentServiceInterface(ctrl)
	mockSvc.EXPECT().
		AddPaymentMethod(gomock.Any(), "user-123", gomock.Any()).
		Return(&service.PaymentMethodResponse{
			MethodID: "pm-new", Type: "credit_card", DisplayName: "Visa •••• 5678",
		}, nil)

	h := NewPaymentHandler(mockSvc)
	body := `{"type":"credit_card","card_token":"tok_abc","set_as_default":true}`
	req := httptest.NewRequest("POST", "/payment-methods", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.AddPaymentMethod(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestAddPaymentMethod_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPaymentServiceInterface(ctrl)
	h := NewPaymentHandler(mockSvc)

	req := httptest.NewRequest("POST", "/payment-methods", strings.NewReader(`{invalid`))
	w := httptest.NewRecorder()

	h.AddPaymentMethod(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDeletePaymentMethod_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPaymentServiceInterface(ctrl)
	mockSvc.EXPECT().
		DeletePaymentMethod(gomock.Any(), "user-123", "pm-1").
		Return(nil)

	h := NewPaymentHandler(mockSvc)
	req := httptest.NewRequest("DELETE", "/payment-methods/pm-1", nil)
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("methodId", "pm-1")
	w := httptest.NewRecorder()

	h.DeletePaymentMethod(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDeletePaymentMethod_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPaymentServiceInterface(ctrl)
	mockSvc.EXPECT().
		DeletePaymentMethod(gomock.Any(), "user-123", "pm-unknown").
		Return(service.ErrPaymentMethodNotFound)

	h := NewPaymentHandler(mockSvc)
	req := httptest.NewRequest("DELETE", "/payment-methods/pm-unknown", nil)
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("methodId", "pm-unknown")
	w := httptest.NewRecorder()

	h.DeletePaymentMethod(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCreateCharge_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPaymentServiceInterface(ctrl)
	mockSvc.EXPECT().
		CreateCharge(gomock.Any(), gomock.Any()).
		Return(&service.ChargeResponse{TransactionID: "tx-1", Status: "completed"}, nil)

	h := NewPaymentHandler(mockSvc)
	body := `{"order_id":"ord-1","user_id":"user-1","amount":50000,"payment_method_id":"pm-1","description":"test"}`
	req := httptest.NewRequest("POST", "/internal/charges", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.CreateCharge(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCreateCharge_InsufficientBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPaymentServiceInterface(ctrl)
	mockSvc.EXPECT().
		CreateCharge(gomock.Any(), gomock.Any()).
		Return(nil, service.ErrInsufficientBalance)

	h := NewPaymentHandler(mockSvc)
	body := `{"order_id":"ord-1","user_id":"user-1","amount":50000,"payment_method_id":"pm-1"}`
	req := httptest.NewRequest("POST", "/internal/charges", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.CreateCharge(w, req)

	if w.Code != http.StatusPaymentRequired {
		t.Errorf("expected 402, got %d", w.Code)
	}
}

func TestCreateRefund_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPaymentServiceInterface(ctrl)
	mockSvc.EXPECT().
		CreateRefund(gomock.Any(), gomock.Any()).
		Return(&service.RefundResponse{RefundID: "ref-1", Status: "processed"}, nil)

	h := NewPaymentHandler(mockSvc)
	body := `{"order_id":"ord-1","user_id":"user-1","amount":50000,"reason":"user_cancelled"}`
	req := httptest.NewRequest("POST", "/internal/refunds", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.CreateRefund(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHoldPayment_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPaymentServiceInterface(ctrl)
	mockSvc.EXPECT().
		HoldPayment(gomock.Any(), gomock.Any()).
		Return(&service.HoldResponse{
			HoldID: "hold-1", OrderID: "ord-1", Amount: 50000,
			Status: "held", ExpiresAt: time.Now().Add(2 * time.Hour),
		}, nil)

	h := NewPaymentHandler(mockSvc)
	body := `{"order_id":"ord-1","user_id":"user-1","amount":50000,"payment_method_type":"clay_wallet"}`
	req := httptest.NewRequest("POST", "/internal/payments/hold", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.HoldPayment(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestInitiateCodVerification_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPaymentServiceInterface(ctrl)
	mockSvc.EXPECT().
		InitiateCodVerification(gomock.Any(), "user-123", gomock.Any()).
		Return(&service.CodVerifyInitiateResponse{
			VerificationID: "ver-1", VerificationType: "whatsapp_otp",
			RecipientMaskedPhone: "+62812****7890",
		}, nil)

	h := NewPaymentHandler(mockSvc)
	body := `{"recipient_phone":"+6281234567890","order_type":"food","order_summary":"GoFood Rp45k"}`
	req := httptest.NewRequest("POST", "/cod/verify/initiate", strings.NewReader(body))
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.InitiateCodVerification(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestCreateSettlement_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPaymentServiceInterface(ctrl)
	mockSvc.EXPECT().
		CreateSettlement(gomock.Any(), gomock.Any()).
		Return(&service.SettlementResponse{
			SettlementID: "stl-1", OrderID: "ord-1", DriverID: "drv-1",
			GrossFare: 45000, PlatformFee: 9000, DriverPayout: 36000, Status: "settled",
		}, nil)

	h := NewPaymentHandler(mockSvc)
	body := `{"order_id":"ord-1","driver_id":"drv-1","gross_fare":45000,"service_type":"ride","platform_fee_pct":0.20}`
	req := httptest.NewRequest("POST", "/internal/settlements/create", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.CreateSettlement(w, req)

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
