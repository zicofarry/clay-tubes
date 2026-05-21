// Package repository implements the data access layer for the Matching Service.
//
// The matching service is fully stateless — all matching/dispatch state lives in
// Redis (per the ERD). This package wraps Redis primitives behind a domain-oriented
// interface so the service layer never deals with Redis directly.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// ── Domain Models ───────────────────────────────────────────────────────────

// DriverStatus holds the live driver heartbeat record.
type DriverStatus struct {
	DriverID    string    `json:"driver_id"`
	Status      string    `json:"status"`       // online | offline | busy
	VehicleType string    `json:"vehicle_type"` // motor | car | truck
	OrderID     string    `json:"order_id,omitempty"`
	Lat         float64   `json:"lat"`
	Lng         float64   `json:"lng"`
	UpdatedAt   time.Time `json:"updated_at"`
	OnlineSince time.Time `json:"online_since,omitempty"`
}

// DriverMode holds the dispatch mode and daily target settings.
type DriverMode struct {
	Mode        string    `json:"mode"` // priority | normal
	DailyTarget int       `json:"daily_target,omitempty"`
	ActivatedAt time.Time `json:"activated_at"`
}

// DriverEarnings holds today's running earnings counter.
type DriverEarnings struct {
	Date          string `json:"date"` // YYYY-MM-DD
	TotalEarnings int    `json:"total_earnings"`
	TripCount     int    `json:"trip_count"`
}

// DriverAcceptance holds the historical accept/reject counters.
type DriverAcceptance struct {
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
}

