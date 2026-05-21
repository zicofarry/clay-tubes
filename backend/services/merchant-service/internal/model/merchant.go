package model

import (
	"time"
)

// MerchantStatus represents merchant account status.
type MerchantStatus string

const (
	StatusPendingReview MerchantStatus = "pending_review"
	StatusActive        MerchantStatus = "active"
	StatusClosed        MerchantStatus = "closed"
	StatusSuspended     MerchantStatus = "suspended"
)

// MerchantCategory classifies the type of merchant.
type MerchantCategory string

const (
	CategoryFood     MerchantCategory = "food"
	CategoryBeverage MerchantCategory = "beverage"
	CategoryGrocery  MerchantCategory = "grocery"
)

// Merchant is the main merchant entity stored in PostgreSQL.
type Merchant struct {
	ID             string           `json:"id"`
	UserID         string           `json:"user_id"`
	Name           string           `json:"name"`
	Description    *string          `json:"description,omitempty"`
	Category       MerchantCategory `json:"category"`
	Status         MerchantStatus   `json:"status"`
	PhoneNumber    string           `json:"phone_number"`
	Email          *string          `json:"email,omitempty"`
	Address        string           `json:"address"`
	City           string           `json:"city"`
	Lat            float64          `json:"lat"`
	Lng            float64          `json:"lng"`
	LogoURL        *string          `json:"logo_url,omitempty"`
	BannerURL      *string          `json:"banner_url,omitempty"`
	MinOrderCents  int64            `json:"min_order_cents"`
	EstDeliveryMin int              `json:"est_delivery_min"`
	Rating         float64          `json:"rating"`
	TotalReviews   int              `json:"total_reviews"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

// OperatingHours defines when a merchant is open for a specific day.
type OperatingHours struct {
	ID         string    `json:"id"`
	MerchantID string    `json:"merchant_id"`
	DayOfWeek  int       `json:"day_of_week"` // 0=Sunday, 6=Saturday
	OpenTime   string    `json:"open_time"`   // "HH:MM"
	CloseTime  string    `json:"close_time"`  // "HH:MM"
	IsClosed   bool      `json:"is_closed"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// BankAccount stores merchant settlement bank accounts in PostgreSQL.
type BankAccount struct {
	ID            string    `json:"id"`
	MerchantID    string    `json:"merchant_id"`
	BankCode      string    `json:"bank_code"`
	AccountNumber string    `json:"account_number"`
	AccountName   string    `json:"account_name"`
	IsPrimary     bool      `json:"is_primary"`
	CreatedAt     time.Time `json:"created_at"`
}

// MenuCategory is stored in MongoDB (flexible structure).
type MenuCategory struct {
	ID           string    `bson:"_id,omitempty" json:"id"`
	MerchantID   string    `bson:"merchant_id" json:"merchant_id"`
	Name         string    `bson:"name" json:"name"`
	Description  *string   `bson:"description,omitempty" json:"description,omitempty"`
	DisplayOrder int       `bson:"display_order" json:"display_order"`
	IsActive     bool      `bson:"is_active" json:"is_active"`
	CreatedAt    time.Time `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time `bson:"updated_at" json:"updated_at"`
}

// MenuItemVariant defines selectable options for a menu item (e.g., Size).
type MenuItemVariant struct {
	ID      string              `bson:"id" json:"id"`
	Name    string              `bson:"name" json:"name"` // e.g., "Size"
	Options []VariantOption     `bson:"options" json:"options"`
	Required bool               `bson:"required" json:"required"`
}

// VariantOption is one choice within a variant group.
type VariantOption struct {
	ID         string `bson:"id" json:"id"`
	Name       string `bson:"name" json:"name"` // e.g., "Large"
	ExtraPrice int64  `bson:"extra_price_cents" json:"extra_price_cents"`
}

// MenuItemAddOn represents a modifiable add-on for a menu item.
type MenuItemAddOn struct {
	ID         string `bson:"id" json:"id"`
	Name       string `bson:"name" json:"name"` // e.g., "Extra Cheese"
	Price      int64  `bson:"price_cents" json:"price_cents"`
	MaxQty     int    `bson:"max_qty" json:"max_qty"`
}

// MenuItem is stored in MongoDB (flexible variants/add-ons).
type MenuItem struct {
	ID          string            `bson:"_id,omitempty" json:"id"`
	MerchantID  string            `bson:"merchant_id" json:"merchant_id"`
	CategoryID  string            `bson:"category_id" json:"category_id"`
	Name        string            `bson:"name" json:"name"`
	Description *string           `bson:"description,omitempty" json:"description,omitempty"`
	PriceCents  int64             `bson:"price_cents" json:"price_cents"`
	ImageURL    *string           `bson:"image_url,omitempty" json:"image_url,omitempty"`
	IsAvailable bool              `bson:"is_available" json:"is_available"`
	Variants    []MenuItemVariant  `bson:"variants,omitempty" json:"variants,omitempty"`
	AddOns      []MenuItemAddOn    `bson:"add_ons,omitempty" json:"add_ons,omitempty"`
	Tags        []string           `bson:"tags,omitempty" json:"tags,omitempty"`
	CreatedAt   time.Time         `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time         `bson:"updated_at" json:"updated_at"`
}

// ── Request/Response DTOs ─────────────────────────────────────────────────────

type RegisterMerchantRequest struct {
	Name           string           `json:"name"`
	Description    *string          `json:"description,omitempty"`
	Category       MerchantCategory `json:"category"`
	PhoneNumber    string           `json:"phone_number"`
	Email          *string          `json:"email,omitempty"`
	Address        string           `json:"address"`
	City           string           `json:"city"`
	Lat            float64          `json:"lat"`
	Lng            float64          `json:"lng"`
	MinOrderCents  int64            `json:"min_order_cents"`
	EstDeliveryMin int              `json:"est_delivery_min"`
}

type UpdateMerchantRequest struct {
	Name           *string  `json:"name,omitempty"`
	Description    *string  `json:"description,omitempty"`
	PhoneNumber    *string  `json:"phone_number,omitempty"`
	Address        *string  `json:"address,omitempty"`
	City           *string  `json:"city,omitempty"`
	Lat            *float64 `json:"lat,omitempty"`
	Lng            *float64 `json:"lng,omitempty"`
	LogoURL        *string  `json:"logo_url,omitempty"`
	BannerURL      *string  `json:"banner_url,omitempty"`
	MinOrderCents  *int64   `json:"min_order_cents,omitempty"`
	EstDeliveryMin *int     `json:"est_delivery_min,omitempty"`
}

type UpdateMerchantStatusRequest struct {
	Status MerchantStatus `json:"status"`
}

type UpsertOperatingHoursItem struct {
	DayOfWeek int    `json:"day_of_week"`
	OpenTime  string `json:"open_time"`
	CloseTime string `json:"close_time"`
	IsClosed  bool   `json:"is_closed"`
}

type UpsertOperatingHoursRequest struct {
	Hours []UpsertOperatingHoursItem `json:"hours"`
}

type AddBankAccountRequest struct {
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
	AccountName   string `json:"account_name"`
	SetPrimary    bool   `json:"set_primary"`
}

type CreateMenuCategoryRequest struct {
	Name         string  `json:"name"`
	Description  *string `json:"description,omitempty"`
	DisplayOrder int     `json:"display_order"`
}

type UpdateMenuCategoryRequest struct {
	Name         *string `json:"name,omitempty"`
	Description  *string `json:"description,omitempty"`
	DisplayOrder *int    `json:"display_order,omitempty"`
}

type ReorderCategoriesRequest struct {
	Orders []struct {
		CategoryID   string `json:"category_id"`
		DisplayOrder int    `json:"display_order"`
	} `json:"orders"`
}

type CreateMenuItemRequest struct {
	CategoryID  string            `json:"category_id"`
	Name        string            `json:"name"`
	Description *string           `json:"description,omitempty"`
	PriceCents  int64             `json:"price_cents"`
	ImageURL    *string           `json:"image_url,omitempty"`
	Variants    []MenuItemVariant  `json:"variants,omitempty"`
	AddOns      []MenuItemAddOn    `json:"add_ons,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
}

type UpdateMenuItemRequest struct {
	CategoryID  *string           `json:"category_id,omitempty"`
	Name        *string           `json:"name,omitempty"`
	Description *string           `json:"description,omitempty"`
	PriceCents  *int64            `json:"price_cents,omitempty"`
	ImageURL    *string           `json:"image_url,omitempty"`
	Variants    []MenuItemVariant  `json:"variants,omitempty"`
	AddOns      []MenuItemAddOn    `json:"add_ons,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
}

type ToggleAvailabilityRequest struct {
	IsAvailable bool `json:"is_available"`
}

type BatchGetMenuItemsRequest struct {
	ItemIDs []string `json:"item_ids"`
}

type IsOpenResponse struct {
	IsOpen   bool   `json:"is_open"`
	Reason   string `json:"reason,omitempty"`
}
