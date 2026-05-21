package service

import (
	"context"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/zicofarry/clay-app/backend/services/tracking-service/internal/cache"
	"github.com/zicofarry/clay-app/backend/services/tracking-service/internal/repository"
)

// ServiceError maps to HTTP status codes
type ServiceError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *ServiceError) Error() string { return e.Message }

var (
	ErrOrderNotFound   = &ServiceError{"ORDER_NOT_FOUND", "Order tracking session not found", http.StatusNotFound}
	ErrRouteNotFound   = &ServiceError{"ROUTE_NOT_FOUND", "Historical trip route not found", http.StatusNotFound}
	ErrInvalidRequest  = &ServiceError{"INVALID_REQUEST", "Invalid tracking request", http.StatusBadRequest}
)

// Request & Response Types mapped from OpenAPI
type StartTrackingRequest struct {
	OrderID        string  `json:"order_id"`
	DriverID       string  `json:"driver_id"`
	PickupLat      float64 `json:"pickup_lat"`
	PickupLng      float64 `json:"pickup_lng"`
	DestinationLat float64 `json:"destination_lat"`
	DestinationLng float64 `json:"destination_lng"`
}

type LocationUpdateEvent struct {
	DriverID  string    `json:"driver_id"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Bearing   float64   `json:"bearing"`
	SpeedKmh  float64   `json:"speed_kmh"`
	Timestamp time.Time `json:"timestamp"`
}

type ETAResponse struct {
	OrderID          string    `json:"order_id"`
	Waypoint         string    `json:"waypoint"` // pickup or destination
	ETAMinutes       int       `json:"eta_minutes"`
	ETAAt            time.Time `json:"eta_at"`
	DistanceKm       float64   `json:"distance_km"`
	TrafficCondition string    `json:"traffic_condition"`
}

type RouteResponse struct {
	OrderID              string      `json:"order_id"`
	Origin               LatLng      `json:"origin"`
	Destination          LatLng      `json:"destination"`
	Waypoint             string      `json:"waypoint"`
	Polyline             string      `json:"polyline"`
	Steps                []RouteStep `json:"steps"`
	TotalDistanceKm      float64     `json:"total_distance_km"`
	TotalDurationMinutes int         `json:"total_duration_minutes"`
}

type LatLng struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type RouteStep struct {
	Instruction     string  `json:"instruction"`
	DistanceM       int     `json:"distance_m"`
	DurationSeconds int     `json:"duration_seconds"`
	StartLat        float64 `json:"start_lat"`
	StartLng        float64 `json:"start_lng"`
}

// TrackingServiceInterface defines core business logic
type TrackingServiceInterface interface {
	GetOrderPosition(ctx context.Context, orderID string) (*cache.OrderPosition, error)
	GetOrderETA(ctx context.Context, orderID string) (*ETAResponse, error)
	GetOrderRoute(ctx context.Context, orderID string) (*RouteResponse, error)
	GetTripRoute(ctx context.Context, orderID string) (*repository.TripRoute, error)
	
	StartTracking(ctx context.Context, req *StartTrackingRequest) error
	StopTracking(ctx context.Context, orderID string) error
	PushLocationUpdate(ctx context.Context, orderID string, req *LocationUpdateEvent) error
}

type TrackingService struct {
	repo   repository.TrackingRepositoryInterface
	c      cache.TrackingCacheInterface
	logger *slog.Logger
}

func NewTrackingService(repo repository.TrackingRepositoryInterface, c cache.TrackingCacheInterface, logger *slog.Logger) *TrackingService {
	return &TrackingService{repo: repo, c: c, logger: logger}
}

func (s *TrackingService) GetOrderPosition(ctx context.Context, orderID string) (*cache.OrderPosition, error) {
	pos, err := s.c.GetOrderPosition(ctx, orderID)
	if err != nil {
		s.logger.Error("failed to get position", slog.String("order_id", orderID), slog.Any("error", err))
		return nil, err
	}
	if pos == nil {
		return nil, ErrOrderNotFound
	}
	return pos, nil
}

func (s *TrackingService) GetOrderETA(ctx context.Context, orderID string) (*ETAResponse, error) {
	_, err := s.GetOrderPosition(ctx, orderID)
	if err != nil {
		return nil, err
	}
	
	// Mock ETA calculation based on static speed (20 km/h) for testing
	// In reality, this calls Google Maps Directions API
	distKm := 3.5 // Mock distance
	etaMins := int(math.Ceil(distKm / 20.0 * 60.0))
	
	return &ETAResponse{
		OrderID:          orderID,
		Waypoint:         "pickup",
		ETAMinutes:       etaMins,
		ETAAt:            time.Now().Add(time.Duration(etaMins) * time.Minute),
		DistanceKm:       distKm,
		TrafficCondition: "moderate",
	}, nil
}

func (s *TrackingService) GetOrderRoute(ctx context.Context, orderID string) (*RouteResponse, error) {
	pos, err := s.GetOrderPosition(ctx, orderID)
	if err != nil {
		return nil, err
	}

	// Mock route polyline for driver to next waypoint
	return &RouteResponse{
		OrderID: orderID,
		Origin: LatLng{Lat: pos.Lat, Lng: pos.Lng},
		Destination: LatLng{Lat: -6.90, Lng: 107.60},
		Waypoint: "pickup",
		Polyline: "mock_encoded_polyline_string",
		TotalDistanceKm: 3.5,
		TotalDurationMinutes: 11,
		Steps: []RouteStep{
			{Instruction: "Head north", DistanceM: 500, DurationSeconds: 120, StartLat: pos.Lat, StartLng: pos.Lng},
		},
	}, nil
}

func (s *TrackingService) GetTripRoute(ctx context.Context, orderID string) (*repository.TripRoute, error) {
	route, err := s.repo.GetTripRoute(ctx, orderID)
	if err != nil {
		s.logger.Error("failed to get trip route from DB", slog.Any("error", err))
		return nil, err
	}
	if route == nil {
		return nil, ErrRouteNotFound
	}
	return route, nil
}

func (s *TrackingService) StartTracking(ctx context.Context, req *StartTrackingRequest) error {
	pos := &cache.OrderPosition{
		OrderID:   req.OrderID,
		DriverID:  req.DriverID,
		Lat:       req.PickupLat, // Start tracking near pickup roughly
		Lng:       req.PickupLng,
		UpdatedAt: time.Now(),
	}
	
	if err := s.c.SetOrderPosition(ctx, pos); err != nil {
		s.logger.Error("failed to set initial position", slog.Any("error", err))
		return err
	}
	
	// Start point
	_ = s.c.AppendRoutePoint(ctx, req.OrderID, req.PickupLat, req.PickupLng, time.Now())
	return nil
}

func (s *TrackingService) PushLocationUpdate(ctx context.Context, orderID string, req *LocationUpdateEvent) error {
	// 1. Fetch current to ensure session exists
	_, err := s.c.GetOrderPosition(ctx, orderID)
	if err != nil {
		return err
	}
	// Note: We don't strictly check for nil here in production if Kafka is fire-and-forget, 
	// but according to openapi, returning 404 if not found for internal update.
	// Actually, if we are internal, maybe we just set it or drop it. We'll set it.
	
	newPos := &cache.OrderPosition{
		OrderID:    orderID,
		DriverID:   req.DriverID,
		Lat:        req.Lat,
		Lng:        req.Lng,
		Bearing:    req.Bearing,
		SpeedKmh:   req.SpeedKmh,
		UpdatedAt:  req.Timestamp,
		ETAMinutes: 5, // Mock dynamically updated ETA
	}
	
	if err := s.c.SetOrderPosition(ctx, newPos); err != nil {
		return err
	}
	
	// Accumulate for trip history
	if err := s.c.AppendRoutePoint(ctx, orderID, req.Lat, req.Lng, req.Timestamp); err != nil {
		s.logger.Error("failed to append route point", slog.Any("error", err))
	}
	
	// TODO: Broadcast to WebSocket clients (omitted for REST implementation simplicity)
	return nil
}

func (s *TrackingService) StopTracking(ctx context.Context, orderID string) error {
	// 1. Get all accumulated points from Redis
	pts, err := s.c.GetActiveRoutePoints(ctx, orderID)
	if err != nil {
		s.logger.Error("failed to get active route points", slog.Any("error", err))
	}
	
	// 2. If we have points, flush to MongoDB
	if len(pts) > 0 {
		routePts := make([]repository.RoutePoint, len(pts))
		for i, p := range pts {
			routePts[i] = repository.RoutePoint{Lat: p.Lat, Lng: p.Lng, Timestamp: p.Timestamp}
		}
		
		trip := &repository.TripRoute{
			OrderID: orderID,
			Points: routePts,
			TotalDistanceKm: calculateDistance(routePts),
			DurationMinutes: int(pts[len(pts)-1].Timestamp.Sub(pts[0].Timestamp).Minutes()),
			StartedAt: pts[0].Timestamp,
			EndedAt: pts[len(pts)-1].Timestamp,
		}
		
		if err := s.repo.SaveTripRoute(ctx, trip); err != nil {
			s.logger.Error("failed to save trip route to DB", slog.Any("error", err))
			// Do not block stopping tracking if DB fails, but log it
		}
	}
	
	// 3. Clear Redis active session
	_ = s.c.DeleteOrderPosition(ctx, orderID)
	_ = s.c.ClearActiveRoutePoints(ctx, orderID)
	
	return nil
}

// calculateDistance computes rough distance of a polyline in km
func calculateDistance(points []repository.RoutePoint) float64 {
	if len(points) < 2 {
		return 0
	}
	var total float64
	for i := 1; i < len(points); i++ {
		total += haversine(points[i-1].Lat, points[i-1].Lng, points[i].Lat, points[i].Lng)
	}
	return total
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371 // Earth radius in km
	dLat := (lat2 - lat1) * math.Pi / 180.0
	dLon := (lon2 - lon1) * math.Pi / 180.0
	
	lat1 = lat1 * math.Pi / 180.0
	lat2 = lat2 * math.Pi / 180.0
	
	a := math.Pow(math.Sin(dLat/2), 2) + math.Pow(math.Sin(dLon/2), 2)*math.Cos(lat1)*math.Cos(lat2)
	c := 2 * math.Asin(math.Sqrt(a))
	return r * c
}