// MatchingSession represents the live matching session for an order.
type MatchingSession struct {
	OrderID            string    `json:"order_id"`
	SessionID          string    `json:"session_id"`
	ServiceType        string    `json:"service_type"`
	Status             string    `json:"status"` // searching | driver_found | no_driver | cancelled
	PickupLat          float64   `json:"pickup_lat"`
	PickupLng          float64   `json:"pickup_lng"`
	SearchRadiusKm     float64   `json:"search_radius_km"`
	MaxRounds          int       `json:"max_rounds"`
	CandidatesTried    int       `json:"candidates_tried"`
	CurrentCandidateID string    `json:"current_candidate_id,omitempty"`
	OfferExpiresAt     time.Time `json:"offer_expires_at,omitempty"`
	MatchedDriverID    string    `json:"matched_driver_id,omitempty"`
	MatchedAt          time.Time `json:"matched_at,omitempty"`
	CallbackURL        string    `json:"callback_url,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// GeoDriver holds a driver's location returned from a geo radius query.
type GeoDriver struct {
	DriverID   string  `json:"driver_id"`
	Lat        float64 `json:"lat"`
	Lng        float64 `json:"lng"`
	DistanceKm float64 `json:"distance_km"`
}

// ZoneStats represents zone-level supply/demand snapshot.
type ZoneStats struct {
	ZoneID         string    `json:"zone_id"`
	OnlineDrivers  int       `json:"online_drivers"`
	PendingOrders  int       `json:"pending_orders"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ── Sentinel errors ─────────────────────────────────────────────────────────

// ErrNotFound is returned when a Redis key/field is missing.
var ErrNotFound = errors.New("matching repository: not found")

// ── Default TTLs ────────────────────────────────────────────────────────────

const (
	StatusTTL      = 60 * time.Second  // heartbeat window
	SessionTTL     = 10 * time.Minute  // matching session lifetime
	OfferTTL       = 15 * time.Second  // single driver-offer window
	RejectTTL      = 10 * time.Minute  // prevent re-broadcasting to same driver
	ZoneStatsTTL   = 30 * time.Second  // zone aggregates
	EarningsTTL    = 48 * time.Hour    // ~2 days, then auto-purge
	RatingCacheTTL = 1 * time.Hour
)

// ── Interface ──────────────────────────────────────────────────────────────

// MatchingRepositoryInterface defines the contract for matching-state data access.
//
//go:generate mockgen -source=matching_repository.go -destination=../../mocks/repomock/mock_matching_repository.go -package=repomock
type MatchingRepositoryInterface interface {
	// Driver status / heartbeat
	SetDriverStatus(ctx context.Context, s *DriverStatus) error
	GetDriverStatus(ctx context.Context, driverID string) (*DriverStatus, error)
	RefreshDriverStatusTTL(ctx context.Context, driverID string) (time.Duration, error)
	DeleteDriverStatus(ctx context.Context, driverID string) error
	UpdateDriverLocation(ctx context.Context, driverID string, lat, lng float64) error

	// Geo index (per vehicle_type)
	AddDriverToGeo(ctx context.Context, vehicleType, driverID string, lat, lng float64) error
	RemoveDriverFromGeo(ctx context.Context, vehicleType, driverID string) error
	NearbyDrivers(ctx context.Context, vehicleType string, lat, lng, radiusKm float64, limit int) ([]GeoDriver, error)

	// Driver mode (priority | normal)
	SetDriverMode(ctx context.Context, driverID string, m *DriverMode) error
	GetDriverMode(ctx context.Context, driverID string) (*DriverMode, error)
	DeleteDriverMode(ctx context.Context, driverID string) error

	// Daily earnings / trips
	IncrementDriverEarnings(ctx context.Context, driverID string, date string, fare int) error
	GetDriverEarnings(ctx context.Context, driverID, date string) (*DriverEarnings, error)

	// Acceptance rate counters
	IncrementAcceptance(ctx context.Context, driverID string, accepted bool) error
	GetAcceptance(ctx context.Context, driverID string) (*DriverAcceptance, error)

	// Active order lock — prevents double-assignment
	SetActiveOrder(ctx context.Context, driverID, orderID string) error
	GetActiveOrder(ctx context.Context, driverID string) (string, error)
	ClearActiveOrder(ctx context.Context, driverID string) error

	// Cached rating (loaded from Rating Service)
	SetCachedRating(ctx context.Context, driverID string, rating float64) error
	GetCachedRating(ctx context.Context, driverID string) (float64, error)

	// Matching session lifecycle
	CreateSession(ctx context.Context, s *MatchingSession) error
	GetSession(ctx context.Context, orderID string) (*MatchingSession, error)
	UpdateSession(ctx context.Context, s *MatchingSession) error
	DeleteSession(ctx context.Context, orderID string) error

	// Reject markers — prevents same driver receiving same order again
	MarkRejected(ctx context.Context, orderID, driverID string) error
	IsRejected(ctx context.Context, orderID, driverID string) (bool, error)

	// Zone aggregates
	UpsertZoneStats(ctx context.Context, vehicleType, zoneID string, online, pending int) error
	GetZoneStats(ctx context.Context, vehicleType, zoneID string) (*ZoneStats, error)
}

// ── Implementation ─────────────────────────────────────────────────────────

// MatchingRepository implements MatchingRepositoryInterface using go-redis/v9.
type MatchingRepository struct {
	rdb *redis.Client
}

// NewMatchingRepository creates a new MatchingRepository.
func NewMatchingRepository(rdb *redis.Client) *MatchingRepository {
	return &MatchingRepository{rdb: rdb}
}

// ── Key builders ───────────────────────────────────────────────────────────

func keyDriverStatus(driverID string) string  { return "driver:status:" + driverID }
func keyDriverMode(driverID string) string    { return "driver:mode:" + driverID }
func keyDriverEarnings(driverID, date string) string {
	return "driver:earnings:" + driverID + ":" + date
}
func keyDriverAcceptance(driverID string) string { return "driver:acceptance:" + driverID }
func keyDriverActiveOrder(driverID string) string {
	return "driver:active_order:" + driverID
}
func keyDriverRating(driverID string) string { return "driver:rating:" + driverID }

func keyDriversGeo(vehicleType string) string  { return "drivers:geo:" + vehicleType }
func keyMatchingSession(orderID string) string { return "matching:session:" + orderID }
func keyMatchingReject(orderID, driverID string) string {
	return "matching:reject:" + orderID + ":" + driverID
}
func keyZoneStats(vehicleType, zoneID string) string {
	return "matching:stats:" + vehicleType + ":" + zoneID
}

// ── Driver status / heartbeat ──────────────────────────────────────────────

func (r *MatchingRepository) SetDriverStatus(ctx context.Context, s *DriverStatus) error {
	if s == nil || s.DriverID == "" {
		return errors.New("driver status: id required")
	}
	now := time.Now().UTC()
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = now
	}
	if s.OnlineSince.IsZero() {
		s.OnlineSince = now
	}
	fields := map[string]any{
		"status":       s.Status,
		"vehicle_type": s.VehicleType,
		"order_id":     s.OrderID,
		"lat":          formatFloat(s.Lat),
		"lng":          formatFloat(s.Lng),
		"updated_at":   s.UpdatedAt.Unix(),
		"online_since": s.OnlineSince.Unix(),
	}
	pipe := r.rdb.TxPipeline()
	pipe.HSet(ctx, keyDriverStatus(s.DriverID), fields)
	pipe.Expire(ctx, keyDriverStatus(s.DriverID), StatusTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *MatchingRepository) GetDriverStatus(ctx context.Context, driverID string) (*DriverStatus, error) {
	res, err := r.rdb.HGetAll(ctx, keyDriverStatus(driverID)).Result()
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, ErrNotFound
	}
	s := &DriverStatus{
		DriverID:    driverID,
		Status:      res["status"],
		VehicleType: res["vehicle_type"],
		OrderID:     res["order_id"],
	}
	s.Lat, _ = strconv.ParseFloat(res["lat"], 64)
	s.Lng, _ = strconv.ParseFloat(res["lng"], 64)
	if v, err := strconv.ParseInt(res["updated_at"], 10, 64); err == nil {
		s.UpdatedAt = time.Unix(v, 0).UTC()
	}
	if v, err := strconv.ParseInt(res["online_since"], 10, 64); err == nil {
		s.OnlineSince = time.Unix(v, 0).UTC()
	}
	return s, nil
}

