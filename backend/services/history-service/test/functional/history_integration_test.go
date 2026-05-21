//go:build functional

package functional

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/history-service/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	dsn := "host=localhost user=clay_user password=clay_password dbname=history_db port=5453 sslmode=disable TimeZone=UTC"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to test postgres: %v", err)
	}

	// Migrate test tables
	err = db.AutoMigrate(&repository.OrderHistory{}, &repository.ActivityFeed{})
	if err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}

	// Clean up tables
	db.Exec("TRUNCATE TABLE order_history, activity_feed RESTART IDENTITY CASCADE;")

	return db
}

func TestHistoryRepository_E2E(t *testing.T) {
	t.Log("Starting functional E2E test for History Service (Postgres Integration)...")
	
	db := setupTestDB(t)
	repo := repository.NewHistoryRepository(db)
	ctx := context.Background()

	t.Run("Create and Get Order History", func(t *testing.T) {
		orderID := uuid.New()
		userID := uuid.New()

		history := &repository.OrderHistory{
			ID:          uuid.New(),
			OrderID:     orderID,
			UserID:      userID,
			OrderType:   "ride",
			FinalStatus: "completed",
			CompletedAt: time.Now().UTC(),
		}

		err := repo.CreateOrUpdateOrderHistory(ctx, history)
		if err != nil {
			t.Fatalf("failed to create order history: %v", err)
		}

		found, err := repo.GetOrderHistoryByOrderID(ctx, orderID)
		if err != nil {
			t.Fatalf("failed to get order history: %v", err)
		}

		if found.ID != history.ID {
			t.Errorf("expected history ID %s, got %s", history.ID, found.ID)
		}

		t.Log("Successfully tested Postgres integration for order history")
	})
}
