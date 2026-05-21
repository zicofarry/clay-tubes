// Package service implements the dispatch & matching business logic
// for the Matching Service.
package service

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/zicofarry/clay-app/backend/services/matching-service/internal/geo"
	"github.com/zicofarry/clay-app/backend/services/matching-service/internal/repository"
)

// ── Service Error ──────────────────────────────────────────────────────────

// ServiceError represents a business-logic error with HTTP status mapping.
type ServiceError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *ServiceError) Error() string { return e.Message }

// Common errors. The HTTP layer maps these to proper status codes.
var (
	ErrValidation         = &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "request validation failed"}
	ErrDriverNotOnline    = &ServiceError{http.StatusNotFound, "DRIVER_NOT_ONLINE", "driver is not online"}
	ErrDriverHasActive    = &ServiceError{http.StatusConflict, "DRIVER_HAS_ACTIVE_ORDER", "driver already has an active order"}
	ErrInvalidMode        = &ServiceError{http.StatusBadRequest, "INVALID_MODE", "mode must be 'priority' or 'normal'"}
	ErrInvalidTarget      = &ServiceError{http.StatusBadRequest, "INVALID_TARGET", "daily_target must be > 0 in priority mode"}
	ErrOfferNotFound      = &ServiceError{http.StatusNotFound, "OFFER_NOT_FOUND", "offer not found or expired"}
	ErrOfferAlreadyClosed = &ServiceError{http.StatusConflict, "OFFER_ALREADY_CLOSED", "offer already responded to"}
	ErrSessionNotFound    = &ServiceError{http.StatusNotFound, "SESSION_NOT_FOUND", "no dispatch session for this order"}
	ErrSessionExists      = &ServiceError{http.StatusConflict, "SESSION_EXISTS", "active dispatch session already exists for this order"}
	ErrInvalidServiceType = &ServiceError{http.StatusBadRequest, "INVALID_SERVICE_TYPE", "service_type must be ride|delivery|food"}
	ErrInvalidCoords      = &ServiceError{http.StatusBadRequest, "INVALID_COORDS", "lat/lng out of range"}
	ErrUpstreamUnavailable = &ServiceError{http.StatusServiceUnavailable, "UPSTREAM_UNAVAILABLE", "geo service unavailable"}
)

// ── Service-type / vehicle-type helpers ───────────────────────────────────

// vehicleTypeForService maps a service type to its primary vehicle bucket
// in the Redis GEO index.
func vehicleTypeForService(serviceType string) string {
	switch serviceType {
	case "ride":
		return "motor" // most rides are motor; car bucket is queried separately if needed
	case "delivery":
		return "motor"
	case "food":
		return "motor"
	default:
		return "motor"
	}
}

func validServiceType(s string) bool {
	return s == "ride" || s == "delivery" || s == "food"
}

func validMode(m string) bool {
	return m == "priority" || m == "normal"
}

// ── Scoring algorithm ──────────────────────────────────────────────────────

// ScoringWeights holds the multi-factor weights that determine candidate rank.
// Total should sum to 1.0; the defaults track the OpenAPI spec.
type ScoringWeights struct {
	Proximity      float64
	Rating         float64
	Acceptance     float64
	ModeBonus      float64
	Distribution   float64
}

// DefaultWeights returns the spec-defined matching algorithm weights.
func DefaultWeights() ScoringWeights {
	return ScoringWeights{
		Proximity:    0.35,
		Rating:       0.20,
		Acceptance:   0.15,
		ModeBonus:    0.20,
		Distribution: 0.10,
	}
}

// ScoreInputs are the per-candidate inputs to the scoring function.
type ScoreInputs struct {
	DistanceKm     float64 // 0..searchRadiusKm
	SearchRadiusKm float64
	Rating         float64 // 1..5
	AcceptanceRate float64 // 0..1
	IsPriority     bool
	TripsToday     int
}

