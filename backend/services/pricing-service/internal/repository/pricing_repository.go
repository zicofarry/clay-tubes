// Package repository implements the data access layer for the Pricing Service.
// Stores fare_rules in PostgreSQL per service_type and zone.
package repository

import (
	"context"
	"database/sql"
	"time"
)

// ── Models ───────────────────────────────────────────────────────────────────

// FareRule represents a row in the `fare_rules` table.
type FareRule struct {
	ID                   string    `json:"id"`
	ServiceType          string    `json:"service_type"`           // ride, delivery, food
	VehicleType          *string   `json:"vehicle_type,omitempty"` // motorcycle, car (ride only)
	ZoneID               *string   `json:"zone_id,omitempty"`
	BaseFare             int       `json:"base_fare"`
	PerKmRate            int       `json:"per_km_rate"`
	PerMinRate           *int      `json:"per_min_rate,omitempty"`
	BookingFee           int       `json:"booking_fee"`
	ServiceFeePct        float64   `json:"service_fee_pct"`         // e.g. 0.05 = 5%
	MinFare              int       `json:"min_fare"`
	WeightRatePerKg      *int      `json:"weight_rate_per_kg,omitempty"`
	SmallOrderThreshold  *int      `json:"small_order_threshold,omitempty"`
	SmallOrderFee        *int      `json:"small_order_fee,omitempty"`
	IsActive             bool      `json:"is_active"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// ── Interface ────────────────────────────────────────────────────────────────

// PricingRepositoryInterface defines the contract for pricing data access.
//
//go:generate mockgen -source=pricing_repository.go -destination=../../mocks/repomock/mock_pricing_repository.go -package=repomock
type PricingRepositoryInterface interface {
	// GetFareRule returns the fare rule for a given service type, vehicle type, and zone.
	GetFareRule(ctx context.Context, serviceType string, vehicleType *string, zoneID *string) (*FareRule, error)

	// CreateFareRule inserts a new fare rule.
	CreateFareRule(ctx context.Context, rule *FareRule) (*FareRule, error)

	// ListFareRules returns all active fare rules.
	ListFareRules(ctx context.Context) ([]FareRule, error)
}

// ── Implementation ───────────────────────────────────────────────────────────

// PricingRepository implements PricingRepositoryInterface using PostgreSQL.
type PricingRepository struct {
	db *sql.DB
}

// NewPricingRepository creates a new PricingRepository.
func NewPricingRepository(db *sql.DB) *PricingRepository {
	return &PricingRepository{db: db}
}

func (r *PricingRepository) GetFareRule(ctx context.Context, serviceType string, vehicleType *string, zoneID *string) (*FareRule, error) {
	query := `
		SELECT id, service_type, vehicle_type, zone_id, base_fare, per_km_rate, per_min_rate,
		       booking_fee, service_fee_pct, min_fare, weight_rate_per_kg,
		       small_order_threshold, small_order_fee, is_active, created_at, updated_at
		FROM fare_rules
		WHERE service_type = $1 AND is_active = true
		  AND ($2::text IS NULL OR vehicle_type = $2)
		  AND ($3::uuid IS NULL OR zone_id = $3)
		ORDER BY zone_id NULLS LAST
		LIMIT 1
	`
	rule := &FareRule{}
	err := r.db.QueryRowContext(ctx, query, serviceType, vehicleType, zoneID).Scan(
		&rule.ID, &rule.ServiceType, &rule.VehicleType, &rule.ZoneID,
		&rule.BaseFare, &rule.PerKmRate, &rule.PerMinRate,
		&rule.BookingFee, &rule.ServiceFeePct, &rule.MinFare, &rule.WeightRatePerKg,
		&rule.SmallOrderThreshold, &rule.SmallOrderFee, &rule.IsActive,
		&rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return rule, nil
}

func (r *PricingRepository) CreateFareRule(ctx context.Context, rule *FareRule) (*FareRule, error) {
	query := `
		INSERT INTO fare_rules (service_type, vehicle_type, zone_id, base_fare, per_km_rate, per_min_rate,
		  booking_fee, service_fee_pct, min_fare, weight_rate_per_kg, small_order_threshold, small_order_fee, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRowContext(ctx, query,
		rule.ServiceType, rule.VehicleType, rule.ZoneID,
		rule.BaseFare, rule.PerKmRate, rule.PerMinRate,
		rule.BookingFee, rule.ServiceFeePct, rule.MinFare, rule.WeightRatePerKg,
		rule.SmallOrderThreshold, rule.SmallOrderFee, rule.IsActive,
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return rule, nil
}

func (r *PricingRepository) ListFareRules(ctx context.Context) ([]FareRule, error) {
	query := `
		SELECT id, service_type, vehicle_type, zone_id, base_fare, per_km_rate, per_min_rate,
		       booking_fee, service_fee_pct, min_fare, weight_rate_per_kg,
		       small_order_threshold, small_order_fee, is_active, created_at, updated_at
		FROM fare_rules WHERE is_active = true ORDER BY service_type, vehicle_type
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []FareRule
	for rows.Next() {
		var rule FareRule
		if err := rows.Scan(
			&rule.ID, &rule.ServiceType, &rule.VehicleType, &rule.ZoneID,
			&rule.BaseFare, &rule.PerKmRate, &rule.PerMinRate,
			&rule.BookingFee, &rule.ServiceFeePct, &rule.MinFare, &rule.WeightRatePerKg,
			&rule.SmallOrderThreshold, &rule.SmallOrderFee, &rule.IsActive,
			&rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}
