//go:build unit

package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/wallet-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/wallet-service/mocks"
	"go.uber.org/mock/gomock"
)

// ── GetWallet ────────────────────────────────────────────────────────────────

func TestGetWallet_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockWalletService(ctrl)

	userID := uuid.New()
	walletID := uuid.New()
	now := time.Now()

	mockSvc.EXPECT().GetBalance(gomock.Any(), userID).Return(&repository.Wallet{
		ID:       walletID,
		UserID:   userID,
		Balance:  50000,
		IsActive: true,
		CreatedAt: now,
	}, nil)

	h := NewWalletHandler(mockSvc)
	req := httptest.NewRequest("GET", "/wallet", nil)
	req.Header.Set("X-User-ID", userID.String())
	w := httptest.NewRecorder()
	h.GetWallet(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetWallet_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockWalletService(ctrl)
	// No EXPECT — service should not be called for invalid UUID

	h := NewWalletHandler(mockSvc)
	req := httptest.NewRequest("GET", "/wallet", nil)
	req.Header.Set("X-User-ID", "not-a-uuid")
	w := httptest.NewRecorder()
	h.GetWallet(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetWallet_MissingUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockWalletService(ctrl)
	// No EXPECT — service should not be called without header

	h := NewWalletHandler(mockSvc)
	req := httptest.NewRequest("GET", "/wallet", nil)
	// Not setting X-User-ID header
	w := httptest.NewRecorder()
	h.GetWallet(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetWallet_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockWalletService(ctrl)

	userID := uuid.New()
	mockSvc.EXPECT().GetBalance(gomock.Any(), userID).Return(nil, errors.New("db error"))

	h := NewWalletHandler(mockSvc)
	req := httptest.NewRequest("GET", "/wallet", nil)
	req.Header.Set("X-User-ID", userID.String())
	w := httptest.NewRecorder()
	h.GetWallet(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// ── TopUp ────────────────────────────────────────────────────────────────────

func TestTopUp_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockWalletService(ctrl)

	userID := uuid.New()
	txID := uuid.New()

	mockSvc.EXPECT().TopUp(gomock.Any(), userID, int64(50000), "gopay").Return(&repository.WalletTransaction{
		ID:           txID,
		Amount:       50000,
		BalanceAfter: 100000,
	}, nil)

	h := NewWalletHandler(mockSvc)
	body := `{"amount":50000,"channel":"gopay"}`
	req := httptest.NewRequest("POST", "/wallet/topup", strings.NewReader(body))
	req.Header.Set("X-User-ID", userID.String())
	w := httptest.NewRecorder()
	h.TopUp(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestTopUp_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockWalletService(ctrl)

	h := NewWalletHandler(mockSvc)
	body := `{"amount":50000,"channel":"gopay"}`
	req := httptest.NewRequest("POST", "/wallet/topup", strings.NewReader(body))
	req.Header.Set("X-User-ID", "bad-id")
	w := httptest.NewRecorder()
	h.TopUp(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTopUp_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockWalletService(ctrl)

	userID := uuid.New()

	h := NewWalletHandler(mockSvc)
	req := httptest.NewRequest("POST", "/wallet/topup", strings.NewReader(`{invalid`))
	req.Header.Set("X-User-ID", userID.String())
	w := httptest.NewRecorder()
	h.TopUp(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTopUp_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockWalletService(ctrl)

	userID := uuid.New()
	mockSvc.EXPECT().TopUp(gomock.Any(), userID, int64(50000), "gopay").Return(nil, errors.New("credit failed"))

	h := NewWalletHandler(mockSvc)
	body := `{"amount":50000,"channel":"gopay"}`
	req := httptest.NewRequest("POST", "/wallet/topup", strings.NewReader(body))
	req.Header.Set("X-User-ID", userID.String())
	w := httptest.NewRecorder()
	h.TopUp(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// ── Debit ────────────────────────────────────────────────────────────────────

func TestDebit_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockWalletService(ctrl)

	userID := uuid.New()
	refID := uuid.New()
	txID := uuid.New()

	mockSvc.EXPECT().Debit(gomock.Any(), userID, int64(15000), refID, "Payment for ride").Return(&repository.WalletTransaction{
		ID:           txID,
		Amount:       -15000,
		BalanceAfter: 35000,
	}, nil)

	h := NewWalletHandler(mockSvc)
	body := `{"user_id":"` + userID.String() + `","amount":15000,"reference_id":"` + refID.String() + `","description":"Payment for ride"}`
	req := httptest.NewRequest("POST", "/internal/wallet/debit", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Debit(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDebit_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockWalletService(ctrl)

	h := NewWalletHandler(mockSvc)
	req := httptest.NewRequest("POST", "/internal/wallet/debit", strings.NewReader(`{bad`))
	w := httptest.NewRecorder()
	h.Debit(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDebit_InsufficientBalance(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockWalletService(ctrl)

	userID := uuid.New()
	refID := uuid.New()

	mockSvc.EXPECT().Debit(gomock.Any(), userID, int64(100000), refID, "Payment").
		Return(nil, repository.ErrInsufficientBalance)

	h := NewWalletHandler(mockSvc)
	body := `{"user_id":"` + userID.String() + `","amount":100000,"reference_id":"` + refID.String() + `","description":"Payment"}`
	req := httptest.NewRequest("POST", "/internal/wallet/debit", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Debit(w, req)

	if w.Code != http.StatusPaymentRequired {
		t.Errorf("expected 402, got %d", w.Code)
	}
}