// computeScore returns a deterministic 0..1 score from the inputs.
// Higher = better candidate.
func computeScore(in ScoreInputs, w ScoringWeights) float64 {
	radius := in.SearchRadiusKm
	if radius <= 0 {
		radius = 5.0
	}
	// proximity: closer is better; clamped [0,1]
	proximity := 1.0 - (in.DistanceKm / radius)
	if proximity < 0 {
		proximity = 0
	}
	if proximity > 1 {
		proximity = 1
	}

	// rating: 1..5 normalized to 0..1
	ratingNorm := (in.Rating - 1.0) / 4.0
	if ratingNorm < 0 {
		ratingNorm = 0
	}
	if ratingNorm > 1 {
		ratingNorm = 1
	}

	// acceptance: 0..1, default 0.5 if no history (treated as new driver, neutral)
	acc := in.AcceptanceRate
	if acc < 0 {
		acc = 0
	}
	if acc > 1 {
		acc = 1
	}

	// distribution: favors drivers with fewer trips today (fairness).
	// 0 trips → 1.0, 10+ trips → 0.0.
	dist := 1.0 - (float64(in.TripsToday) / 10.0)
	if dist < 0 {
		dist = 0
	}
	if dist > 1 {
		dist = 1
	}

	mode := 0.0
	if in.IsPriority {
		mode = 1.0
	}

	return w.Proximity*proximity +
		w.Rating*ratingNorm +
		w.Acceptance*acc +
		w.ModeBonus*mode +
		w.Distribution*dist
}

// ── Request/Response DTOs ──────────────────────────────────────────────────

type GoOnlineRequest struct {
	ServiceType string  `json:"service_type"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
}

type DriverStatusResponse struct {
	DriverID  string    `json:"driver_id"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

type LocationUpdateRequest struct {
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Bearing  float64 `json:"bearing,omitempty"`
	SpeedKmh float64 `json:"speed_kmh,omitempty"`
}

type HeartbeatResponse struct {
	DriverID   string `json:"driver_id"`
	Status     string `json:"status"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type OfferResponseRequest struct {
	OrderID      string `json:"order_id"`
	Action       string `json:"action"`        // accept | reject
	RejectReason string `json:"reject_reason,omitempty"` // too_far | busy | low_fare | other
}

type SetDispatchModeRequest struct {
	Mode        string `json:"mode"`
	DailyTarget int    `json:"daily_target,omitempty"`
}

type DispatchModeResponse struct {
	Mode               string    `json:"mode"`
	DailyTarget        int       `json:"daily_target,omitempty"`
	EarningsToday      int       `json:"earnings_today"`
	TargetProgressPct  float64   `json:"target_progress_pct"`
	ActivatedAt        time.Time `json:"activated_at"`
}

type FullDriverStatusResponse struct {
	DriverID       string    `json:"driver_id"`
	Status         string    `json:"status"`
	Mode           string    `json:"mode"`
	DailyTarget    int       `json:"daily_target,omitempty"`
	EarningsToday  int       `json:"earnings_today"`
	TripsToday     int       `json:"trips_today"`
	AcceptanceRate float64   `json:"acceptance_rate"`
	Rating         float64   `json:"rating"`
	ActiveOrderID  string    `json:"active_order_id,omitempty"`
	OnlineSince    time.Time `json:"online_since,omitempty"`
}

type EarningsTodayResponse struct {
	Date              string  `json:"date"`
	TotalEarnings     int     `json:"total_earnings"`
	TripCount         int     `json:"trip_count"`
	Mode              string  `json:"mode"`
	DailyTarget       int     `json:"daily_target,omitempty"`
	TargetProgressPct float64 `json:"target_progress_pct,omitempty"`
	TargetRemaining   int     `json:"target_remaining,omitempty"`
	AvgFare           int     `json:"avg_fare"`
}

type DispatchRequest struct {
	OrderID         string  `json:"order_id"`
	ServiceType     string  `json:"service_type"`
	PickupLat       float64 `json:"pickup_lat"`
	PickupLng       float64 `json:"pickup_lng"`
	DestLat         float64 `json:"dest_lat,omitempty"`
	DestLng         float64 `json:"dest_lng,omitempty"`
	SearchRadiusKm  float64 `json:"search_radius_km,omitempty"`
	MaxRounds       int     `json:"max_rounds,omitempty"`
	CallbackURL     string  `json:"callback_url,omitempty"`
}

type DispatchSessionResponse struct {
	SessionID string    `json:"session_id"`
	OrderID   string    `json:"order_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type DispatchSessionDetail struct {
	DispatchSessionResponse
	ServiceType        string    `json:"service_type"`
	CandidatesTried    int       `json:"candidates_tried"`
	CurrentCandidateID string    `json:"current_candidate_id,omitempty"`
	OfferExpiresAt     time.Time `json:"offer_expires_at,omitempty"`
	MatchedDriverID    string    `json:"matched_driver_id,omitempty"`
	MatchedAt          time.Time `json:"matched_at,omitempty"`
}

type CancelDispatchRequest struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason,omitempty"` // order_cancelled | timeout | system
}

type NearbyDriversQuery struct {
	Lat         float64
	Lng         float64
	RadiusKm    float64
	ServiceType string
	Limit       int
}

type NearbyDriversResponse struct {
	Drivers []NearbyDriverSummary `json:"drivers"`
	Total   int                   `json:"total"`
}

