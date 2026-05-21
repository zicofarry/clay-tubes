// Package service implements the business logic for the Ride Order Service.
package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"net/http"
	"time"

	"github.com/zicofarry/clay-app/backend/services/ride-order-service/internal/repository"
)

// ── Service Error ────────────────────────────────────────────────────────────

// ServiceError represents a business-logic error with HTTP status mapping.
type ServiceError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *ServiceError) Error() string { return e.Message }

// Common errors. The HTTP layer maps these to the proper status codes.
var (
	ErrOrderNotFound          = &ServiceError{http.StatusNotFound, "ORDER_NOT_FOUND", "ride order not found"}
	ErrNoActiveOrder          = &ServiceError{http.StatusNotFound, "NO_ACTIVE_ORDER", "no active order found for this user"}
	ErrActiveOrderExists      = &ServiceError{http.StatusConflict, "ACTIVE_ORDER_EXISTS", "user already has an active ride order"}
	ErrForbidden              = &ServiceError{http.StatusForbidden, "FORBIDDEN", "you are not allowed to access this resource"}
	ErrInvalidStateTransition = &ServiceError{http.StatusBadRequest, "INVALID_STATE_TRANSITION", "order is not in a state that allows this action"}
	ErrCannotCancelOnTrip     = &ServiceError{http.StatusBadRequest, "CANNOT_CANCEL_ON_TRIP", "cannot cancel an order that is already on trip"}
	ErrOrderAlreadyTaken      = &ServiceError{http.StatusConflict, "ORDER_ALREADY_TAKEN", "this order has already been accepted by another driver"}
	ErrInvalidOTP             = &ServiceError{http.StatusBadRequest, "INVALID_OTP", "OTP code is incorrect"}
	ErrPromoInvalid           = &ServiceError{http.StatusUnprocessableEntity, "PROMO_INVALID", "promo code is invalid or expired"}
	ErrUpstreamUnavailable    = &ServiceError{http.StatusServiceUnavailable, "UPSTREAM_UNAVAILABLE", "upstream service unavailable"}
	ErrOrderNotCompleted      = &ServiceError{http.StatusUnprocessableEntity, "ORDER_NOT_COMPLETED", "can only rate a completed order"}
	ErrRatingAlreadySubmitted = &ServiceError{http.StatusConflict, "RATING_ALREADY_SUBMITTED", "rating already submitted for this order"}
	ErrFareNotFinalized       = &ServiceError{http.StatusNotFound, "FARE_NOT_FINALIZED", "fare not yet finalized for this order"}
	ErrValidation             = &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "request body validation failed"}
)

// ── Cancellable / state machine config ──────────────────────────────────────

var cancellableStates = map[string]bool{
	"pending":        true,
	"finding_driver": true,
	"assigned":       true,
	"on_pickup":      true,
}

// validTransitions defines allowed driver actions.
//   key   = (action, fromState)
//   value = toState
var driverTransitions = map[string]struct {
	from string
	to   string
}{
	"arrived_at_pickup": {from: "assigned", to: "on_pickup"},
	"start_trip":        {from: "on_pickup", to: "on_trip"},
	"complete_trip":     {from: "on_trip", to: "completed"},
}

// ── Request/Response DTOs ───────────────────────────────────────────────────

type CreateRideOrderRequest struct {
	ServiceType   string  `json:"service_type"`
	VehicleType   string  `json:"vehicle_type"`
	OriginLat     float64 `json:"origin_lat"`
	OriginLng     float64 `json:"origin_lng"`
	OriginAddress string  `json:"origin_address"`
	DestLat       float64 `json:"dest_lat"`
	DestLng       float64 `json:"dest_lng"`
	DestAddress   string  `json:"dest_address"`
	PaymentMethod string  `json:"payment_method"`
	PromoID       string  `json:"promo_id,omitempty"`
	FareEstimate  float64 `json:"fare_estimate,omitempty"`
}

type InternalCreateOrderRequest struct {
	UserID string `json:"user_id"`
	CreateRideOrderRequest
}

type FareEstimateRequest struct {
	OriginLat   float64 `json:"origin_lat"`
	OriginLng   float64 `json:"origin_lng"`
	DestLat     float64 `json:"dest_lat"`
	DestLng     float64 `json:"dest_lng"`
	VehicleType string  `json:"vehicle_type"`
	PromoID     string  `json:"promo_id,omitempty"`
}

