// Package service implements the business logic for the Delivery Order Service.
package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/zicofarry/clay-app/backend/services/delivery-order-service/internal/repository"
)

// ── Service Error ────────────────────────────────────────────────────────────

// ServiceError represents a business-logic error with HTTP status mapping.
type ServiceError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *ServiceError) Error() string { return e.Message }

// Common errors.
var (
	ErrOrderNotFound            = &ServiceError{http.StatusNotFound, "ORDER_NOT_FOUND", "delivery order not found"}
	ErrNoActiveOrder            = &ServiceError{http.StatusNotFound, "NO_ACTIVE_ORDER", "no active delivery order found for this user"}
	ErrActiveOrderExists        = &ServiceError{http.StatusConflict, "ACTIVE_ORDER_EXISTS", "user already has an active delivery order"}
	ErrForbidden                = &ServiceError{http.StatusForbidden, "FORBIDDEN", "you are not allowed to access this resource"}
	ErrInvalidStateTransition   = &ServiceError{http.StatusBadRequest, "INVALID_STATE_TRANSITION", "order is not in a state that allows this action"}
	ErrCannotCancelPickedUp     = &ServiceError{http.StatusBadRequest, "CANNOT_CANCEL_PICKED_UP", "cannot cancel an order after package has been picked up"}
	ErrOrderAlreadyTaken        = &ServiceError{http.StatusConflict, "ORDER_ALREADY_TAKEN", "this order has already been accepted by another driver"}
	ErrPickupPhotoRequired      = &ServiceError{http.StatusBadRequest, "PICKUP_PHOTO_REQUIRED", "pickup_photo_url is required for picked_up action"}
	ErrDeliveryPhotoRequired    = &ServiceError{http.StatusBadRequest, "DELIVERY_PHOTO_REQUIRED", "delivery_photo_url is required for complete_delivery action"}
	ErrDeliveryFieldsRequired   = &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "actual_distance_km and actual_duration_min are required for complete_delivery"}
	ErrPromoInvalid             = &ServiceError{http.StatusUnprocessableEntity, "PROMO_INVALID", "promo code is invalid or expired"}
	ErrUpstreamUnavailable      = &ServiceError{http.StatusServiceUnavailable, "UPSTREAM_UNAVAILABLE", "upstream service unavailable"}
	ErrOrderNotDelivered        = &ServiceError{http.StatusUnprocessableEntity, "ORDER_NOT_DELIVERED", "can only rate a delivered order"}
	ErrRatingAlreadySubmitted   = &ServiceError{http.StatusConflict, "RATING_ALREADY_SUBMITTED", "rating already submitted for this order"}
	ErrFareNotFinalized         = &ServiceError{http.StatusNotFound, "FARE_NOT_FINALIZED", "fare not yet finalized for this order"}
	ErrValidation               = &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "request body validation failed"}
)

// ── State machine config ─────────────────────────────────────────────────────

// cancellableStates defines which states allow user cancellation.
// Once driver picks up the package (picked_up), user can no longer cancel.
var cancellableStates = map[string]bool{
	"pending":        true,
	"finding_driver": true,
	"assigned":       true,
	"on_pickup":      true,
}

// driverTransitions maps driver action → allowed state transition.
var driverTransitions = map[string]struct {
	from string
	to   string
}{
	"arrived_at_pickup": {from: "assigned", to: "on_pickup"},
	"picked_up":         {from: "on_pickup", to: "picked_up"},
	"start_delivery":    {from: "picked_up", to: "on_delivery"},
	"complete_delivery": {from: "on_delivery", to: "delivered"},
}

// ── Request/Response DTOs ───────────────────────────────────────────────────

type PackageInput struct {
	Category       string  `json:"category"`
	WeightKg       float64 `json:"weight_kg,omitempty"`
	Size           string  `json:"size"`
	IsFragile      bool    `json:"is_fragile,omitempty"`
	Description    string  `json:"description,omitempty"`
	InsuranceValue float64 `json:"insurance_value,omitempty"`
}

