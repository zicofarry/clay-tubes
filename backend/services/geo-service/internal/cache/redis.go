// Package cache provides Redis-backed storage for driver locations (Geo),
// ETA tracking, and Maps API response caching.
//
// In production, this wraps Redis Geo commands (GEOADD, GEORADIUS).
// For local dev/testing, InMemoryGeoCache provides an in-memory substitute
// using the Haversine formula.
package cache

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ── Data Types ───────────────────────────────────────────────────────────────

// DriverLocation holds a driver's current position and metadata.
type DriverLocation struct {
	DriverID  string    `json:"driver_id"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Bearing   float64   `json:"bearing"`
	SpeedKmh  float64   `json:"speed_kmh"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NearbyDriver is a driver found within a radius search.
type NearbyDriver struct {
	DriverID   string  `json:"driver_id"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	DistanceKm float64 `json:"distance_km"`
	Bearing    float64 `json:"bearing"`
}

// ETAData holds live ETA information for a driver/order pair.
type ETAData struct {
	DriverID            string    `json:"driver_id"`
	OrderID             string    `json:"order_id"`
	ETASeconds          int       `json:"eta_seconds"`
	ETAText             string    `json:"eta_text"`
	DistanceRemainingKm float64   `json:"distance_remaining_km"`
	DestinationType     string    `json:"destination_type"` // pickup, delivery
	UpdatedAt           time.Time `json:"updated_at"`
}

// ── Interface ────────────────────────────────────────────────────────────────

// GeoCacheInterface defines the contract for geo caching operations.
type GeoCacheInterface interface {
	// Driver Locations
	UpdateDriverLocation(ctx context.Context, serviceType string, loc *DriverLocation) error
	GetDriverLocation(ctx context.Context, driverID string) (*DriverLocation, error)
	FindNearbyDrivers(ctx context.Context, serviceType string, lat, lng, radiusKm float64, limit int) ([]NearbyDriver, error)
	RemoveDriver(ctx context.Context, serviceType, driverID string) error
	BatchGetDriverLocations(ctx context.Context, driverIDs []string) (map[string]*DriverLocation, error)

	// ETA
	SetETA(ctx context.Context, eta *ETAData) error
	GetETA(ctx context.Context, driverID, orderID string) (*ETAData, error)
	DeleteETA(ctx context.Context, driverID, orderID string) error
}

// ── In-Memory Implementation ─────────────────────────────────────────────────

// InMemoryGeoCache implements GeoCacheInterface using in-memory data structures.
// Uses Haversine formula for distance calculations.
type InMemoryGeoCache struct {
	mu        sync.RWMutex
	drivers   map[string]*DriverLocation            // driverID → location
	geoSets   map[string]map[string]*DriverLocation  // serviceType → driverID → location
	etas      map[string]*ETAData                    // "driverID:orderID" → eta
}

// NewInMemoryGeoCache creates a new in-memory geo cache.
func NewInMemoryGeoCache() *InMemoryGeoCache {
	return &InMemoryGeoCache{
		drivers: make(map[string]*DriverLocation),
		geoSets: make(map[string]map[string]*DriverLocation),
		etas:    make(map[string]*ETAData),
	}
}

func (c *InMemoryGeoCache) UpdateDriverLocation(_ context.Context, serviceType string, loc *DriverLocation) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	loc.UpdatedAt = time.Now()
	c.drivers[loc.DriverID] = loc

	if c.geoSets[serviceType] == nil {
		c.geoSets[serviceType] = make(map[string]*DriverLocation)
	}
	c.geoSets[serviceType][loc.DriverID] = loc
	return nil
}

func (c *InMemoryGeoCache) GetDriverLocation(_ context.Context, driverID string) (*DriverLocation, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	loc, ok := c.drivers[driverID]
	if !ok {
		return nil, fmt.Errorf("driver not found")
	}
	return loc, nil
}

func (c *InMemoryGeoCache) FindNearbyDrivers(_ context.Context, serviceType string, lat, lng, radiusKm float64, limit int) ([]NearbyDriver, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	set, ok := c.geoSets[serviceType]
	if !ok {
		return []NearbyDriver{}, nil
	}

	var results []NearbyDriver
	for _, loc := range set {
		dist := haversineKm(lat, lng, loc.Lat, loc.Lng)
		if dist <= radiusKm {
			results = append(results, NearbyDriver{
				DriverID: loc.DriverID, Lat: loc.Lat, Lng: loc.Lng,
				DistanceKm: math.Round(dist*100) / 100, Bearing: loc.Bearing,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].DistanceKm < results[j].DistanceKm
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func (c *InMemoryGeoCache) RemoveDriver(_ context.Context, serviceType, driverID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.drivers, driverID)
	if set, ok := c.geoSets[serviceType]; ok {
		delete(set, driverID)
	}
	return nil
}

func (c *InMemoryGeoCache) BatchGetDriverLocations(_ context.Context, driverIDs []string) (map[string]*DriverLocation, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*DriverLocation, len(driverIDs))
	for _, id := range driverIDs {
		if loc, ok := c.drivers[id]; ok {
			result[id] = loc
		}
	}
	return result, nil
}

func (c *InMemoryGeoCache) SetETA(_ context.Context, eta *ETAData) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	eta.UpdatedAt = time.Now()
	key := eta.DriverID + ":" + eta.OrderID
	c.etas[key] = eta
	return nil
}

func (c *InMemoryGeoCache) GetETA(_ context.Context, driverID, orderID string) (*ETAData, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := driverID + ":" + orderID
	eta, ok := c.etas[key]
	if !ok {
		return nil, fmt.Errorf("eta not found")
	}
	return eta, nil
}

func (c *InMemoryGeoCache) DeleteETA(_ context.Context, driverID, orderID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := driverID + ":" + orderID
	delete(c.etas, key)
	return nil
}

// ── Haversine ────────────────────────────────────────────────────────────────

// haversineKm calculates the great-circle distance between two points in km.
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0 // Earth radius in km
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func toRad(deg float64) float64 {
	return deg * math.Pi / 180
}

// HaversineKm is an exported version for use by the service layer.
func HaversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	return haversineKm(lat1, lng1, lat2, lng2)
}
