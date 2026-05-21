package models

import (
	"time"

	"github.com/google/uuid"
)

// DriverProfile represents a driver's specific profile info.
type DriverProfile struct {
	ID                 uuid.UUID `json:"id"`
	UserID             uuid.UUID `json:"user_id"`
	VehicleType        string    `json:"vehicle_type"` // motor, car, truck
	PlateNumber        string    `json:"plate_number"`
	VehicleBrand       string    `json:"vehicle_brand"`
	VehicleModel       string    `json:"vehicle_model"`
	VehicleYear        int16     `json:"vehicle_year"`
	VehicleColor       string    `json:"vehicle_color"`
	SimNumber          string    `json:"sim_number"`
	KtpNumber          string    `json:"ktp_number"`
	VerificationStatus string    `json:"verification_status"` // pending, verified, rejected, suspended
	RatingAvg          float64   `json:"rating_avg"`
	TotalTrips         int       `json:"total_trips"`
	IsOnline           bool      `json:"is_online"`
	LastOnlineAt       *time.Time `json:"last_online_at,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// DriverDocument represents a verified document (KTP, SIM, STNK, Selfie).
type DriverDocument struct {
	ID              uuid.UUID  `json:"id"`
	DriverID        uuid.UUID  `json:"driver_id"`
	Type            string     `json:"type"` // ktp, sim, stnk, selfie
	FileURL         string     `json:"file_url"`
	Status          string     `json:"status"` // pending, approved, rejected
	RejectionReason string     `json:"rejection_reason"`
	VerifiedAt      *time.Time `json:"verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
