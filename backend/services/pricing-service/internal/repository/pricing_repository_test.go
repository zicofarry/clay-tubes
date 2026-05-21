//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetFareRule_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewPricingRepository(db)
	perMin := 300
	vehicleType := "motorcycle"

	mock.ExpectQuery(`^SELECT (.+) FROM fare_rules`).
		WithArgs("ride", &vehicleType, nil).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "service_type", "vehicle_type", "zone_id",
			"base_fare", "per_km_rate", "per_min_rate",
			"booking_fee", "service_fee_pct", "min_fare", "weight_rate_per_kg",
			"small_order_threshold", "small_order_fee", "is_active",
			"created_at", "updated_at",
		}).AddRow(
			"rule-1", "ride", &vehicleType, nil,
			7000, 2500, &perMin,
			2000, 0.05, 10000, nil,
			nil, nil, true,
			time.Now(), time.Now(),
		))

	rule, err := repo.GetFareRule(context.Background(), "ride", &vehicleType, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.BaseFare != 7000 {
		t.Errorf("expected base_fare 7000, got %d", rule.BaseFare)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled: %v", err)
	}
}

func TestCreateFareRule_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewPricingRepository(db)
	rule := &FareRule{
		ServiceType: "ride", BaseFare: 7000, PerKmRate: 2500,
		BookingFee: 2000, ServiceFeePct: 0.05, MinFare: 10000, IsActive: true,
	}

	mock.ExpectQuery(`^INSERT INTO fare_rules`).
		WithArgs(rule.ServiceType, rule.VehicleType, rule.ZoneID,
			rule.BaseFare, rule.PerKmRate, rule.PerMinRate,
			rule.BookingFee, rule.ServiceFeePct, rule.MinFare, rule.WeightRatePerKg,
			rule.SmallOrderThreshold, rule.SmallOrderFee, rule.IsActive).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow("rule-uuid-1", time.Now(), time.Now()))

	created, err := repo.CreateFareRule(context.Background(), rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID != "rule-uuid-1" {
		t.Errorf("expected rule-uuid-1, got %s", created.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled: %v", err)
	}
}

func TestListFareRules_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	repo := NewPricingRepository(db)
	mock.ExpectQuery(`^SELECT (.+) FROM fare_rules WHERE is_active = true`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "service_type", "vehicle_type", "zone_id",
			"base_fare", "per_km_rate", "per_min_rate",
			"booking_fee", "service_fee_pct", "min_fare", "weight_rate_per_kg",
			"small_order_threshold", "small_order_fee", "is_active",
			"created_at", "updated_at",
		}).AddRow(
			"rule-1", "ride", nil, nil,
			7000, 2500, nil,
			2000, 0.05, 10000, nil,
			nil, nil, true,
			time.Now(), time.Now(),
		))

	rules, err := repo.ListFareRules(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rules))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled: %v", err)
	}
}
