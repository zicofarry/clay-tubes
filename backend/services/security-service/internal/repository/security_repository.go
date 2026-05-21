// Package repository implements the data access layer for the Security Service.
package repository

import (
	"context"
	"database/sql"
	"time"
)

// ── Models ────────────────────────────────────────────────────────────────────

// LoginAttempt represents a row in the `login_attempts` table.
type LoginAttempt struct {
	ID            string         `json:"id"`
	UserID        string         `json:"user_id"`
	IPAddress     string         `json:"ip_address"`
	UserAgent     sql.NullString `json:"user_agent,omitempty"`
	Success       bool           `json:"success"`
	FailureReason sql.NullString `json:"failure_reason,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

// FraudFlag represents a row in the `fraud_flags` table.
type FraudFlag struct {
	ID             string         `json:"id"`
	UserID         string         `json:"user_id"`
	FlagType       string         `json:"flag_type"`
	Severity       string         `json:"severity"`
	Description    sql.NullString `json:"description,omitempty"`
	Source         sql.NullString `json:"source,omitempty"`
	Resolved       bool           `json:"resolved"`
	ResolvedBy     sql.NullString `json:"resolved_by,omitempty"`
	ResolvedAt     sql.NullTime   `json:"resolved_at,omitempty"`
	ResolutionNote sql.NullString `json:"resolution_note,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
}

// IPBlacklistEntry represents a row in the `ip_blacklist` table.
type IPBlacklistEntry struct {
	ID        string         `json:"id"`
	IPAddress string         `json:"ip_address"`
	Reason    sql.NullString `json:"reason,omitempty"`
	BlockedBy sql.NullString `json:"blocked_by,omitempty"`
	ExpiresAt sql.NullTime   `json:"expires_at,omitempty"`
	IsActive  bool           `json:"is_active"`
	CreatedAt time.Time      `json:"created_at"`
}

// LoginAttemptFilter holds filter options for ListLoginAttempts.
type LoginAttemptFilter struct {
	UserID    string
	IPAddress string
	Success   *bool
	From      *time.Time
	To        *time.Time
	Limit     int
	Offset    int
}

// FraudFlagFilter holds filter options for ListFraudFlags.
type FraudFlagFilter struct {
	UserID   string
	Severity string
	FlagType string
	Resolved *bool
	Limit    int
	Offset   int
}

// IPBlacklistFilter holds filter options for ListBlockedIPs.
type IPBlacklistFilter struct {
	Query      string
	ActiveOnly bool
	Limit      int
	Offset     int
}

// ── Interface ─────────────────────────────────────────────────────────────────

// SecurityRepositoryInterface defines the contract for security data access.
//
//go:generate mockgen -source=security_repository.go -destination=../../mocks/repomock/mock_security_repository.go -package=repomock
type SecurityRepositoryInterface interface {
	// Login attempts
	InsertLoginAttempt(ctx context.Context, a *LoginAttempt) (*LoginAttempt, error)
	ListLoginAttempts(ctx context.Context, f LoginAttemptFilter) ([]*LoginAttempt, int, error)
	CountFailedLoginsInWindow(ctx context.Context, userID string, since time.Time) (int, error)

	// Fraud flags
	InsertFraudFlag(ctx context.Context, f *FraudFlag) (*FraudFlag, error)
	GetFraudFlagByID(ctx context.Context, flagID string) (*FraudFlag, error)
	ListFraudFlags(ctx context.Context, f FraudFlagFilter) ([]*FraudFlag, int, error)
	ResolveFraudFlag(ctx context.Context, flagID, resolvedBy, note string, resolvedAt time.Time) (*FraudFlag, error)
	CountActiveFlagsForUser(ctx context.Context, userID string) (int, error)
	GetMaxSeverityForUser(ctx context.Context, userID string) (string, error)

	// IP blacklist
	InsertIPBlock(ctx context.Context, e *IPBlacklistEntry) (*IPBlacklistEntry, error)
	GetIPBlockByID(ctx context.Context, blockID string) (*IPBlacklistEntry, error)
	GetActiveIPBlock(ctx context.Context, ipAddress string) (*IPBlacklistEntry, error)
	ListBlockedIPs(ctx context.Context, f IPBlacklistFilter) ([]*IPBlacklistEntry, int, error)
	DeactivateIPBlock(ctx context.Context, blockID string) error
}

// ── Implementation ────────────────────────────────────────────────────────────

// SecurityRepository implements SecurityRepositoryInterface using PostgreSQL.
type SecurityRepository struct {
	db    *sql.DB
	redis interface{}
}

// NewSecurityRepository creates a new SecurityRepository.
func NewSecurityRepository(db *sql.DB, redis interface{}) *SecurityRepository {
	return &SecurityRepository{db: db, redis: redis}
}

// ── Login Attempts ────────────────────────────────────────────────────────────