func (r *MatchingRepository) RefreshDriverStatusTTL(ctx context.Context, driverID string) (time.Duration, error) {
	ok, err := r.rdb.Expire(ctx, keyDriverStatus(driverID), StatusTTL).Result()
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNotFound
	}
	return StatusTTL, nil
}

func (r *MatchingRepository) DeleteDriverStatus(ctx context.Context, driverID string) error {
	_, err := r.rdb.Del(ctx, keyDriverStatus(driverID)).Result()
	return err
}

func (r *MatchingRepository) UpdateDriverLocation(ctx context.Context, driverID string, lat, lng float64) error {
	pipe := r.rdb.TxPipeline()
	pipe.HSet(ctx, keyDriverStatus(driverID), map[string]any{
		"lat":        formatFloat(lat),
		"lng":        formatFloat(lng),
		"updated_at": time.Now().UTC().Unix(),
	})
	pipe.Expire(ctx, keyDriverStatus(driverID), StatusTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// ── Geo index ──────────────────────────────────────────────────────────────

func (r *MatchingRepository) AddDriverToGeo(ctx context.Context, vehicleType, driverID string, lat, lng float64) error {
	if lat < -85 || lat > 85 || lng < -180 || lng > 180 {
		return fmt.Errorf("invalid coords: lat=%f lng=%f", lat, lng)
	}
	_, err := r.rdb.GeoAdd(ctx, keyDriversGeo(vehicleType), &redis.GeoLocation{
		Name:      driverID,
		Latitude:  lat,
		Longitude: lng,
	}).Result()
	return err
}

func (r *MatchingRepository) RemoveDriverFromGeo(ctx context.Context, vehicleType, driverID string) error {
	_, err := r.rdb.ZRem(ctx, keyDriversGeo(vehicleType), driverID).Result()
	return err
}

func (r *MatchingRepository) NearbyDrivers(ctx context.Context, vehicleType string, lat, lng, radiusKm float64, limit int) ([]GeoDriver, error) {
	if limit <= 0 {
		limit = 20
	}
	res, err := r.rdb.GeoSearchLocation(ctx, keyDriversGeo(vehicleType), &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  lng,
			Latitude:   lat,
			Radius:     radiusKm,
			RadiusUnit: "km",
			Sort:       "ASC",
			Count:      limit,
		},
		WithCoord: true,
		WithDist:  true,
	}).Result()
	if err != nil {
		return nil, err
	}
	out := make([]GeoDriver, 0, len(res))
	for _, l := range res {
		out = append(out, GeoDriver{
			DriverID:   l.Name,
			Lat:        l.Latitude,
			Lng:        l.Longitude,
			DistanceKm: l.Dist,
		})
	}
	return out, nil
}

