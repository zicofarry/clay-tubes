package model

import (
	"time"
)

// OrderStatus represents the food order state machine.
type OrderStatus string

const (
	StatusPending    OrderStatus = "pending"
	StatusConfirmed  OrderStatus = "confirmed"
	StatusPreparing  OrderStatus = "preparing"
	StatusReady      OrderStatus = "ready"
	StatusPickedUp   OrderStatus = "picked_up"
	StatusOnDelivery OrderStatus = "on_delivery"
	StatusDelivered  OrderStatus = "delivered"
	StatusCancelled  OrderStatus = "cancelled"
)

// CancelledBy identifies who cancelled the order.
type CancelledBy string

const (
	CancelledByUser     CancelledBy = "user"
	CancelledByMerchant CancelledBy = "merchant"
	CancelledByDriver   CancelledBy = "driver"
	CancelledBySystem   CancelledBy = "system"
)

// PaymentMethod identifies how the order is paid.
type PaymentMethod string

const (
	PaymentGoPay PaymentMethod = "gopay"
	PaymentCash  PaymentMethod = "cash"
)

// FoodOrder is the primary food order entity stored in PostgreSQL.
type FoodOrder struct {
	ID             string        `json:"id"`
	UserID         string        `json:"user_id"`
	MerchantID     string        `json:"merchant_id"`
	DriverID       *string       `json:"driver_id,omitempty"`
	Status         OrderStatus   `json:"status"`
	PaymentMethod  PaymentMethod `json:"payment_method"`
	PaymentHoldID  *string       `json:"payment_hold_id,omitempty"`
	SubtotalCents  int64         `json:"subtotal_cents"`
	DeliveryFee    int64         `json:"delivery_fee_cents"`
	DiscountCents  int64         `json:"discount_cents"`
	TotalCents     int64         `json:"total_cents"`
	PromoCode      *string       `json:"promo_code,omitempty"`
	Notes          *string       `json:"notes,omitempty"`
	EstPrepTimeMин *int          `json:"est_prep_time_min,omitempty"`
	CancelledBy    *CancelledBy  `json:"cancelled_by,omitempty"`
	CancelReason   *string       `json:"cancel_reason,omitempty"`
	RatingSubmitted bool         `json:"rating_submitted"`
	ConfirmedAt    *time.Time    `json:"confirmed_at,omitempty"`
	CancelDeadline *time.Time    `json:"cancel_deadline,omitempty"`
	DeliveredAt    *time.Time    `json:"delivered_at,omitempty"`

	// Delivery address snapshot (stored at order creation)
	DeliveryLat     float64 `json:"delivery_lat"`
	DeliveryLng     float64 `json:"delivery_lng"`
	DeliveryAddress string  `json:"delivery_address"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FoodOrderStateLog records each state transition.
type FoodOrderStateLog struct {
	ID        string      `json:"id"`
	OrderID   string      `json:"order_id"`
	FromState OrderStatus `json:"from_state"`
	ToState   OrderStatus `json:"to_state"`
	ActorID   string      `json:"actor_id"`
	ActorRole string      `json:"actor_role"`
	Notes     *string     `json:"notes,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

// FoodOrderItem is stored in MongoDB (flexible variants & add-ons).
type FoodOrderItem struct {
	ID          string        `bson:"_id,omitempty" json:"id"`
	OrderID     string        `bson:"order_id" json:"order_id"`
	MenuItemID  string        `bson:"menu_item_id" json:"menu_item_id"`
	Name        string        `bson:"name" json:"name"`
	Quantity    int           `bson:"quantity" json:"quantity"`
	UnitPrice   int64         `bson:"unit_price_cents" json:"unit_price_cents"`
	Subtotal    int64         `bson:"subtotal_cents" json:"subtotal_cents"`
	Variants    []ItemVariant `bson:"variants,omitempty" json:"variants,omitempty"`
	AddOns      []ItemAddOn   `bson:"add_ons,omitempty" json:"add_ons,omitempty"`
	Notes       *string       `bson:"notes,omitempty" json:"notes,omitempty"`
}

// ItemVariant represents a selected variant (e.g., size: Large).
type ItemVariant struct {
	VariantID   string `bson:"variant_id" json:"variant_id"`
	VariantName string `bson:"variant_name" json:"variant_name"`
	OptionID    string `bson:"option_id" json:"option_id"`
	OptionName  string `bson:"option_name" json:"option_name"`
	ExtraPrice  int64  `bson:"extra_price_cents" json:"extra_price_cents"`
}

// ItemAddOn represents a selected add-on (e.g., extra cheese).
type ItemAddOn struct {
	AddOnID   string `bson:"add_on_id" json:"add_on_id"`
	Name      string `bson:"name" json:"name"`
	Price     int64  `bson:"price_cents" json:"price_cents"`
	Quantity  int    `bson:"quantity" json:"quantity"`
}

// FareBreakdown stores the final fare details after order completion.
type FareBreakdown struct {
	ID              string    `json:"id"`
	OrderID         string    `json:"order_id"`
	SubtotalCents   int64     `json:"subtotal_cents"`
	DeliveryFee     int64     `json:"delivery_fee_cents"`
	ServiceFee      int64     `json:"service_fee_cents"`
	DiscountCents   int64     `json:"discount_cents"`
	TotalCents      int64     `json:"total_cents"`
	DistanceKm      float64   `json:"distance_km"`
	CreatedAt       time.Time `json:"created_at"`
}

// ── Request / Response DTOs ───────────────────────────────────────────────────

type FareEstimateRequest struct {
	MerchantID    string   `json:"merchant_id"`
	UserLat       float64  `json:"user_lat"`
	UserLng       float64  `json:"user_lng"`
	ItemsSubtotal *int64   `json:"items_subtotal_cents,omitempty"`
}

type FareEstimateResponse struct {
	DeliveryFeeCents int64   `json:"delivery_fee_cents"`
	ServiceFeeCents  int64   `json:"service_fee_cents"`
	DistanceKm       float64 `json:"distance_km"`
	EstTotalCents    *int64  `json:"est_total_cents,omitempty"`
}

type CreateFoodOrderItem struct {
	MenuItemID string           `json:"menu_item_id"`
	Quantity   int              `json:"quantity"`
	Variants   []ItemVariant    `json:"variants,omitempty"`
	AddOns     []ItemAddOn      `json:"add_ons,omitempty"`
	Notes      *string          `json:"notes,omitempty"`
}

type CreateFoodOrderRequest struct {
	MerchantID      string               `json:"merchant_id"`
	Items           []CreateFoodOrderItem `json:"items"`
	DeliveryLat     float64              `json:"delivery_lat"`
	DeliveryLng     float64              `json:"delivery_lng"`
	DeliveryAddress string               `json:"delivery_address"`
	PaymentMethod   PaymentMethod        `json:"payment_method"`
	PromoCode       *string              `json:"promo_code,omitempty"`
	Notes           *string              `json:"notes,omitempty"`
}

type MerchantConfirmRequest struct {
	EstPrepTimeMin int `json:"est_prep_time_min"`
}

type MerchantRejectRequest struct {
	Reason string  `json:"reason"`
	Notes  *string `json:"notes,omitempty"`
}

type MerchantUpdateStatusRequest struct {
	Action string `json:"action"` // start_preparing | mark_ready
}

type CancelOrderRequest struct {
	Reason *string `json:"reason,omitempty"`
}

type SubmitFoodRatingRequest struct {
	DriverRating   int     `json:"driver_rating"`
	MerchantRating int     `json:"merchant_rating"`
	Comment        *string `json:"comment,omitempty"`
}

type AssignDriverRequest struct {
	DriverID string `json:"driver_id"`
}

type FoodOrderListResponse struct {
	Orders []FoodOrder `json:"orders"`
	Meta   PaginationMeta `json:"meta"`
}

type PaginationMeta struct {
	Total int `json:"total"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
}