func (r *SecurityRepository) InsertLoginAttempt(ctx context.Context, a *LoginAttempt) (*LoginAttempt, error) {
	const q = `
		INSERT INTO login_attempts (user_id, ip_address, user_agent, success, failure_reason)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, q,
		a.UserID, a.IPAddress, a.UserAgent, a.Success, a.FailureReason,
	).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *SecurityRepository) ListLoginAttempts(ctx context.Context, f LoginAttemptFilter) ([]*LoginAttempt, int, error) {
	const countQ = `
		SELECT COUNT(1) FROM login_attempts
		WHERE ($1 = '' OR user_id::text = $1)
		  AND ($2 = '' OR ip_address = $2)
		  AND ($3::boolean IS NULL OR success = $3)
		  AND ($4::timestamptz IS NULL OR created_at >= $4)
		  AND ($5::timestamptz IS NULL OR created_at <= $5)
	`
	var total int
	if err := r.db.QueryRowContext(ctx, countQ,
		f.UserID, f.IPAddress, f.Success, f.From, f.To,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	const q = `
		SELECT id, user_id, ip_address, user_agent, success, failure_reason, created_at
		FROM login_attempts
		WHERE ($1 = '' OR user_id::text = $1)
		  AND ($2 = '' OR ip_address = $2)
		  AND ($3::boolean IS NULL OR success = $3)
		  AND ($4::timestamptz IS NULL OR created_at >= $4)
		  AND ($5::timestamptz IS NULL OR created_at <= $5)
		ORDER BY created_at DESC
		LIMIT $6 OFFSET $7
	`
	rows, err := r.db.QueryContext(ctx, q,
		f.UserID, f.IPAddress, f.Success, f.From, f.To, f.Limit, f.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*LoginAttempt
	for rows.Next() {
		a := &LoginAttempt{}
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.IPAddress, &a.UserAgent,
			&a.Success, &a.FailureReason, &a.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

func (r *SecurityRepository) CountFailedLoginsInWindow(ctx context.Context, userID string, since time.Time) (int, error) {
	const q = `
		SELECT COUNT(1) FROM login_attempts
		WHERE user_id::text = $1
		  AND success = false
		  AND created_at >= $2
	`
	var count int
	err := r.db.QueryRowContext(ctx, q, userID, since).Scan(&count)
	return count, err
}

// ── Fraud Flags ───────────────────────────────────────────────────────────────

func (r *SecurityRepository) InsertFraudFlag(ctx context.Context, f *FraudFlag) (*FraudFlag, error) {
	const q = `
		INSERT INTO fraud_flags (user_id, flag_type, severity, description, source)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, resolved, created_at
	`
	err := r.db.QueryRowContext(ctx, q,
		f.UserID, f.FlagType, f.Severity, f.Description, f.Source,
	).Scan(&f.ID, &f.Resolved, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (r *SecurityRepository) GetFraudFlagByID(ctx context.Context, flagID string) (*FraudFlag, error) {
	const q = `
		SELECT id, user_id, flag_type, severity, description, source,
		       resolved, resolved_by, resolved_at, resolution_note, created_at
		FROM fraud_flags WHERE id = $1
	`
	f := &FraudFlag{}
	err := r.db.QueryRowContext(ctx, q, flagID).Scan(
		&f.ID, &f.UserID, &f.FlagType, &f.Severity, &f.Description, &f.Source,
		&f.Resolved, &f.ResolvedBy, &f.ResolvedAt, &f.ResolutionNote, &f.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (r *SecurityRepository) ListFraudFlags(ctx context.Context, f FraudFlagFilter) ([]*FraudFlag, int, error) {
	const countQ = `
		SELECT COUNT(1) FROM fraud_flags
		WHERE ($1 = '' OR user_id::text = $1)
		  AND ($2 = '' OR severity = $2)
		  AND ($3 = '' OR flag_type = $3)
		  AND ($4::boolean IS NULL OR resolved = $4)
	`
	var total int
	if err := r.db.QueryRowContext(ctx, countQ,
		f.UserID, f.Severity, f.FlagType, f.Resolved,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	const q = `
		SELECT id, user_id, flag_type, severity, description, source,
		       resolved, resolved_by, resolved_at, resolution_note, created_at
		FROM fraud_flags
		WHERE ($1 = '' OR user_id::text = $1)
		  AND ($2 = '' OR severity = $2)
		  AND ($3 = '' OR flag_type = $3)
		  AND ($4::boolean IS NULL OR resolved = $4)
		ORDER BY created_at DESC
		LIMIT $5 OFFSET $6
	`
	rows, err := r.db.QueryContext(ctx, q,
		f.UserID, f.Severity, f.FlagType, f.Resolved, f.Limit, f.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*FraudFlag
	for rows.Next() {
		ff := &FraudFlag{}
		if err := rows.Scan(
			&ff.ID, &ff.UserID, &ff.FlagType, &ff.Severity, &ff.Description, &ff.Source,
			&ff.Resolved, &ff.ResolvedBy, &ff.ResolvedAt, &ff.ResolutionNote, &ff.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, ff)
	}
	return out, total, rows.Err()
}

func (r *SecurityRepository) ResolveFraudFlag(ctx context.Context, flagID, resolvedBy, note string, resolvedAt time.Time) (*FraudFlag, error) {
	const q = `
		UPDATE fraud_flags
		SET resolved = true, resolved_by = $1, resolved_at = $2, resolution_note = $3
		WHERE id = $4 AND resolved = false
		RETURNING id, user_id, flag_type, severity, description, source,
		          resolved, resolved_by, resolved_at, resolution_note, created_at
	`
	f := &FraudFlag{}
	err := r.db.QueryRowContext(ctx, q, resolvedBy, resolvedAt, note, flagID).Scan(
		&f.ID, &f.UserID, &f.FlagType, &f.Severity, &f.Description, &f.Source,
		&f.Resolved, &f.ResolvedBy, &f.ResolvedAt, &f.ResolutionNote, &f.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (r *SecurityRepository) CountActiveFlagsForUser(ctx context.Context, userID string) (int, error) {
	const q = `SELECT COUNT(1) FROM fraud_flags WHERE user_id::text = $1 AND resolved = false`
	var count int
	err := r.db.QueryRowContext(ctx, q, userID).Scan(&count)
	return count, err
}

func (r *SecurityRepository) GetMaxSeverityForUser(ctx context.Context, userID string) (string, error) {
	const q = `
		SELECT COALESCE(
			(SELECT severity FROM fraud_flags
			 WHERE user_id::text = $1 AND resolved = false
			 ORDER BY CASE severity
			   WHEN 'critical' THEN 4
			   WHEN 'high'     THEN 3
			   WHEN 'medium'   THEN 2
			   WHEN 'low'      THEN 1
			   ELSE 0
			 END DESC
			 LIMIT 1),
		'')
	`
	var severity string
	err := r.db.QueryRowContext(ctx, q, userID).Scan(&severity)
	return severity, err
}

// ── IP Blacklist ──────────────────────────────────────────────────────────────

func (r *SecurityRepository) InsertIPBlock(ctx context.Context, e *IPBlacklistEntry) (*IPBlacklistEntry, error) {
	const q = `
		INSERT INTO ip_blacklist (ip_address, reason, blocked_by, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, is_active, created_at
	`
	err := r.db.QueryRowContext(ctx, q,
		e.IPAddress, e.Reason, e.BlockedBy, e.ExpiresAt,
	).Scan(&e.ID, &e.IsActive, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *SecurityRepository) GetIPBlockByID(ctx context.Context, blockID string) (*IPBlacklistEntry, error) {
	const q = `
		SELECT id, ip_address, reason, blocked_by, expires_at, is_active, created_at
		FROM ip_blacklist WHERE id = $1
	`
	e := &IPBlacklistEntry{}
	err := r.db.QueryRowContext(ctx, q, blockID).Scan(
		&e.ID, &e.IPAddress, &e.Reason, &e.BlockedBy, &e.ExpiresAt, &e.IsActive, &e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *SecurityRepository) GetActiveIPBlock(ctx context.Context, ipAddress string) (*IPBlacklistEntry, error) {
	const q = `
		SELECT id, ip_address, reason, blocked_by, expires_at, is_active, created_at
		FROM ip_blacklist
		WHERE ip_address = $1
		  AND is_active = true
		  AND (expires_at IS NULL OR expires_at > NOW())
		LIMIT 1
	`
	e := &IPBlacklistEntry{}
	err := r.db.QueryRowContext(ctx, q, ipAddress).Scan(
		&e.ID, &e.IPAddress, &e.Reason, &e.BlockedBy, &e.ExpiresAt, &e.IsActive, &e.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (r *SecurityRepository) ListBlockedIPs(ctx context.Context, f IPBlacklistFilter) ([]*IPBlacklistEntry, int, error) {
	const countQ = `
		SELECT COUNT(1) FROM ip_blacklist
		WHERE ($1 = '' OR ip_address LIKE $1 || '%')
		  AND (NOT $2 OR (is_active = true AND (expires_at IS NULL OR expires_at > NOW())))
	`
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, f.Query, f.ActiveOnly).Scan(&total); err != nil {
		return nil, 0, err
	}

	const q = `
		SELECT id, ip_address, reason, blocked_by, expires_at, is_active, created_at
		FROM ip_blacklist
		WHERE ($1 = '' OR ip_address LIKE $1 || '%')
		  AND (NOT $2 OR (is_active = true AND (expires_at IS NULL OR expires_at > NOW())))
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	rows, err := r.db.QueryContext(ctx, q, f.Query, f.ActiveOnly, f.Limit, f.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*IPBlacklistEntry
	for rows.Next() {
		e := &IPBlacklistEntry{}
		if err := rows.Scan(
			&e.ID, &e.IPAddress, &e.Reason, &e.BlockedBy, &e.ExpiresAt, &e.IsActive, &e.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func (r *SecurityRepository) DeactivateIPBlock(ctx context.Context, blockID string) error {
	const q = `UPDATE ip_blacklist SET is_active = false WHERE id = $1`
	res, err := r.db.ExecContext(ctx, q, blockID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