// ── Driver mode ────────────────────────────────────────────────────────────

func (r *MatchingRepository) SetDriverMode(ctx context.Context, driverID string, m *DriverMode) error {
	if m == nil {
		return errors.New("nil mode")
	}
	if m.ActivatedAt.IsZero() {
		m.ActivatedAt = time.Now().UTC()
	}
	return r.rdb.HSet(ctx, keyDriverMode(driverID), map[string]any{
		"mode":         m.Mode,
		"daily_target": m.DailyTarget,
		"activated_at": m.ActivatedAt.Unix(),
	}).Err()
}

func (r *MatchingRepository) GetDriverMode(ctx context.Context, driverID string) (*DriverMode, error) {
	res, err := r.rdb.HGetAll(ctx, keyDriverMode(driverID)).Result()
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, ErrNotFound
	}
	m := &DriverMode{Mode: res["mode"]}
	if v, err := strconv.Atoi(res["daily_target"]); err == nil {
		m.DailyTarget = v
	}
	if v, err := strconv.ParseInt(res["activated_at"], 10, 64); err == nil {
		m.ActivatedAt = time.Unix(v, 0).UTC()
	}
	return m, nil
}

func (r *MatchingRepository) DeleteDriverMode(ctx context.Context, driverID string) error {
	return r.rdb.Del(ctx, keyDriverMode(driverID)).Err()
}

// ── Earnings / trips ───────────────────────────────────────────────────────

func (r *MatchingRepository) IncrementDriverEarnings(ctx context.Context, driverID, date string, fare int) error {
	pipe := r.rdb.TxPipeline()
	pipe.HIncrBy(ctx, keyDriverEarnings(driverID, date), "total_earnings", int64(fare))
	pipe.HIncrBy(ctx, keyDriverEarnings(driverID, date), "trip_count", 1)
	pipe.Expire(ctx, keyDriverEarnings(driverID, date), EarningsTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *MatchingRepository) GetDriverEarnings(ctx context.Context, driverID, date string) (*DriverEarnings, error) {
	res, err := r.rdb.HGetAll(ctx, keyDriverEarnings(driverID, date)).Result()
	if err != nil {
		return nil, err
	}
	out := &DriverEarnings{Date: date}
	if v, err := strconv.Atoi(res["total_earnings"]); err == nil {
		out.TotalEarnings = v
	}
	if v, err := strconv.Atoi(res["trip_count"]); err == nil {
		out.TripCount = v
	}
	return out, nil
}

// ── Acceptance counters ────────────────────────────────────────────────────

func (r *MatchingRepository) IncrementAcceptance(ctx context.Context, driverID string, accepted bool) error {
	field := "rejected"
	if accepted {
		field = "accepted"
	}
	return r.rdb.HIncrBy(ctx, keyDriverAcceptance(driverID), field, 1).Err()
}

func (r *MatchingRepository) GetAcceptance(ctx context.Context, driverID string) (*DriverAcceptance, error) {
	res, err := r.rdb.HGetAll(ctx, keyDriverAcceptance(driverID)).Result()
	if err != nil {
		return nil, err
	}
	out := &DriverAcceptance{}
	if v, err := strconv.Atoi(res["accepted"]); err == nil {
		out.Accepted = v
	}
	if v, err := strconv.Atoi(res["rejected"]); err == nil {
		out.Rejected = v
	}
	return out, nil
}

// ── Active order lock ──────────────────────────────────────────────────────

func (r *MatchingRepository) SetActiveOrder(ctx context.Context, driverID, orderID string) error {
	// SETNX-style: only succeeds if no current active order. Prevents double-assignment.
	ok, err := r.rdb.SetNX(ctx, keyDriverActiveOrder(driverID), orderID, 2*time.Hour).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound // caller treats as conflict
	}
	return nil
}

