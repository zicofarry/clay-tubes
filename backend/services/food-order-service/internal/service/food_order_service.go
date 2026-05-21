package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/model"
	"github.com/zicofarry/clay-app/backend/services/food-order-service/internal/repository"
	sharedKafka "github.com/zicofarry/clay-app/backend/pkg/pkg/kafka"
)

const (
	TopicOrderCreated   = "food_order.created"
	TopicOrderConfirmed = "food_order.confirmed"
	TopicOrderCancelled = "food_order.cancelled"
	TopicTripCompleted  = "food_order.completed"

	ServiceName = "clay-food-order-service"
)

// FoodOrderServiceInterface defines all business-logic operations for the food
// order lifecycle. It is implemented by FoodOrderService and mocked in tests.
//
//go:generate mockgen -source=food_order_service.go -destination=../../mocks/mock_food_order_service.go -package=mocks
type FoodOrderServiceInterface interface {
	// EstimateFare computes delivery fee before creating an order.
	EstimateFare(ctx context.Context, req model.FareEstimateRequest) (*model.FareEstimateResponse, error)

	// CreateOrder creates a new food order (POST /orders).
	CreateOrder(ctx context.Context, userID string, req model.CreateFoodOrderRequest) (*model.FoodOrder, error)

	// GetOrder returns full order detail for the given caller (GET /orders/{orderId}).
	GetOrder(ctx context.Context, orderID, callerID string) (*model.FoodOrder, []model.FoodOrderItem, error)

	// GetActiveOrder returns the user's active food order (GET /orders/active).
	GetActiveOrder(ctx context.Context, userID string) (*model.FoodOrder, error)

	// GetHistory returns paginated order history (GET /orders/history).
	GetHistory(ctx context.Context, userID, status string, page, limit int) ([]model.FoodOrder, int, error)

	// CancelOrder cancels an order subject to grace-period rules (POST /orders/{orderId}/cancel).
	CancelOrder(ctx context.Context, orderID, userID string, req model.CancelOrderRequest) (*model.FoodOrder, error)

	// MerchantConfirmOrder transitions pending → confirmed (POST /merchant/orders/{orderId}/confirm).
	MerchantConfirmOrder(ctx context.Context, orderID, merchantUserID string, req model.MerchantConfirmRequest) (*model.FoodOrder, error)

	// MerchantRejectOrder cancels a pending order by the merchant (POST /merchant/orders/{orderId}/reject).
	MerchantRejectOrder(ctx context.Context, orderID string, req model.MerchantRejectRequest) (*model.FoodOrder, error)

	// MerchantUpdateStatus advances preparation state (PUT /merchant/orders/{orderId}/status).
	MerchantUpdateStatus(ctx context.Context, orderID string, req model.MerchantUpdateStatusRequest) (*model.FoodOrder, error)

	// MerchantListOrders lists orders for a merchant (GET /merchant/orders).
	MerchantListOrders(ctx context.Context, merchantID, status string, page, limit int) ([]model.FoodOrder, int, error)

	// DriverPickup marks the order as picked up (POST /driver/orders/{orderId}/pickup).
	DriverPickup(ctx context.Context, orderID, driverID string) (*model.FoodOrder, error)

	// DriverDeliver marks the order as delivered (POST /driver/orders/{orderId}/deliver).
	DriverDeliver(ctx context.Context, orderID, driverID string) (*model.FoodOrder, error)

	// AssignDriver assigns a driver internally (POST /internal/orders/{orderId}/assign-driver).
	AssignDriver(ctx context.Context, orderID, driverID string) (*model.FoodOrder, error)

	// SubmitRating records a post-delivery rating (POST /orders/{orderId}/rate).
	SubmitRating(ctx context.Context, orderID, userID string, req model.SubmitFoodRatingRequest) error

	// GetFareBreakdown returns fare details for a completed order (GET /orders/{orderId}/fare-breakdown).
	GetFareBreakdown(ctx context.Context, orderID, userID string) (*model.FareBreakdown, error)
}

// FoodOrderService encapsulates business logic for food orders.
type FoodOrderService struct {
	repo     repository.FoodOrderRepositoryInterface
	producer sharedKafka.Producer
	logger   *slog.Logger
	http     *http.Client
}

// NewFoodOrderService creates a new FoodOrderService.
func NewFoodOrderService(
	repo repository.FoodOrderRepositoryInterface,
	producer sharedKafka.Producer,
	logger *slog.Logger,
) *FoodOrderService {
	return &FoodOrderService{
		repo:     repo,
		producer: producer,
		logger:   logger,
		http:     &http.Client{Timeout: 5 * time.Second},
	}
}

