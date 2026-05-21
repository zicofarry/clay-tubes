// Package geo defines the HTTP client contract for clay-geo-service.
//
// The matching service maintains its own Redis GEO index for fast nearby-driver
// lookups during dispatch, but the clay-geo-service is the system-of-record for
// driver locations across the platform. This interface lets the service layer
// proxy go-online/go-offline/location-update events to clay-geo-service without
// being coupled to its HTTP wire format.
package geo

import (
	"context"
)

// LocationUpdate carries the bare-minimum fields a geo service needs.
type LocationUpdate struct {
	DriverID    string  `json:"driver_id"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	Bearing     float64 `json:"bearing,omitempty"`
	SpeedKmh    float64 `json:"speed_kmh,omitempty"`
	VehicleType string  `json:"vehicle_type,omitempty"`
}

// Client is the contract for clay-geo-service interactions.
//
//go:generate mockgen -source=geo_client.go -destination=../../mocks/geomock/mock_geo_client.go -package=geomock
type Client interface {
	// RegisterDriver tells the geo service the driver is now online at a location.
	RegisterDriver(ctx context.Context, u LocationUpdate) error

	// UnregisterDriver removes the driver from the geo index when going offline.
	UnregisterDriver(ctx context.Context, driverID string) error

	// UpdateLocation pushes a real-time location update for an online driver.
	UpdateLocation(ctx context.Context, u LocationUpdate) error
}

// NoopClient is a stub implementation used when no geo service is wired up.
// All methods succeed silently — useful for local development & tests where
// only the local Redis GEO index matters.
type NoopClient struct{}

// NewNoopClient returns a NoopClient.
func NewNoopClient() *NoopClient { return &NoopClient{} }

func (NoopClient) RegisterDriver(_ context.Context, _ LocationUpdate) error   { return nil }
func (NoopClient) UnregisterDriver(_ context.Context, _ string) error         { return nil }
func (NoopClient) UpdateLocation(_ context.Context, _ LocationUpdate) error   { return nil }
