//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreateZone_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil { t.Fatalf("failed to open mock db: %v", err) }
	defer db.Close()

	repo := NewGeoRepository(db)
	surcharge := 15000
	zone := &GeofenceZone{
		Name: "Bandara Husein", Type: "airport_surcharge",
		CenterLat: -6.9006, CenterLng: 107.5764, RadiusM: 2000,
		SurchargeAmount: &surcharge, IsActive: true,
	}

	mock.ExpectQuery(`^INSERT INTO geofence_zones`).
		WithArgs(zone.Name, zone.Type, zone.CenterLat, zone.CenterLng, zone.RadiusM, zone.SurchargeAmount, zone.IsActive).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow("zone-uuid-1", time.Now()))

	created, err := repo.CreateZone(context.Background(), zone)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if created.ID != "zone-uuid-1" { t.Errorf("expected zone-uuid-1, got %s", created.ID) }
	if err := mock.ExpectationsWereMet(); err != nil { t.Errorf("unfulfilled: %v", err) }
}

func TestListZones_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil { t.Fatalf("failed to open mock db: %v", err) }
	defer db.Close()

	repo := NewGeoRepository(db)
	surcharge := 15000
	mock.ExpectQuery(`^SELECT (.+) FROM geofence_zones WHERE is_active = true`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "type", "center_lat", "center_lng", "radius_m", "surcharge_amount", "is_active", "created_at",
		}).AddRow("zone-1", "Bandara", "airport_surcharge", -6.90, 107.57, 2000.0, &surcharge, true, time.Now()))

	zones, err := repo.ListZones(context.Background())
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if len(zones) != 1 { t.Errorf("expected 1 zone, got %d", len(zones)) }
}

func TestFindZonesByPoint_Found(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil { t.Fatalf("failed to open mock db: %v", err) }
	defer db.Close()

	repo := NewGeoRepository(db)
	surcharge := 15000
	mock.ExpectQuery(`^SELECT (.+) FROM geofence_zones`).
		WithArgs(-6.90, 107.57).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "type", "center_lat", "center_lng", "radius_m", "surcharge_amount", "is_active", "created_at",
		}).AddRow("zone-1", "Bandara", "airport_surcharge", -6.90, 107.57, 2000.0, &surcharge, true, time.Now()))

	zones, err := repo.FindZonesByPoint(context.Background(), -6.90, 107.57)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if len(zones) != 1 { t.Errorf("expected 1 zone, got %d", len(zones)) }
}
