//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *sql.DB) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})

	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	return gormDB, mock, db
}

func TestGetOrderHistoryByID_Found(t *testing.T) {
	gormDB, mock, db := setupMockDB(t)
	defer db.Close()

	repo := NewHistoryRepository(gormDB)

	id := uuid.New()
	orderID := uuid.New()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "driver_id", "order_id", "order_type", "service_type",
		"final_status", "origin_address", "dest_address", "fare_total", "payment_method", "completed_at",
	}).AddRow(
		id.String(), uuid.New().String(), nil, orderID.String(), "ride", "bike",
		"completed", "Origin", "Dest", 15000.0, "wallet", time.Now(),
	)

	mock.ExpectQuery(`^SELECT (.+) FROM "order_history" WHERE id = \$1 ORDER BY "order_history"."id" LIMIT \$2`).
		WithArgs(id.String(), 1).
		WillReturnRows(rows)

	history, err := repo.GetOrderHistoryByID(context.Background(), id)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if history == nil || history.OrderID != orderID {
		t.Errorf("expected order history with orderID %s", orderID)
	}
}

func TestGetOrderHistoryByID_NotFound(t *testing.T) {
	gormDB, mock, db := setupMockDB(t)
	defer db.Close()

	repo := NewHistoryRepository(gormDB)

	id := uuid.New()

	mock.ExpectQuery(`^SELECT (.+) FROM "order_history" WHERE id = \$1 ORDER BY "order_history"."id" LIMIT \$2`).
		WithArgs(id.String(), 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := repo.GetOrderHistoryByID(context.Background(), id)

	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
	}
}

func TestCreateActivityFeed_Success(t *testing.T) {
	gormDB, mock, db := setupMockDB(t)
	defer db.Close()

	repo := NewHistoryRepository(gormDB)

	feed := &ActivityFeed{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		EventType: "promo_applied",
		Title:     "Promo Applied",
		CreatedAt: time.Now(),
	}

	mock.ExpectBegin()
	mock.ExpectExec(`^INSERT INTO "activity_feed"`).
		WithArgs(
			feed.ID.String(),
			feed.UserID.String(),
			feed.EventType,
			feed.Title,
			sqlmock.AnyArg(), // Description
			sqlmock.AnyArg(), // Metadata
			sqlmock.AnyArg(), // OrderID
			sqlmock.AnyArg(), // CreatedAt
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.CreateActivityFeed(context.Background(), feed)

	if err != nil {
		t.Errorf("unexpected error while inserting feed: %s", err)
	}
}
