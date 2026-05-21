//go:build unit

package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/wallet-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/wallet-service/mocks"
	"go.uber.org/mock/gomock"
)

func setupService(t *testing.T) (WalletService, *mocks.MockWalletRepository) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockWalletRepository(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewWalletService(mockRepo, logger)
	return svc, mockRepo
}

// ── GetBalance ───────────────────────────────────────────────────────────────

func TestGetBalance_ExistingWallet(t *testing.T) {
	svc, mockRepo := setupService(t)

	userID := uuid.New()
	expectedWallet := &repository.Wallet{
		ID:      uuid.New(),
		UserID:  userID,
		Balance: 10000,
	}
	mockRepo.EXPECT().GetWalletByUserID(gomock.Any(), userID).Return(expectedWallet, nil)

	wallet, err := svc.GetBalance(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wallet.Balance != 10000 {
		t.Errorf("expected balance 10000, got %d", wallet.Balance)
	}
}

func TestGetBalance_AutoCreateWallet(t *testing.T) {
	svc, mockRepo := setupService(t)

	userID := uuid.New()
	createdWallet := &repository.Wallet{
		ID:      uuid.New(),
		UserID:  userID,
		Balance: 0,
	}

	mockRepo.EXPECT().GetWalletByUserID(gomock.Any(), userID).Return(nil, repository.ErrWalletNotFound)
	mockRepo.EXPECT().CreateWallet(gomock.Any(), userID).Return(createdWallet, nil)

	wallet, err := svc.GetBalance(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wallet.Balance != 0 {
		t.Errorf("expected balance 0, got %d", wallet.Balance)
	}
}

func TestGetBalance_AutoCreateError(t *testing.T) {
	svc, mockRepo := setupService(t)

	userID := uuid.New()
	mockRepo.EXPECT().GetWalletByUserID(gomock.Any(), userID).Return(nil, repository.ErrWalletNotFound)
	mockRepo.EXPECT().CreateWallet(gomock.Any(), userID).Return(nil, errors.New("db constraint error"))

	_, err := svc.GetBalance(context.Background(), userID)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGetBalance_DBError(t *testing.T) {
	svc, mockRepo := setupService(t)

	userID := uuid.New()
	mockRepo.EXPECT().GetWalletByUserID(gomock.Any(), userID).Return(nil, errors.New("connection refused"))

	_, err := svc.GetBalance(context.Background(), userID)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// ── TopUp ────────────────────────────────────────────────────────────────────

func TestTopUp_Success(t *testing.T) {
	svc, mockRepo := setupService(t)

	userID := uuid.New()
	mockRepo.EXPECT().CreditWallet(gomock.Any(), userID, int64(50000), "top_up", gomock.Any(), gomock.Any()).
		Return(&repository.WalletTransaction{
			ID:           uuid.New(),
			Amount:       50000,
			BalanceAfter: 50000,
		}, nil)

	tx, err := svc.TopUp(context.Background(), userID, 50000, "gopay")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.BalanceAfter != 50000 {
		t.Errorf("expected balance after 50000, got %d", tx.BalanceAfter)
	}
}

func TestTopUp_AutoCreateWalletThenCredit(t *testing.T) {
	svc, mockRepo := setupService(t)

	userID := uuid.New()

	// First credit fails because wallet doesn't exist
	mockRepo.EXPECT().CreditWallet(gomock.Any(), userID, int64(50000), "top_up", gomock.Any(), gomock.Any()).
		Return(nil, repository.ErrWalletNotFound)
	// Auto-create wallet
	mockRepo.EXPECT().CreateWallet(gomock.Any(), userID).Return(&repository.Wallet{
		ID: uuid.New(), UserID: userID, Balance: 0,
	}, nil)
	// Second credit succeeds
	mockRepo.EXPECT().CreditWallet(gomock.Any(), userID, int64(50000), "top_up", gomock.Any(), gomock.Any()).
		Return(&repository.WalletTransaction{
			ID:           uuid.New(),
			Amount:       50000,
			BalanceAfter: 50000,
		}, nil)

	tx, err := svc.TopUp(context.Background(), userID, 50000, "bank_transfer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Amount != 50000 {
		t.Errorf("expected amount 50000, got %d", tx.Amount)
	}
}

func TestTopUp_AutoCreateFails(t *testing.T) {
	svc, mockRepo := setupService(t)

	userID := uuid.New()

	mockRepo.EXPECT().CreditWallet(gomock.Any(), userID, int64(50000), "top_up", gomock.Any(), gomock.Any()).
		Return(nil, repository.ErrWalletNotFound)
	mockRepo.EXPECT().CreateWallet(gomock.Any(), userID).Return(nil, errors.New("db error"))

	_, err := svc.TopUp(context.Background(), userID, 50000, "gopay")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestTopUp_CreditError(t *testing.T) {
	svc, mockRepo := setupService(t)

	userID := uuid.New()
	mockRepo.EXPECT().CreditWallet(gomock.Any(), userID, int64(50000), "top_up", gomock.Any(), gomock.Any()).
		Return(nil, errors.New("transaction failed"))

	_, err := svc.TopUp(context.Background(), userID, 50000, "gopay")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// ── Debit ────────────────────────────────────────────────────────────────────

func TestDebit_Success(t *testing.T) {
	svc, mockRepo := setupService(t)

	userID := uuid.New()
	refID := uuid.New()

	mockRepo.EXPECT().DebitWallet(gomock.Any(), userID, int64(15000), "debit", "ride payment", refID).
		Return(&repository.WalletTransaction{
			ID:           uuid.New(),
			Amount:       -15000,
			BalanceAfter: 35000,
		}, nil)

	tx, err := svc.Debit(context.Background(), userID, 15000, refID, "ride payment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.BalanceAfter != 35000 {
		t.Errorf("expected balance after 35000, got %d", tx.BalanceAfter)
	}
}

func TestDebit_InsufficientBalance(t *testing.T) {
	svc, mockRepo := setupService(t)

	userID := uuid.New()
	refID := uuid.New()

	mockRepo.EXPECT().DebitWallet(gomock.Any(), userID, int64(100000), "debit", "expensive ride", refID).
		Return(nil, repository.ErrInsufficientBalance)

	_, err := svc.Debit(context.Background(), userID, 100000, refID, "expensive ride")
	if err != repository.ErrInsufficientBalance {
		t.Errorf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestDebit_WalletNotFound(t *testing.T) {
	svc, mockRepo := setupService(t)

	userID := uuid.New()
	refID := uuid.New()

	mockRepo.EXPECT().DebitWallet(gomock.Any(), userID, int64(5000), "debit", "test", refID).
		Return(nil, repository.ErrWalletNotFound)

	_, err := svc.Debit(context.Background(), userID, 5000, refID, "test")
	if err != repository.ErrWalletNotFound {
		t.Errorf("expected ErrWalletNotFound, got %v", err)
	}
}

// ── Credit ───────────────────────────────────────────────────────────────────

func TestCredit_Success(t *testing.T) {
	svc, mockRepo := setupService(t)

	userID := uuid.New()
	refID := uuid.New()

	mockRepo.EXPECT().CreditWallet(gomock.Any(), userID, int64(10000), "refund", "Order refund", refID).
		Return(&repository.WalletTransaction{
			ID:           uuid.New(),
			Amount:       10000,
			BalanceAfter: 60000,
		}, nil)

	tx, err := svc.Credit(context.Background(), userID, 10000, refID, "refund", "Order refund")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.BalanceAfter != 60000 {
		t.Errorf("expected balance after 60000, got %d", tx.BalanceAfter)
	}
}

func TestCredit_Error(t *testing.T) {
	svc, mockRepo := setupService(t)

	userID := uuid.New()
	refID := uuid.New()

	mockRepo.EXPECT().CreditWallet(gomock.Any(), userID, int64(10000), "refund", "test", refID).
		Return(nil, errors.New("db error"))

	_, err := svc.Credit(context.Background(), userID, 10000, refID, "refund", "test")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