type CreateDeliveryOrderRequest struct {
	SenderName     string       `json:"sender_name"`
	SenderPhone    string       `json:"sender_phone"`
	PickupLat      float64      `json:"pickup_lat"`
	PickupLng      float64      `json:"pickup_lng"`
	PickupAddress  string       `json:"pickup_address"`
	PickupNotes    string       `json:"pickup_notes,omitempty"`
	RecipientName  string       `json:"recipient_name"`
	RecipientPhone string       `json:"recipient_phone"`
	DestLat        float64      `json:"dest_lat"`
	DestLng        float64      `json:"dest_lng"`
	DestAddress    string       `json:"dest_address"`
	DestNotes      string       `json:"dest_notes,omitempty"`
	PaymentMethod  string       `json:"payment_method"`
	PromoID        string       `json:"promo_id,omitempty"`
	Package        PackageInput `json:"package"`
	FareEstimate   float64      `json:"fare_estimate,omitempty"`
}

type InternalCreateOrderRequest struct {
	UserID string `json:"user_id"`
	CreateDeliveryOrderRequest
}

type FareEstimateRequest struct {
	PickupLat float64      `json:"pickup_lat"`
	PickupLng float64      `json:"pickup_lng"`
	DestLat   float64      `json:"dest_lat"`
	DestLng   float64      `json:"dest_lng"`
	Package   PackageInput `json:"package"`
	PromoID   string       `json:"promo_id,omitempty"`
}

type FareEstimateResponse struct {
	DistanceKm      float64              `json:"distance_km"`
	DurationMin     int                  `json:"duration_min"`
	FareEstimate    float64              `json:"fare_estimate"`
	SurgeMultiplier float64              `json:"surge_multiplier"`
	PromoDiscount   float64              `json:"promo_discount"`
	FareAfterPromo  float64              `json:"fare_after_promo"`
	Breakdown       FareBreakdownResponse `json:"breakdown"`
}

type FareBreakdownResponse struct {
	BaseFare        float64 `json:"base_fare"`
	DistanceFare    float64 `json:"distance_fare"`
	WeightSurcharge float64 `json:"weight_surcharge"`
	InsuranceFee    float64 `json:"insurance_fee"`
	PromoDiscount   float64 `json:"promo_discount"`
	PlatformFee     float64 `json:"platform_fee"`
	Total           float64 `json:"total"`
}