type NearbyDriverSummary struct {
	DriverID       string  `json:"driver_id"`
	Lat            float64 `json:"lat"`
	Lng            float64 `json:"lng"`
	DistanceKm     float64 `json:"distance_km"`
	Rating         float64 `json:"rating"`
	AcceptanceRate float64 `json:"acceptance_rate"`
	Mode           string  `json:"mode"`
	Score          float64 `json:"score"`
}

type ZoneStatsResponse struct {
	ZoneID            string    `json:"zone_id"`
	OnlineDrivers     int       `json:"online_drivers"`
	PendingOrders     int       `json:"pending_orders"`
	SupplyDemandRatio float64   `json:"supply_demand_ratio"`
	SuggestedSurge    float64   `json:"suggested_surge"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type FreeDriverRequest struct {
	TripFare int `json:"trip_fare,omitempty"`
}

// ── Interface ─────────────────────────────────────────────────────────────

// MatchingServiceInterface defines the matching service contract.
//
//go:generate mockgen -source=matching_service.go -destination=../../mocks/mock_matching_service.go -package=mocks
type MatchingServiceInterface interface {
	// Driver-facing
	GoOnline(ctx context.Context, driverID string, req *GoOnlineRequest) (*DriverStatusResponse, error)
	GoOffline(ctx context.Context, driverID string) (*DriverStatusResponse, error)
	UpdateLocation(ctx context.Context, driverID string, req *LocationUpdateRequest) error
	Heartbeat(ctx context.Context, driverID string) (*HeartbeatResponse, error)
	Respond(ctx context.Context, driverID string, req *OfferResponseRequest) error
	SetMode(ctx context.Context, driverID string, req *SetDispatchModeRequest) (*DispatchModeResponse, error)
	GetFullStatus(ctx context.Context, driverID string) (*FullDriverStatusResponse, error)
	GetTodayEarnings(ctx context.Context, driverID string) (*EarningsTodayResponse, error)

	// Internal (service-to-service)
	StartDispatch(ctx context.Context, req *DispatchRequest) (*DispatchSessionResponse, error)
	CancelDispatch(ctx context.Context, req *CancelDispatchRequest) error
	NearbyActiveDrivers(ctx context.Context, q NearbyDriversQuery) (*NearbyDriversResponse, error)
	GetSession(ctx context.Context, orderID string) (*DispatchSessionDetail, error)
	GetZoneStats(ctx context.Context, vehicleType, zoneID string) (*ZoneStatsResponse, error)
	FreeDriver(ctx context.Context, driverID string, req *FreeDriverRequest) error
}

// ── Implementation ────────────────────────────────────────────────────────

// MatchingService is the concrete implementation.
type MatchingService struct {
	repo    repository.MatchingRepositoryInterface
	geo     geo.Client
	logger  *slog.Logger
	weights ScoringWeights
	now     func() time.Time // injectable for tests
}

// NewMatchingService creates a new MatchingService.
func NewMatchingService(repo repository.MatchingRepositoryInterface, geoClient geo.Client, logger *slog.Logger) *MatchingService {
	return &MatchingService{
		repo:    repo,
		geo:     geoClient,
		logger:  logger,
		weights: DefaultWeights(),
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// ── Driver-facing methods ─────────────────────────────────────────────────

func (s *MatchingService) GoOnline(ctx context.Context, driverID string, req *GoOnlineRequest) (*DriverStatusResponse, error) {
	if driverID == "" {
		return nil, ErrValidation
	}
	if req == nil || !validServiceType(req.ServiceType) {
		return nil, ErrInvalidServiceType
	}
	if err := validateLatLng(req.Lat, req.Lng); err != nil {
		return nil, err
	}

	// Block if driver still has an active order from a previous trip
	if existingOrder, err := s.repo.GetActiveOrder(ctx, driverID); err == nil && existingOrder != "" {
		return nil, ErrDriverHasActive
	}

	vehicleType := vehicleTypeForService(req.ServiceType)
	now := s.now()
	st := &repository.DriverStatus{
		DriverID:    driverID,
		Status:      "online",
		VehicleType: vehicleType,
		Lat:         req.Lat,
		Lng:         req.Lng,
		UpdatedAt:   now,
		OnlineSince: now,
	}
	if err := s.repo.SetDriverStatus(ctx, st); err != nil {
		return nil, err
	}
	if err := s.repo.AddDriverToGeo(ctx, vehicleType, driverID, req.Lat, req.Lng); err != nil {
		// Best-effort — don't fail the whole go-online if geo index is flaky.
		s.logger.Warn("geo add failed", slog.String("driver_id", driverID), slog.Any("err", err))
	}
	if err := s.geo.RegisterDriver(ctx, geo.LocationUpdate{
		DriverID: driverID, Lat: req.Lat, Lng: req.Lng, VehicleType: vehicleType,
	}); err != nil {
		s.logger.Warn("upstream geo register failed", slog.Any("err", err))
	}

	s.logger.Info("driver online",
		slog.String("driver_id", driverID),
		slog.String("service_type", req.ServiceType),
	)
	return &DriverStatusResponse{
		DriverID:  driverID,
		Status:    "online",
		UpdatedAt: now,
	}, nil
}

func (s *MatchingService) GoOffline(ctx context.Context, driverID string) (*DriverStatusResponse, error) {
	if driverID == "" {
		return nil, ErrValidation
	}
	st, err := s.repo.GetDriverStatus(ctx, driverID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	vehicleType := ""
	if st != nil {
		vehicleType = st.VehicleType
	}
	if vehicleType != "" {
		_ = s.repo.RemoveDriverFromGeo(ctx, vehicleType, driverID)
	}
	_ = s.repo.DeleteDriverStatus(ctx, driverID)
	_ = s.repo.DeleteDriverMode(ctx, driverID) // priority mode auto-disables
	_ = s.geo.UnregisterDriver(ctx, driverID)

	s.logger.Info("driver offline", slog.String("driver_id", driverID))
	return &DriverStatusResponse{
		DriverID:  driverID,
		Status:    "offline",
		UpdatedAt: s.now(),
	}, nil
}

func (s *MatchingService) UpdateLocation(ctx context.Context, driverID string, req *LocationUpdateRequest) error {
	if driverID == "" || req == nil {
		return ErrValidation
	}
	if err := validateLatLng(req.Lat, req.Lng); err != nil {
		return err
	}
	st, err := s.repo.GetDriverStatus(ctx, driverID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrDriverNotOnline
		}
		return err
	}
	if err := s.repo.UpdateDriverLocation(ctx, driverID, req.Lat, req.Lng); err != nil {
		return err
	}
	// Refresh geo index entry to new coords
	_ = s.repo.AddDriverToGeo(ctx, st.VehicleType, driverID, req.Lat, req.Lng)

	// Best-effort upstream proxy
	_ = s.geo.UpdateLocation(ctx, geo.LocationUpdate{
		DriverID:    driverID,
		Lat:         req.Lat,
		Lng:         req.Lng,
		Bearing:     req.Bearing,
		SpeedKmh:    req.SpeedKmh,
		VehicleType: st.VehicleType,
	})
	return nil
}

func (s *MatchingService) Heartbeat(ctx context.Context, driverID string) (*HeartbeatResponse, error) {
	if driverID == "" {
		return nil, ErrValidation
	}
	st, err := s.repo.GetDriverStatus(ctx, driverID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrDriverNotOnline
		}
		return nil, err
	}
	ttl, err := s.repo.RefreshDriverStatusTTL(ctx, driverID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrDriverNotOnline
		}
		return nil, err
	}
	return &HeartbeatResponse{
		DriverID:   driverID,
		Status:     st.Status,
		TTLSeconds: int(ttl.Seconds()),
	}, nil
}

func (s *MatchingService) Respond(ctx context.Context, driverID string, req *OfferResponseRequest) error {
	if driverID == "" || req == nil || req.OrderID == "" {
		return ErrValidation
	}
	if req.Action != "accept" && req.Action != "reject" {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "action must be accept or reject"}
	}

	sess, err := s.repo.GetSession(ctx, req.OrderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrOfferNotFound
		}
		return err
	}
	if sess.CurrentCandidateID != driverID {
		// Either this driver wasn't the active candidate, or the offer already moved on.
		return ErrOfferNotFound
	}
	if sess.Status != "searching" {
		return ErrOfferAlreadyClosed
	}
	if !sess.OfferExpiresAt.IsZero() && s.now().After(sess.OfferExpiresAt) {
		return ErrOfferNotFound
	}

	switch req.Action {
	case "accept":
		// Lock driver to order — fails if another order claimed them already.
		if err := s.repo.SetActiveOrder(ctx, driverID, req.OrderID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return ErrDriverHasActive
			}
			return err
		}
		// Mark driver busy in status hash
		st, _ := s.repo.GetDriverStatus(ctx, driverID)
		if st != nil {
			st.Status = "busy"
			st.OrderID = req.OrderID
			_ = s.repo.SetDriverStatus(ctx, st)
		}
		_ = s.repo.IncrementAcceptance(ctx, driverID, true)

		// Mark session matched
		now := s.now()
		sess.Status = "driver_found"
		sess.MatchedDriverID = driverID
		sess.MatchedAt = now
		sess.OfferExpiresAt = time.Time{}
		if err := s.repo.UpdateSession(ctx, sess); err != nil {
			return err
		}
		s.logger.Info("driver accepted",
			slog.String("order_id", req.OrderID),
			slog.String("driver_id", driverID),
		)
		// TODO: emit Kafka `matching.driver_found`
		return nil

	case "reject":
		_ = s.repo.MarkRejected(ctx, req.OrderID, driverID)
		_ = s.repo.IncrementAcceptance(ctx, driverID, false)
		// Move session to next candidate (caller / dispatcher loop handles re-broadcast).
		sess.CurrentCandidateID = ""
		sess.OfferExpiresAt = time.Time{}
		if err := s.repo.UpdateSession(ctx, sess); err != nil {
			return err
		}
		s.logger.Info("driver rejected",
			slog.String("order_id", req.OrderID),
			slog.String("driver_id", driverID),
			slog.String("reason", req.RejectReason),
		)
		return nil
	}
	return nil
}

func (s *MatchingService) SetMode(ctx context.Context, driverID string, req *SetDispatchModeRequest) (*DispatchModeResponse, error) {
	if driverID == "" || req == nil {
		return nil, ErrValidation
	}
	if !validMode(req.Mode) {
		return nil, ErrInvalidMode
	}
	if req.Mode == "priority" && req.DailyTarget <= 0 {
		return nil, ErrInvalidTarget
	}
	// Driver must be online to set priority mode
	if _, err := s.repo.GetDriverStatus(ctx, driverID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrDriverNotOnline
		}
		return nil, err
	}

	now := s.now()
	mode := &repository.DriverMode{
		Mode:        req.Mode,
		DailyTarget: req.DailyTarget,
		ActivatedAt: now,
	}
	if err := s.repo.SetDriverMode(ctx, driverID, mode); err != nil {
		return nil, err
	}

	earnings, _ := s.repo.GetDriverEarnings(ctx, driverID, dateOnly(now))
	progress := 0.0
	if req.Mode == "priority" && req.DailyTarget > 0 && earnings != nil {
		progress = float64(earnings.TotalEarnings) / float64(req.DailyTarget)
		if progress > 1 {
			progress = 1
		}
	}

	s.logger.Info("dispatch mode updated",
		slog.String("driver_id", driverID),
		slog.String("mode", req.Mode),
		slog.Int("daily_target", req.DailyTarget),
	)

	resp := &DispatchModeResponse{
		Mode:              req.Mode,
		DailyTarget:       req.DailyTarget,
		TargetProgressPct: progress,
		ActivatedAt:       now,
	}
	if earnings != nil {
		resp.EarningsToday = earnings.TotalEarnings
	}
	return resp, nil
}

func (s *MatchingService) GetFullStatus(ctx context.Context, driverID string) (*FullDriverStatusResponse, error) {
	if driverID == "" {
		return nil, ErrValidation
	}
	st, err := s.repo.GetDriverStatus(ctx, driverID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return &FullDriverStatusResponse{
				DriverID: driverID,
				Status:   "offline",
				Mode:     "normal",
			}, nil
		}
		return nil, err
	}

	mode, _ := s.repo.GetDriverMode(ctx, driverID)
	earnings, _ := s.repo.GetDriverEarnings(ctx, driverID, dateOnly(s.now()))
	acc, _ := s.repo.GetAcceptance(ctx, driverID)
	rating, _ := s.repo.GetCachedRating(ctx, driverID)
	activeOrder, _ := s.repo.GetActiveOrder(ctx, driverID)

	resp := &FullDriverStatusResponse{
		DriverID:       driverID,
		Status:         st.Status,
		Mode:           "normal",
		Rating:         rating,
		ActiveOrderID:  activeOrder,
		OnlineSince:    st.OnlineSince,
		AcceptanceRate: computeAcceptanceRate(acc),
	}
	if mode != nil {
		resp.Mode = mode.Mode
		resp.DailyTarget = mode.DailyTarget
	}
	if earnings != nil {
		resp.EarningsToday = earnings.TotalEarnings
		resp.TripsToday = earnings.TripCount
	}
	return resp, nil
}

func (s *MatchingService) GetTodayEarnings(ctx context.Context, driverID string) (*EarningsTodayResponse, error) {
	if driverID == "" {
		return nil, ErrValidation
	}
	now := s.now()
	date := dateOnly(now)
	earnings, _ := s.repo.GetDriverEarnings(ctx, driverID, date)
	mode, _ := s.repo.GetDriverMode(ctx, driverID)

	resp := &EarningsTodayResponse{
		Date: date,
		Mode: "normal",
	}
	if mode != nil {
		resp.Mode = mode.Mode
		resp.DailyTarget = mode.DailyTarget
	}
	if earnings != nil {
		resp.TotalEarnings = earnings.TotalEarnings
		resp.TripCount = earnings.TripCount
		if earnings.TripCount > 0 {
			resp.AvgFare = earnings.TotalEarnings / earnings.TripCount
		}
	}
	if resp.Mode == "priority" && resp.DailyTarget > 0 {
		progress := float64(resp.TotalEarnings) / float64(resp.DailyTarget)
		if progress > 1 {
			progress = 1
		}
		resp.TargetProgressPct = progress
		remaining := resp.DailyTarget - resp.TotalEarnings
		if remaining < 0 {
			remaining = 0
		}
		resp.TargetRemaining = remaining
	}
	return resp, nil
}

// ── Internal methods ──────────────────────────────────────────────────────

// StartDispatch creates a matching session and synchronously picks the top
// candidate. Re-broadcasting on reject/timeout happens via Respond() (driver
// rejects clears the current candidate; a worker loop — outside this service —
// picks the next candidate). We expose helpers so a worker can call into this
// service to advance the session.
func (s *MatchingService) StartDispatch(ctx context.Context, req *DispatchRequest) (*DispatchSessionResponse, error) {
	if req == nil || req.OrderID == "" {
		return nil, ErrValidation
	}
	if !validServiceType(req.ServiceType) {
		return nil, ErrInvalidServiceType
	}
	if err := validateLatLng(req.PickupLat, req.PickupLng); err != nil {
		return nil, err
	}

	// Defaults
	if req.SearchRadiusKm <= 0 {
		req.SearchRadiusKm = 5.0
	}
	if req.MaxRounds <= 0 {
		req.MaxRounds = 5
	}

	now := s.now()
	sess := &repository.MatchingSession{
		OrderID:        req.OrderID,
		SessionID:      uuid.New().String(),
		ServiceType:    req.ServiceType,
		Status:         "searching",
		PickupLat:      req.PickupLat,
		PickupLng:      req.PickupLng,
		SearchRadiusKm: req.SearchRadiusKm,
		MaxRounds:      req.MaxRounds,
		CallbackURL:    req.CallbackURL,
		CreatedAt:      now,
	}
	if err := s.repo.CreateSession(ctx, sess); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSessionExists
		}
		return nil, err
	}

	// Pick first candidate immediately so the dispatcher worker can broadcast it.
	if cand, err := s.pickNextCandidate(ctx, sess); err == nil && cand != nil {
		sess.CurrentCandidateID = cand.DriverID
		sess.OfferExpiresAt = now.Add(repository.OfferTTL)
		sess.CandidatesTried++
		_ = s.repo.UpdateSession(ctx, sess)
	}

	s.logger.Info("dispatch started",
		slog.String("order_id", req.OrderID),
		slog.String("session_id", sess.SessionID),
		slog.String("service_type", req.ServiceType),
	)
	// TODO: emit Kafka `matching.offer_sent`

	return &DispatchSessionResponse{
		SessionID: sess.SessionID,
		OrderID:   sess.OrderID,
		Status:    sess.Status,
		CreatedAt: sess.CreatedAt,
	}, nil
}

func (s *MatchingService) CancelDispatch(ctx context.Context, req *CancelDispatchRequest) error {
	if req == nil || req.OrderID == "" {
		return ErrValidation
	}
	sess, err := s.repo.GetSession(ctx, req.OrderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrSessionNotFound
		}
		return err
	}
	sess.Status = "cancelled"
	if err := s.repo.UpdateSession(ctx, sess); err != nil {
		return err
	}
	// Free any held candidate
	if sess.CurrentCandidateID != "" {
		_ = s.repo.ClearActiveOrder(ctx, sess.CurrentCandidateID)
	}
	_ = s.repo.DeleteSession(ctx, req.OrderID)

	s.logger.Info("dispatch cancelled",
		slog.String("order_id", req.OrderID),
		slog.String("reason", req.Reason),
	)
	return nil
}

func (s *MatchingService) NearbyActiveDrivers(ctx context.Context, q NearbyDriversQuery) (*NearbyDriversResponse, error) {
	if !validServiceType(q.ServiceType) {
		return nil, ErrInvalidServiceType
	}
	if err := validateLatLng(q.Lat, q.Lng); err != nil {
		return nil, err
	}
	if q.RadiusKm <= 0 {
		q.RadiusKm = 5.0
	}
	if q.Limit <= 0 || q.Limit > 50 {
		q.Limit = 20
	}
	scored, err := s.scoreCandidates(ctx, q.ServiceType, q.Lat, q.Lng, q.RadiusKm, q.Limit, "")
	if err != nil {
		return nil, err
	}
	out := &NearbyDriversResponse{
		Drivers: make([]NearbyDriverSummary, 0, len(scored)),
		Total:   len(scored),
	}
	for _, c := range scored {
		out.Drivers = append(out.Drivers, NearbyDriverSummary{
			DriverID:       c.DriverID,
			Lat:            c.Lat,
			Lng:            c.Lng,
			DistanceKm:     round3(c.DistanceKm),
			Rating:         c.Rating,
			AcceptanceRate: c.AcceptanceRate,
			Mode:           c.Mode,
			Score:          round3(c.Score),
		})
	}
	return out, nil
}

func (s *MatchingService) GetSession(ctx context.Context, orderID string) (*DispatchSessionDetail, error) {
	if orderID == "" {
		return nil, ErrValidation
	}
	sess, err := s.repo.GetSession(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &DispatchSessionDetail{
		DispatchSessionResponse: DispatchSessionResponse{
			SessionID: sess.SessionID,
			OrderID:   sess.OrderID,
			Status:    sess.Status,
			CreatedAt: sess.CreatedAt,
		},
		ServiceType:        sess.ServiceType,
		CandidatesTried:    sess.CandidatesTried,
		CurrentCandidateID: sess.CurrentCandidateID,
		OfferExpiresAt:     sess.OfferExpiresAt,
		MatchedDriverID:    sess.MatchedDriverID,
		MatchedAt:          sess.MatchedAt,
	}, nil
}

func (s *MatchingService) GetZoneStats(ctx context.Context, vehicleType, zoneID string) (*ZoneStatsResponse, error) {
	if zoneID == "" {
		return nil, ErrValidation
	}
	if vehicleType == "" {
		vehicleType = "motor"
	}
	stats, err := s.repo.GetZoneStats(ctx, vehicleType, zoneID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// No stats yet — return zeros, suggested_surge=1.0
			return &ZoneStatsResponse{
				ZoneID:            zoneID,
				SupplyDemandRatio: 1.0,
				SuggestedSurge:    1.0,
				UpdatedAt:         s.now(),
			}, nil
		}
		return nil, err
	}
	ratio := 1.0
	if stats.PendingOrders > 0 {
		ratio = float64(stats.OnlineDrivers) / float64(stats.PendingOrders)
	}
	return &ZoneStatsResponse{
		ZoneID:            zoneID,
		OnlineDrivers:     stats.OnlineDrivers,
		PendingOrders:     stats.PendingOrders,
		SupplyDemandRatio: round3(ratio),
		SuggestedSurge:    suggestedSurge(ratio),
		UpdatedAt:         stats.UpdatedAt,
	}, nil
}

func (s *MatchingService) FreeDriver(ctx context.Context, driverID string, req *FreeDriverRequest) error {
	if driverID == "" {
		return ErrValidation
	}
	_ = s.repo.ClearActiveOrder(ctx, driverID)

	st, err := s.repo.GetDriverStatus(ctx, driverID)
	if err == nil && st != nil {
		st.Status = "online"
		st.OrderID = ""
		_ = s.repo.SetDriverStatus(ctx, st)
	}

	if req != nil && req.TripFare > 0 {
		_ = s.repo.IncrementDriverEarnings(ctx, driverID, dateOnly(s.now()), req.TripFare)

		// Auto-disable priority mode when daily target is reached
		mode, _ := s.repo.GetDriverMode(ctx, driverID)
		if mode != nil && mode.Mode == "priority" && mode.DailyTarget > 0 {
			earn, _ := s.repo.GetDriverEarnings(ctx, driverID, dateOnly(s.now()))
			if earn != nil && earn.TotalEarnings >= mode.DailyTarget {
				_ = s.repo.DeleteDriverMode(ctx, driverID)
				s.logger.Info("priority mode auto-disabled (target reached)",
					slog.String("driver_id", driverID),
					slog.Int("earnings", earn.TotalEarnings),
					slog.Int("target", mode.DailyTarget),
				)
			}
		}
	}
	s.logger.Info("driver freed", slog.String("driver_id", driverID))
	return nil
}

// ── Candidate selection ───────────────────────────────────────────────────

// scoredCandidate is the internal representation used during ranking.
type scoredCandidate struct {
	DriverID       string
	Lat            float64
	Lng            float64
	DistanceKm     float64
	Rating         float64
	AcceptanceRate float64
	Mode           string
	IsPriority     bool
	TripsToday     int
	Score          float64
}

// scoreCandidates queries the geo index, fetches per-driver attributes, computes
// the score for each and returns them sorted descending. The optional excludeOrderID
// filters drivers who already rejected this order.
func (s *MatchingService) scoreCandidates(
	ctx context.Context,
	serviceType string,
	lat, lng, radiusKm float64,
	limit int,
	excludeOrderID string,
) ([]scoredCandidate, error) {
	vehicleType := vehicleTypeForService(serviceType)
	geoDrivers, err := s.repo.NearbyDrivers(ctx, vehicleType, lat, lng, radiusKm, limit*2)
	if err != nil {
		return nil, err
	}

	out := make([]scoredCandidate, 0, len(geoDrivers))
	today := dateOnly(s.now())

	for _, d := range geoDrivers {
		// Filter: must be online, not busy, not have an active order
		st, err := s.repo.GetDriverStatus(ctx, d.DriverID)
		if err != nil || st == nil || st.Status != "online" {
			continue
		}
		if active, _ := s.repo.GetActiveOrder(ctx, d.DriverID); active != "" {
			continue
		}
		if excludeOrderID != "" {
			if rejected, _ := s.repo.IsRejected(ctx, excludeOrderID, d.DriverID); rejected {
				continue
			}
		}

		mode, _ := s.repo.GetDriverMode(ctx, d.DriverID)
		isPriority := mode != nil && mode.Mode == "priority"
		modeStr := "normal"
		if isPriority {
			modeStr = "priority"
		}
		acc, _ := s.repo.GetAcceptance(ctx, d.DriverID)
		rating, _ := s.repo.GetCachedRating(ctx, d.DriverID)
		earn, _ := s.repo.GetDriverEarnings(ctx, d.DriverID, today)
		tripsToday := 0
		if earn != nil {
			tripsToday = earn.TripCount
		}

		// Default rating for new drivers — neutral 4.5 (per industry baseline)
		if rating == 0 {
			rating = 4.5
		}

		c := scoredCandidate{
			DriverID:       d.DriverID,
			Lat:            d.Lat,
			Lng:            d.Lng,
			DistanceKm:     d.DistanceKm,
			Rating:         rating,
			AcceptanceRate: computeAcceptanceRate(acc),
			Mode:           modeStr,
			IsPriority:     isPriority,
			TripsToday:     tripsToday,
		}
		c.Score = computeScore(ScoreInputs{
			DistanceKm:     c.DistanceKm,
			SearchRadiusKm: radiusKm,
			Rating:         c.Rating,
			AcceptanceRate: c.AcceptanceRate,
			IsPriority:     c.IsPriority,
			TripsToday:     c.TripsToday,
		}, s.weights)
		out = append(out, c)
	}

	// Sort by score desc; tie-break by distance asc
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].DistanceKm < out[j].DistanceKm
		}
		return out[i].Score > out[j].Score
	})

	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// pickNextCandidate selects the top-ranked remaining candidate for a session,
// excluding any driver who already rejected this order.
func (s *MatchingService) pickNextCandidate(ctx context.Context, sess *repository.MatchingSession) (*scoredCandidate, error) {
	candidates, err := s.scoreCandidates(ctx, sess.ServiceType, sess.PickupLat, sess.PickupLng, sess.SearchRadiusKm, sess.MaxRounds, sess.OrderID)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, errors.New("no candidates")
	}
	c := candidates[0]
	return &c, nil
}

// ── Helpers ──────────────────────────────────────────────────────────────

func validateLatLng(lat, lng float64) error {
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return ErrInvalidCoords
	}
	return nil
}

func computeAcceptanceRate(a *repository.DriverAcceptance) float64 {
	if a == nil {
		return 0.5 // neutral default for new drivers
	}
	total := a.Accepted + a.Rejected
	if total == 0 {
		return 0.5
	}
	return float64(a.Accepted) / float64(total)
}

func dateOnly(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

func suggestedSurge(ratio float64) float64 {
	// supply_demand_ratio < 1 means shortage → bump surge.
	// Tiered: <0.5 → 2.0x, <0.75 → 1.5x, <1.0 → 1.25x, ≥1.0 → 1.0x
	switch {
	case ratio < 0.5:
		return 2.0
	case ratio < 0.75:
		return 1.5
	case ratio < 1.0:
		return 1.25
	default:
		return 1.0
	}
}

func round3(x float64) float64 { return math.Round(x*1000) / 1000 }