func (r *MatchingRepository) GetActiveOrder(ctx context.Context, driverID string) (string, error) {
	v, err := r.rdb.Get(ctx, keyDriverActiveOrder(driverID)).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return v, nil
}

func (r *MatchingRepository) ClearActiveOrder(ctx context.Context, driverID string) error {
	return r.rdb.Del(ctx, keyDriverActiveOrder(driverID)).Err()
}

// ── Cached rating ─────────────────────────────────────────────────────────

func (r *MatchingRepository) SetCachedRating(ctx context.Context, driverID string, rating float64) error {
	return r.rdb.Set(ctx, keyDriverRating(driverID), formatFloat(rating), RatingCacheTTL).Err()
}

func (r *MatchingRepository) GetCachedRating(ctx context.Context, driverID string) (float64, error) {
	v, err := r.rdb.Get(ctx, keyDriverRating(driverID)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(v, 64)
}

// ── Matching session ──────────────────────────────────────────────────────

func (r *MatchingRepository) CreateSession(ctx context.Context, s *MatchingSession) error {
	if s == nil || s.OrderID == "" {
		return errors.New("session: order_id required")
	}
	exists, err := r.rdb.Exists(ctx, keyMatchingSession(s.OrderID)).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		return ErrNotFound // caller treats as conflict (already exists)
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	pipe := r.rdb.TxPipeline()
	pipe.HSet(ctx, keyMatchingSession(s.OrderID), sessionFields(s))
	pipe.Expire(ctx, keyMatchingSession(s.OrderID), SessionTTL)
	_, err = pipe.Exec(ctx)
	return err
}

func (r *MatchingRepository) GetSession(ctx context.Context, orderID string) (*MatchingSession, error) {
	res, err := r.rdb.HGetAll(ctx, keyMatchingSession(orderID)).Result()
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, ErrNotFound
	}
	return parseSession(orderID, res), nil
}