type FareEstimateResponse struct {
	VehicleType     string                  `json:"vehicle_type"`
	DistanceKm      float64                 `json:"distance_km"`
	DurationMin     int                     `json:"duration_min"`
	FareEstimate    float64                 `json:"fare_estimate"`
	SurgeMultiplier float64                 `json:"surge_multiplier"`
	PromoDiscount   float64                 `json:"promo_discount"`
	FareAfterPromo  float64                 `json:"fare_after_promo"`
	Breakdown       FareBreakdownResponse   `json:"breakdown"`
}

type FareBreakdownResponse struct {
	BaseFare        float64 `json:"base_fare"`
	DistanceFare    float64 `json:"distance_fare"`
	TimeFare        float64 `json:"time_fare"`
	SurgeMultiplier float64 `json:"surge_multiplier"`
	PromoDiscount   float64 `json:"promo_discount"`
	PlatformFee     float64 `json:"platform_fee"`
	Total           float64 `json:"total"`
}

type RideOrderResponse struct {
	ID            string  `json:"id"`
	UserID        string  `json:"user_id"`
	DriverID      string  `json:"driver_id,omitempty"`
	ServiceType   string  `json:"service_type"`
	VehicleType   string  `json:"vehicle_type"`
	Status        string  `json:"status"`
	OriginLat     float64 `json:"origin_lat"`
	OriginLng     float64 `json:"origin_lng"`
	OriginAddress string  `json:"origin_address,omitempty"`
	DestLat       float64 `json:"dest_lat"`
	DestLng       float64 `json:"dest_lng"`
	DestAddress   string  `json:"dest_address,omitempty"`
	FareEstimate  float64 `json:"fare_estimate,omitempty"`
	FareFinal     float64 `json:"fare_final,omitempty"`
	PaymentMethod string  `json:"payment_method"`
	OTPCode       string  `json:"otp_code,omitempty"`
	CancelReason  string  `json:"cancel_reason,omitempty"`
	CancelledBy   string  `json:"cancelled_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type RideOrderDetailResponse struct {
	RideOrderResponse
	TripDetails *TripDetailsResponse  `json:"trip_details,omitempty"`
	StateLogs   []OrderStateLogResponse `json:"state_logs,omitempty"`
}

type TripDetailsResponse struct {
	Polyline           string  `json:"polyline,omitempty"`
	EstDistanceKm      float64 `json:"est_distance_km,omitempty"`
	EstDurationMin     int     `json:"est_duration_min,omitempty"`
	ActualDistanceKm   float64 `json:"actual_distance_km,omitempty"`
	ActualDurationMin  int     `json:"actual_duration_min,omitempty"`
	RouteDeviationKm   float64 `json:"route_deviation_km,omitempty"`
	PickupTime         *time.Time `json:"pickup_time,omitempty"`
	DropoffTime        *time.Time `json:"dropoff_time,omitempty"`
}

type OrderStateLogResponse struct {
	FromState string    `json:"from_state,omitempty"`
	ToState   string    `json:"to_state"`
	ActorType string    `json:"actor_type"`
	Reason    string    `json:"reason,omitempty"`
	ChangedAt time.Time `json:"changed_at"`
}

type RideOrderHistoryResponse struct {
	Orders []RideOrderResponse `json:"orders"`
	Total  int                 `json:"total"`
	Page   int                 `json:"page"`
	Limit  int                 `json:"limit"`
}

type HistoryQuery struct {
	Status      string
	ServiceType string
	From        string // YYYY-MM-DD
	To          string // YYYY-MM-DD
	Page        int
	Limit       int
}

type CancelOrderRequest struct {
	Reason string `json:"reason"`
}

type DriverUpdateStatusRequest struct {
	Action            string  `json:"action"`
	OTPCode           string  `json:"otp_code,omitempty"`
	ActualDistanceKm  float64 `json:"actual_distance_km,omitempty"`
	ActualDurationMin int     `json:"actual_duration_min,omitempty"`
	DropoffLat        float64 `json:"dropoff_lat,omitempty"`
	DropoffLng        float64 `json:"dropoff_lng,omitempty"`
}

type DriverRejectRequest struct {
	Reason string `json:"reason,omitempty"`
}

type DriverAcceptResponse struct {
	OrderID       string  `json:"order_id"`
	UserID        string  `json:"user_id"`
	OTPCode       string  `json:"otp_code"`
	OriginLat     float64 `json:"origin_lat"`
	OriginLng     float64 `json:"origin_lng"`
	OriginAddress string  `json:"origin_address,omitempty"`
	DestAddress   string  `json:"dest_address,omitempty"`
	FareEstimate  float64 `json:"fare_estimate"`
	PaymentMethod string  `json:"payment_method"`
	Status        string  `json:"status"`
}

type SubmitRatingRequest struct {
	Score   int      `json:"score"`
	Comment string   `json:"comment,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

type InternalUpdateStatusRequest struct {
	Status    string                 `json:"status"`
	ActorType string                 `json:"actor_type"`
	ActorID   string                 `json:"actor_id,omitempty"`
	Reason    string                 `json:"reason,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type InternalAssignDriverRequest struct {
	DriverID   string `json:"driver_id"`
	ETASeconds int    `json:"eta_seconds"`
}

type InternalAssignDriverResponse struct {
	OrderID  string `json:"order_id"`
	DriverID string `json:"driver_id"`
	OTPCode  string `json:"otp_code"`
	Status   string `json:"status"`
}

// ── Interface ───────────────────────────────────────────────────────────────

// RideOrderServiceInterface defines the service contract.
//
//go:generate mockgen -source=ride_order_service.go -destination=../../mocks/mock_ride_order_service.go -package=mocks
type RideOrderServiceInterface interface {
	// User-facing
	EstimateFare(ctx context.Context, req *FareEstimateRequest) (*FareEstimateResponse, error)
	CreateOrder(ctx context.Context, userID string, req *CreateRideOrderRequest) (*RideOrderResponse, error)
	GetActiveOrder(ctx context.Context, userID string) (*RideOrderResponse, error)
	GetOrderHistory(ctx context.Context, userID string, q HistoryQuery) (*RideOrderHistoryResponse, error)
	GetOrder(ctx context.Context, userID, role, orderID string) (*RideOrderDetailResponse, error)
	CancelOrder(ctx context.Context, userID, orderID string, req *CancelOrderRequest) (*RideOrderResponse, error)
	SubmitRating(ctx context.Context, userID, orderID string, req *SubmitRatingRequest) error
	GetFareBreakdown(ctx context.Context, userID, orderID string) (*FareBreakdownResponse, error)

	// Driver-facing
	DriverAcceptOrder(ctx context.Context, driverID, orderID string) (*DriverAcceptResponse, error)
	DriverRejectOrder(ctx context.Context, driverID, orderID string, req *DriverRejectRequest) error
	DriverUpdateOrderStatus(ctx context.Context, driverID, orderID string, req *DriverUpdateStatusRequest) (*RideOrderResponse, error)

	// Internal
	InternalCreateOrder(ctx context.Context, req *InternalCreateOrderRequest) (*RideOrderResponse, error)
	InternalGetOrder(ctx context.Context, orderID string) (*RideOrderResponse, error)
	InternalUpdateStatus(ctx context.Context, orderID string, req *InternalUpdateStatusRequest) (*RideOrderResponse, error)
	InternalAssignDriver(ctx context.Context, orderID string, req *InternalAssignDriverRequest) (*InternalAssignDriverResponse, error)
}

// ── Implementation ──────────────────────────────────────────────────────────

// RideOrderService implements RideOrderServiceInterface.
type RideOrderService struct {
	repo   repository.RideOrderRepositoryInterface
	logger *slog.Logger
}

// NewRideOrderService creates a new RideOrderService.
func NewRideOrderService(repo repository.RideOrderRepositoryInterface, logger *slog.Logger) *RideOrderService {
	return &RideOrderService{repo: repo, logger: logger}
}

// ── User-facing methods ─────────────────────────────────────────────────────

func (s *RideOrderService) EstimateFare(ctx context.Context, req *FareEstimateRequest) (*FareEstimateResponse, error) {
	if err := validateLatLng(req.OriginLat, req.OriginLng); err != nil {
		return nil, err
	}
	if err := validateLatLng(req.DestLat, req.DestLng); err != nil {
		return nil, err
	}
	if req.VehicleType != "motor" && req.VehicleType != "car" {
		return nil, &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "vehicle_type must be motor or car"}
	}

	// Stubbed pricing model — production would call Pricing/Geo services.
	distanceKm := haversineKm(req.OriginLat, req.OriginLng, req.DestLat, req.DestLng)
	durationMin := int(distanceKm * 4) // rough: ~15km/h average
	baseFare := 5000.0
	perKm := 2500.0
	perMin := 200.0
	if req.VehicleType == "car" {
		baseFare = 10000.0
		perKm = 4000.0
		perMin = 350.0
	}
	distanceFare := distanceKm * perKm
	timeFare := float64(durationMin) * perMin
	surge := 1.0
	platformFee := 1000.0

	subtotal := (baseFare + distanceFare + timeFare) * surge
	promoDiscount := 0.0
	if req.PromoID != "" {
		// Stub: 10% off, capped at 5000
		promoDiscount = subtotal * 0.10
		if promoDiscount > 5000 {
			promoDiscount = 5000
		}
	}
	total := subtotal + platformFee - promoDiscount
	if total < 0 {
		total = 0
	}

	return &FareEstimateResponse{
		VehicleType:     req.VehicleType,
		DistanceKm:      round2(distanceKm),
		DurationMin:     durationMin,
		FareEstimate:    round2(subtotal + platformFee),
		SurgeMultiplier: surge,
		PromoDiscount:   round2(promoDiscount),
		FareAfterPromo:  round2(total),
		Breakdown: FareBreakdownResponse{
			BaseFare:        baseFare,
			DistanceFare:    round2(distanceFare),
			TimeFare:        round2(timeFare),
			SurgeMultiplier: surge,
			PromoDiscount:   round2(promoDiscount),
			PlatformFee:     platformFee,
			Total:           round2(total),
		},
	}, nil
}

func (s *RideOrderService) CreateOrder(ctx context.Context, userID string, req *CreateRideOrderRequest) (*RideOrderResponse, error) {
	if userID == "" {
		return nil, ErrForbidden
	}
	if err := validateCreateOrder(req); err != nil {
		return nil, err
	}

	// Anti-double-booking
	if active, _ := s.repo.GetActiveOrderByUserID(ctx, userID); active != nil {
		return nil, ErrActiveOrderExists
	}

	// Build order in pending → finding_driver state.
	o := &repository.RideOrder{
		UserID:        userID,
		ServiceType:   req.ServiceType,
		VehicleType:   req.VehicleType,
		Status:        "finding_driver",
		OriginLat:     req.OriginLat,
		OriginLng:     req.OriginLng,
		OriginAddress: nullableString(req.OriginAddress),
		DestLat:       req.DestLat,
		DestLng:       req.DestLng,
		DestAddress:   nullableString(req.DestAddress),
		FareEstimate:  nullableFloat(req.FareEstimate),
		PromoID:       nullableString(req.PromoID),
		PaymentMethod: req.PaymentMethod,
	}

	created, err := s.repo.CreateOrder(ctx, o)
	if err != nil {
		return nil, err
	}

	// State log: nil → finding_driver
	_ = s.repo.InsertStateLog(ctx, &repository.OrderStateLog{
		OrderID:   created.ID,
		ToState:   "finding_driver",
		ActorID:   sql.NullString{String: userID, Valid: true},
		ActorType: "user",
	})

	s.logger.Info("ride order created",
		slog.String("order_id", created.ID),
		slog.String("user_id", userID),
		slog.String("service_type", req.ServiceType),
	)

	// TODO: publish Order_Created Kafka event
	// TODO: SET user:active_order:{user_id} in Redis (TTL 2h)

	return toOrderResponse(created, false), nil
}

func (s *RideOrderService) GetActiveOrder(ctx context.Context, userID string) (*RideOrderResponse, error) {
	if userID == "" {
		return nil, ErrForbidden
	}
	o, err := s.repo.GetActiveOrderByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoActiveOrder
		}
		return nil, err
	}
	return toOrderResponse(o, false), nil
}

func (s *RideOrderService) GetOrderHistory(ctx context.Context, userID string, q HistoryQuery) (*RideOrderHistoryResponse, error) {
	if userID == "" {
		return nil, ErrForbidden
	}
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 || q.Limit > 50 {
		q.Limit = 10
	}

	f := repository.HistoryFilter{
		Status:      q.Status,
		ServiceType: q.ServiceType,
		Limit:       q.Limit,
		Offset:      (q.Page - 1) * q.Limit,
	}
	if q.From != "" {
		if t, err := time.Parse("2006-01-02", q.From); err == nil {
			f.From = &t
		}
	}
	if q.To != "" {
		if t, err := time.Parse("2006-01-02", q.To); err == nil {
			t = t.Add(24*time.Hour - time.Second)
			f.To = &t
		}
	}

	rows, total, err := s.repo.ListUserHistory(ctx, userID, f)
	if err != nil {
		return nil, err
	}
	out := &RideOrderHistoryResponse{
		Orders: make([]RideOrderResponse, 0, len(rows)),
		Total:  total,
		Page:   q.Page,
		Limit:  q.Limit,
	}
	for _, o := range rows {
		out.Orders = append(out.Orders, *toOrderResponse(o, false))
	}
	return out, nil
}

func (s *RideOrderService) GetOrder(ctx context.Context, userID, role, orderID string) (*RideOrderDetailResponse, error) {
	o, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	// Authorization: user owns it OR (role=driver AND driver_id matches)
	if role == "driver" {
		if !o.DriverID.Valid || o.DriverID.String != userID {
			return nil, ErrForbidden
		}
	} else {
		if o.UserID != userID {
			return nil, ErrForbidden
		}
	}

	resp := &RideOrderDetailResponse{RideOrderResponse: *toOrderResponse(o, role == "driver")}

	if td, err := s.repo.GetTripDetails(ctx, orderID); err == nil && td != nil {
		resp.TripDetails = toTripDetailsResponse(td)
	}
	if logs, err := s.repo.ListStateLogs(ctx, orderID); err == nil {
		for _, l := range logs {
			resp.StateLogs = append(resp.StateLogs, *toStateLogResponse(l))
		}
	}
	return resp, nil
}

func (s *RideOrderService) CancelOrder(ctx context.Context, userID, orderID string, req *CancelOrderRequest) (*RideOrderResponse, error) {
	o, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	if o.UserID != userID {
		return nil, ErrForbidden
	}
	if o.Status == "on_trip" {
		return nil, ErrCannotCancelOnTrip
	}
	if !cancellableStates[o.Status] {
		return nil, ErrInvalidStateTransition
	}

	reason := ""
	if req != nil {
		reason = req.Reason
	}
	if err := s.repo.SetCancelled(ctx, orderID, reason, "user"); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidStateTransition
		}
		return nil, err
	}

	_ = s.repo.InsertStateLog(ctx, &repository.OrderStateLog{
		OrderID:   orderID,
		FromState: sql.NullString{String: o.Status, Valid: true},
		ToState:   "cancelled",
		ActorID:   sql.NullString{String: userID, Valid: true},
		ActorType: "user",
		Reason:    sql.NullString{String: reason, Valid: reason != ""},
	})

	s.logger.Info("order cancelled", slog.String("order_id", orderID), slog.String("by", "user"))

	// TODO: publish Order_Cancelled Kafka event

	updated, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return toOrderResponse(updated, false), nil
}

func (s *RideOrderService) SubmitRating(ctx context.Context, userID, orderID string, req *SubmitRatingRequest) error {
	if req == nil || req.Score < 1 || req.Score > 5 {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "score must be between 1 and 5"}
	}
	o, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrOrderNotFound
		}
		return err
	}
	if o.UserID != userID {
		return ErrForbidden
	}
	if o.Status != "completed" {
		return ErrOrderNotCompleted
	}

	// TODO: forward to Rating Service via gRPC/HTTP and dedupe via shared idempotency.
	s.logger.Info("rating submitted",
		slog.String("order_id", orderID),
		slog.String("user_id", userID),
		slog.Int("score", req.Score),
	)
	return nil
}

func (s *RideOrderService) GetFareBreakdown(ctx context.Context, userID, orderID string) (*FareBreakdownResponse, error) {
	o, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	// Owner OR assigned driver
	if o.UserID != userID && (!o.DriverID.Valid || o.DriverID.String != userID) {
		return nil, ErrForbidden
	}
	fb, err := s.repo.GetFareBreakdown(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFareNotFinalized
		}
		return nil, err
	}
	return &FareBreakdownResponse{
		BaseFare:        fb.BaseFare,
		DistanceFare:    fb.DistanceFare,
		TimeFare:        fb.TimeFare,
		SurgeMultiplier: fb.SurgeMultiplier,
		PromoDiscount:   fb.PromoDiscount,
		PlatformFee:     fb.PlatformFee,
		Total:           fb.Total,
	}, nil
}

// ── Driver-facing methods ───────────────────────────────────────────────────

func (s *RideOrderService) DriverAcceptOrder(ctx context.Context, driverID, orderID string) (*DriverAcceptResponse, error) {
	o, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	if o.Status != "finding_driver" {
		return nil, ErrOrderAlreadyTaken
	}

	otp := generateOTP()
	if err := s.repo.AssignDriver(ctx, orderID, driverID, otp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderAlreadyTaken
		}
		return nil, err
	}

	_ = s.repo.InsertStateLog(ctx, &repository.OrderStateLog{
		OrderID:   orderID,
		FromState: sql.NullString{String: "finding_driver", Valid: true},
		ToState:   "assigned",
		ActorID:   sql.NullString{String: driverID, Valid: true},
		ActorType: "driver",
	})

	s.logger.Info("driver accepted order",
		slog.String("order_id", orderID),
		slog.String("driver_id", driverID),
	)

	updated, _ := s.repo.GetOrderByID(ctx, orderID)
	if updated == nil {
		updated = o
	}

	return &DriverAcceptResponse{
		OrderID:       orderID,
		UserID:        updated.UserID,
		OTPCode:       otp,
		OriginLat:     updated.OriginLat,
		OriginLng:     updated.OriginLng,
		OriginAddress: updated.OriginAddress.String,
		DestAddress:   updated.DestAddress.String,
		FareEstimate:  updated.FareEstimate.Float64,
		PaymentMethod: updated.PaymentMethod,
		Status:        "assigned",
	}, nil
}

func (s *RideOrderService) DriverRejectOrder(ctx context.Context, driverID, orderID string, req *DriverRejectRequest) error {
	o, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrOrderNotFound
		}
		return err
	}

	reason := ""
	if req != nil {
		reason = req.Reason
	}

	_ = s.repo.InsertStateLog(ctx, &repository.OrderStateLog{
		OrderID:   o.ID,
		FromState: sql.NullString{String: o.Status, Valid: true},
		ToState:   o.Status,
		ActorID:   sql.NullString{String: driverID, Valid: true},
		ActorType: "driver",
		Reason:    sql.NullString{String: "rejected:" + reason, Valid: true},
	})

	s.logger.Info("driver rejected order",
		slog.String("order_id", orderID),
		slog.String("driver_id", driverID),
		slog.String("reason", reason),
	)

	// TODO: notify Matching Service to re-broadcast.
	return nil
}

func (s *RideOrderService) DriverUpdateOrderStatus(ctx context.Context, driverID, orderID string, req *DriverUpdateStatusRequest) (*RideOrderResponse, error) {
	if req == nil {
		return nil, ErrValidation
	}
	tr, ok := driverTransitions[req.Action]
	if !ok {
		return nil, &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "unknown action: " + req.Action}
	}

	o, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	if !o.DriverID.Valid || o.DriverID.String != driverID {
		return nil, ErrForbidden
	}
	if o.Status != tr.from {
		return nil, &ServiceError{
			http.StatusBadRequest,
			"INVALID_STATE_TRANSITION",
			fmt.Sprintf("cannot perform %s on order in %s state", req.Action, o.Status),
		}
	}

	// Action-specific guards
	switch req.Action {
	case "start_trip":
		if !o.OTPCode.Valid || req.OTPCode == "" || req.OTPCode != o.OTPCode.String {
			return nil, ErrInvalidOTP
		}
	case "complete_trip":
		if req.ActualDistanceKm <= 0 || req.ActualDurationMin <= 0 {
			return nil, &ServiceError{
				http.StatusBadRequest,
				"VALIDATION_ERROR",
				"actual_distance_km and actual_duration_min are required for complete_trip",
			}
		}
	}

	if err := s.repo.UpdateStatus(ctx, orderID, tr.from, tr.to); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidStateTransition
		}
		return nil, err
	}

	_ = s.repo.InsertStateLog(ctx, &repository.OrderStateLog{
		OrderID:   orderID,
		FromState: sql.NullString{String: tr.from, Valid: true},
		ToState:   tr.to,
		ActorID:   sql.NullString{String: driverID, Valid: true},
		ActorType: "driver",
	})

	// Side effects per action
	now := time.Now().UTC()
	switch req.Action {
	case "start_trip":
		_ = s.repo.UpsertTripDetails(ctx, &repository.TripDetails{
			OrderID:    orderID,
			PickupTime: sql.NullTime{Time: now, Valid: true},
		})
	case "complete_trip":
		_ = s.repo.UpsertTripDetails(ctx, &repository.TripDetails{
			OrderID:           orderID,
			ActualDistanceKm:  sql.NullFloat64{Float64: req.ActualDistanceKm, Valid: true},
			ActualDurationMin: sql.NullInt32{Int32: int32(req.ActualDurationMin), Valid: true},
			DropoffTime:       sql.NullTime{Time: now, Valid: true},
		})

		// Compute fare_final + breakdown
		fb := calculateFinalFare(o.VehicleType, req.ActualDistanceKm, req.ActualDurationMin, o.FareEstimate.Float64)
		fb.OrderID = orderID
		_ = s.repo.UpsertFareBreakdown(ctx, fb)
		_ = s.repo.SetFareFinal(ctx, orderID, fb.Total)
		// TODO: publish Trip_Completed Kafka event.
	}

	s.logger.Info("driver updated order status",
		slog.String("order_id", orderID),
		slog.String("driver_id", driverID),
		slog.String("action", req.Action),
		slog.String("from", tr.from),
		slog.String("to", tr.to),
	)

	updated, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return toOrderResponse(updated, true), nil
}

// ── Internal methods ────────────────────────────────────────────────────────

func (s *RideOrderService) InternalCreateOrder(ctx context.Context, req *InternalCreateOrderRequest) (*RideOrderResponse, error) {
	if req == nil || req.UserID == "" {
		return nil, ErrValidation
	}
	return s.CreateOrder(ctx, req.UserID, &req.CreateRideOrderRequest)
}

func (s *RideOrderService) InternalGetOrder(ctx context.Context, orderID string) (*RideOrderResponse, error) {
	o, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return toOrderResponse(o, true), nil
}

func (s *RideOrderService) InternalUpdateStatus(ctx context.Context, orderID string, req *InternalUpdateStatusRequest) (*RideOrderResponse, error) {
	if req == nil || req.Status == "" || req.ActorType == "" {
		return nil, ErrValidation
	}
	o, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	if o.Status == "completed" || o.Status == "cancelled" {
		return nil, &ServiceError{http.StatusConflict, "TERMINAL_STATE", "order is already in a terminal state"}
	}
	if err := s.repo.UpdateStatus(ctx, orderID, o.Status, req.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidStateTransition
		}
		return nil, err
	}

	_ = s.repo.InsertStateLog(ctx, &repository.OrderStateLog{
		OrderID:   orderID,
		FromState: sql.NullString{String: o.Status, Valid: true},
		ToState:   req.Status,
		ActorID:   sql.NullString{String: req.ActorID, Valid: req.ActorID != ""},
		ActorType: req.ActorType,
		Reason:    sql.NullString{String: req.Reason, Valid: req.Reason != ""},
	})

	updated, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return toOrderResponse(updated, true), nil
}

func (s *RideOrderService) InternalAssignDriver(ctx context.Context, orderID string, req *InternalAssignDriverRequest) (*InternalAssignDriverResponse, error) {
	if req == nil || req.DriverID == "" {
		return nil, ErrValidation
	}
	o, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	if o.Status != "finding_driver" {
		return nil, ErrOrderAlreadyTaken
	}

	otp := generateOTP()
	if err := s.repo.AssignDriver(ctx, orderID, req.DriverID, otp); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderAlreadyTaken
		}
		return nil, err
	}

	_ = s.repo.InsertStateLog(ctx, &repository.OrderStateLog{
		OrderID:   orderID,
		FromState: sql.NullString{String: "finding_driver", Valid: true},
		ToState:   "assigned",
		ActorType: "system",
	})

	return &InternalAssignDriverResponse{
		OrderID:  orderID,
		DriverID: req.DriverID,
		OTPCode:  otp,
		Status:   "assigned",
	}, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func validateLatLng(lat, lng float64) error {
	if lat < -90 || lat > 90 {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "latitude must be between -90 and 90"}
	}
	if lng < -180 || lng > 180 {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "longitude must be between -180 and 180"}
	}
	return nil
}

func validateCreateOrder(req *CreateRideOrderRequest) error {
	if req == nil {
		return ErrValidation
	}
	if req.ServiceType != "goride" && req.ServiceType != "gocar" {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "service_type must be goride or gocar"}
	}
	if req.VehicleType != "motor" && req.VehicleType != "car" {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "vehicle_type must be motor or car"}
	}
	if req.PaymentMethod != "gopay" && req.PaymentMethod != "cash" {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "payment_method must be gopay or cash"}
	}
	if err := validateLatLng(req.OriginLat, req.OriginLng); err != nil {
		return err
	}
	if err := validateLatLng(req.DestLat, req.DestLng); err != nil {
		return err
	}
	return nil
}

func generateOTP() string {
	const digits = "0123456789"
	b := make([]byte, 6)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		b[i] = digits[n.Int64()]
	}
	return string(b)
}

// haversineKm returns approximate distance in km between two coordinates.
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const earthKm = 6371.0
	rad := math.Pi / 180
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	rLat1 := lat1 * rad
	rLat2 := lat2 * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rLat1)*math.Cos(rLat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return earthKm * 2 * math.Asin(math.Sqrt(a))
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

func calculateFinalFare(vehicleType string, distanceKm float64, durationMin int, estimate float64) *repository.FareBreakdown {
	baseFare := 5000.0
	perKm := 2500.0
	perMin := 200.0
	if vehicleType == "car" {
		baseFare = 10000.0
		perKm = 4000.0
		perMin = 350.0
	}
	distanceFare := round2(distanceKm * perKm)
	timeFare := round2(float64(durationMin) * perMin)
	platformFee := 1000.0
	surge := 1.0
	total := round2(baseFare + distanceFare + timeFare + platformFee)

	return &repository.FareBreakdown{
		BaseFare:        baseFare,
		DistanceFare:    distanceFare,
		TimeFare:        timeFare,
		SurgeMultiplier: surge,
		PromoDiscount:   0,
		PlatformFee:     platformFee,
		Total:           total,
	}
}

func nullableString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
func nullableFloat(f float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: f, Valid: f != 0}
}

// ── Mappers ─────────────────────────────────────────────────────────────────

func toOrderResponse(o *repository.RideOrder, includeOTP bool) *RideOrderResponse {
	r := &RideOrderResponse{
		ID:            o.ID,
		UserID:        o.UserID,
		ServiceType:   o.ServiceType,
		VehicleType:   o.VehicleType,
		Status:        o.Status,
		OriginLat:     o.OriginLat,
		OriginLng:     o.OriginLng,
		DestLat:       o.DestLat,
		DestLng:       o.DestLng,
		PaymentMethod: o.PaymentMethod,
		CreatedAt:     o.CreatedAt,
		UpdatedAt:     o.UpdatedAt,
	}
	if o.DriverID.Valid {
		r.DriverID = o.DriverID.String
	}
	if o.OriginAddress.Valid {
		r.OriginAddress = o.OriginAddress.String
	}
	if o.DestAddress.Valid {
		r.DestAddress = o.DestAddress.String
	}
	if o.FareEstimate.Valid {
		r.FareEstimate = o.FareEstimate.Float64
	}
	if o.FareFinal.Valid {
		r.FareFinal = o.FareFinal.Float64
	}
	if o.CancelReason.Valid {
		r.CancelReason = o.CancelReason.String
	}
	if o.CancelledBy.Valid {
		r.CancelledBy = o.CancelledBy.String
	}
	if includeOTP && o.OTPCode.Valid {
		r.OTPCode = o.OTPCode.String
	}
	return r
}

func toTripDetailsResponse(td *repository.TripDetails) *TripDetailsResponse {
	r := &TripDetailsResponse{}
	if td.Polyline.Valid {
		r.Polyline = td.Polyline.String
	}
	if td.EstDistanceKm.Valid {
		r.EstDistanceKm = td.EstDistanceKm.Float64
	}
	if td.EstDurationMin.Valid {
		r.EstDurationMin = int(td.EstDurationMin.Int32)
	}
	if td.ActualDistanceKm.Valid {
		r.ActualDistanceKm = td.ActualDistanceKm.Float64
	}
	if td.ActualDurationMin.Valid {
		r.ActualDurationMin = int(td.ActualDurationMin.Int32)
	}
	if td.RouteDeviationKm.Valid {
		r.RouteDeviationKm = td.RouteDeviationKm.Float64
	}
	if td.PickupTime.Valid {
		t := td.PickupTime.Time
		r.PickupTime = &t
	}
	if td.DropoffTime.Valid {
		t := td.DropoffTime.Time
		r.DropoffTime = &t
	}
	return r
}

func toStateLogResponse(l *repository.OrderStateLog) *OrderStateLogResponse {
	r := &OrderStateLogResponse{
		ToState:   l.ToState,
		ActorType: l.ActorType,
		ChangedAt: l.ChangedAt,
	}
	if l.FromState.Valid {
		r.FromState = l.FromState.String
	}
	if l.Reason.Valid {
		r.Reason = l.Reason.String
	}
	return r
}
