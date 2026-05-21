// Package repository implements the data access layer for the Geo Service.
// Only geofence zone definitions are stored in PostgreSQL.
// Driver locations and ETA data are stored in Redis (see cache package).
package repository

import (
	"context"
	"database/sql"
	"time"
)

// ── Models ───────────────────────────────────────────────────────────────────

// GeofenceZone represents a row in the `geofence_zones` table.
type GeofenceZone struct {
	ID              string   `json:"zone_id"`
	Name            string   `json:"name"`
	Type            string   `json:"type"` // airport_surcharge, no_pickup, surge_pricing, restricted
	CenterLat       float64  `json:"center_lat"`
	CenterLng       float64  `json:"center_lng"`
	RadiusM         float64  `json:"radius_m"`
	SurchargeAmount *int     `json:"surcharge_amount,omitempty"`
	IsActive        bool     `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
}

// ── Interface ────────────────────────────────────────────────────────────────

// GeoRepositoryInterface defines the contract for geo data access.
//
//go:generate mockgen -source=geo_repository.go -destination=../../mocks/repomock/mock_geo_repository.go -package=repomock
type GeoRepositoryInterface interface {
	// Geofence Zones
	CreateZone(ctx context.Context, zone *GeofenceZone) (*GeofenceZone, error)
	ListZones(ctx context.Context) ([]GeofenceZone, error)
	FindZonesByPoint(ctx context.Context, lat, lng float64) ([]GeofenceZone, error)
}

// ── Implementation ───────────────────────────────────────────────────────────

// GeoRepository implements GeoRepositoryInterface using PostgreSQL.
type GeoRepository struct {
	db *sql.DB
}

// NewGeoRepository creates a new GeoRepository.
func NewGeoRepository(db *sql.DB) *GeoRepository {
	return &GeoRepository{db: db}
}

func (r *GeoRepository) CreateZone(ctx context.Context, zone *GeofenceZone) (*GeofenceZone, error) {
	query := `
		INSERT INTO geofence_zones (name, type, center_lat, center_lng, radius_m, surcharge_amount, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, query,
		zone.Name, zone.Type, zone.CenterLat, zone.CenterLng,
		zone.RadiusM, zone.SurchargeAmount, zone.IsActive,
	).Scan(&zone.ID, &zone.CreatedAt)
	if err != nil {
		return nil, err
	}
	return zone, nil
}

func (r *GeoRepository) ListZones(ctx context.Context) ([]GeofenceZone, error) {
	query := `
		SELECT id, name, type, center_lat, center_lng, radius_m, surcharge_amount, is_active, created_at
		FROM geofence_zones WHERE is_active = true ORDER BY name
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zones []GeofenceZone
	for rows.Next() {
		var z GeofenceZone
		if err := rows.Scan(
			&z.ID, &z.Name, &z.Type, &z.CenterLat, &z.CenterLng,
			&z.RadiusM, &z.SurchargeAmount, &z.IsActive, &z.CreatedAt,
		); err != nil {
			return nil, err
		}
		zones = append(zones, z)
	}
	return zones, rows.Err()
}

// FindZonesByPoint finds all active zones that contain the given lat/lng.
// Uses simple radius-based check (Haversine approximation).
func (r *GeoRepository) FindZonesByPoint(ctx context.Context, lat, lng float64) ([]GeofenceZone, error) {
	// Approximate: 1 degree latitude ≈ 111,320 meters
	// We use PostgreSQL to compute distance using the Haversine formula
	query := `
		SELECT id, name, type, center_lat, center_lng, radius_m, surcharge_amount, is_active, created_at
		FROM geofence_zones
		WHERE is_active = true
		  AND (
		    6371000 * acos(
		      cos(radians($1)) * cos(radians(center_lat)) *
		      cos(radians(center_lng) - radians($2)) +
		      sin(radians($1)) * sin(radians(center_lat))
		    )
		  ) <= radius_m
	`
	rows, err := r.db.QueryContext(ctx, query, lat, lng)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zones []GeofenceZone
	for rows.Next() {
		var z GeofenceZone
		if err := rows.Scan(
			&z.ID, &z.Name, &z.Type, &z.CenterLat, &z.CenterLng,
			&z.RadiusM, &z.SurchargeAmount, &z.IsActive, &z.CreatedAt,
		); err != nil {
			return nil, err
		}
		zones = append(zones, z)
	}
	return zones, rows.Err()
}
