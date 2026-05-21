package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/model"
)

// MerchantRepositoryInterface defines persistence operations for merchants.
type MerchantRepositoryInterface interface {
	Create(ctx context.Context, m *model.Merchant) error
	GetByID(ctx context.Context, id string) (*model.Merchant, error)
	GetByUserID(ctx context.Context, userID string) (*model.Merchant, error)
	Update(ctx context.Context, id string, req model.UpdateMerchantRequest) (*model.Merchant, error)
	UpdateStatus(ctx context.Context, id string, status model.MerchantStatus) (*model.Merchant, error)
	ExistsByUserID(ctx context.Context, userID string) (bool, error)
	GetOperatingHours(ctx context.Context, merchantID string) ([]model.OperatingHours, error)
	UpsertOperatingHours(ctx context.Context, merchantID string, req model.UpsertOperatingHoursRequest) ([]model.OperatingHours, error)
	ListBankAccounts(ctx context.Context, merchantID string) ([]model.BankAccount, error)
	AddBankAccount(ctx context.Context, merchantID string, req model.AddBankAccountRequest) (*model.BankAccount, error)
	DeleteBankAccount(ctx context.Context, accountID string) error
	SetPrimaryBankAccount(ctx context.Context, merchantID, accountID string) (*model.BankAccount, error)
}

// MerchantRepository handles PostgreSQL persistence for merchants.
type MerchantRepository struct {
	db *sql.DB
}

// NewMerchantRepository creates a new MerchantRepository.
func NewMerchantRepository(db *sql.DB) *MerchantRepository {
	return &MerchantRepository{db: db}
}

func (r *MerchantRepository) Create(ctx context.Context, m *model.Merchant) error {
	m.ID = uuid.New().String()
	m.Status = model.StatusPendingReview
	m.CreatedAt = time.Now().UTC()
	m.UpdatedAt = time.Now().UTC()

	query := `
		INSERT INTO merchants (
			id, user_id, name, description, category, status,
			phone_number, email, address, city, lat, lng,
			min_order_cents, est_delivery_min, rating, total_reviews,
			created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`
	_, err := r.db.ExecContext(ctx, query,
		m.ID, m.UserID, m.Name, m.Description, m.Category, m.Status,
		m.PhoneNumber, m.Email, m.Address, m.City, m.Lat, m.Lng,
		m.MinOrderCents, m.EstDeliveryMin, 0.0, 0,
		m.CreatedAt, m.UpdatedAt,
	)
	return err
}

