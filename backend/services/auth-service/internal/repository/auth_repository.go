// Package repository implements the data access layer for the Auth Service.
package repository

import (
	"context"
	"database/sql"
	"time"
)

// ── Models ───────────────────────────────────────────────────────────────────

// Credential represents a row in the `credentials` table.
type Credential struct {
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	Phone          string    `json:"phone"`
	PasswordHash   string    `json:"-"`
	Role           string    `json:"role"`   // user | driver | admin
	Status         string    `json:"status"` // active | suspended
	EmailVerified  bool      `json:"email_verified"`
	PhoneVerified  bool      `json:"phone_verified"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ── Interface ────────────────────────────────────────────────────────────────

// AuthRepositoryInterface defines the contract for auth data access.
// Used by service layer and for mock generation in tests.
//go:generate mockgen -source=auth_repository.go -destination=../../mocks/repomock/mock_auth_repository.go -package=repomock
type AuthRepositoryInterface interface {
	// CreateCredential inserts a new credential record.
	CreateCredential(ctx context.Context, cred *Credential) (*Credential, error)

	// FindByID returns a credential by user ID.
	FindByID(ctx context.Context, id string) (*Credential, error)

	// FindByIdentifier returns a credential by email or phone.
	FindByIdentifier(ctx context.Context, identifier string) (*Credential, error)

	// ExistsByEmailOrPhone checks if an account with the given email or phone exists.
	ExistsByEmailOrPhone(ctx context.Context, email, phone string) (bool, error)

	// UpdatePassword updates the hashed password for a user.
	UpdatePassword(ctx context.Context, userID, hashedPassword string) error

	// SetPhoneVerified marks a user's phone as verified.
	SetPhoneVerified(ctx context.Context, phone string) error
}

// ── Implementation ───────────────────────────────────────────────────────────

// AuthRepository implements AuthRepositoryInterface using PostgreSQL.
type AuthRepository struct {
	db    *sql.DB
	redis interface{} // TODO: Replace with Redis client type
}

// NewAuthRepository creates a new AuthRepository.
func NewAuthRepository(db *sql.DB, redis interface{}) *AuthRepository {
	return &AuthRepository{db: db, redis: redis}
}

func (r *AuthRepository) CreateCredential(ctx context.Context, cred *Credential) (*Credential, error) {
	query := `
		INSERT INTO credentials (email, phone, password_hash, role, status, email_verified, phone_verified)
		VALUES ($1, $2, $3, $4, 'active', false, false)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRowContext(ctx, query,
		cred.Email, cred.Phone, cred.PasswordHash, cred.Role,
	).Scan(&cred.ID, &cred.CreatedAt, &cred.UpdatedAt)

	if err != nil {
		return nil, err
	}

	cred.Status = "active"
	cred.PhoneVerified = false
	return cred, nil
}

func (r *AuthRepository) FindByID(ctx context.Context, id string) (*Credential, error) {
	query := `
		SELECT id, email, phone, password_hash, role, status, email_verified, phone_verified, created_at, updated_at
		FROM credentials WHERE id = $1
	`

	cred := &Credential{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&cred.ID, &cred.Email, &cred.Phone, &cred.PasswordHash,
		&cred.Role, &cred.Status, &cred.EmailVerified, &cred.PhoneVerified,
		&cred.CreatedAt, &cred.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return cred, nil
}

func (r *AuthRepository) FindByIdentifier(ctx context.Context, identifier string) (*Credential, error) {
	query := `
		SELECT id, email, phone, password_hash, role, status, email_verified, phone_verified, created_at, updated_at
		FROM credentials WHERE email = $1 OR phone = $1
	`

	cred := &Credential{}
	err := r.db.QueryRowContext(ctx, query, identifier).Scan(
		&cred.ID, &cred.Email, &cred.Phone, &cred.PasswordHash,
		&cred.Role, &cred.Status, &cred.EmailVerified, &cred.PhoneVerified,
		&cred.CreatedAt, &cred.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return cred, nil
}

func (r *AuthRepository) ExistsByEmailOrPhone(ctx context.Context, email, phone string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM credentials WHERE email = $1 OR phone = $2)`

	var exists bool
	err := r.db.QueryRowContext(ctx, query, email, phone).Scan(&exists)
	return exists, err
}

func (r *AuthRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	query := `UPDATE credentials SET password_hash = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, passwordHash, userID)
	return err
}

func (r *AuthRepository) SetPhoneVerified(ctx context.Context, phone string) error {
	query := `UPDATE credentials SET phone_verified = true, updated_at = NOW() WHERE phone = $1`
	_, err := r.db.ExecContext(ctx, query, phone)
	return err
}