func (r *MatchingRepository) UpdateSession(ctx context.Context, s *MatchingSession) error {
	if s == nil || s.OrderID == "" {
		return errors.New("session: order_id required")
	}
	pipe := r.rdb.TxPipeline()
	pipe.HSet(ctx, keyMatchingSession(s.OrderID), sessionFields(s))
	pipe.Expire(ctx, keyMatchingSession(s.OrderID), SessionTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *MatchingRepository) DeleteSession(ctx context.Context, orderID string) error {
	return r.rdb.Del(ctx, keyMatchingSession(orderID)).Err()
}

func sessionFields(s *MatchingSession) map[string]any {
	fields := map[string]any{
		"session_id":          s.SessionID,
		"service_type":        s.ServiceType,
		"status":              s.Status,
		"pickup_lat":          formatFloat(s.PickupLat),
		"pickup_lng":          formatFloat(s.PickupLng),
		"search_radius_km":    formatFloat(s.SearchRadiusKm),
		"max_rounds":          s.MaxRounds,
		"candidates_tried":    s.CandidatesTried,
		"current_candidate":   s.CurrentCandidateID,
		"matched_driver":      s.MatchedDriverID,
		"callback_url":        s.CallbackURL,
		"created_at":          s.CreatedAt.Unix(),
	}
	if !s.OfferExpiresAt.IsZero() {
		fields["offer_expires_at"] = s.OfferExpiresAt.Unix()
	}
	if !s.MatchedAt.IsZero() {
		fields["matched_at"] = s.MatchedAt.Unix()
	}
	return fields
}

func parseSession(orderID string, res map[string]string) *MatchingSession {
	s := &MatchingSession{
		OrderID:            orderID,
		SessionID:          res["session_id"],
		ServiceType:        res["service_type"],
		Status:             res["status"],
		CurrentCandidateID: res["current_candidate"],
		MatchedDriverID:    res["matched_driver"],
		CallbackURL:        res["callback_url"],
	}
	s.PickupLat, _ = strconv.ParseFloat(res["pickup_lat"], 64)
	s.PickupLng, _ = strconv.ParseFloat(res["pickup_lng"], 64)
	s.SearchRadiusKm, _ = strconv.ParseFloat(res["search_radius_km"], 64)
	if v, err := strconv.Atoi(res["max_rounds"]); err == nil {
		s.MaxRounds = v
	}
	if v, err := strconv.Atoi(res["candidates_tried"]); err == nil {
		s.CandidatesTried = v
	}
	if v, err := strconv.ParseInt(res["created_at"], 10, 64); err == nil {
		s.CreatedAt = time.Unix(v, 0).UTC()
	}
	if v, err := strconv.ParseInt(res["offer_expires_at"], 10, 64); err == nil {
		s.OfferExpiresAt = time.Unix(v, 0).UTC()
	}
	if v, err := strconv.ParseInt(res["matched_at"], 10, 64); err == nil {
		s.MatchedAt = time.Unix(v, 0).UTC()
	}
	return s
}

// ── Reject markers ────────────────────────────────────────────────────────

func (r *MatchingRepository) MarkRejected(ctx context.Context, orderID, driverID string) error {
	return r.rdb.Set(ctx, keyMatchingReject(orderID, driverID), "1", RejectTTL).Err()
}

func (r *MatchingRepository) IsRejected(ctx context.Context, orderID, driverID string) (bool, error) {
	v, err := r.rdb.Exists(ctx, keyMatchingReject(orderID, driverID)).Result()
	if err != nil {
		return false, err
	}
	return v > 0, nil
}

// ── Zone stats ────────────────────────────────────────────────────────────

func (r *MatchingRepository) UpsertZoneStats(ctx context.Context, vehicleType, zoneID string, online, pending int) error {
	pipe := r.rdb.TxPipeline()
	pipe.HSet(ctx, keyZoneStats(vehicleType, zoneID), map[string]any{
		"available_count": online,
		"pending_orders":  pending,
		"updated_at":      time.Now().UTC().Unix(),
	})
	pipe.Expire(ctx, keyZoneStats(vehicleType, zoneID), ZoneStatsTTL)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *MatchingRepository) GetZoneStats(ctx context.Context, vehicleType, zoneID string) (*ZoneStats, error) {
	res, err := r.rdb.HGetAll(ctx, keyZoneStats(vehicleType, zoneID)).Result()
	if err != nil {
		return nil, err
	}
	if len(res) == 0 {
		return nil, ErrNotFound
	}
	out := &ZoneStats{ZoneID: zoneID}
	if v, err := strconv.Atoi(res["available_count"]); err == nil {
		out.OnlineDrivers = v
	}
	if v, err := strconv.Atoi(res["pending_orders"]); err == nil {
		out.PendingOrders = v
	}
	if v, err := strconv.ParseInt(res["updated_at"], 10, 64); err == nil {
		out.UpdatedAt = time.Unix(v, 0).UTC()
	}
	return out, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
