//go:build functional

package functional

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/zicofarry/clay-app/backend/services/rating-service/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	dsn := "host=localhost user=clay_user password=clay_password dbname=rating_db port=5445 sslmode=disable TimeZone=UTC"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to test postgres: %v", err)
	}

	err = db.AutoMigrate(
		&repository.Rating{},
		&repository.DriverScoreAggregate{},
		&repository.MerchantScoreAggregate{},
	)
	if err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}

	db.Exec("TRUNCATE TABLE ratings, driver_score_aggregates, merchant_score_aggregates RESTART IDENTITY CASCADE;")

	return db
}

func TestRatingRepository_E2E(t *testing.T) {
	t.Log("Starting functional E2E test for Rating Service (Postgres Integration)...")
	
	db := setupTestDB(t)
	repo := repository.NewRatingRepository(db)
	ctx := context.Background()

	t.Run("Create and Get Rating", func(t *testing.T) {
		rID := uuid.New()
		orderID := uuid.New()
		raterID := uuid.New()
		rateeID := uuid.New()

		rating := &repository.Rating{
			ID:        rID,
			OrderID:   orderID,
			OrderType: "ride",
			RaterID:   raterID,
			RateeID:   rateeID,
			RateeType: "driver",
			Score:     5,
			Comment:   "Great driver!",
			Tags:      pq.StringArray{"friendly", "on-time"},
			CreatedAt: time.Now().UTC(),
		}

		err := repo.CreateRating(ctx, rating)
		if err != nil {
			t.Fatalf("failed to create rating: %v", err)
		}

		ratings, err := repo.GetRatingsByOrder(ctx, orderID)
		if err != nil {
			t.Fatalf("failed to get rating: %v", err)
		}

		if len(ratings) != 1 || ratings[0].ID != rID {
			t.Errorf("expected to find the rating, got %v", ratings)
		}

		t.Log("Successfully tested Postgres integration for rating")
	})
}
