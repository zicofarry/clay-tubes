//go:build unit

package service

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/geo-service/internal/cache"
	"github.com/zicofarry/clay-app/backend/services/geo-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/geo-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

func newTestService(t *testing.T) (*GeoService, *repomock.MockGeoRepositoryInterface, *cache.InMemoryGeoCache) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockGeoRepositoryInterface(ctrl)
	geoCache := cache.NewInMemoryGeoCache()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewGeoService(mockRepo, geoCache, logger)
	return svc, mockRepo, geoCache
}

func TestUpdateAndGetDriverLocation(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	err := svc.UpdateDriverLocation(ctx, "drv-1", &UpdateLocationRequest{
		Lat: -6.9733, Lng: 107.6310, ServiceType: "ride", Bearing: 180, SpeedKmh: 35,
	})
	if err != nil { t.Fatalf("unexpected error: %v", err) }

	loc, err := svc.GetDriverLocation(ctx, "drv-1")
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if loc.Lat != -6.9733 { t.Errorf("expected lat -6.9733, got %f", loc.Lat) }
}

func TestGetDriverLocation_NotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.GetDriverLocation(context.Background(), "nonexistent")
	if err != ErrDriverNotFound { t.Errorf("expected ErrDriverNotFound, got %v", err) }
}

func TestFindNearbyDrivers_WithResults(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	// Place drivers at known positions around Bandung
	svc.UpdateDriverLocation(ctx, "drv-near", &UpdateLocationRequest{Lat: -6.9740, Lng: 107.6320, ServiceType: "ride"})
	svc.UpdateDriverLocation(ctx, "drv-far", &UpdateLocationRequest{Lat: -7.0500, Lng: 107.7000, ServiceType: "ride"})

	result, err := svc.FindNearbyDrivers(ctx, -6.9733, 107.6310, 2.0, "ride", 10)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if result.Total != 1 { t.Errorf("expected 1 nearby driver, got %d", result.Total) }
	if result.Drivers[0].DriverID != "drv-near" { t.Errorf("expected drv-near, got %s", result.Drivers[0].DriverID) }
}

func TestEstimateRoute_Haversine(t *testing.T) {
	svc, _, _ := newTestService(t)
	result, err := svc.EstimateRoute(context.Background(), &RouteEstimateRequest{
		Origin: LatLng{Lat: -6.9733, Lng: 107.6310}, Destination: LatLng{Lat: -6.9000, Lng: 107.6000},
	})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if result.DistanceKm < 5.0 || result.DistanceKm > 15.0 {
		t.Errorf("expected reasonable distance, got %f km", result.DistanceKm)
	}
	if result.DurationSeconds <= 0 { t.Error("expected positive duration") }
}

func TestCalculateDistance_Haversine(t *testing.T) {
	svc, _, _ := newTestService(t)
	result, err := svc.CalculateDistance(context.Background(), 0, 0, 0, 1)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	// 1 degree longitude at equator ≈ 111.32 km
	if result.DistanceKm < 110 || result.DistanceKm > 112 {
		t.Errorf("expected ~111 km, got %f", result.DistanceKm)
	}
}

func TestReverseGeocode_InvalidCoords(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.ReverseGeocode(context.Background(), &ReverseGeocodeRequest{Lat: 100, Lng: 200})
	if err != ErrInvalidCoordinates { t.Errorf("expected ErrInvalidCoordinates, got %v", err) }
}

func TestCheckGeofence_InsideZone(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)
	surcharge := 15000
	mockRepo.EXPECT().FindZonesByPoint(gomock.Any(), -6.90, 107.57).
		Return([]repository.GeofenceZone{
			{ID: "zone-1", Name: "Bandara Husein", Type: "airport_surcharge", SurchargeAmount: &surcharge},
		}, nil)

	result, err := svc.CheckGeofence(context.Background(), &GeofenceCheckRequest{Lat: -6.90, Lng: 107.57})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if len(result.InsideZones) != 1 { t.Errorf("expected 1 zone, got %d", len(result.InsideZones)) }
	if result.InsideZones[0].Name != "Bandara Husein" { t.Error("wrong zone name") }
}

func TestCheckGeofence_Restricted(t *testing.T) {
	svc, mockRepo, _ := newTestService(t)
	mockRepo.EXPECT().FindZonesByPoint(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]repository.GeofenceZone{
			{ID: "zone-r", Name: "No Pickup Zone", Type: "no_pickup"},
		}, nil)

	result, err := svc.CheckGeofence(context.Background(), &GeofenceCheckRequest{Lat: -6.90, Lng: 107.57})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if !result.IsRestricted { t.Error("expected IsRestricted=true") }
}

func TestETA_SetAndGet(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()

	resp, err := svc.UpdateDriverETA(ctx, "drv-1", "ord-1", &UpdateEtaRequest{
		ETASeconds: 480, DistanceRemainingKm: 3.2, DestinationType: "pickup",
	})
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if resp.ETAText != "8 menit" { t.Errorf("expected '8 menit', got '%s'", resp.ETAText) }

	got, err := svc.GetDriverETA(ctx, "drv-1", "ord-1")
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if got.ETASeconds != 480 { t.Errorf("expected 480, got %d", got.ETASeconds) }
}

func TestETA_NotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.GetDriverETA(context.Background(), "drv-x", "ord-x")
	if err != ErrETANotFound { t.Errorf("expected ErrETANotFound, got %v", err) }
}
