//go:build functional

package functional

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/zicofarry/clay-app/backend/services/geo-service/internal/repository"
)

func setupTestDB(t *testing.T) *sql.DB {
	dsn := "postgres://clay_user:clay_password@localhost:5439/geo_db?sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil { t.Fatalf("failed to open database: %v", err) }

	for i := 0; i < 5; i++ {
		err = db.Ping()
		if err == nil { break }
		time.Sleep(1 * time.Second)
	}
	if err != nil { t.Fatalf("failed to connect to database: %v", err) }

	schema := `
	CREATE EXTENSION IF NOT EXISTS "pgcrypto";
	DROP TABLE IF EXISTS geofence_zones CASCADE;
	CREATE TABLE IF NOT EXISTS geofence_zones (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		name VARCHAR(255) NOT NULL,
		type VARCHAR(50) NOT NULL,
		center_lat DOUBLE PRECISION NOT NULL,
		center_lng DOUBLE PRECISION NOT NULL,
		radius_m DOUBLE PRECISION NOT NULL,
		surcharge_amount INTEGER,
		is_active BOOLEAN NOT NULL DEFAULT true,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);
	TRUNCATE TABLE geofence_zones CASCADE;
	`
	if _, err := db.Exec(schema); err != nil { t.Fatalf("failed to create schema: %v", err) }
	return db
}

func TestGeoRepository_E2E(t *testing.T) {
	t.Log("Starting functional E2E test for Geo Service (Database Integration)...")
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewGeoRepository(db)
	ctx := context.Background()

	t.Run("Create and List Geofence Zone", func(t *testing.T) {
		surcharge := 15000
		zone := &repository.GeofenceZone{
			Name: "Bandara Husein Sastranegara", Type: "airport_surcharge",
			CenterLat: -6.9006, CenterLng: 107.5764, RadiusM: 2000,
			SurchargeAmount: &surcharge, IsActive: true,
		}
		created, err := repo.CreateZone(ctx, zone)
		if err != nil { t.Fatalf("failed to create zone: %v", err) }
		t.Logf("Created zone ID: %s", created.ID)
		if created.ID == "" { t.Error("expected generated ID") }

		zones, err := repo.ListZones(ctx)
		if err != nil { t.Fatalf("failed to list zones: %v", err) }
		if len(zones) != 1 { t.Errorf("expected 1 zone, got %d", len(zones)) }
		t.Log("Zone created and listed successfully")
	})

	t.Run("Find Zone by Point Inside", func(t *testing.T) {
		zones, err := repo.FindZonesByPoint(ctx, -6.9006, 107.5764) // center point
		if err != nil { t.Fatalf("failed to find zones: %v", err) }
		if len(zones) != 1 { t.Errorf("expected 1 zone (point at center), got %d", len(zones)) }
		t.Log("Point-in-zone check works correctly")
	})

	t.Run("Find Zone by Point Outside", func(t *testing.T) {
		zones, err := repo.FindZonesByPoint(ctx, -6.5000, 107.0000) // far away
		if err != nil { t.Fatalf("failed to find zones: %v", err) }
		if len(zones) != 0 { t.Errorf("expected 0 zones (point outside), got %d", len(zones)) }
		t.Log("Point outside zone correctly returns empty")
	})
}