type DeliveryOrderResponse struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	DriverID       string    `json:"driver_id,omitempty"`
	Status         string    `json:"status"`
	SenderName     string    `json:"sender_name"`
	SenderPhone    string    `json:"sender_phone"`
	PickupLat      float64   `json:"pickup_lat"`
	PickupLng      float64   `json:"pickup_lng"`
	PickupAddress  string    `json:"pickup_address"`
	PickupNotes    string    `json:"pickup_notes,omitempty"`
	RecipientName  string    `json:"recipient_name"`
	RecipientPhone string    `json:"recipient_phone"`
	DestLat        float64   `json:"dest_lat"`
	DestLng        float64   `json:"dest_lng"`
	DestAddress    string    `json:"dest_address"`
	DestNotes      string    `json:"dest_notes,omitempty"`
	FareEstimate   float64   `json:"fare_estimate,omitempty"`
	FareFinal      float64   `json:"fare_final,omitempty"`
	PaymentMethod  string    `json:"payment_method"`
	CancelReason   string    `json:"cancel_reason,omitempty"`
	CancelledBy    string    `json:"cancelled_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type PackageResponse struct {
	ID             string  `json:"id"`
	Category       string  `json:"category"`
	WeightKg       float64 `json:"weight_kg,omitempty"`
	Size           string  `json:"size"`
	IsFragile      bool    `json:"is_fragile"`
	Description    string  `json:"description,omitempty"`
	InsuranceValue float64 `json:"insurance_value,omitempty"`
	PhotoURL       string  `json:"photo_url,omitempty"`
}

type DeliveryProofResponse struct {
	PickupPhotoURL   string     `json:"pickup_photo_url,omitempty"`
	DeliveryPhotoURL string     `json:"delivery_photo_url,omitempty"`
	PickedUpAt       *time.Time `json:"picked_up_at,omitempty"`
	DeliveredAt      *time.Time `json:"delivered_at,omitempty"`
}

type OrderStateLogResponse struct {
	FromState string    `json:"from_state,omitempty"`
	ToState   string    `json:"to_state"`
	ActorType string    `json:"actor_type"`
	Reason    string    `json:"reason,omitempty"`
	ChangedAt time.Time `json:"changed_at"`
}

type DeliveryOrderDetailResponse struct {
	DeliveryOrderResponse
	Package   *PackageResponse       `json:"package,omitempty"`
	Proof     *DeliveryProofResponse `json:"proof,omitempty"`
	StateLogs []OrderStateLogResponse `json:"state_logs,omitempty"`
}

type DeliveryOrderHistoryResponse struct {
	Orders []DeliveryOrderResponse `json:"orders"`
	Total  int                     `json:"total"`
	Page   int                     `json:"page"`
	Limit  int                     `json:"limit"`
}

type HistoryQuery struct {
	Status string
	From   string // YYYY-MM-DD
	To     string // YYYY-MM-DD
	Page   int
	Limit  int
}

type CancelOrderRequest struct {
	Reason string `json:"reason"`
}

type DriverUpdateStatusRequest struct {
	Action            string  `json:"action"`
	PickupPhotoURL    string  `json:"pickup_photo_url,omitempty"`
	ActualDistanceKm  float64 `json:"actual_distance_km,omitempty"`
	ActualDurationMin int     `json:"actual_duration_min,omitempty"`
	DeliveryPhotoURL  string  `json:"delivery_photo_url,omitempty"`
	DeliveryLat       float64 `json:"delivery_lat,omitempty"`
	DeliveryLng       float64 `json:"delivery_lng,omitempty"`
}

type DriverRejectRequest struct {
	Reason string `json:"reason,omitempty"`
}

type DriverAcceptResponse struct {
	OrderID        string          `json:"order_id"`
	Status         string          `json:"status"`
	SenderName     string          `json:"sender_name"`
	SenderPhone    string          `json:"sender_phone"`
	PickupLat      float64         `json:"pickup_lat"`
	PickupLng      float64         `json:"pickup_lng"`
	PickupAddress  string          `json:"pickup_address"`
	PickupNotes    string          `json:"pickup_notes,omitempty"`
	RecipientName  string          `json:"recipient_name"`
	RecipientPhone string          `json:"recipient_phone"`
	DestAddress    string          `json:"dest_address"`
	FareEstimate   float64         `json:"fare_estimate"`
	PaymentMethod  string          `json:"payment_method"`
	Package        *PackageResponse `json:"package,omitempty"`
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
	OrderID    string `json:"order_id"`
	DriverID   string `json:"driver_id"`
	Status     string `json:"status"`
	ETASeconds int    `json:"eta_seconds"`
}

// ── Interface ───────────────────────────────────────────────────────────────

// DeliveryOrderServiceInterface defines the service contract.
//
//go:generate mockgen -source=delivery_order_service.go -destination=../../mocks/mock_delivery_order_service.go -package=mocks
type DeliveryOrderServiceInterface interface {
	// User-facing
	EstimateFare(ctx context.Context, req *FareEstimateRequest) (*FareEstimateResponse, error)
	CreateOrder(ctx context.Context, userID string, req *CreateDeliveryOrderRequest) (*DeliveryOrderResponse, error)
	GetActiveOrder(ctx context.Context, userID string) (*DeliveryOrderResponse, error)
	GetOrderHistory(ctx context.Context, userID string, q HistoryQuery) (*DeliveryOrderHistoryResponse, error)
	GetOrder(ctx context.Context, userID, role, orderID string) (*DeliveryOrderDetailResponse, error)
	CancelOrder(ctx context.Context, userID, orderID string, req *CancelOrderRequest) (*DeliveryOrderResponse, error)
	SubmitRating(ctx context.Context, userID, orderID string, req *SubmitRatingRequest) error
	GetFareBreakdown(ctx context.Context, userID, orderID string) (*FareBreakdownResponse, error)

	// Driver-facing
	DriverAcceptOrder(ctx context.Context, driverID, orderID string) (*DriverAcceptResponse, error)
	DriverRejectOrder(ctx context.Context, driverID, orderID string, req *DriverRejectRequest) error
	DriverUpdateOrderStatus(ctx context.Context, driverID, orderID string, req *DriverUpdateStatusRequest) (*DeliveryOrderResponse, error)

	// Internal
	InternalCreateOrder(ctx context.Context, req *InternalCreateOrderRequest) (*DeliveryOrderResponse, error)
	InternalGetOrder(ctx context.Context, orderID string) (*DeliveryOrderResponse, error)
	InternalUpdateStatus(ctx context.Context, orderID string, req *InternalUpdateStatusRequest) (*DeliveryOrderResponse, error)
	InternalAssignDriver(ctx context.Context, orderID string, req *InternalAssignDriverRequest) (*InternalAssignDriverResponse, error)
}

// ── Implementation ──────────────────────────────────────────────────────────

// DeliveryOrderService implements DeliveryOrderServiceInterface.
type DeliveryOrderService struct {
	repo   repository.DeliveryOrderRepositoryInterface
	logger *slog.Logger
}

// NewDeliveryOrderService creates a new DeliveryOrderService.
func NewDeliveryOrderService(repo repository.DeliveryOrderRepositoryInterface, logger *slog.Logger) *DeliveryOrderService {
	return &DeliveryOrderService{repo: repo, logger: logger}
}

// ── User-facing methods ─────────────────────────────────────────────────────

func (s *DeliveryOrderService) EstimateFare(ctx context.Context, req *FareEstimateRequest) (*FareEstimateResponse, error) {
	if err := validateLatLng(req.PickupLat, req.PickupLng); err != nil {
		return nil, err
	}
	if err := validateLatLng(req.DestLat, req.DestLng); err != nil {
		return nil, err
	}
	if err := validatePackage(&req.Package); err != nil {
		return nil, err
	}

	distanceKm := haversineKm(req.PickupLat, req.PickupLng, req.DestLat, req.DestLng)
	durationMin := int(distanceKm * 3) // delivery is slower than ride: ~20km/h

	fb := calculateFinalFare(distanceKm, req.Package.WeightKg, req.Package.InsuranceValue)

	promoDiscount := 0.0
	if req.PromoID != "" {
		promoDiscount = round2(fb.Total * 0.10)
		if promoDiscount > 5000 {
			promoDiscount = 5000
		}
	}
	fareAfterPromo := round2(fb.Total - promoDiscount)
	if fareAfterPromo < 0 {
		fareAfterPromo = 0
	}

	return &FareEstimateResponse{
		DistanceKm:      round2(distanceKm),
		DurationMin:     durationMin,
		FareEstimate:    fb.Total,
		SurgeMultiplier: 1.0,
		PromoDiscount:   promoDiscount,
		FareAfterPromo:  fareAfterPromo,
		Breakdown: FareBreakdownResponse{
			BaseFare:        fb.BaseFare,
			DistanceFare:    fb.DistanceFare,
			WeightSurcharge: fb.WeightSurcharge,
			InsuranceFee:    fb.InsuranceFee,
			PromoDiscount:   promoDiscount,
			PlatformFee:     fb.PlatformFee,
			Total:           fareAfterPromo,
		},
	}, nil
}

func (s *DeliveryOrderService) CreateOrder(ctx context.Context, userID string, req *CreateDeliveryOrderRequest) (*DeliveryOrderResponse, error) {
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

	o := &repository.DeliveryOrder{
		UserID:         userID,
		Status:         "finding_driver",
		SenderName:     req.SenderName,
		SenderPhone:    req.SenderPhone,
		PickupLat:      req.PickupLat,
		PickupLng:      req.PickupLng,
		PickupAddress:  req.PickupAddress,
		PickupNotes:    nullableString(req.PickupNotes),
		RecipientName:  req.RecipientName,
		RecipientPhone: req.RecipientPhone,
		DestLat:        req.DestLat,
		DestLng:        req.DestLng,
		DestAddress:    req.DestAddress,
		DestNotes:      nullableString(req.DestNotes),
		FareEstimate:   nullableFloat(req.FareEstimate),
		PromoID:        nullableString(req.PromoID),
		PaymentMethod:  req.PaymentMethod,
	}

	pkg := &repository.DeliveryPackage{
		Category:       req.Package.Category,
		WeightKg:       nullableFloat(req.Package.WeightKg),
		Size:           req.Package.Size,
		IsFragile:      req.Package.IsFragile,
		Description:    nullableString(req.Package.Description),
		InsuranceValue: nullableFloat(req.Package.InsuranceValue),
	}

	created, err := s.repo.CreateOrder(ctx, o, pkg)
	if err != nil {
		return nil, err
	}

	_ = s.repo.InsertStateLog(ctx, &repository.DeliveryStateLog{
		OrderID:   created.ID,
		ToState:   "finding_driver",
		ActorID:   sql.NullString{String: userID, Valid: true},
		ActorType: "user",
	})

	s.logger.Info("delivery order created",
		slog.String("order_id", created.ID),
		slog.String("user_id", userID),
	)

	// TODO: publish Order_Created Kafka event
	// TODO: SET user:active_delivery:{user_id} in Redis (TTL 2h)

	return toOrderResponse(created), nil
}

func (s *DeliveryOrderService) GetActiveOrder(ctx context.Context, userID string) (*DeliveryOrderResponse, error) {
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
	return toOrderResponse(o), nil
}

func (s *DeliveryOrderService) GetOrderHistory(ctx context.Context, userID string, q HistoryQuery) (*DeliveryOrderHistoryResponse, error) {
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
		Status: q.Status,
		Limit:  q.Limit,
		Offset: (q.Page - 1) * q.Limit,
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
	out := &DeliveryOrderHistoryResponse{
		Orders: make([]DeliveryOrderResponse, 0, len(rows)),
		Total:  total,
		Page:   q.Page,
		Limit:  q.Limit,
	}
	for _, o := range rows {
		out.Orders = append(out.Orders, *toOrderResponse(o))
	}
	return out, nil
}

func (s *DeliveryOrderService) GetOrder(ctx context.Context, userID, role, orderID string) (*DeliveryOrderDetailResponse, error) {
	o, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	if role == "driver" {
		if !o.DriverID.Valid || o.DriverID.String != userID {
			return nil, ErrForbidden
		}
	} else {
		if o.UserID != userID {
			return nil, ErrForbidden
		}
	}

	resp := &DeliveryOrderDetailResponse{DeliveryOrderResponse: *toOrderResponse(o)}

	if pkg, err := s.repo.GetPackageByOrderID(ctx, orderID); err == nil {
		resp.Package = toPackageResponse(pkg)
	}

	resp.Proof = toProofResponse(o)

	if logs, err := s.repo.ListStateLogs(ctx, orderID); err == nil {
		for _, l := range logs {
			resp.StateLogs = append(resp.StateLogs, *toStateLogResponse(l))
		}
	}
	return resp, nil
}

func (s *DeliveryOrderService) CancelOrder(ctx context.Context, userID, orderID string, req *CancelOrderRequest) (*DeliveryOrderResponse, error) {
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
	// picked_up and beyond cannot be cancelled
	if o.Status == "picked_up" || o.Status == "on_delivery" {
		return nil, ErrCannotCancelPickedUp
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

	_ = s.repo.InsertStateLog(ctx, &repository.DeliveryStateLog{
		OrderID:   orderID,
		FromState: sql.NullString{String: o.Status, Valid: true},
		ToState:   "cancelled",
		ActorID:   sql.NullString{String: userID, Valid: true},
		ActorType: "user",
		Reason:    sql.NullString{String: reason, Valid: reason != ""},
	})

	s.logger.Info("delivery order cancelled", slog.String("order_id", orderID), slog.String("by", "user"))

	// TODO: publish Order_Cancelled Kafka event

	updated, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return toOrderResponse(updated), nil
}

func (s *DeliveryOrderService) SubmitRating(ctx context.Context, userID, orderID string, req *SubmitRatingRequest) error {
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
	if o.Status != "delivered" {
		return ErrOrderNotDelivered
	}

	// TODO: forward to Rating Service via gRPC/HTTP
	s.logger.Info("delivery rating submitted",
		slog.String("order_id", orderID),
		slog.String("user_id", userID),
		slog.Int("score", req.Score),
	)
	return nil
}

func (s *DeliveryOrderService) GetFareBreakdown(ctx context.Context, userID, orderID string) (*FareBreakdownResponse, error) {
	o, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
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
		WeightSurcharge: fb.WeightSurcharge,
		InsuranceFee:    fb.InsuranceFee,
		PromoDiscount:   fb.PromoDiscount,
		PlatformFee:     fb.PlatformFee,
		Total:           fb.Total,
	}, nil
}

// ── Driver-facing methods ───────────────────────────────────────────────────

func (s *DeliveryOrderService) DriverAcceptOrder(ctx context.Context, driverID, orderID string) (*DriverAcceptResponse, error) {
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

	// Atomically assign driver — no OTP for delivery orders (photo proof instead)
	if err := s.repo.AssignDriver(ctx, orderID, driverID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderAlreadyTaken
		}
		return nil, err
	}

	_ = s.repo.InsertStateLog(ctx, &repository.DeliveryStateLog{
		OrderID:   orderID,
		FromState: sql.NullString{String: "finding_driver", Valid: true},
		ToState:   "assigned",
		ActorID:   sql.NullString{String: driverID, Valid: true},
		ActorType: "driver",
	})

	s.logger.Info("driver accepted delivery order",
		slog.String("order_id", orderID),
		slog.String("driver_id", driverID),
	)

	pkg, _ := s.repo.GetPackageByOrderID(ctx, orderID)

	return &DriverAcceptResponse{
		OrderID:        orderID,
		Status:         "assigned",
		SenderName:     o.SenderName,
		SenderPhone:    o.SenderPhone,
		PickupLat:      o.PickupLat,
		PickupLng:      o.PickupLng,
		PickupAddress:  o.PickupAddress,
		PickupNotes:    o.PickupNotes.String,
		RecipientName:  o.RecipientName,
		RecipientPhone: o.RecipientPhone,
		DestAddress:    o.DestAddress,
		FareEstimate:   o.FareEstimate.Float64,
		PaymentMethod:  o.PaymentMethod,
		Package:        toPackageResponse(pkg),
	}, nil
}

func (s *DeliveryOrderService) DriverRejectOrder(ctx context.Context, driverID, orderID string, req *DriverRejectRequest) error {
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

	_ = s.repo.InsertStateLog(ctx, &repository.DeliveryStateLog{
		OrderID:   o.ID,
		FromState: sql.NullString{String: o.Status, Valid: true},
		ToState:   o.Status,
		ActorID:   sql.NullString{String: driverID, Valid: true},
		ActorType: "driver",
		Reason:    sql.NullString{String: "rejected:" + reason, Valid: true},
	})

	s.logger.Info("driver rejected delivery order",
		slog.String("order_id", orderID),
		slog.String("driver_id", driverID),
		slog.String("reason", reason),
	)

	// TODO: notify Matching Service to re-broadcast
	return nil
}

func (s *DeliveryOrderService) DriverUpdateOrderStatus(ctx context.Context, driverID, orderID string, req *DriverUpdateStatusRequest) (*DeliveryOrderResponse, error) {
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

	// Action-specific validation
	switch req.Action {
	case "picked_up":
		if req.PickupPhotoURL == "" {
			return nil, ErrPickupPhotoRequired
		}
	case "complete_delivery":
		if req.ActualDistanceKm <= 0 || req.ActualDurationMin <= 0 {
			return nil, ErrDeliveryFieldsRequired
		}
		if req.DeliveryPhotoURL == "" {
			return nil, ErrDeliveryPhotoRequired
		}
	}

	if err := s.repo.UpdateStatus(ctx, orderID, tr.from, tr.to); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidStateTransition
		}
		return nil, err
	}

	_ = s.repo.InsertStateLog(ctx, &repository.DeliveryStateLog{
		OrderID:   orderID,
		FromState: sql.NullString{String: tr.from, Valid: true},
		ToState:   tr.to,
		ActorID:   sql.NullString{String: driverID, Valid: true},
		ActorType: "driver",
	})

	// Side effects per action
	switch req.Action {
	case "picked_up":
		_ = s.repo.SetPickupProof(ctx, orderID, req.PickupPhotoURL)
	case "complete_delivery":
		_ = s.repo.SetDeliveryDetails(ctx, orderID, req.DeliveryPhotoURL, req.ActualDistanceKm, req.ActualDurationMin)
		pkg, _ := s.repo.GetPackageByOrderID(ctx, orderID)
		weightKg := 0.0
		insuranceValue := 0.0
		if pkg != nil {
			weightKg = pkg.WeightKg.Float64
			insuranceValue = pkg.InsuranceValue.Float64
		}
		fb := calculateFinalFare(req.ActualDistanceKm, weightKg, insuranceValue)
		fb.OrderID = orderID
		_ = s.repo.UpsertFareBreakdown(ctx, fb)
		_ = s.repo.SetFareFinal(ctx, orderID, fb.Total)
		// TODO: publish Trip_Completed Kafka event
	}

	s.logger.Info("driver updated delivery status",
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
	return toOrderResponse(updated), nil
}

// ── Internal methods ────────────────────────────────────────────────────────

func (s *DeliveryOrderService) InternalCreateOrder(ctx context.Context, req *InternalCreateOrderRequest) (*DeliveryOrderResponse, error) {
	if req == nil || req.UserID == "" {
		return nil, ErrValidation
	}
	return s.CreateOrder(ctx, req.UserID, &req.CreateDeliveryOrderRequest)
}

func (s *DeliveryOrderService) InternalGetOrder(ctx context.Context, orderID string) (*DeliveryOrderResponse, error) {
	o, err := s.repo.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	return toOrderResponse(o), nil
}

func (s *DeliveryOrderService) InternalUpdateStatus(ctx context.Context, orderID string, req *InternalUpdateStatusRequest) (*DeliveryOrderResponse, error) {
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
	if o.Status == "delivered" || o.Status == "cancelled" {
		return nil, &ServiceError{http.StatusConflict, "TERMINAL_STATE", "order is already in a terminal state"}
	}
	if err := s.repo.UpdateStatus(ctx, orderID, o.Status, req.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidStateTransition
		}
		return nil, err
	}

	_ = s.repo.InsertStateLog(ctx, &repository.DeliveryStateLog{
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
	return toOrderResponse(updated), nil
}

func (s *DeliveryOrderService) InternalAssignDriver(ctx context.Context, orderID string, req *InternalAssignDriverRequest) (*InternalAssignDriverResponse, error) {
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

	if err := s.repo.AssignDriver(ctx, orderID, req.DriverID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderAlreadyTaken
		}
		return nil, err
	}

	_ = s.repo.InsertStateLog(ctx, &repository.DeliveryStateLog{
		OrderID:   orderID,
		FromState: sql.NullString{String: "finding_driver", Valid: true},
		ToState:   "assigned",
		ActorType: "system",
	})

	return &InternalAssignDriverResponse{
		OrderID:    orderID,
		DriverID:   req.DriverID,
		Status:     "assigned",
		ETASeconds: req.ETASeconds,
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

func validatePackage(pkg *PackageInput) error {
	if pkg == nil {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "package is required"}
	}
	validCategories := map[string]bool{"document": true, "food": true, "electronics": true, "clothing": true, "fragile": true, "other": true}
	if !validCategories[pkg.Category] {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "invalid package category"}
	}
	validSizes := map[string]bool{"small": true, "medium": true, "large": true}
	if !validSizes[pkg.Size] {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "size must be small, medium, or large"}
	}
	return nil
}

func validateCreateOrder(req *CreateDeliveryOrderRequest) error {
	if req == nil {
		return ErrValidation
	}
	if req.SenderName == "" {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "sender_name is required"}
	}
	if req.SenderPhone == "" {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "sender_phone is required"}
	}
	if req.RecipientName == "" {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "recipient_name is required"}
	}
	if req.RecipientPhone == "" {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "recipient_phone is required"}
	}
	if req.PickupAddress == "" {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "pickup_address is required"}
	}
	if req.DestAddress == "" {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "dest_address is required"}
	}
	if req.PaymentMethod != "gopay" && req.PaymentMethod != "cash" {
		return &ServiceError{http.StatusBadRequest, "VALIDATION_ERROR", "payment_method must be gopay or cash"}
	}
	if err := validateLatLng(req.PickupLat, req.PickupLng); err != nil {
		return err
	}
	if err := validateLatLng(req.DestLat, req.DestLng); err != nil {
		return err
	}
	return validatePackage(&req.Package)
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

func calculateFinalFare(distanceKm, weightKg, insuranceValue float64) *repository.DeliveryFareBreakdown {
	baseFare := 5000.0
	perKm := 3000.0
	distanceFare := round2(distanceKm * perKm)

	// Weight surcharge: 500/kg above 1kg
	weightSurcharge := 0.0
	if weightKg > 1.0 {
		weightSurcharge = round2((weightKg - 1.0) * 500)
	}

	// Insurance fee: 0.5% of declared value, minimum 0
	insuranceFee := 0.0
	if insuranceValue > 0 {
		insuranceFee = round2(insuranceValue * 0.005)
	}

	platformFee := 1000.0
	total := round2(baseFare + distanceFare + weightSurcharge + insuranceFee + platformFee)

	return &repository.DeliveryFareBreakdown{
		BaseFare:        baseFare,
		DistanceFare:    distanceFare,
		WeightSurcharge: weightSurcharge,
		InsuranceFee:    insuranceFee,
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

func toOrderResponse(o *repository.DeliveryOrder) *DeliveryOrderResponse {
	r := &DeliveryOrderResponse{
		ID:             o.ID,
		UserID:         o.UserID,
		Status:         o.Status,
		SenderName:     o.SenderName,
		SenderPhone:    o.SenderPhone,
		PickupLat:      o.PickupLat,
		PickupLng:      o.PickupLng,
		PickupAddress:  o.PickupAddress,
		RecipientName:  o.RecipientName,
		RecipientPhone: o.RecipientPhone,
		DestLat:        o.DestLat,
		DestLng:        o.DestLng,
		DestAddress:    o.DestAddress,
		PaymentMethod:  o.PaymentMethod,
		CreatedAt:      o.CreatedAt,
		UpdatedAt:      o.UpdatedAt,
	}
	if o.DriverID.Valid {
		r.DriverID = o.DriverID.String
	}
	if o.PickupNotes.Valid {
		r.PickupNotes = o.PickupNotes.String
	}
	if o.DestNotes.Valid {
		r.DestNotes = o.DestNotes.String
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
	return r
}

func toPackageResponse(pkg *repository.DeliveryPackage) *PackageResponse {
	if pkg == nil {
		return nil
	}
	r := &PackageResponse{
		ID:        pkg.ID,
		Category:  pkg.Category,
		Size:      pkg.Size,
		IsFragile: pkg.IsFragile,
	}
	if pkg.WeightKg.Valid {
		r.WeightKg = pkg.WeightKg.Float64
	}
	if pkg.Description.Valid {
		r.Description = pkg.Description.String
	}
	if pkg.InsuranceValue.Valid {
		r.InsuranceValue = pkg.InsuranceValue.Float64
	}
	if pkg.PhotoURL.Valid {
		r.PhotoURL = pkg.PhotoURL.String
	}
	return r
}

func toProofResponse(o *repository.DeliveryOrder) *DeliveryProofResponse {
	proof := &DeliveryProofResponse{}
	hasData := false
	if o.PickupPhotoURL.Valid {
		proof.PickupPhotoURL = o.PickupPhotoURL.String
		hasData = true
	}
	if o.DeliveryPhotoURL.Valid {
		proof.DeliveryPhotoURL = o.DeliveryPhotoURL.String
		hasData = true
	}
	if o.PickedUpAt.Valid {
		t := o.PickedUpAt.Time
		proof.PickedUpAt = &t
		hasData = true
	}
	if o.DeliveredAt.Valid {
		t := o.DeliveredAt.Time
		proof.DeliveredAt = &t
		hasData = true
	}
	if !hasData {
		return nil
	}
	return proof
}

func toStateLogResponse(l *repository.DeliveryStateLog) *OrderStateLogResponse {
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
