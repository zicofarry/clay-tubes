// Package repository implements the data access layer for the Payment Service.
package repository

import (
	"context"
	"database/sql"
	"time"
)

// ── Models ───────────────────────────────────────────────────────────────────

// PaymentMethod represents a row in the `payment_methods` table.
type PaymentMethod struct {
	ID          string    `json:"method_id"`
	UserID      string    `json:"user_id"`
	Type        string    `json:"type"`         // clay_wallet, credit_card, debit_card, bank_transfer, gopay, ovo, dana, cod
	DisplayName string    `json:"display_name"` // e.g. "Visa •••• 1234"
	LastFour    *string   `json:"last_four,omitempty"`
	ExpiryMonth *int      `json:"expiry_month,omitempty"`
	ExpiryYear  *int      `json:"expiry_year,omitempty"`
	IsDefault   bool      `json:"is_default"`
	CardToken   *string   `json:"-"` // tokenized card from gateway
	CreatedAt   time.Time `json:"created_at"`
}

// Transaction represents a row in the `transactions` table.
type Transaction struct {
	ID                string    `json:"transaction_id"`
	UserID            string    `json:"user_id"`
	OrderID           *string   `json:"order_id,omitempty"`
	Type              string    `json:"type"`   // charge, refund, top_up
	Status            string    `json:"status"` // pending, completed, failed, refunded
	Amount            int       `json:"amount"` // IDR
	PaymentMethodType string    `json:"payment_method_type"`
	Description       string    `json:"description"`
	GatewayReference  *string   `json:"gateway_reference,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Hold represents a row in the `holds` table.
type Hold struct {
	ID                string    `json:"hold_id"`
	OrderID           string    `json:"order_id"`
	UserID            string    `json:"user_id"`
	Amount            int       `json:"amount"`
	PaymentMethodType string    `json:"payment_method_type"`
	PaymentMethodID   *string   `json:"payment_method_id,omitempty"`
	Status            string    `json:"status"` // held, captured, released, expired
	ExpiresAt         time.Time `json:"expires_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Refund represents a row in the `refunds` table.
type Refund struct {
	ID                  string     `json:"refund_id"`
	TransactionID       string     `json:"transaction_id"`
	OrderID             string     `json:"order_id"`
	UserID              string     `json:"user_id"`
	Amount              int        `json:"amount"`
	Reason              string     `json:"reason"` // user_cancelled, driver_cancelled, system_error, fraud
	Status              string     `json:"status"` // processed, pending
	EstimatedCompletion *time.Time `json:"estimated_completion,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

// Settlement represents a row in the `settlements` table.
type Settlement struct {
	ID           string    `json:"settlement_id"`
	OrderID      string    `json:"order_id"`
	DriverID     string    `json:"driver_id"`
	GrossFare    int       `json:"gross_fare"`
	PlatformFee  int       `json:"platform_fee"`
	DriverPayout int       `json:"driver_payout"`
	ServiceType  string    `json:"service_type"` // ride, food, delivery
	Status       string    `json:"status"`       // settled, pending
	CreatedAt    time.Time `json:"created_at"`
}

// CodVerification represents a row in the `cod_verifications` table.
type CodVerification struct {
	ID                     string    `json:"verification_id"`
	UserID                 string    `json:"user_id"`
	RecipientPhone         string    `json:"recipient_phone"`
	OrderType              string    `json:"order_type"` // food, delivery
	OrderSummary           string    `json:"order_summary"`
	VerificationType       string    `json:"verification_type"` // push_confirmation, whatsapp_otp
	RecipientHasClayAcct   bool      `json:"recipient_has_clay_account"`
	Status                 string    `json:"status"` // pending, accepted, rejected, verified, expired, failed
	OTPHash                *string   `json:"-"`
	OTPAttemptsRemaining   int       `json:"otp_attempts_remaining"`
	CodToken               *string   `json:"cod_token,omitempty"`
	ExpiresAt              time.Time `json:"expires_at"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// ── Interface ────────────────────────────────────────────────────────────────

// PaymentRepositoryInterface defines the contract for payment data access.
// Used by service layer and for mock generation in tests.
//go:generate mockgen -source=payment_repository.go -destination=../../mocks/repomock/mock_payment_repository.go -package=repomock
type PaymentRepositoryInterface interface {
	// Payment Methods
	CreatePaymentMethod(ctx context.Context, pm *PaymentMethod) (*PaymentMethod, error)
	ListPaymentMethods(ctx context.Context, userID string) ([]PaymentMethod, error)
	DeletePaymentMethod(ctx context.Context, userID, methodID string) error
	SetDefaultPaymentMethod(ctx context.Context, userID, methodID string) error
	FindPaymentMethodByID(ctx context.Context, methodID string) (*PaymentMethod, error)

	// Transactions
	CreateTransaction(ctx context.Context, tx *Transaction) (*Transaction, error)
	FindTransactionByID(ctx context.Context, id string) (*Transaction, error)
	ListTransactions(ctx context.Context, userID string, txType string, page, limit int) ([]Transaction, int, error)
	UpdateTransactionStatus(ctx context.Context, id, status string) error
	FindTransactionByOrderID(ctx context.Context, orderID string) (*Transaction, error)

	// Holds
	CreateHold(ctx context.Context, hold *Hold) (*Hold, error)
	FindHoldByID(ctx context.Context, id string) (*Hold, error)
	UpdateHoldStatus(ctx context.Context, id, status string) error

	// Refunds
	CreateRefund(ctx context.Context, refund *Refund) (*Refund, error)

	// Settlements
	CreateSettlement(ctx context.Context, settlement *Settlement) (*Settlement, error)
	SettlementExistsByOrderID(ctx context.Context, orderID string) (bool, error)

	// COD Verification
	CreateCodVerification(ctx context.Context, cv *CodVerification) (*CodVerification, error)
	FindCodVerificationByID(ctx context.Context, id string) (*CodVerification, error)
	UpdateCodVerification(ctx context.Context, cv *CodVerification) error
}

// ── Implementation ───────────────────────────────────────────────────────────

// PaymentRepository implements PaymentRepositoryInterface using PostgreSQL.
type PaymentRepository struct {
	db    *sql.DB
	redis interface{} // TODO: Replace with Redis client type
}

// NewPaymentRepository creates a new PaymentRepository.
func NewPaymentRepository(db *sql.DB, redis interface{}) *PaymentRepository {
	return &PaymentRepository{db: db, redis: redis}
}

// ── Payment Methods ──────────────────────────────────────────────────────────

func (r *PaymentRepository) CreatePaymentMethod(ctx context.Context, pm *PaymentMethod) (*PaymentMethod, error) {
	query := `
		INSERT INTO payment_methods (user_id, type, display_name, last_four, expiry_month, expiry_year, is_default, card_token)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, query,
		pm.UserID, pm.Type, pm.DisplayName, pm.LastFour, pm.ExpiryMonth, pm.ExpiryYear, pm.IsDefault, pm.CardToken,
	).Scan(&pm.ID, &pm.CreatedAt)
	if err != nil {
		return nil, err
	}
	return pm, nil
}

func (r *PaymentRepository) ListPaymentMethods(ctx context.Context, userID string) ([]PaymentMethod, error) {
	query := `
		SELECT id, user_id, type, display_name, last_four, expiry_month, expiry_year, is_default, created_at
		FROM payment_methods WHERE user_id = $1 ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var methods []PaymentMethod
	for rows.Next() {
		var pm PaymentMethod
		if err := rows.Scan(
			&pm.ID, &pm.UserID, &pm.Type, &pm.DisplayName,
			&pm.LastFour, &pm.ExpiryMonth, &pm.ExpiryYear, &pm.IsDefault, &pm.CreatedAt,
		); err != nil {
			return nil, err
		}
		methods = append(methods, pm)
	}
	return methods, rows.Err()
}

func (r *PaymentRepository) DeletePaymentMethod(ctx context.Context, userID, methodID string) error {
	query := `DELETE FROM payment_methods WHERE id = $1 AND user_id = $2`
	result, err := r.db.ExecContext(ctx, query, methodID, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PaymentRepository) SetDefaultPaymentMethod(ctx context.Context, userID, methodID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unset all defaults for user
	if _, err := tx.ExecContext(ctx, `UPDATE payment_methods SET is_default = false WHERE user_id = $1`, userID); err != nil {
		return err
	}
	// Set the specified method as default
	if _, err := tx.ExecContext(ctx, `UPDATE payment_methods SET is_default = true WHERE id = $1 AND user_id = $2`, methodID, userID); err != nil {
		return err
	}

	return tx.Commit()
}

func (r *PaymentRepository) FindPaymentMethodByID(ctx context.Context, methodID string) (*PaymentMethod, error) {
	query := `
		SELECT id, user_id, type, display_name, last_four, expiry_month, expiry_year, is_default, created_at
		FROM payment_methods WHERE id = $1
	`
	pm := &PaymentMethod{}
	err := r.db.QueryRowContext(ctx, query, methodID).Scan(
		&pm.ID, &pm.UserID, &pm.Type, &pm.DisplayName,
		&pm.LastFour, &pm.ExpiryMonth, &pm.ExpiryYear, &pm.IsDefault, &pm.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return pm, nil
}

// ── Transactions ─────────────────────────────────────────────────────────────

func (r *PaymentRepository) CreateTransaction(ctx context.Context, tx *Transaction) (*Transaction, error) {
	query := `
		INSERT INTO transactions (user_id, order_id, type, status, amount, payment_method_type, description)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRowContext(ctx, query,
		tx.UserID, tx.OrderID, tx.Type, tx.Status, tx.Amount, tx.PaymentMethodType, tx.Description,
	).Scan(&tx.ID, &tx.CreatedAt, &tx.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (r *PaymentRepository) FindTransactionByID(ctx context.Context, id string) (*Transaction, error) {
	query := `
		SELECT id, user_id, order_id, type, status, amount, payment_method_type, description, gateway_reference, created_at, updated_at
		FROM transactions WHERE id = $1
	`
	tx := &Transaction{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tx.ID, &tx.UserID, &tx.OrderID, &tx.Type, &tx.Status,
		&tx.Amount, &tx.PaymentMethodType, &tx.Description, &tx.GatewayReference,
		&tx.CreatedAt, &tx.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func (r *PaymentRepository) ListTransactions(ctx context.Context, userID string, txType string, page, limit int) ([]Transaction, int, error) {
	offset := (page - 1) * limit

	// Count total
	countQuery := `SELECT COUNT(*) FROM transactions WHERE user_id = $1`
	args := []interface{}{userID}
	if txType != "" {
		countQuery += ` AND type = $2`
		args = append(args, txType)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch page
	dataQuery := `
		SELECT id, user_id, order_id, type, status, amount, payment_method_type, description, gateway_reference, created_at, updated_at
		FROM transactions WHERE user_id = $1
	`
	dataArgs := []interface{}{userID}
	if txType != "" {
		dataQuery += ` AND type = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
		dataArgs = append(dataArgs, txType, limit, offset)
	} else {
		dataQuery += ` ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		dataArgs = append(dataArgs, limit, offset)
	}

	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(
			&tx.ID, &tx.UserID, &tx.OrderID, &tx.Type, &tx.Status,
			&tx.Amount, &tx.PaymentMethodType, &tx.Description, &tx.GatewayReference,
			&tx.CreatedAt, &tx.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		transactions = append(transactions, tx)
	}
	return transactions, total, rows.Err()
}

func (r *PaymentRepository) UpdateTransactionStatus(ctx context.Context, id, status string) error {
	query := `UPDATE transactions SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, status, id)
	return err
}

func (r *PaymentRepository) FindTransactionByOrderID(ctx context.Context, orderID string) (*Transaction, error) {
	query := `
		SELECT id, user_id, order_id, type, status, amount, payment_method_type, description, gateway_reference, created_at, updated_at
		FROM transactions WHERE order_id = $1 AND type = 'charge' ORDER BY created_at DESC LIMIT 1
	`
	tx := &Transaction{}
	err := r.db.QueryRowContext(ctx, query, orderID).Scan(
		&tx.ID, &tx.UserID, &tx.OrderID, &tx.Type, &tx.Status,
		&tx.Amount, &tx.PaymentMethodType, &tx.Description, &tx.GatewayReference,
		&tx.CreatedAt, &tx.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

// ── Holds ────────────────────────────────────────────────────────────────────

func (r *PaymentRepository) CreateHold(ctx context.Context, hold *Hold) (*Hold, error) {
	query := `
		INSERT INTO holds (order_id, user_id, amount, payment_method_type, payment_method_id, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRowContext(ctx, query,
		hold.OrderID, hold.UserID, hold.Amount, hold.PaymentMethodType,
		hold.PaymentMethodID, hold.Status, hold.ExpiresAt,
	).Scan(&hold.ID, &hold.CreatedAt, &hold.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return hold, nil
}

func (r *PaymentRepository) FindHoldByID(ctx context.Context, id string) (*Hold, error) {
	query := `
		SELECT id, order_id, user_id, amount, payment_method_type, payment_method_id, status, expires_at, created_at, updated_at
		FROM holds WHERE id = $1
	`
	hold := &Hold{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&hold.ID, &hold.OrderID, &hold.UserID, &hold.Amount,
		&hold.PaymentMethodType, &hold.PaymentMethodID, &hold.Status,
		&hold.ExpiresAt, &hold.CreatedAt, &hold.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return hold, nil
}

func (r *PaymentRepository) UpdateHoldStatus(ctx context.Context, id, status string) error {
	query := `UPDATE holds SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, status, id)
	return err
}

// ── Refunds ──────────────────────────────────────────────────────────────────

func (r *PaymentRepository) CreateRefund(ctx context.Context, refund *Refund) (*Refund, error) {
	query := `
		INSERT INTO refunds (transaction_id, order_id, user_id, amount, reason, status, estimated_completion)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, query,
		refund.TransactionID, refund.OrderID, refund.UserID,
		refund.Amount, refund.Reason, refund.Status, refund.EstimatedCompletion,
	).Scan(&refund.ID, &refund.CreatedAt)
	if err != nil {
		return nil, err
	}
	return refund, nil
}

// ── Settlements ──────────────────────────────────────────────────────────────

func (r *PaymentRepository) CreateSettlement(ctx context.Context, s *Settlement) (*Settlement, error) {
	query := `
		INSERT INTO settlements (order_id, driver_id, gross_fare, platform_fee, driver_payout, service_type, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, query,
		s.OrderID, s.DriverID, s.GrossFare, s.PlatformFee,
		s.DriverPayout, s.ServiceType, s.Status,
	).Scan(&s.ID, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (r *PaymentRepository) SettlementExistsByOrderID(ctx context.Context, orderID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM settlements WHERE order_id = $1)`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, orderID).Scan(&exists)
	return exists, err
}

// ── COD Verification ─────────────────────────────────────────────────────────

func (r *PaymentRepository) CreateCodVerification(ctx context.Context, cv *CodVerification) (*CodVerification, error) {
	query := `
		INSERT INTO cod_verifications (user_id, recipient_phone, order_type, order_summary, verification_type, recipient_has_clay_acct, status, otp_hash, otp_attempts_remaining, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRowContext(ctx, query,
		cv.UserID, cv.RecipientPhone, cv.OrderType, cv.OrderSummary,
		cv.VerificationType, cv.RecipientHasClayAcct, cv.Status,
		cv.OTPHash, cv.OTPAttemptsRemaining, cv.ExpiresAt,
	).Scan(&cv.ID, &cv.CreatedAt, &cv.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return cv, nil
}

func (r *PaymentRepository) FindCodVerificationByID(ctx context.Context, id string) (*CodVerification, error) {
	query := `
		SELECT id, user_id, recipient_phone, order_type, order_summary, verification_type,
			recipient_has_clay_acct, status, otp_hash, otp_attempts_remaining, cod_token,
			expires_at, created_at, updated_at
		FROM cod_verifications WHERE id = $1
	`
	cv := &CodVerification{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&cv.ID, &cv.UserID, &cv.RecipientPhone, &cv.OrderType, &cv.OrderSummary,
		&cv.VerificationType, &cv.RecipientHasClayAcct, &cv.Status,
		&cv.OTPHash, &cv.OTPAttemptsRemaining, &cv.CodToken,
		&cv.ExpiresAt, &cv.CreatedAt, &cv.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return cv, nil
}

func (r *PaymentRepository) UpdateCodVerification(ctx context.Context, cv *CodVerification) error {
	query := `
		UPDATE cod_verifications
		SET status = $1, otp_attempts_remaining = $2, cod_token = $3, updated_at = NOW()
		WHERE id = $4
	`
	_, err := r.db.ExecContext(ctx, query, cv.Status, cv.OTPAttemptsRemaining, cv.CodToken, cv.ID)
	return err
}
