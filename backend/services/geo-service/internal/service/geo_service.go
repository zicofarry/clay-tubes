// Package service implements the business logic for the Geo Service.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/zicofarry/clay-app/backend/services/geo-service/internal/cache"
	"github.com/zicofarry/clay-app/backend/services/geo-service/internal/repository"
)

// ── Service Error ────────────────────────────────────────────────────────────

type ServiceError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *ServiceError) Error() string { return e.Message }

var (
	ErrDriverNotFound     = &ServiceError{http.StatusNotFound, "DRIVER_NOT_FOUND", "driver not online or location unavailable"}
	ErrInvalidCoordinates = &ServiceError{http.StatusBadRequest, "INVALID_COORDINATES", "invalid coordinates"}
	ErrPlaceNotFound      = &ServiceError{http.StatusNotFound, "PLACE_NOT_FOUND", "place not found"}
	ErrETANotFound        = &ServiceError{http.StatusNotFound, "ETA_NOT_FOUND", "no active ETA tracking for this driver/order"}
)

// ── Request/Response DTOs ────────────────────────────────────────────────────

type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type UpdateLocationRequest struct {
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	ServiceType string  `json:"service_type"`
	Bearing     float64 `json:"bearing"`
	SpeedKmh    float64 `json:"speed_kmh"`
}

type NearbyDriversResponse struct {
	Drivers []cache.NearbyDriver `json:"drivers"`
	Total   int                  `json:"total"`
}

type RouteEstimateRequest struct {
	Origin        LatLng  `json:"origin"`
	Destination   LatLng  `json:"destination"`
	ServiceType   string  `json:"service_type"`
	DepartureTime *string `json:"departure_time,omitempty"`
}

type RouteEstimateResponse struct {
	DistanceKm      float64 `json:"distance_km"`
	DistanceM       int     `json:"distance_m"`
	DurationSeconds int     `json:"duration_seconds"`
	DurationText    string  `json:"duration_text"`
	Polyline        string  `json:"polyline"`
	TrafficLevel    string  `json:"traffic_level"`
}

type PolylineResponse struct {
	Polyline        string  `json:"polyline"`
	DistanceKm      float64 `json:"distance_km"`
	DurationSeconds int     `json:"duration_seconds"`
}

type RouteStep struct {
	Instruction     string  `json:"instruction"`
	Maneuver        string  `json:"maneuver"`
	DistanceM       int     `json:"distance_m"`
	DurationSeconds int     `json:"duration_seconds"`
	StartLat        float64 `json:"start_lat"`
	StartLng        float64 `json:"start_lng"`
	EndLat          float64 `json:"end_lat"`
	EndLng          float64 `json:"end_lng"`
	Polyline        string  `json:"polyline"`
}

type RoutingResponse struct {
	DistanceKm      float64     `json:"distance_km"`
	DurationSeconds int         `json:"duration_seconds"`
	Polyline        string      `json:"polyline"`
	Steps           []RouteStep `json:"steps"`
}

type SnappingRequest struct {
	Points      []LatLng `json:"points"`
	Interpolate bool     `json:"interpolate"`
}

type SnappedPoint struct {
	Original      LatLng  `json:"original"`
	Snapped       LatLng  `json:"snapped"`
	RoadName      *string `json:"road_name,omitempty"`
	SpeedLimitKmh *int    `json:"speed_limit_kmh,omitempty"`
}

type SnappingResponse struct {
	SnappedPoints []SnappedPoint `json:"snapped_points"`
}

type TrafficSegment struct {
	RoadName        string  `json:"road_name"`
	Level           string  `json:"level"`
	SpeedKmh        float64 `json:"speed_kmh"`
	FreeFlowSpeedKmh float64 `json:"free_flow_speed_kmh"`
	LengthM         int     `json:"length_m"`
}

type TrafficResponse struct {
	OverallLevel            string           `json:"overall_level"`
	DelaySeconds            int              `json:"delay_seconds"`
	FreeFlowDurationSeconds int              `json:"free_flow_duration_seconds"`
	TrafficDurationSeconds  int              `json:"traffic_duration_seconds"`
	Segments                []TrafficSegment `json:"segments"`
}

type ForwardGeocodeRequest struct {
	Address string `json:"address"`
}

type GeocodeResult struct {
	FormattedAddress string  `json:"formatted_address"`
	PlaceName        string  `json:"place_name"`
	City             string  `json:"city"`
	Province         string  `json:"province"`
	PostalCode       string  `json:"postal_code"`
	Lat              float64 `json:"lat"`
	Lng              float64 `json:"lng"`
}

type ForwardGeocodeResponse struct {
	Results []GeocodeResult `json:"results"`
}

type ReverseGeocodeRequest struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type DistanceResponse struct {
	DistanceKm float64 `json:"distance_km"`
	DistanceM  float64 `json:"distance_m"`
}

