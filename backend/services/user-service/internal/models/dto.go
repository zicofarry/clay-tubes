package models

import (
	"time"

	"github.com/google/uuid"
)

// Profile DTOs
type CreateProfileRequest struct {
	FullName  string `json:"full_name"`
	BirthDate string `json:"birth_date,omitempty"` // YYYY-MM-DD
	Gender    string `json:"gender,omitempty"`
}

type UpdateProfileRequest struct {
	FullName  string `json:"full_name,omitempty"`
	BirthDate string `json:"birth_date,omitempty"`
	Gender    string `json:"gender,omitempty"`
}

type ProfileResponse struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	FullName     string    `json:"full_name"`
	AvatarURL    string    `json:"avatar_url"`
	BirthDate    string    `json:"birth_date"`
	Gender       string    `json:"gender"`
	ReferralCode string    `json:"referral_code"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PublicProfileResponse struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	FullName  string    `json:"full_name"`
	AvatarURL string    `json:"avatar_url"`
}

// Address DTOs
type AddressRequest struct {
	Label     string  `json:"label"`
	Address   string  `json:"address"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	Notes     string  `json:"notes,omitempty"`
	IsDefault bool    `json:"is_default,omitempty"`
}

type AddressResponse struct {
	ID        uuid.UUID `json:"id"`
	Label     string    `json:"label"`
	Address   string    `json:"address"`
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Notes     string    `json:"notes"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type AddressListResponse struct {
	Status string            `json:"status"`
	Data   []AddressResponse `json:"data"`
}

// Driver DTOs
type CreateDriverProfileRequest struct {
	VehicleType  string `json:"vehicle_type"`
	PlateNumber  string `json:"plate_number"`
	VehicleBrand string `json:"vehicle_brand"`
	VehicleModel string `json:"vehicle_model"`
	VehicleYear  int16  `json:"vehicle_year"`
	VehicleColor string `json:"vehicle_color"`
	SimNumber    string `json:"sim_number"`
	KtpNumber    string `json:"ktp_number"`
}

type UpdateDriverProfileRequest struct {
	VehicleType  string `json:"vehicle_type,omitempty"`
	PlateNumber  string `json:"plate_number,omitempty"`
	VehicleBrand string `json:"vehicle_brand,omitempty"`
	VehicleModel string `json:"vehicle_model,omitempty"`
	VehicleYear  int16  `json:"vehicle_year,omitempty"`
	VehicleColor string `json:"vehicle_color,omitempty"`
}

type DriverProfileResponse struct {
	ID                 uuid.UUID  `json:"id"`
	UserID             uuid.UUID  `json:"user_id"`
	VehicleType        string     `json:"vehicle_type"`
	PlateNumber        string     `json:"plate_number"`
	VehicleBrand       string     `json:"vehicle_brand"`
	VehicleModel       string     `json:"vehicle_model"`
	VehicleYear        int16      `json:"vehicle_year"`
	VehicleColor       string     `json:"vehicle_color"`
	SimNumber          string     `json:"sim_number"`
	KtpNumber          string     `json:"ktp_number"`
	VerificationStatus string     `json:"verification_status"`
	RatingAvg          float64    `json:"rating_avg"`
	TotalTrips         int        `json:"total_trips"`
	IsOnline           bool       `json:"is_online"`
	LastOnlineAt       *time.Time `json:"last_online_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

type DriverPublicProfileResponse struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	VehicleType  string    `json:"vehicle_type"`
	PlateNumber  string    `json:"plate_number"`
	VehicleBrand string    `json:"vehicle_brand"`
	VehicleColor string    `json:"vehicle_color"`
	RatingAvg    float64   `json:"rating_avg"`
}

// Document DTOs
type DocumentResponse struct {
	ID              uuid.UUID  `json:"id"`
	Type            string     `json:"type"`
	FileURL         string     `json:"file_url"`
	Status          string     `json:"status"`
	RejectionReason string     `json:"rejection_reason,omitempty"`
	VerifiedAt      *time.Time `json:"verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type DocumentListResponse struct {
	Status string             `json:"status"`
	Data   []DocumentResponse `json:"data"`
}

// Settings DTOs
type SettingsResponse struct {
	Language         string `json:"language"`
	NotifEnabled     bool   `json:"notif_enabled"`
	MarketingEnabled bool   `json:"marketing_enabled"`
}

type UpdateSettingsRequest struct {
	Language         string `json:"language,omitempty"`
	NotifEnabled     *bool  `json:"notif_enabled,omitempty"`
	MarketingEnabled *bool  `json:"marketing_enabled,omitempty"`
}

// Internal Phone Lookup DTO
type LookupUserByPhoneRequest struct {
	Phone string `json:"phone"`
}

type LookupUserByPhoneResponse struct {
	Found    bool       `json:"found"`
	UserID   *uuid.UUID `json:"user_id,omitempty"`
	FullName string     `json:"full_name,omitempty"`
}