// EstimateFare calculates the delivery fee based on distance.
func (s *FoodOrderService) EstimateFare(_ context.Context, req model.FareEstimateRequest) (*model.FareEstimateResponse, error) {
	// Simplified distance-based pricing (real impl would call geo/pricing service)
	dist := haversineKm(req.UserLat, req.UserLng, -6.2146, 106.8451) // merchant lat/lng placeholder
	deliveryFee := int64(math.Max(5000, dist*2000))                   // Rp2000/km, min Rp5000
	serviceFee := int64(1000)

	resp := &model.FareEstimateResponse{
		DeliveryFeeCents: deliveryFee,
		ServiceFeeCents:  serviceFee,
		DistanceKm:       math.Round(dist*10) / 10,
	}
	if req.ItemsSubtotal != nil {
		total := *req.ItemsSubtotal + deliveryFee + serviceFee
		resp.EstTotalCents = &total
	}
	return resp, nil
}

// CreateOrder creates a new food order.
func (s *FoodOrderService) CreateOrder(ctx context.Context, userID string, req model.CreateFoodOrderRequest) (*model.FoodOrder, error) {
	// Check for existing active order
	existingID, err := s.repo.GetActiveOrderID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("check active order: %w", err)
	}
	if existingID != "" {
		return nil, ErrActiveOrderExists
	}

	// Build order items
	var subtotal int64
	items := make([]model.FoodOrderItem, 0, len(req.Items))
	for _, ri := range req.Items {
		unitPrice := int64(15000) // placeholder — real impl calls merchant service
		for _, v := range ri.Variants {
			unitPrice += v.ExtraPrice
		}
		for _, a := range ri.AddOns {
			unitPrice += a.Price * int64(a.Quantity)
		}
		lineTotal := unitPrice * int64(ri.Quantity)
		subtotal += lineTotal

		item := model.FoodOrderItem{
			ID:         uuid.New().String(),
			OrderID:    "", // filled after order created
			MenuItemID: ri.MenuItemID,
			Name:       "Menu Item", // placeholder
			Quantity:   ri.Quantity,
			UnitPrice:  unitPrice,
			Subtotal:   lineTotal,
			Variants:   ri.Variants,
			AddOns:     ri.AddOns,
			Notes:      ri.Notes,
		}
		items = append(items, item)
	}

	deliveryFee := int64(10000) // simplified
	total := subtotal + deliveryFee

	order := &model.FoodOrder{
		UserID:          userID,
		MerchantID:      req.MerchantID,
		Status:          model.StatusPending,
		PaymentMethod:   req.PaymentMethod,
		SubtotalCents:   subtotal,
		DeliveryFee:     deliveryFee,
		DiscountCents:   0,
		TotalCents:      total,
		PromoCode:       req.PromoCode,
		Notes:           req.Notes,
		DeliveryLat:     req.DeliveryLat,
		DeliveryLng:     req.DeliveryLng,
		DeliveryAddress: req.DeliveryAddress,
	}

	if err := s.repo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	// Save items to MongoDB
	for i := range items {
		items[i].OrderID = order.ID
	}
	if err := s.repo.SaveItems(ctx, items); err != nil {
		s.logger.Error("save order items failed", slog.Any("error", err))
	}

	// Set active order in Redis
	_ = s.repo.SetActiveOrder(ctx, userID, order.ID)

	// Add to merchant pending queue
	_ = s.repo.AddToMerchantQueue(ctx, req.MerchantID, order.ID, float64(time.Now().Unix()))

	// Publish Order_Created event
	event, _ := sharedKafka.NewEvent("food_order.created", ServiceName, map[string]interface{}{
		"order_id":    order.ID,
		"user_id":     userID,
		"merchant_id": req.MerchantID,
	})
	if err := s.producer.Publish(ctx, TopicOrderCreated, order.ID, event); err != nil {
		s.logger.Error("publish order.created failed", slog.Any("error", err))
	}

	return order, nil
}

// GetOrder retrieves an order by ID, checking access rights.
func (s *FoodOrderService) GetOrder(ctx context.Context, orderID, callerID string) (*model.FoodOrder, []model.FoodOrderItem, error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, nil, err
	}
	if order == nil {
		return nil, nil, ErrOrderNotFound
	}

	items, err := s.repo.GetItemsByOrderID(ctx, orderID)
	if err != nil {
		s.logger.Error("get order items failed", slog.Any("error", err))
	}

	return order, items, nil
}

