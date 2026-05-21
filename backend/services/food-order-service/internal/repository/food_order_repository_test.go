//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/model"
)

func TestCreate_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// Initialize repository with mocked DB, and nil for mongo/redis since they are unused in this test
	repo := NewFoodOrderRepository(db, nil, nil)

	order := &model.FoodOrder{
		UserID:          "user-123",
		MerchantID:      "merchant-456",
		Status:          model.StatusPending,
		PaymentMethod:   model.PaymentGoPay,
		SubtotalCents:   20000,
		DeliveryFee:     10000,
		DiscountCents:   0,
		TotalCents:      30000,
		DeliveryLat:     -6.2,
		DeliveryLng:     106.8,
		DeliveryAddress: "Test Address",
	}

	mock.ExpectExec(`^INSERT INTO food_orders`).
		WithArgs(
			sqlmock.AnyArg(), // id (generated uuid)
			order.UserID,
			order.MerchantID,
			order.Status,
			order.PaymentMethod,
			order.SubtotalCents,
			order.DeliveryFee,
			order.DiscountCents,
			order.TotalCents,
			order.PromoCode,
			order.Notes,
			order.DeliveryLat,
			order.DeliveryLng,
			order.DeliveryAddress,
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.Create(context.Background(), order)

	if err != nil {
		t.Errorf("error was not expected while inserting order: %s", err)
	}

	if order.ID == "" {
		t.Errorf("expected id to be generated")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestGetByID_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewFoodOrderRepository(db, nil, nil)

	now := time.Now()

	mock.ExpectQuery(`^SELECT (.+) FROM food_orders WHERE id = \$1$`).
		WithArgs("order-123").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "merchant_id", "driver_id", "status", "payment_method",
			"payment_hold_id", "subtotal_cents", "delivery_fee_cents", "discount_cents",
			"total_cents", "promo_code", "notes", "est_prep_time_min", "cancelled_by",
			"cancel_reason", "rating_submitted", "confirmed_at", "cancel_deadline",
			"delivered_at", "delivery_lat", "delivery_lng", "delivery_address",
			"created_at", "updated_at",
		}).AddRow(
			"order-123", "user-123", "merchant-456", nil, "pending", "gopay",
			nil, 20000, 10000, 0,
			30000, nil, nil, nil, nil,
			nil, false, nil, nil,
			nil, -6.2, 106.8, "Test Address",
			now, now,
		))

	order, err := repo.GetByID(context.Background(), "order-123")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if order == nil || order.TotalCents != 30000 {
		t.Errorf("expected order with total 30000")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewFoodOrderRepository(db, nil, nil)

	mock.ExpectQuery(`^SELECT (.+) FROM food_orders WHERE id = \$1$`).
		WithArgs("unknown").
		WillReturnError(sql.ErrNoRows)

	order, err := repo.GetByID(context.Background(), "unknown")

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if order != nil {
		t.Errorf("expected nil order, got %v", order)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestUpdateStatus_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewFoodOrderRepository(db, nil, nil)

	mock.ExpectExec(`^UPDATE food_orders SET status = \$1, updated_at = \$2 WHERE id = \$3$`).
		WithArgs(model.StatusPreparing, sqlmock.AnyArg(), "order-123").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.UpdateStatus(context.Background(), "order-123", model.StatusPreparing, nil)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
