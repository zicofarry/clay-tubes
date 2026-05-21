//go:build unit

package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zicofarry/clay-app/backend/services/security-service/internal/repository"
)

func newMockRepo(t *testing.T) (*repository.SecurityRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return repository.NewSecurityRepository(db, nil), mock
}

// ── InsertLoginAttempt ────────────────────────────────────────────────────────

func TestRepo_InsertLoginAttempt(t *testing.T) {
	repo, mock := newMockRepo(t)
	ctx := context.Background()

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO login_attempts`).
		WithArgs("user-1", "1.2.3.4",
			sql.NullString{String: "ClayApp/1.0", Valid: true},
			false,
			sql.NullString{String: "wrong_password", Valid: true},
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
			AddRow("attempt-1", now))

	a := &repository.LoginAttempt{
		UserID:        "user-1",
		IPAddress:     "1.2.3.4",
		UserAgent:     sql.NullString{String: "ClayApp/1.0", Valid: true},
		Success:       false,
		FailureReason: sql.NullString{String: "wrong_password", Valid: true},
	}
	got, err := repo.InsertLoginAttempt(ctx, a)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "attempt-1" {
		t.Errorf("want attempt-1, got %s", got.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ── CountFailedLoginsInWindow ─────────────────────────────────────────────────

func TestRepo_CountFailedLoginsInWindow(t *testing.T) {
	repo, mock := newMockRepo(t)
	ctx := context.Background()

	since := time.Now().Add(-15 * time.Minute)
	mock.ExpectQuery(`SELECT COUNT\(1\) FROM login_attempts`).
		WithArgs("user-1", since).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	count, err := repo.CountFailedLoginsInWindow(ctx, "user-1", since)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("want 3, got %d", count)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ── InsertFraudFlag ───────────────────────────────────────────────────────────

func TestRepo_InsertFraudFlag(t *testing.T) {
	repo, mock := newMockRepo(t)
	ctx := context.Background()

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO fraud_flags`).
		WithArgs(
			"user-1", "suspicious_login", "medium",
			sql.NullString{String: "5 failed logins", Valid: true},
			sql.NullString{String: "auto_rule", Valid: true},
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "resolved", "created_at"}).
			AddRow("flag-1", false, now))

	f := &repository.FraudFlag{
		UserID:      "user-1",
		FlagType:    "suspicious_login",
		Severity:    "medium",
		Description: sql.NullString{String: "5 failed logins", Valid: true},
		Source:      sql.NullString{String: "auto_rule", Valid: true},
	}
	got, err := repo.InsertFraudFlag(ctx, f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "flag-1" {
		t.Errorf("want flag-1, got %s", got.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ── GetFraudFlagByID ──────────────────────────────────────────────────────────

func TestRepo_GetFraudFlagByID_Found(t *testing.T) {
	repo, mock := newMockRepo(t)
	ctx := context.Background()

	now := time.Now()
	cols := []string{"id", "user_id", "flag_type", "severity", "description", "source",
		"resolved", "resolved_by", "resolved_at", "resolution_note", "created_at"}
	mock.ExpectQuery(`SELECT .+ FROM fraud_flags WHERE id`).
		WithArgs("flag-1").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			"flag-1", "user-1", "suspicious_login", "medium",
			sql.NullString{Valid: false}, sql.NullString{Valid: false},
			false, sql.NullString{Valid: false}, sql.NullTime{Valid: false},
			sql.NullString{Valid: false}, now,
		))

	got, err := repo.GetFraudFlagByID(ctx, "flag-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "flag-1" {
		t.Errorf("want flag-1, got %s", got.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRepo_GetFraudFlagByID_NotFound(t *testing.T) {
	repo, mock := newMockRepo(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT .+ FROM fraud_flags WHERE id`).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetFraudFlagByID(ctx, "missing")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ── InsertIPBlock ─────────────────────────────────────────────────────────────

func TestRepo_InsertIPBlock(t *testing.T) {
	repo, mock := newMockRepo(t)
	ctx := context.Background()

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO ip_blacklist`).
		WithArgs(
			"10.0.0.1",
			sql.NullString{String: "brute-force", Valid: true},
			sql.NullString{String: "admin-1", Valid: true},
			sql.NullTime{Valid: false},
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "is_active", "created_at"}).
			AddRow("block-1", true, now))

	e := &repository.IPBlacklistEntry{
		IPAddress: "10.0.0.1",
		Reason:    sql.NullString{String: "brute-force", Valid: true},
		BlockedBy: sql.NullString{String: "admin-1", Valid: true},
		ExpiresAt: sql.NullTime{Valid: false},
	}
	got, err := repo.InsertIPBlock(ctx, e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "block-1" {
		t.Errorf("want block-1, got %s", got.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ── DeactivateIPBlock ─────────────────────────────────────────────────────────

func TestRepo_DeactivateIPBlock_Success(t *testing.T) {
	repo, mock := newMockRepo(t)
	ctx := context.Background()

	mock.ExpectExec(`UPDATE ip_blacklist SET is_active = false`).
		WithArgs("block-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.DeactivateIPBlock(ctx, "block-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRepo_DeactivateIPBlock_NotFound(t *testing.T) {
	repo, mock := newMockRepo(t)
	ctx := context.Background()

	mock.ExpectExec(`UPDATE ip_blacklist SET is_active = false`).
		WithArgs("missing").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.DeactivateIPBlock(ctx, "missing")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ── CountActiveFlagsForUser ───────────────────────────────────────────────────

func TestRepo_CountActiveFlagsForUser(t *testing.T) {
	repo, mock := newMockRepo(t)
	ctx := context.Background()

	mock.ExpectQuery(`SELECT COUNT\(1\) FROM fraud_flags`).
		WithArgs("user-1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	count, err := repo.CountActiveFlagsForUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("want 2, got %d", count)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
