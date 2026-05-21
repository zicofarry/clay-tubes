//go:build functional

package functional

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/zicofarry/clay-app/backend/services/pricing-service/internal/repository"
)

func setupTestDB(t *testing.T) *sql.DB {
	dsn := "postgres://clay_user:clay_password@localhost:5442/pricing_db?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	for i := 0; i < 5; i++ {
		err = db.Ping()
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	schema := `
	CREATE EXTENSION IF NOT EXISTS "pgcrypto";
	DROP TABLE IF EXISTS fare_rules CASCADE;
	CREATE TABLE IF NOT EXISTS fare_rules (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		service_type VARCHAR(50) NOT NULL,
		vehicle_type VARCHAR(50),
		zone_id UUID,
		base_fare INTEGER NOT NULL DEFAULT 0,
		per_km_rate INTEGER NOT NULL DEFAULT 0,
		per_min_rate INTEGER,
		booking_fee INTEGER NOT NULL DEFAULT 0,
		service_fee_pct DOUBLE PRECISION NOT NULL DEFAULT 0.0,
		min_fare INTEGER NOT NULL DEFAULT 0,
		weight_rate_per_kg INTEGER,
		small_order_threshold INTEGER,
		small_order_fee INTEGER,
		is_active BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	TRUNCATE TABLE fare_rules CASCADE;
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("failed to create schema: %v", err)
	}
	return db
}

func TestPricingRepository_E2E(t *testing.T) {
	t.Log("Starting functional E2E test for Pricing Service (Database Integration)...")
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewPricingRepository(db)
	ctx := context.Background()

	t.Run("Create Ride Fare Rule", func(t *testing.T) {
		perMin := 300
		vehicleType := "motorcycle"
		rule := &repository.FareRule{
			ServiceType: "ride", VehicleType: &vehicleType,
			BaseFare: 7000, PerKmRate: 2500, PerMinRate: &perMin,
			BookingFee: 2000, ServiceFeePct: 0.05, MinFare: 10000,
			IsActive: true,
		}
		created, err := repo.CreateFareRule(ctx, rule)
		if err != nil {
			t.Fatalf("failed to create fare rule: %v", err)
		}
		t.Logf("Created fare rule ID: %s", created.ID)
		if created.ID == "" {
			t.Error("expected generated ID")
		}
	})

	t.Run("Create Delivery Fare Rule", func(t *testing.T) {
		weightRate := 2000
		rule := &repository.FareRule{
			ServiceType: "delivery",
			BaseFare: 8000, PerKmRate: 3000,
			BookingFee: 1000, ServiceFeePct: 0.03, MinFare: 10000,
			WeightRatePerKg: &weightRate, IsActive: true,
		}
		created, err := repo.CreateFareRule(ctx, rule)
		if err != nil {
			t.Fatalf("failed to create fare rule: %v", err)
		}
		t.Logf("Created delivery fare rule ID: %s", created.ID)
	})

	t.Run("Create Food Fare Rule", func(t *testing.T) {
		threshold := 20000
		smallFee := 5000
		rule := &repository.FareRule{
			ServiceType: "food",
			BaseFare: 5000, PerKmRate: 2000,
			BookingFee: 0, ServiceFeePct: 0.05, MinFare: 5000,
			SmallOrderThreshold: &threshold, SmallOrderFee: &smallFee, IsActive: true,
		}
		created, err := repo.CreateFareRule(ctx, rule)
		if err != nil {
			t.Fatalf("failed to create fare rule: %v", err)
		}
		t.Logf("Created food fare rule ID: %s", created.ID)
	})

	t.Run("List All Fare Rules", func(t *testing.T) {
		rules, err := repo.ListFareRules(ctx)
		if err != nil {
			t.Fatalf("failed to list fare rules: %v", err)
		}
		if len(rules) != 3 {
			t.Errorf("expected 3 rules, got %d", len(rules))
		}
		t.Logf("Listed %d fare rules successfully", len(rules))
	})

	t.Run("Get Ride Fare Rule", func(t *testing.T) {
		vehicleType := "motorcycle"
		rule, err := repo.GetFareRule(ctx, "ride", &vehicleType, nil)
		if err != nil {
			t.Fatalf("failed to get fare rule: %v", err)
		}
		if rule.BaseFare != 7000 {
			t.Errorf("expected base_fare 7000, got %d", rule.BaseFare)
		}
		t.Logf("Got ride fare rule: base=%d, per_km=%d", rule.BaseFare, rule.PerKmRate)
	})

	t.Run("Get Delivery Fare Rule", func(t *testing.T) {
		rule, err := repo.GetFareRule(ctx, "delivery", nil, nil)
		if err != nil {
			t.Fatalf("failed to get fare rule: %v", err)
		}
		if rule.BaseFare != 8000 {
			t.Errorf("expected base_fare 8000, got %d", rule.BaseFare)
		}
		t.Logf("Got delivery fare rule: base=%d, weight_rate=%d", rule.BaseFare, *rule.WeightRatePerKg)
	})
}
