//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreatePaymentMethod_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewPaymentRepository(db, nil)

	pm := &PaymentMethod{
		UserID:      "user-123",
		Type:        "credit_card",
		DisplayName: "Visa •••• 1234",
		IsDefault:   true,
	}

	mock.ExpectQuery(`^INSERT INTO payment_methods`).
		WithArgs(pm.UserID, pm.Type, pm.DisplayName, pm.LastFour, pm.ExpiryMonth, pm.ExpiryYear, pm.IsDefault, pm.CardToken).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
			AddRow("pm-uuid-123", time.Now()))

	created, err := repo.CreatePaymentMethod(context.Background(), pm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID != "pm-uuid-123" {
		t.Errorf("expected id pm-uuid-123, got %s", created.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestCreateTransaction_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewPaymentRepository(db, nil)
	orderID := "ord-123"
	tx := &Transaction{
		UserID:            "user-123",
		OrderID:           &orderID,
		Type:              "charge",
		Status:            "completed",
		Amount:            50000,
		PaymentMethodType: "clay_wallet",
		Description:       "ClayRide payment",
	}

	mock.ExpectQuery(`^INSERT INTO transactions`).
		WithArgs(tx.UserID, tx.OrderID, tx.Type, tx.Status, tx.Amount, tx.PaymentMethodType, tx.Description).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow("tx-uuid-123", time.Now(), time.Now()))

	created, err := repo.CreateTransaction(context.Background(), tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID != "tx-uuid-123" {
		t.Errorf("expected id tx-uuid-123, got %s", created.ID)
	}
}

func TestFindTransactionByID_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewPaymentRepository(db, nil)
	orderID := "ord-123"

	mock.ExpectQuery(`^SELECT (.+) FROM transactions WHERE id = \$1$`).
		WithArgs("tx-123").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "order_id", "type", "status", "amount",
			"payment_method_type", "description", "gateway_reference", "created_at", "updated_at",
		}).AddRow("tx-123", "user-1", &orderID, "charge", "completed", 50000,
			"clay_wallet", "test", nil, time.Now(), time.Now()))

	tx, err := repo.FindTransactionByID(context.Background(), "tx-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tx.Amount != 50000 {
		t.Errorf("expected amount 50000, got %d", tx.Amount)
	}
}

func TestFindTransactionByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewPaymentRepository(db, nil)

	mock.ExpectQuery(`^SELECT (.+) FROM transactions WHERE id = \$1$`).
		WithArgs("unknown").
		WillReturnError(sql.ErrNoRows)

	_, err = repo.FindTransactionByID(context.Background(), "unknown")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}
