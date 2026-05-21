package models

import (
	"time"

	"github.com/google/uuid"
)

// UserProfile represents a user's core profile.
type UserProfile struct {
	ID           uuid.UUID  `json:"id"`
	UserID       uuid.UUID  `json:"user_id"`
	FullName     string     `json:"full_name"`
	AvatarURL    string     `json:"avatar_url"`
	BirthDate    *string    `json:"birth_date,omitempty"` // Simple string for YYYY-MM-DD
	Gender       string     `json:"gender"`               // male, female, other
	ReferralCode string     `json:"referral_code"`
	ReferredBy   *uuid.UUID `json:"referred_by,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// UserAddress represents a saved address.
type UserAddress struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Label     string    `json:"label"`
	Address   string    `json:"address"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Notes     string    `json:"notes"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UserSettings represents user preferences.
type UserSettings struct {
	UserID           uuid.UUID `json:"user_id"`
	Language         string    `json:"language"`
	NotifEnabled     bool      `json:"notif_enabled"`
	MarketingEnabled bool      `json:"marketing_enabled"`
	UpdatedAt        time.Time `json:"updated_at"`
}
