//go:build functional

package functional

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/promotion-service/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	dsn := "host=localhost user=clay_user password=clay_password dbname=promotion_db port=5443 sslmode=disable TimeZone=UTC"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to test postgres: %v", err)
	}

	err = db.AutoMigrate(
		&repository.PromoCode{},
		&repository.PromoTarget{},
		&repository.UserPromo{},
		&repository.PromoUsage{},
	)
	if err != nil {
		t.Fatalf("failed to migrate tables: %v", err)
	}

	db.Exec("TRUNCATE TABLE promo_codes, promo_targets, user_promos, promo_usages RESTART IDENTITY CASCADE;")

	return db
}

func TestPromotionRepository_E2E(t *testing.T) {
	t.Log("Starting functional E2E test for Promotion Service (Postgres Integration)...")
	
	db := setupTestDB(t)
	repo := repository.NewPromotionRepository(db)
	ctx := context.Background()

	t.Run("Create and Get Promo", func(t *testing.T) {
		promo := &repository.PromoCode{
			ID:          uuid.New(),
			Code:        "E2ETEST10",
			Type:        "fixed_off",
			Value:       10000,
			ServiceType: "all",
			IsActive:    true,
			ValidFrom:   time.Now().UTC(),
			ValidUntil:  time.Now().Add(24 * time.Hour).UTC(),
		}

		err := repo.CreatePromoCode(ctx, promo)
		if err != nil {
			t.Fatalf("failed to create promo: %v", err)
		}

		found, err := repo.GetPromoCodeByCode(ctx, "E2ETEST10")
		if err != nil {
			t.Fatalf("failed to get promo: %v", err)
		}

		if found.ID != promo.ID {
			t.Errorf("expected promo ID %s, got %s", promo.ID, found.ID)
		}

		t.Log("Successfully tested Postgres integration for promo code")
	})
}
