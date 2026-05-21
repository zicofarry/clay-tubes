//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

// ── GetWalletByUserID ────────────────────────────────────────────────────────

func TestGetWalletByUserID_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewWalletRepository(db)

	userID := uuid.New()
	walletID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`^SELECT (.+) FROM wallets WHERE user_id = \$1$`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "is_active", "created_at", "updated_at"}).
			AddRow(walletID, userID, 50000, true, now, now))

	wallet, err := repo.GetWalletByUserID(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wallet.ID != walletID {
		t.Errorf("expected wallet ID %s, got %s", walletID, wallet.ID)
	}
	if wallet.Balance != 50000 {
		t.Errorf("expected balance 50000, got %d", wallet.Balance)
	}
	if !wallet.IsActive {
		t.Error("expected wallet to be active")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestGetWalletByUserID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewWalletRepository(db)
	userID := uuid.New()

	mock.ExpectQuery(`^SELECT (.+) FROM wallets WHERE user_id = \$1$`).
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)

	wallet, err := repo.GetWalletByUserID(context.Background(), userID)
	if err != ErrWalletNotFound {
		t.Errorf("expected ErrWalletNotFound, got %v", err)
	}
	if wallet != nil {
		t.Errorf("expected nil wallet, got %+v", wallet)
	}
}

// ── CreateWallet ─────────────────────────────────────────────────────────────

func TestCreateWallet_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewWalletRepository(db)
	userID := uuid.New()
	walletID := uuid.New()
	now := time.Now()

	mock.ExpectQuery(`^INSERT INTO wallets`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "balance", "is_active", "created_at", "updated_at"}).
			AddRow(walletID, userID, 0, true, now, now))

	wallet, err := repo.CreateWallet(context.Background(), userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wallet.UserID != userID {
		t.Errorf("expected user ID %s, got %s", userID, wallet.UserID)
	}
	if wallet.Balance != 0 {
		t.Errorf("expected balance 0, got %d", wallet.Balance)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestCreateWallet_DBError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewWalletRepository(db)
	userID := uuid.New()

	mock.ExpectQuery(`^INSERT INTO wallets`).
		WithArgs(userID).
		WillReturnError(sql.ErrConnDone)

	wallet, err := repo.CreateWallet(context.Background(), userID)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if wallet != nil {
		t.Errorf("expected nil wallet on error, got %+v", wallet)
	}
}

// ── CreditWallet ─────────────────────────────────────────────────────────────

func TestCreditWallet_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewWalletRepository(db)

	userID := uuid.New()
	walletID := uuid.New()
	refID := uuid.New()
	txID := uuid.New()
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery(`^SELECT id, balance FROM wallets WHERE user_id = \$1 FOR UPDATE$`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance"}).AddRow(walletID, int64(10000)))
	mock.ExpectExec(`^UPDATE wallets SET balance = \$1, updated_at = NOW\(\) WHERE id = \$2$`).
		WithArgs(int64(15000), walletID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`^INSERT INTO wallet_transactions`).
		WithArgs(walletID, "top_up", int64(5000), int64(15000), refID, "Top up test").
		WillReturnRows(sqlmock.NewRows([]string{"id", "wallet_id", "type", "amount", "balance_after", "reference_id", "description", "created_at"}).
			AddRow(txID, walletID, "top_up", int64(5000), int64(15000), refID, "Top up test", now))
	mock.ExpectCommit()

	tx, err := repo.CreditWallet(context.Background(), userID, 5000, "top_up", "Top up test", refID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.BalanceAfter != 15000 {
		t.Errorf("expected balance after 15000, got %d", tx.BalanceAfter)
	}
	if tx.Amount != 5000 {
		t.Errorf("expected amount 5000, got %d", tx.Amount)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestCreditWallet_WalletNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewWalletRepository(db)
	userID := uuid.New()
	refID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(`^SELECT id, balance FROM wallets WHERE user_id = \$1 FOR UPDATE$`).
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err = repo.CreditWallet(context.Background(), userID, 5000, "top_up", "test", refID)
	if err != ErrWalletNotFound {
		t.Errorf("expected ErrWalletNotFound, got %v", err)
	}
}

// ── DebitWallet ──────────────────────────────────────────────────────────────

func TestDebitWallet_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewWalletRepository(db)

	userID := uuid.New()
	walletID := uuid.New()
	refID := uuid.New()
	txID := uuid.New()
	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery(`^SELECT id, balance FROM wallets WHERE user_id = \$1 FOR UPDATE$`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance"}).AddRow(walletID, int64(20000)))
	mock.ExpectExec(`^UPDATE wallets SET balance = \$1, updated_at = NOW\(\) WHERE id = \$2$`).
		WithArgs(int64(15000), walletID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`^INSERT INTO wallet_transactions`).
		WithArgs(walletID, "debit", int64(-5000), int64(15000), refID, "Payment for order").
		WillReturnRows(sqlmock.NewRows([]string{"id", "wallet_id", "type", "amount", "balance_after", "reference_id", "description", "created_at"}).
			AddRow(txID, walletID, "debit", int64(-5000), int64(15000), refID, "Payment for order", now))
	mock.ExpectCommit()

	tx, err := repo.DebitWallet(context.Background(), userID, 5000, "debit", "Payment for order", refID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.BalanceAfter != 15000 {
		t.Errorf("expected balance after 15000, got %d", tx.BalanceAfter)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestDebitWallet_InsufficientBalance(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewWalletRepository(db)
	userID := uuid.New()
	walletID := uuid.New()
	refID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(`^SELECT id, balance FROM wallets WHERE user_id = \$1 FOR UPDATE$`).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "balance"}).AddRow(walletID, int64(3000)))
	mock.ExpectRollback()

	_, err = repo.DebitWallet(context.Background(), userID, 5000, "debit", "test", refID)
	if err != ErrInsufficientBalance {
		t.Errorf("expected ErrInsufficientBalance, got %v", err)
	}
}

func TestDebitWallet_WalletNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewWalletRepository(db)
	userID := uuid.New()
	refID := uuid.New()

	mock.ExpectBegin()
	mock.ExpectQuery(`^SELECT id, balance FROM wallets WHERE user_id = \$1 FOR UPDATE$`).
		WithArgs(userID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, err = repo.DebitWallet(context.Background(), userID, 5000, "debit", "test", refID)
	if err != ErrWalletNotFound {
		t.Errorf("expected ErrWalletNotFound, got %v", err)
	}
}