type PlacePrediction struct {
	PlaceID       string `json:"place_id"`
	Description   string `json:"description"`
	MainText      string `json:"main_text"`
	SecondaryText string `json:"secondary_text"`
}

type AutocompleteResponse struct {
	Predictions []PlacePrediction `json:"predictions"`
}

type PlaceDetail struct {
	PlaceID          string   `json:"place_id"`
	Name             string   `json:"name"`
	FormattedAddress string   `json:"formatted_address"`
	Lat              float64  `json:"lat"`
	Lng              float64  `json:"lng"`
	Types            []string `json:"types"`
}

type GeofenceCheckRequest struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type GeofenceZoneResponse struct {
	ZoneID          string `json:"zone_id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	SurchargeAmount *int   `json:"surcharge_amount,omitempty"`
}

type GeofenceCheckResponse struct {
	InsideZones  []GeofenceZoneResponse `json:"inside_zones"`
	IsRestricted bool                   `json:"is_restricted"`
}

type BatchLocationRequest struct {
	DriverIDs []string `json:"driver_ids"`
}

type UpdateEtaRequest struct {
	ETASeconds          int     `json:"eta_seconds"`
	DistanceRemainingKm float64 `json:"distance_remaining_km"`
	DestinationType     string  `json:"destination_type"`
	DriverLat           float64 `json:"driver_lat"`
	DriverLng           float64 `json:"driver_lng"`
}

type LiveEtaResponse struct {
	DriverID            string    `json:"driver_id"`
	OrderID             string    `json:"order_id"`
	ETASeconds          int       `json:"eta_seconds"`
	ETAText             string    `json:"eta_text"`
	DistanceRemainingKm float64   `json:"distance_remaining_km"`
	DestinationType     string    `json:"destination_type"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ── Interface ────────────────────────────────────────────────────────────────

//go:generate mockgen -source=geo_service.go -destination=../../mocks/mock_geo_service.go -package=mocks
type GeoServiceInterface interface {
	// Location
	UpdateDriverLocation(ctx context.Context, driverID string, req *UpdateLocationRequest) error
	GetDriverLocation(ctx context.Context, driverID string) (*cache.DriverLocation, error)
	FindNearbyDrivers(ctx context.Context, lat, lng, radiusKm float64, serviceType string, limit int) (*NearbyDriversResponse, error)

	// Maps
	EstimateRoute(ctx context.Context, req *RouteEstimateRequest) (*RouteEstimateResponse, error)
	GetPolyline(ctx context.Context, originLat, originLng, destLat, destLng float64) (*PolylineResponse, error)
	GetRouting(ctx context.Context, originLat, originLng, destLat, destLng float64, mode string) (*RoutingResponse, error)
	SnapToRoad(ctx context.Context, req *SnappingRequest) (*SnappingResponse, error)
	GetTraffic(ctx context.Context, originLat, originLng, destLat, destLng float64) (*TrafficResponse, error)

	// Geocoding
	ForwardGeocode(ctx context.Context, req *ForwardGeocodeRequest) (*ForwardGeocodeResponse, error)
	ReverseGeocode(ctx context.Context, req *ReverseGeocodeRequest) (*GeocodeResult, error)
	CalculateDistance(ctx context.Context, originLat, originLng, destLat, destLng float64) (*DistanceResponse, error)

	// Places
	PlacesAutocomplete(ctx context.Context, query string, lat, lng *float64, radiusM int) (*AutocompleteResponse, error)
	GetPlaceDetail(ctx context.Context, placeID string) (*PlaceDetail, error)

	// Geofence
	CheckGeofence(ctx context.Context, req *GeofenceCheckRequest) (*GeofenceCheckResponse, error)

	// Internal
	BatchGetDriverLocations(ctx context.Context, req *BatchLocationRequest) (map[string]*cache.DriverLocation, error)
	GetDriverETA(ctx context.Context, driverID, orderID string) (*LiveEtaResponse, error)
	UpdateDriverETA(ctx context.Context, driverID, orderID string, req *UpdateEtaRequest) (*LiveEtaResponse, error)
}

// ── Implementation ───────────────────────────────────────────────────────────

type GeoService struct {
	repo     repository.GeoRepositoryInterface
	cache    cache.GeoCacheInterface
	logger   *slog.Logger
}

func NewGeoService(repo repository.GeoRepositoryInterface, geoCache cache.GeoCacheInterface, logger *slog.Logger) *GeoService {
	return &GeoService{repo: repo, cache: geoCache, logger: logger}
}

// ── Location ─────────────────────────────────────────────────────────────────

func (s *GeoService) UpdateDriverLocation(ctx context.Context, driverID string, req *UpdateLocationRequest) error {
	loc := &cache.DriverLocation{
		DriverID: driverID, Lat: req.Lat, Lng: req.Lng,
		Bearing: req.Bearing, SpeedKmh: req.SpeedKmh,
	}
	if err := s.cache.UpdateDriverLocation(ctx, req.ServiceType, loc); err != nil {
		return err
	}
	s.logger.Debug("driver location updated", slog.String("driver_id", driverID))
	return nil
}

func (s *GeoService) GetDriverLocation(ctx context.Context, driverID string) (*cache.DriverLocation, error) {
	loc, err := s.cache.GetDriverLocation(ctx, driverID)
	if err != nil {
		return nil, ErrDriverNotFound
	}
	return loc, nil
}

func (s *GeoService) FindNearbyDrivers(ctx context.Context, lat, lng, radiusKm float64, serviceType string, limit int) (*NearbyDriversResponse, error) {
	drivers, err := s.cache.FindNearbyDrivers(ctx, serviceType, lat, lng, radiusKm, limit)
	if err != nil {
		return nil, err
	}
	return &NearbyDriversResponse{Drivers: drivers, Total: len(drivers)}, nil
}

// ── Maps (stubs — replace with Google Maps API calls in production) ──────────

func (s *GeoService) EstimateRoute(_ context.Context, req *RouteEstimateRequest) (*RouteEstimateResponse, error) {
	dist := cache.HaversineKm(req.Origin.Lat, req.Origin.Lng, req.Destination.Lat, req.Destination.Lng)
	distM := int(dist * 1000)
	durationSec := int(dist / 30.0 * 3600) // assume 30 km/h avg
	durationText := formatDuration(durationSec)

	return &RouteEstimateResponse{
		DistanceKm: math.Round(dist*100) / 100, DistanceM: distM,
		DurationSeconds: durationSec, DurationText: durationText,
		Polyline: "stub_polyline", TrafficLevel: "moderate",
	}, nil
}

func (s *GeoService) GetPolyline(_ context.Context, originLat, originLng, destLat, destLng float64) (*PolylineResponse, error) {
	dist := cache.HaversineKm(originLat, originLng, destLat, destLng)
	return &PolylineResponse{
		Polyline: "stub_encoded_polyline", DistanceKm: math.Round(dist*100) / 100,
		DurationSeconds: int(dist / 30.0 * 3600),
	}, nil
}

func (s *GeoService) GetRouting(_ context.Context, originLat, originLng, destLat, destLng float64, mode string) (*RoutingResponse, error) {
	dist := cache.HaversineKm(originLat, originLng, destLat, destLng)
	return &RoutingResponse{
		DistanceKm: math.Round(dist*100) / 100, DurationSeconds: int(dist / 30.0 * 3600),
		Polyline: "stub_polyline",
		Steps: []RouteStep{
			{Instruction: "Mulai perjalanan", Maneuver: "straight", DistanceM: int(dist * 1000),
				DurationSeconds: int(dist / 30.0 * 3600), StartLat: originLat, StartLng: originLng,
				EndLat: destLat, EndLng: destLng, Polyline: "stub_step_polyline"},
		},
	}, nil
}

func (s *GeoService) SnapToRoad(_ context.Context, req *SnappingRequest) (*SnappingResponse, error) {
	snapped := make([]SnappedPoint, 0, len(req.Points))
	for _, p := range req.Points {
		snapped = append(snapped, SnappedPoint{
			Original: p, Snapped: LatLng{Lat: p.Lat, Lng: p.Lng},
		})
	}
	return &SnappingResponse{SnappedPoints: snapped}, nil
}

func (s *GeoService) GetTraffic(_ context.Context, originLat, originLng, destLat, destLng float64) (*TrafficResponse, error) {
	dist := cache.HaversineKm(originLat, originLng, destLat, destLng)
	freeFlow := int(dist / 40.0 * 3600)
	withTraffic := int(float64(freeFlow) * 1.5)
	return &TrafficResponse{
		OverallLevel: "moderate", DelaySeconds: withTraffic - freeFlow,
		FreeFlowDurationSeconds: freeFlow, TrafficDurationSeconds: withTraffic,
		Segments: []TrafficSegment{
			{RoadName: "Jl. Utama", Level: "moderate", SpeedKmh: 25, FreeFlowSpeedKmh: 40, LengthM: int(dist * 1000)},
		},
	}, nil
}

// ── Geocoding (stubs) ────────────────────────────────────────────────────────

func (s *GeoService) ForwardGeocode(_ context.Context, req *ForwardGeocodeRequest) (*ForwardGeocodeResponse, error) {
	return &ForwardGeocodeResponse{
		Results: []GeocodeResult{
			{FormattedAddress: req.Address, PlaceName: req.Address, City: "Bandung",
				Province: "Jawa Barat", PostalCode: "40257", Lat: -6.9733, Lng: 107.6310},
		},
	}, nil
}

func (s *GeoService) ReverseGeocode(_ context.Context, req *ReverseGeocodeRequest) (*GeocodeResult, error) {
	if req.Lat < -90 || req.Lat > 90 || req.Lng < -180 || req.Lng > 180 {
		return nil, ErrInvalidCoordinates
	}
	return &GeocodeResult{
		FormattedAddress: fmt.Sprintf("%.4f, %.4f", req.Lat, req.Lng),
		PlaceName: "Stub Location", City: "Bandung", Province: "Jawa Barat",
		PostalCode: "40257", Lat: req.Lat, Lng: req.Lng,
	}, nil
}

func (s *GeoService) CalculateDistance(_ context.Context, originLat, originLng, destLat, destLng float64) (*DistanceResponse, error) {
	km := cache.HaversineKm(originLat, originLng, destLat, destLng)
	return &DistanceResponse{
		DistanceKm: math.Round(km*100) / 100,
		DistanceM:  math.Round(km*1000*100) / 100,
	}, nil
}

// ── Places (stubs) ───────────────────────────────────────────────────────────

func (s *GeoService) PlacesAutocomplete(_ context.Context, query string, lat, lng *float64, radiusM int) (*AutocompleteResponse, error) {
	return &AutocompleteResponse{
		Predictions: []PlacePrediction{
			{PlaceID: "stub_place_1", Description: query + ", Bandung, Jawa Barat",
				MainText: query, SecondaryText: "Bandung, Jawa Barat"},
		},
	}, nil
}

func (s *GeoService) GetPlaceDetail(_ context.Context, placeID string) (*PlaceDetail, error) {
	if placeID == "" {
		return nil, ErrPlaceNotFound
	}
	return &PlaceDetail{
		PlaceID: placeID, Name: "Stub Place", FormattedAddress: "Jl. Stub, Bandung",
		Lat: -6.9733, Lng: 107.6310, Types: []string{"establishment"},
	}, nil
}

// ── Geofence ─────────────────────────────────────────────────────────────────

func (s *GeoService) CheckGeofence(ctx context.Context, req *GeofenceCheckRequest) (*GeofenceCheckResponse, error) {
	zones, err := s.repo.FindZonesByPoint(ctx, req.Lat, req.Lng)
	if err != nil {
		return nil, err
	}

	resp := &GeofenceCheckResponse{InsideZones: make([]GeofenceZoneResponse, 0, len(zones))}
	for _, z := range zones {
		gzr := GeofenceZoneResponse{ZoneID: z.ID, Name: z.Name, Type: z.Type, SurchargeAmount: z.SurchargeAmount}
		resp.InsideZones = append(resp.InsideZones, gzr)
		if z.Type == "restricted" || z.Type == "no_pickup" {
			resp.IsRestricted = true
		}
	}
	return resp, nil
}

// ── Internal ─────────────────────────────────────────────────────────────────

func (s *GeoService) BatchGetDriverLocations(ctx context.Context, req *BatchLocationRequest) (map[string]*cache.DriverLocation, error) {
	return s.cache.BatchGetDriverLocations(ctx, req.DriverIDs)
}

func (s *GeoService) GetDriverETA(ctx context.Context, driverID, orderID string) (*LiveEtaResponse, error) {
	eta, err := s.cache.GetETA(ctx, driverID, orderID)
	if err != nil {
		return nil, ErrETANotFound
	}
	return &LiveEtaResponse{
		DriverID: eta.DriverID, OrderID: eta.OrderID,
		ETASeconds: eta.ETASeconds, ETAText: eta.ETAText,
		DistanceRemainingKm: eta.DistanceRemainingKm,
		DestinationType: eta.DestinationType, UpdatedAt: eta.UpdatedAt,
	}, nil
}

func (s *GeoService) UpdateDriverETA(ctx context.Context, driverID, orderID string, req *UpdateEtaRequest) (*LiveEtaResponse, error) {
	eta := &cache.ETAData{
		DriverID: driverID, OrderID: orderID,
		ETASeconds: req.ETASeconds, ETAText: formatDuration(req.ETASeconds),
		DistanceRemainingKm: req.DistanceRemainingKm, DestinationType: req.DestinationType,
	}
	if err := s.cache.SetETA(ctx, eta); err != nil {
		return nil, err
	}
	return &LiveEtaResponse{
		DriverID: driverID, OrderID: orderID,
		ETASeconds: eta.ETASeconds, ETAText: eta.ETAText,
		DistanceRemainingKm: eta.DistanceRemainingKm,
		DestinationType: eta.DestinationType, UpdatedAt: eta.UpdatedAt,
	}, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%d detik", seconds)
	}
	return fmt.Sprintf("%d menit", seconds/60)
}