func (r *MerchantRepository) GetByID(ctx context.Context, id string) (*model.Merchant, error) {
	query := `
		SELECT id, user_id, name, description, category, status,
		       phone_number, email, address, city, lat, lng,
		       logo_url, banner_url, min_order_cents, est_delivery_min,
		       rating, total_reviews, created_at, updated_at
		FROM merchants WHERE id = $1`
	var m model.Merchant
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&m.ID, &m.UserID, &m.Name, &m.Description, &m.Category, &m.Status,
		&m.PhoneNumber, &m.Email, &m.Address, &m.City, &m.Lat, &m.Lng,
		&m.LogoURL, &m.BannerURL, &m.MinOrderCents, &m.EstDeliveryMin,
		&m.Rating, &m.TotalReviews, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (r *MerchantRepository) GetByUserID(ctx context.Context, userID string) (*model.Merchant, error) {
	query := `
		SELECT id, user_id, name, description, category, status,
		       phone_number, email, address, city, lat, lng,
		       logo_url, banner_url, min_order_cents, est_delivery_min,
		       rating, total_reviews, created_at, updated_at
		FROM merchants WHERE user_id = $1`
	var m model.Merchant
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&m.ID, &m.UserID, &m.Name, &m.Description, &m.Category, &m.Status,
		&m.PhoneNumber, &m.Email, &m.Address, &m.City, &m.Lat, &m.Lng,
		&m.LogoURL, &m.BannerURL, &m.MinOrderCents, &m.EstDeliveryMin,
		&m.Rating, &m.TotalReviews, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (r *MerchantRepository) Update(ctx context.Context, id string, req model.UpdateMerchantRequest) (*model.Merchant, error) {
	now := time.Now().UTC()
	query := `
		UPDATE merchants SET
			name = COALESCE($1, name),
			description = COALESCE($2, description),
			phone_number = COALESCE($3, phone_number),
			address = COALESCE($4, address),
			city = COALESCE($5, city),
			lat = COALESCE($6, lat),
			lng = COALESCE($7, lng),
			logo_url = COALESCE($8, logo_url),
			banner_url = COALESCE($9, banner_url),
			min_order_cents = COALESCE($10, min_order_cents),
			est_delivery_min = COALESCE($11, est_delivery_min),
			updated_at = $12
		WHERE id = $13`
	_, err := r.db.ExecContext(ctx, query,
		req.Name, req.Description, req.PhoneNumber, req.Address, req.City,
		req.Lat, req.Lng, req.LogoURL, req.BannerURL, req.MinOrderCents,
		req.EstDeliveryMin, now, id,
	)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *MerchantRepository) UpdateStatus(ctx context.Context, id string, status model.MerchantStatus) (*model.Merchant, error) {
	query := `UPDATE merchants SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, time.Now().UTC(), id)
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, id)
}

func (r *MerchantRepository) ExistsByUserID(ctx context.Context, userID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM merchants WHERE user_id = $1`, userID).Scan(&count)
	return count > 0, err
}

// ── Operating Hours ───────────────────────────────────────────────────────────

func (r *MerchantRepository) GetOperatingHours(ctx context.Context, merchantID string) ([]model.OperatingHours, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, merchant_id, day_of_week, open_time, close_time, is_closed, updated_at
		FROM merchant_operating_hours WHERE merchant_id = $1 ORDER BY day_of_week`, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hours []model.OperatingHours
	for rows.Next() {
		var h model.OperatingHours
		if err := rows.Scan(&h.ID, &h.MerchantID, &h.DayOfWeek, &h.OpenTime, &h.CloseTime, &h.IsClosed, &h.UpdatedAt); err != nil {
			return nil, err
		}
		hours = append(hours, h)
	}
	return hours, rows.Err()
}

func (r *MerchantRepository) UpsertOperatingHours(ctx context.Context, merchantID string, req model.UpsertOperatingHoursRequest) ([]model.OperatingHours, error) {
	now := time.Now().UTC()
	for _, item := range req.Hours {
		query := `
			INSERT INTO merchant_operating_hours (id, merchant_id, day_of_week, open_time, close_time, is_closed, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (merchant_id, day_of_week) DO UPDATE SET
				open_time = EXCLUDED.open_time,
				close_time = EXCLUDED.close_time,
				is_closed = EXCLUDED.is_closed,
				updated_at = EXCLUDED.updated_at`
		_, err := r.db.ExecContext(ctx, query,
			uuid.New().String(), merchantID, item.DayOfWeek,
			item.OpenTime, item.CloseTime, item.IsClosed, now,
		)
		if err != nil {
			return nil, fmt.Errorf("upsert day %d: %w", item.DayOfWeek, err)
		}
	}
	return r.GetOperatingHours(ctx, merchantID)
}

// ── Bank Accounts ─────────────────────────────────────────────────────────────

func (r *MerchantRepository) ListBankAccounts(ctx context.Context, merchantID string) ([]model.BankAccount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, merchant_id, bank_code, account_number, account_name, is_primary, created_at
		FROM merchant_bank_accounts WHERE merchant_id = $1 ORDER BY is_primary DESC, created_at`, merchantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []model.BankAccount
	for rows.Next() {
		var ba model.BankAccount
		if err := rows.Scan(&ba.ID, &ba.MerchantID, &ba.BankCode, &ba.AccountNumber, &ba.AccountName, &ba.IsPrimary, &ba.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, ba)
	}
	return accounts, rows.Err()
}

func (r *MerchantRepository) AddBankAccount(ctx context.Context, merchantID string, req model.AddBankAccountRequest) (*model.BankAccount, error) {
	ba := &model.BankAccount{
		ID:            uuid.New().String(),
		MerchantID:    merchantID,
		BankCode:      req.BankCode,
		AccountNumber: req.AccountNumber,
		AccountName:   req.AccountName,
		IsPrimary:     req.SetPrimary,
		CreatedAt:     time.Now().UTC(),
	}
	if req.SetPrimary {
		_, _ = r.db.ExecContext(ctx, `UPDATE merchant_bank_accounts SET is_primary = false WHERE merchant_id = $1`, merchantID)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO merchant_bank_accounts (id, merchant_id, bank_code, account_number, account_name, is_primary, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		ba.ID, ba.MerchantID, ba.BankCode, ba.AccountNumber, ba.AccountName, ba.IsPrimary, ba.CreatedAt,
	)
	return ba, err
}

func (r *MerchantRepository) DeleteBankAccount(ctx context.Context, accountID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM merchant_bank_accounts WHERE id = $1`, accountID)
	return err
}

func (r *MerchantRepository) SetPrimaryBankAccount(ctx context.Context, merchantID, accountID string) (*model.BankAccount, error) {
	_, err := r.db.ExecContext(ctx, `UPDATE merchant_bank_accounts SET is_primary = false WHERE merchant_id = $1`, merchantID)
	if err != nil {
		return nil, err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE merchant_bank_accounts SET is_primary = true WHERE id = $1`, accountID)
	if err != nil {
		return nil, err
	}
	var ba model.BankAccount
	err = r.db.QueryRowContext(ctx, `
		SELECT id, merchant_id, bank_code, account_number, account_name, is_primary, created_at
		FROM merchant_bank_accounts WHERE id = $1`, accountID,
	).Scan(&ba.ID, &ba.MerchantID, &ba.BankCode, &ba.AccountNumber, &ba.AccountName, &ba.IsPrimary, &ba.CreatedAt)
	return &ba, err
}