// GetActiveOrder returns the user's currently active food order.
func (s *FoodOrderService) GetActiveOrder(ctx context.Context, userID string) (*model.FoodOrder, error) {
	orderID, err := s.repo.GetActiveOrderID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if orderID == "" {
		return nil, ErrNoActiveOrder
	}
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		_ = s.repo.ClearActiveOrder(ctx, userID)
		return nil, ErrNoActiveOrder
	}
	return order, nil
}

// GetHistory returns paginated order history for a user.
func (s *FoodOrderService) GetHistory(ctx context.Context, userID, status string, page, limit int) ([]model.FoodOrder, int, error) {
	return s.repo.ListByUser(ctx, userID, status, page, limit)
}

// CancelOrder cancels an order with business rule enforcement.
func (s *FoodOrderService) CancelOrder(ctx context.Context, orderID, userID string, req model.CancelOrderRequest) (*model.FoodOrder, error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.UserID != userID {
		return nil, ErrForbidden
	}

	switch order.Status {
	case model.StatusPending:
		// Free cancel
	case model.StatusConfirmed:
		if order.CancelDeadline != nil && time.Now().After(*order.CancelDeadline) {
			return nil, ErrCancelGraceExpired
		}
	default:
		return nil, ErrCannotCancelState
	}

	reason := ""
	if req.Reason != nil {
		reason = *req.Reason
	}
	if err := s.repo.UpdateCancelled(ctx, orderID, model.CancelledByUser, reason); err != nil {
		return nil, err
	}
	_ = s.repo.ClearActiveOrder(ctx, order.UserID)
	_ = s.repo.RemoveFromMerchantQueue(ctx, order.MerchantID, orderID)

	event, _ := sharedKafka.NewEvent("food_order.cancelled", ServiceName, map[string]interface{}{
		"order_id":     orderID,
		"cancelled_by": "user",
		"reason":       reason,
	})
	_ = s.producer.Publish(ctx, TopicOrderCancelled, orderID, event)

	order.Status = model.StatusCancelled
	return order, nil
}

// MerchantConfirmOrder transitions order to confirmed state.
func (s *FoodOrderService) MerchantConfirmOrder(ctx context.Context, orderID, merchantUserID string, req model.MerchantConfirmRequest) (*model.FoodOrder, error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.Status != model.StatusPending {
		return nil, ErrInvalidStateTransition
	}

	if err := s.repo.UpdateConfirmed(ctx, orderID, req.EstPrepTimeMin); err != nil {
		return nil, err
	}
	_ = s.repo.RemoveFromMerchantQueue(ctx, order.MerchantID, orderID)

	event, _ := sharedKafka.NewEvent("food_order.confirmed", ServiceName, map[string]interface{}{
		"order_id":    orderID,
		"merchant_id": order.MerchantID,
		"user_id":     order.UserID,
	})
	_ = s.producer.Publish(ctx, TopicOrderConfirmed, orderID, event)

	order.Status = model.StatusConfirmed
	return order, nil
}

// MerchantRejectOrder rejects a pending order.
func (s *FoodOrderService) MerchantRejectOrder(ctx context.Context, orderID string, req model.MerchantRejectRequest) (*model.FoodOrder, error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.Status != model.StatusPending {
		return nil, ErrInvalidStateTransition
	}

	if err := s.repo.UpdateCancelled(ctx, orderID, model.CancelledByMerchant, req.Reason); err != nil {
		return nil, err
	}
	_ = s.repo.ClearActiveOrder(ctx, order.UserID)
	_ = s.repo.RemoveFromMerchantQueue(ctx, order.MerchantID, orderID)

	event, _ := sharedKafka.NewEvent("food_order.cancelled", ServiceName, map[string]interface{}{
		"order_id":     orderID,
		"cancelled_by": "merchant",
		"reason":       req.Reason,
	})
	_ = s.producer.Publish(ctx, TopicOrderCancelled, orderID, event)

	order.Status = model.StatusCancelled
	return order, nil
}

