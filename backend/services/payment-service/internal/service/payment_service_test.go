//go:build unit

package service

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/payment-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/payment-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

func newTestService(t *testing.T) (*PaymentService, *repomock.MockPaymentRepositoryInterface, *gomock.Controller) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockPaymentRepositoryInterface(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewPaymentService(mockRepo, logger, nil, nil)
	return svc, mockRepo, ctrl
}

func TestAddPaymentMethod_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		CreatePaymentMethod(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, pm *repository.PaymentMethod) (*repository.PaymentMethod, error) {
			pm.ID = "pm-generated"
			return pm, nil
		})

	result, err := svc.AddPaymentMethod(context.Background(), "user-123", &AddPaymentMethodRequest{
		Type: "credit_card", SetAsDefault: true,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MethodID != "pm-generated" {
		t.Errorf("expected pm-generated, got %s", result.MethodID)
	}
}

func TestCreateCharge_WalletSuccess(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindPaymentMethodByID(gomock.Any(), "pm-wallet").
		Return(&repository.PaymentMethod{ID: "pm-wallet", Type: "clay_wallet", UserID: "user-1"}, nil)

	mockRepo.EXPECT().
		CreateTransaction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tx *repository.Transaction) (*repository.Transaction, error) {
			tx.ID = "tx-generated"
			return tx, nil
		})

	result, err := svc.CreateCharge(context.Background(), &ChargeRequest{
		OrderID: "ord-1", UserID: "user-1", Amount: 50000, PaymentMethodID: "pm-wallet",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Errorf("expected completed, got %s", result.Status)
	}
}

func TestCreateCharge_InvalidPaymentMethod(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindPaymentMethodByID(gomock.Any(), "pm-unknown").
		Return(nil, sql.ErrNoRows)

	_, err := svc.CreateCharge(context.Background(), &ChargeRequest{
		OrderID: "ord-1", UserID: "user-1", Amount: 50000, PaymentMethodID: "pm-unknown",
	})

	if err != ErrInvalidPaymentMethod {
		t.Errorf("expected ErrInvalidPaymentMethod, got %v", err)
	}
}

func TestCreateRefund_TransactionNotFound(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindTransactionByOrderID(gomock.Any(), "ord-unknown").
		Return(nil, sql.ErrNoRows)

	_, err := svc.CreateRefund(context.Background(), &RefundRequest{
		OrderID: "ord-unknown", UserID: "user-1", Amount: 50000, Reason: "user_cancelled",
	})

	if err != ErrTransactionNotFound {
		t.Errorf("expected ErrTransactionNotFound, got %v", err)
	}
}

func TestHoldPayment_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		CreateHold(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, hold *repository.Hold) (*repository.Hold, error) {
			hold.ID = "hold-generated"
			return hold, nil
		})

	result, err := svc.HoldPayment(context.Background(), &HoldRequest{
		OrderID: "ord-1", UserID: "user-1", Amount: 50000, PaymentMethodType: "clay_wallet",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "held" {
		t.Errorf("expected held, got %s", result.Status)
	}
}

func TestCapturePayment_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindHoldByID(gomock.Any(), "hold-1").
		Return(&repository.Hold{
			ID: "hold-1", OrderID: "ord-1", UserID: "user-1",
			Amount: 50000, PaymentMethodType: "clay_wallet", Status: "held",
		}, nil)

	mockRepo.EXPECT().
		CreateTransaction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, tx *repository.Transaction) (*repository.Transaction, error) {
			tx.ID = "tx-captured"
			return tx, nil
		})

	mockRepo.EXPECT().
		UpdateHoldStatus(gomock.Any(), "hold-1", "captured").
		Return(nil)

	result, err := svc.CapturePayment(context.Background(), &CaptureRequest{
		HoldID: "hold-1", CaptureAmount: 45000,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CapturedAmount != 45000 {
		t.Errorf("expected captured 45000, got %d", result.CapturedAmount)
	}
	if result.ReleasedAmount != 5000 {
		t.Errorf("expected released 5000, got %d", result.ReleasedAmount)
	}
}

func TestCapturePayment_ExceedsHold(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindHoldByID(gomock.Any(), "hold-1").
		Return(&repository.Hold{ID: "hold-1", Amount: 50000, Status: "held"}, nil)

	_, err := svc.CapturePayment(context.Background(), &CaptureRequest{
		HoldID: "hold-1", CaptureAmount: 60000,
	})

	if err != ErrCaptureExceedsHold {
		t.Errorf("expected ErrCaptureExceedsHold, got %v", err)
	}
}

func TestReleasePayment_NotFound(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		FindHoldByID(gomock.Any(), "hold-unknown").
		Return(nil, sql.ErrNoRows)

	err := svc.ReleasePayment(context.Background(), &ReleaseRequest{
		HoldID: "hold-unknown", Reason: "order_cancelled",
	})

	if err != ErrHoldNotFound {
		t.Errorf("expected ErrHoldNotFound, got %v", err)
	}
}

func TestCreateSettlement_Duplicate(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		SettlementExistsByOrderID(gomock.Any(), "ord-dup").
		Return(true, nil)

	_, err := svc.CreateSettlement(context.Background(), &CreateSettlementRequest{
		OrderID: "ord-dup", DriverID: "drv-1", GrossFare: 45000, ServiceType: "ride",
	})

	if err != ErrSettlementDuplicate {
		t.Errorf("expected ErrSettlementDuplicate, got %v", err)
	}
}

func TestCreateSettlement_Success(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)

	mockRepo.EXPECT().
		SettlementExistsByOrderID(gomock.Any(), "ord-1").
		Return(false, nil)

	mockRepo.EXPECT().
		CreateSettlement(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, s *repository.Settlement) (*repository.Settlement, error) {
			s.ID = "stl-generated"
			return s, nil
		})

	result, err := svc.CreateSettlement(context.Background(), &CreateSettlementRequest{
		OrderID: "ord-1", DriverID: "drv-1", GrossFare: 45000, ServiceType: "ride", PlatformFeePct: 0.20,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PlatformFee != 9000 {
		t.Errorf("expected platform fee 9000, got %d", result.PlatformFee)
	}
	if result.DriverPayout != 36000 {
		t.Errorf("expected driver payout 36000, got %d", result.DriverPayout)
	}
}
