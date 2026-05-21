package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/zicofarry/clay-app/backend/services/user-service/internal/models"
)

//go:generate mockgen -source=user_repository.go -destination=../../mocks/repomock/mock_user_repository.go -package=repomock
type UserRepositoryInterface interface {
	// Profile
	CreateProfile(ctx context.Context, profile *models.UserProfile) error
	UpdateProfile(ctx context.Context, profile *models.UserProfile) error
	GetProfileByUserID(ctx context.Context, userID uuid.UUID) (*models.UserProfile, error)
	ApplyReferral(ctx context.Context, userID uuid.UUID, referredBy uuid.UUID) error

	// Address
	CreateAddress(ctx context.Context, address *models.UserAddress) error
	UpdateAddress(ctx context.Context, address *models.UserAddress) error
	DeleteAddress(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	GetAddress(ctx context.Context, id uuid.UUID) (*models.UserAddress, error)
	ListAddresses(ctx context.Context, userID uuid.UUID) ([]models.UserAddress, error)
	SetDefaultAddress(ctx context.Context, id uuid.UUID, userID uuid.UUID) error

	// Driver
	CreateDriverProfile(ctx context.Context, profile *models.DriverProfile) error
	UpdateDriverProfile(ctx context.Context, profile *models.DriverProfile) error
	GetDriverProfileByUserID(ctx context.Context, userID uuid.UUID) (*models.DriverProfile, error)
	GetDriverProfileByID(ctx context.Context, driverID uuid.UUID) (*models.DriverProfile, error)
	ToggleDriverOnline(ctx context.Context, driverID uuid.UUID, isOnline bool) error

	// Driver Document
	CreateDriverDocument(ctx context.Context, doc *models.DriverDocument) error
	UpdateDriverDocumentStatus(ctx context.Context, id uuid.UUID, status string, rejectionReason string) error
	GetDriverDocument(ctx context.Context, id uuid.UUID) (*models.DriverDocument, error)
	ListDriverDocuments(ctx context.Context, driverID uuid.UUID) ([]models.DriverDocument, error)
	DeleteDriverDocument(ctx context.Context, id uuid.UUID, driverID uuid.UUID) error

	// Settings
	UpdateSettings(ctx context.Context, settings *models.UserSettings) error
	GetSettings(ctx context.Context, userID uuid.UUID) (*models.UserSettings, error)

	// Internal
	LookupUserByPhone(ctx context.Context, phone string) (*models.UserProfile, error)
}

type UserRepository struct {
	db    *sql.DB
	redis *redis.Client
}

func NewUserRepository(db *sql.DB, rdb *redis.Client) *UserRepository {
	return &UserRepository{db: db, redis: rdb}
}

// --- Profile Repository ---

func (r *UserRepository) CreateProfile(ctx context.Context, profile *models.UserProfile) error {
	query := `
		INSERT INTO user_profiles (id, user_id, full_name, avatar_url, birth_date, gender, referral_code, referred_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.ExecContext(ctx, query,
		profile.ID, profile.UserID, profile.FullName, profile.AvatarURL,
		profile.BirthDate, profile.Gender, profile.ReferralCode, profile.ReferredBy,
		profile.CreatedAt, profile.UpdatedAt,
	)
	return err
}

func (r *UserRepository) UpdateProfile(ctx context.Context, profile *models.UserProfile) error {
	query := `
		UPDATE user_profiles
		SET full_name = $1, avatar_url = $2, birth_date = $3, gender = $4, updated_at = $5
		WHERE user_id = $6
	`
	_, err := r.db.ExecContext(ctx, query,
		profile.FullName, profile.AvatarURL, profile.BirthDate, profile.Gender, profile.UpdatedAt,
		profile.UserID,
	)
	if err != nil {
		return err
	}

	// Invalidate cache
	if r.redis != nil {
		_ = r.redis.Del(ctx, fmt.Sprintf("profile:%s", profile.UserID.String())).Err()
	}
	return nil
}

func (r *UserRepository) GetProfileByUserID(ctx context.Context, userID uuid.UUID) (*models.UserProfile, error) {
	cacheKey := fmt.Sprintf("profile:%s", userID.String())
	if r.redis != nil {
		cached, err := r.redis.HGetAll(ctx, cacheKey).Result()
		if err == nil && len(cached) > 0 {
			// (Future implementation: return cached data)
		}
	}

	query := `
		SELECT id, user_id, full_name, avatar_url, birth_date, gender, referral_code, referred_by, created_at, updated_at
		FROM user_profiles
		WHERE user_id = $1
	`
	var p models.UserProfile
	var birthDate sql.NullString
	var referredBy uuid.NullUUID
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&p.ID, &p.UserID, &p.FullName, &p.AvatarURL,
		&birthDate, &p.Gender, &p.ReferralCode, &referredBy,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found
		}
		return nil, err
	}
	if birthDate.Valid {
		p.BirthDate = &birthDate.String
	}
	if referredBy.Valid {
		ref := referredBy.UUID
		p.ReferredBy = &ref
	}

	// Cache it
	if r.redis != nil {
		_ = r.redis.HSet(ctx, cacheKey, map[string]interface{}{
			"full_name":     p.FullName,
			"avatar_url":    p.AvatarURL,
			"referral_code": p.ReferralCode,
			"role":          "user",
			"phone":         "",
		}).Err()
		r.redis.Expire(ctx, cacheKey, 10*time.Minute)
	}

	return &p, nil
}

func (r *UserRepository) ApplyReferral(ctx context.Context, userID uuid.UUID, referredBy uuid.UUID) error {
	query := `UPDATE user_profiles SET referred_by = $1, updated_at = $2 WHERE user_id = $3`
	_, err := r.db.ExecContext(ctx, query, referredBy, time.Now(), userID)
	return err
}

// --- Address Repository ---

func (r *UserRepository) CreateAddress(ctx context.Context, address *models.UserAddress) error {
	query := `
		INSERT INTO user_addresses (id, user_id, label, address, lat, lng, notes, is_default, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.ExecContext(ctx, query,
		address.ID, address.UserID, address.Label, address.Address,
		address.Lat, address.Lng, address.Notes, address.IsDefault,
		address.CreatedAt, address.UpdatedAt,
	)
	return err
}

func (r *UserRepository) UpdateAddress(ctx context.Context, address *models.UserAddress) error {
	query := `
		UPDATE user_addresses
		SET label = $1, address = $2, lat = $3, lng = $4, notes = $5, is_default = $6, updated_at = $7
		WHERE id = $8 AND user_id = $9
	`
	_, err := r.db.ExecContext(ctx, query,
		address.Label, address.Address, address.Lat, address.Lng,
		address.Notes, address.IsDefault, address.UpdatedAt,
		address.ID, address.UserID,
	)
	return err
}

func (r *UserRepository) DeleteAddress(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	query := `DELETE FROM user_addresses WHERE id = $1 AND user_id = $2`
	_, err := r.db.ExecContext(ctx, query, id, userID)
	return err
}

func (r *UserRepository) GetAddress(ctx context.Context, id uuid.UUID) (*models.UserAddress, error) {
	query := `
		SELECT id, user_id, label, address, lat, lng, notes, is_default, created_at, updated_at
		FROM user_addresses
		WHERE id = $1
	`
	var a models.UserAddress
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&a.ID, &a.UserID, &a.Label, &a.Address,
		&a.Lat, &a.Lng, &a.Notes, &a.IsDefault,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *UserRepository) ListAddresses(ctx context.Context, userID uuid.UUID) ([]models.UserAddress, error) {
	query := `
		SELECT id, user_id, label, address, lat, lng, notes, is_default, created_at, updated_at
		FROM user_addresses
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addresses []models.UserAddress
	for rows.Next() {
		var a models.UserAddress
		err := rows.Scan(
			&a.ID, &a.UserID, &a.Label, &a.Address,
			&a.Lat, &a.Lng, &a.Notes, &a.IsDefault,
			&a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, a)
	}
	return addresses, nil
}

func (r *UserRepository) SetDefaultAddress(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `UPDATE user_addresses SET is_default = false WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `UPDATE user_addresses SET is_default = true WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// --- Driver Repository ---

func (r *UserRepository) CreateDriverProfile(ctx context.Context, profile *models.DriverProfile) error {
	query := `
		INSERT INTO driver_profiles (id, user_id, vehicle_type, plate_number, vehicle_brand, vehicle_model, vehicle_year, vehicle_color, sim_number, ktp_number, verification_status, rating_avg, total_trips, is_online, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	_, err := r.db.ExecContext(ctx, query,
		profile.ID, profile.UserID, profile.VehicleType, profile.PlateNumber,
		profile.VehicleBrand, profile.VehicleModel, profile.VehicleYear, profile.VehicleColor,
		profile.SimNumber, profile.KtpNumber, profile.VerificationStatus, profile.RatingAvg,
		profile.TotalTrips, profile.IsOnline, profile.CreatedAt,
	)
	return err
}

func (r *UserRepository) UpdateDriverProfile(ctx context.Context, profile *models.DriverProfile) error {
	query := `
		UPDATE driver_profiles
		SET vehicle_type = $1, plate_number = $2, vehicle_brand = $3, vehicle_model = $4, vehicle_year = $5, vehicle_color = $6
		WHERE id = $7 AND user_id = $8
	`
	_, err := r.db.ExecContext(ctx, query,
		profile.VehicleType, profile.PlateNumber, profile.VehicleBrand,
		profile.VehicleModel, profile.VehicleYear, profile.VehicleColor,
		profile.ID, profile.UserID,
	)
	if err == nil && r.redis != nil {
		_ = r.redis.Del(ctx, fmt.Sprintf("driver:meta:%s", profile.ID.String())).Err()
	}
	return err
}

func (r *UserRepository) GetDriverProfileByUserID(ctx context.Context, userID uuid.UUID) (*models.DriverProfile, error) {
	query := `
		SELECT id, user_id, vehicle_type, plate_number, vehicle_brand, vehicle_model, vehicle_year, vehicle_color, sim_number, ktp_number, verification_status, rating_avg, total_trips, is_online, last_online_at, created_at
		FROM driver_profiles
		WHERE user_id = $1
	`
	var d models.DriverProfile
	var lastOnlineAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&d.ID, &d.UserID, &d.VehicleType, &d.PlateNumber, &d.VehicleBrand, &d.VehicleModel,
		&d.VehicleYear, &d.VehicleColor, &d.SimNumber, &d.KtpNumber, &d.VerificationStatus,
		&d.RatingAvg, &d.TotalTrips, &d.IsOnline, &lastOnlineAt, &d.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if lastOnlineAt.Valid {
		d.LastOnlineAt = &lastOnlineAt.Time
	}
	return &d, nil
}

func (r *UserRepository) GetDriverProfileByID(ctx context.Context, driverID uuid.UUID) (*models.DriverProfile, error) {
	query := `
		SELECT id, user_id, vehicle_type, plate_number, vehicle_brand, vehicle_model, vehicle_year, vehicle_color, sim_number, ktp_number, verification_status, rating_avg, total_trips, is_online, last_online_at, created_at
		FROM driver_profiles
		WHERE id = $1
	`
	var d models.DriverProfile
	var lastOnlineAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, driverID).Scan(
		&d.ID, &d.UserID, &d.VehicleType, &d.PlateNumber, &d.VehicleBrand, &d.VehicleModel,
		&d.VehicleYear, &d.VehicleColor, &d.SimNumber, &d.KtpNumber, &d.VerificationStatus,
		&d.RatingAvg, &d.TotalTrips, &d.IsOnline, &lastOnlineAt, &d.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if lastOnlineAt.Valid {
		d.LastOnlineAt = &lastOnlineAt.Time
	}
	return &d, nil
}

func (r *UserRepository) ToggleDriverOnline(ctx context.Context, driverID uuid.UUID, isOnline bool) error {
	query := `UPDATE driver_profiles SET is_online = $1, last_online_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, isOnline, time.Now(), driverID)
	return err
}

// --- Driver Document Repository ---

func (r *UserRepository) CreateDriverDocument(ctx context.Context, doc *models.DriverDocument) error {
	query := `
		INSERT INTO driver_documents (id, driver_id, type, file_url, status, rejection_reason, verified_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.db.ExecContext(ctx, query,
		doc.ID, doc.DriverID, doc.Type, doc.FileURL, doc.Status,
		doc.RejectionReason, doc.VerifiedAt, doc.CreatedAt, doc.UpdatedAt,
	)
	return err
}

func (r *UserRepository) UpdateDriverDocumentStatus(ctx context.Context, id uuid.UUID, status string, rejectionReason string) error {
	var verifiedAt *time.Time
	if status == "approved" {
		now := time.Now()
		verifiedAt = &now
	}
	query := `
		UPDATE driver_documents
		SET status = $1, rejection_reason = $2, verified_at = $3, updated_at = $4
		WHERE id = $5
	`
	_, err := r.db.ExecContext(ctx, query, status, rejectionReason, verifiedAt, time.Now(), id)
	return err
}

func (r *UserRepository) GetDriverDocument(ctx context.Context, id uuid.UUID) (*models.DriverDocument, error) {
	query := `
		SELECT id, driver_id, type, file_url, status, rejection_reason, verified_at, created_at, updated_at
		FROM driver_documents
		WHERE id = $1
	`
	var d models.DriverDocument
	var verifiedAt sql.NullTime
	var rejectReason sql.NullString
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&d.ID, &d.DriverID, &d.Type, &d.FileURL, &d.Status,
		&rejectReason, &verifiedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if verifiedAt.Valid {
		d.VerifiedAt = &verifiedAt.Time
	}
	if rejectReason.Valid {
		d.RejectionReason = rejectReason.String
	}
	return &d, nil
}

func (r *UserRepository) ListDriverDocuments(ctx context.Context, driverID uuid.UUID) ([]models.DriverDocument, error) {
	query := `
		SELECT id, driver_id, type, file_url, status, rejection_reason, verified_at, created_at, updated_at
		FROM driver_documents
		WHERE driver_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, driverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []models.DriverDocument
	for rows.Next() {
		var d models.DriverDocument
		var verifiedAt sql.NullTime
		var rejectReason sql.NullString
		err := rows.Scan(
			&d.ID, &d.DriverID, &d.Type, &d.FileURL, &d.Status,
			&rejectReason, &verifiedAt, &d.CreatedAt, &d.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if verifiedAt.Valid {
			d.VerifiedAt = &verifiedAt.Time
		}
		if rejectReason.Valid {
			d.RejectionReason = rejectReason.String
		}
		docs = append(docs, d)
	}
	return docs, nil
}

func (r *UserRepository) DeleteDriverDocument(ctx context.Context, id uuid.UUID, driverID uuid.UUID) error {
	query := `DELETE FROM driver_documents WHERE id = $1 AND driver_id = $2 AND status = 'pending'`
	res, err := r.db.ExecContext(ctx, query, id, driverID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("document not found or cannot be deleted")
	}
	return nil
}

// --- Settings Repository ---

func (r *UserRepository) UpdateSettings(ctx context.Context, settings *models.UserSettings) error {
	query := `
		INSERT INTO user_settings (user_id, language, notif_enabled, marketing_enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET language = EXCLUDED.language,
		    notif_enabled = EXCLUDED.notif_enabled,
		    marketing_enabled = EXCLUDED.marketing_enabled,
		    updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.ExecContext(ctx, query,
		settings.UserID, settings.Language, settings.NotifEnabled, settings.MarketingEnabled, time.Now(),
	)
	return err
}

func (r *UserRepository) GetSettings(ctx context.Context, userID uuid.UUID) (*models.UserSettings, error) {
	query := `
		SELECT user_id, language, notif_enabled, marketing_enabled, updated_at
		FROM user_settings
		WHERE user_id = $1
	`
	var s models.UserSettings
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&s.UserID, &s.Language, &s.NotifEnabled, &s.MarketingEnabled, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &models.UserSettings{
				UserID:           userID,
				Language:         "en",
				NotifEnabled:     true,
				MarketingEnabled: false,
				UpdatedAt:        time.Now(),
			}, nil
		}
		return nil, err
	}
	return &s, nil
}

// --- Internal ---

func (r *UserRepository) LookupUserByPhone(ctx context.Context, phone string) (*models.UserProfile, error) {
	query := `SELECT id, user_id, full_name FROM user_profiles WHERE phone = $1`
	var p models.UserProfile
	err := r.db.QueryRowContext(ctx, query, phone).Scan(&p.ID, &p.UserID, &p.FullName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}