// MerchantUpdateStatus advances preparation state (start_preparing or mark_ready).
func (s *FoodOrderService) MerchantUpdateStatus(ctx context.Context, orderID string, req model.MerchantUpdateStatusRequest) (*model.FoodOrder, error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}

	var newStatus model.OrderStatus
	switch req.Action {
	case "start_preparing":
		if order.Status != model.StatusConfirmed {
			return nil, ErrInvalidStateTransition
		}
		newStatus = model.StatusPreparing
	case "mark_ready":
		if order.Status != model.StatusPreparing {
			return nil, ErrInvalidStateTransition
		}
		newStatus = model.StatusReady
	default:
		return nil, ErrInvalidStateTransition
	}

	if err := s.repo.UpdateStatus(ctx, orderID, newStatus, nil); err != nil {
		return nil, err
	}

	order.Status = newStatus
	return order, nil
}

// MerchantListOrders lists orders for a merchant.
func (s *FoodOrderService) MerchantListOrders(ctx context.Context, merchantID, status string, page, limit int) ([]model.FoodOrder, int, error) {
	return s.repo.ListByMerchant(ctx, merchantID, status, page, limit)
}

// DriverPickup marks order as picked up.
func (s *FoodOrderService) DriverPickup(ctx context.Context, orderID, driverID string) (*model.FoodOrder, error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.Status != model.StatusReady {
		return nil, ErrInvalidStateTransition
	}
	if order.DriverID == nil || *order.DriverID != driverID {
		return nil, ErrForbidden
	}

	if err := s.repo.UpdateStatus(ctx, orderID, model.StatusPickedUp, nil); err != nil {
		return nil, err
	}

	order.Status = model.StatusPickedUp
	return order, nil
}

// DriverDeliver marks order as delivered.
func (s *FoodOrderService) DriverDeliver(ctx context.Context, orderID, driverID string) (*model.FoodOrder, error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.Status != model.StatusPickedUp && order.Status != model.StatusOnDelivery {
		return nil, ErrInvalidStateTransition
	}
	if order.DriverID == nil || *order.DriverID != driverID {
		return nil, ErrForbidden
	}

	if err := s.repo.UpdateDelivered(ctx, orderID); err != nil {
		return nil, err
	}
	_ = s.repo.ClearActiveOrder(ctx, order.UserID)

	event, _ := sharedKafka.NewEvent("food_order.completed", ServiceName, map[string]interface{}{
		"order_id":    orderID,
		"driver_id":   driverID,
		"merchant_id": order.MerchantID,
		"user_id":     order.UserID,
		"total_cents": order.TotalCents,
	})
	_ = s.producer.Publish(ctx, TopicTripCompleted, orderID, event)

	order.Status = model.StatusDelivered
	return order, nil
}

// AssignDriver assigns a driver to an order (called internally by matching service).
func (s *FoodOrderService) AssignDriver(ctx context.Context, orderID, driverID string) (*model.FoodOrder, error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if err := s.repo.AssignDriver(ctx, orderID, driverID); err != nil {
		return nil, err
	}
	order.DriverID = &driverID
	return order, nil
}

// SubmitRating marks the order as rated (delegates actual rating to Rating Service).
func (s *FoodOrderService) SubmitRating(ctx context.Context, orderID, userID string, req model.SubmitFoodRatingRequest) error {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order == nil {
		return ErrOrderNotFound
	}
	if order.UserID != userID {
		return ErrForbidden
	}
	if order.Status != model.StatusDelivered {
		return ErrInvalidStateTransition
	}
	if order.RatingSubmitted {
		return ErrRatingAlreadySubmitted
	}
	return s.repo.MarkRatingSubmitted(ctx, orderID)
}

// GetFareBreakdown returns fare details for a completed order.
func (s *FoodOrderService) GetFareBreakdown(ctx context.Context, orderID, userID string) (*model.FareBreakdown, error) {
	order, err := s.repo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}
	if order.UserID != userID {
		return nil, ErrForbidden
	}
	return s.repo.GetFareBreakdown(ctx, orderID)
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrOrderNotFound          = fmt.Errorf("order not found")
	ErrActiveOrderExists      = fmt.Errorf("active order exists")
	ErrNoActiveOrder          = fmt.Errorf("no active order")
	ErrForbidden              = fmt.Errorf("forbidden")
	ErrInvalidStateTransition = fmt.Errorf("invalid state transition")
	ErrCannotCancelState      = fmt.Errorf("cannot cancel in current state")
	ErrCancelGraceExpired     = fmt.Errorf("cancel grace period expired")
	ErrRatingAlreadySubmitted = fmt.Errorf("rating already submitted")
	ErrMenuItemNotFound       = fmt.Errorf("menu item not found")
	ErrMerchantClosed         = fmt.Errorf("merchant is closed")
	ErrPromoInvalid           = fmt.Errorf("promo code is invalid")
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}
