//go:build functional

// Package functional contains end-to-end integration tests that connect
// directly to a real PostgreSQL instance provisioned via docker-compose.
//
// These tests will FAIL when the docker compose stack is not running
// (no DB to connect to) and PASS once `docker compose up -d` has provisioned
// PostgreSQL on localhost:5450.
//
// Run with:
//
//	docker compose up -d
//	go test -tags=functional -v ./test/functional/...
package functional

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/zicofarry/clay-app/backend/services/security-service/internal/repository"
)

// dsn returns the test DSN, allowing CI override via TEST_DATABASE_URL.
func dsn() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://clay_user:clay_password@localhost:5450/security_db?sslmode=disable"
}

// setupTestDB connects to the docker-compose PostgreSQL, applies the schema,
// truncates all tables, and returns a clean *sql.DB.
// Fails the test immediately if the DB is unreachable.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("postgres", dsn())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		if err = db.PingContext(context.Background()); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("cannot reach PostgreSQL at %s — did you run `docker compose up -d`? last err: %v", dsn(), err)
		}
		time.Sleep(500 * time.Millisecond)
	}

	if _, err := db.Exec(schemaDDL); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	if _, err := db.Exec(`TRUNCATE login_attempts, fraud_flags, ip_blacklist`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

const schemaDDL = `
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS login_attempts (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL,
    ip_address     VARCHAR(45) NOT NULL,
    user_agent     VARCHAR(500),
    success        BOOLEAN NOT NULL,
    failure_reason VARCHAR(100),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_login_attempts_user_id      ON login_attempts(user_id);
CREATE INDEX IF NOT EXISTS idx_login_attempts_ip_address   ON login_attempts(ip_address);
CREATE INDEX IF NOT EXISTS idx_login_attempts_user_created ON login_attempts(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_attempts_ip_created   ON login_attempts(ip_address, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_attempts_success      ON login_attempts(success, created_at DESC);

CREATE TABLE IF NOT EXISTS fraud_flags (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    flag_type       VARCHAR(50) NOT NULL,
    severity        VARCHAR(20) NOT NULL CHECK (severity IN ('low','medium','high','critical')),
    description     TEXT,
    source          VARCHAR(50),
    resolved        BOOLEAN NOT NULL DEFAULT FALSE,
    resolved_by     UUID,
    resolved_at     TIMESTAMPTZ,
    resolution_note TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fraud_flags_user_id           ON fraud_flags(user_id);
CREATE INDEX IF NOT EXISTS idx_fraud_flags_user_resolved     ON fraud_flags(user_id, resolved);
CREATE INDEX IF NOT EXISTS idx_fraud_flags_severity_resolved ON fraud_flags(severity, resolved);
CREATE INDEX IF NOT EXISTS idx_fraud_flags_flag_type         ON fraud_flags(flag_type, created_at DESC);

CREATE TABLE IF NOT EXISTS ip_blacklist (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ip_address  VARCHAR(45) NOT NULL UNIQUE,
    reason      TEXT,
    blocked_by  UUID,
    expires_at  TIMESTAMPTZ,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ip_blacklist_ip_address ON ip_blacklist(ip_address);
CREATE INDEX IF NOT EXISTS idx_ip_blacklist_is_active  ON ip_blacklist(is_active, expires_at);
CREATE INDEX IF NOT EXISTS idx_ip_blacklist_created_at ON ip_blacklist(created_at DESC);
`

// Deterministic UUIDs for test actors.
const (
	userUUID   = "00000000-0000-0000-0000-000000000001"
	user2UUID  = "00000000-0000-0000-0000-000000000002"
	adminUUID  = "00000000-0000-0000-0000-000000000099"
)

// ── E2E: login attempt recording, filtering, and failure window counting ──────

func TestSecurityRepository_LoginAttemptE2E(t *testing.T) {
	t.Log("Starting login-attempt E2E (DB integration)…")

	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewSecurityRepository(db, nil)
	ctx := context.Background()

	// 1. Insert a mix of successful and failed attempts for two users.
	attempts := []struct {
		userID string
		ip     string
		ua     string
		ok     bool
		reason string
	}{
		{userUUID, "1.2.3.4", "ClayApp/2.0", true, ""},
		{userUUID, "1.2.3.4", "ClayApp/2.0", false, "wrong_password"},
		{userUUID, "5.6.7.8", "ClayApp/2.0", false, "wrong_password"},
		{userUUID, "5.6.7.8", "ClayApp/2.0", false, "account_locked"},
		{user2UUID, "9.9.9.9", "Mozilla/5.0", true, ""},
	}
	for _, a := range attempts {
		_, err := repo.InsertLoginAttempt(ctx, &repository.LoginAttempt{
			UserID:        a.userID,
			IPAddress:     a.ip,
			UserAgent:     sql.NullString{String: a.ua, Valid: a.ua != ""},
			Success:       a.ok,
			FailureReason: sql.NullString{String: a.reason, Valid: a.reason != ""},
		})
		if err != nil {
			t.Fatalf("insert attempt: %v", err)
		}
	}
	t.Log("Inserted 5 login attempts (4 for user1, 1 for user2)")

	// 2. List all attempts — should return 5 total.
	rows, total, err := repo.ListLoginAttempts(ctx, repository.LoginAttemptFilter{Limit: 20})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 5 {
		t.Errorf("want total=5, got %d", total)
	}
	t.Logf("Listed all attempts: total=%d returned=%d", total, len(rows))

	// 3. Filter by user_id — should return 4 for user1.
	rows, total, err = repo.ListLoginAttempts(ctx, repository.LoginAttemptFilter{
		UserID: userUUID, Limit: 20,
	})
	if err != nil {
		t.Fatalf("list by user: %v", err)
	}
	if total != 4 {
		t.Errorf("want total=4 for user1, got %d", total)
	}
	for _, r := range rows {
		if r.UserID != userUUID {
			t.Errorf("got attempt for wrong user: %s", r.UserID)
		}
	}

	// 4. Filter by ip_address.
	rows, total, err = repo.ListLoginAttempts(ctx, repository.LoginAttemptFilter{
		IPAddress: "5.6.7.8", Limit: 20,
	})
	if err != nil {
		t.Fatalf("list by ip: %v", err)
	}
	if total != 2 {
		t.Errorf("want total=2 for ip 5.6.7.8, got %d", total)
	}

	// 5. Filter success=false — should return 3 failed attempts.
	f := false
	rows, total, err = repo.ListLoginAttempts(ctx, repository.LoginAttemptFilter{
		Success: &f, Limit: 20,
	})
	if err != nil {
		t.Fatalf("list by success=false: %v", err)
	}
	if total != 3 {
		t.Errorf("want total=3 failed attempts, got %d", total)
	}

	// 6. Filter success=true — should return 2 successful attempts.
	tr := true
	rows, total, err = repo.ListLoginAttempts(ctx, repository.LoginAttemptFilter{
		Success: &tr, Limit: 20,
	})
	if err != nil {
		t.Fatalf("list by success=true: %v", err)
	}
	if total != 2 {
		t.Errorf("want total=2 successful attempts, got %d", total)
	}

	// 7. Pagination: limit=2, offset=0 returns 2; offset=4 returns 1 for user1.
	rows, total, err = repo.ListLoginAttempts(ctx, repository.LoginAttemptFilter{
		UserID: userUUID, Limit: 2, Offset: 0,
	})
	if err != nil {
		t.Fatalf("paginate page1: %v", err)
	}
	if total != 4 || len(rows) != 2 {
		t.Errorf("page1: want total=4 rows=2, got total=%d rows=%d", total, len(rows))
	}

	rows, total, err = repo.ListLoginAttempts(ctx, repository.LoginAttemptFilter{
		UserID: userUUID, Limit: 2, Offset: 2,
	})
	if err != nil {
		t.Fatalf("paginate page2: %v", err)
	}
	if total != 4 || len(rows) != 2 {
		t.Errorf("page2: want total=4 rows=2, got total=%d rows=%d", total, len(rows))
	}

	// 8. CountFailedLoginsInWindow: 3 failed for user1 within last 15 min.
	since := time.Now().Add(-15 * time.Minute)
	count, err := repo.CountFailedLoginsInWindow(ctx, userUUID, since)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 3 {
		t.Errorf("want count=3 failed in window, got %d", count)
	}
	t.Logf("Failed logins in 15-min window: %d", count)

	// 9. Old window (before inserts) should return 0.
	beforeAll := time.Now().Add(time.Minute)
	count, err = repo.CountFailedLoginsInWindow(ctx, userUUID, beforeAll)
	if err != nil {
		t.Fatalf("count future window: %v", err)
	}
	if count != 0 {
		t.Errorf("future since should return 0, got %d", count)
	}
}

// ── E2E: fraud flag full lifecycle ────────────────────────────────────────────

func TestSecurityRepository_FraudFlagLifecycleE2E(t *testing.T) {
	t.Log("Starting fraud-flag lifecycle E2E…")

	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewSecurityRepository(db, nil)
	ctx := context.Background()

	// 1. Insert flags for two users with varying severities.
	flags := []struct {
		userID   string
		flagType string
		severity string
		source   string
	}{
		{userUUID, "suspicious_login", "medium", "auto_rule"},
		{userUUID, "chargeback", "high", "manual"},
		{userUUID, "account_sharing", "low", "manual"},
		{user2UUID, "account_takeover", "critical", "manual"},
	}
	createdIDs := make([]string, 0, len(flags))
	for _, f := range flags {
		ff, err := repo.InsertFraudFlag(ctx, &repository.FraudFlag{
			UserID:      f.userID,
			FlagType:    f.flagType,
			Severity:    f.severity,
			Description: sql.NullString{String: "E2E test flag", Valid: true},
			Source:      sql.NullString{String: f.source, Valid: true},
		})
		if err != nil {
			t.Fatalf("insert flag (%s/%s): %v", f.userID, f.severity, err)
		}
		createdIDs = append(createdIDs, ff.ID)
		t.Logf("Inserted flag id=%s user=%s severity=%s", ff.ID, f.userID, f.severity)
	}

	// 2. GetFraudFlagByID — retrieve the first flag and verify fields.
	got, err := repo.GetFraudFlagByID(ctx, createdIDs[0])
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.ID != createdIDs[0] {
		t.Errorf("want id=%s, got %s", createdIDs[0], got.ID)
	}
	if got.Severity != "medium" {
		t.Errorf("want severity=medium, got %s", got.Severity)
	}
	if got.Resolved {
		t.Error("newly inserted flag must not be resolved")
	}

	// 3. GetFraudFlagByID — non-existent ID must return sql.ErrNoRows.
	_, err = repo.GetFraudFlagByID(ctx, "00000000-0000-0000-0000-000000000000")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows for missing flag, got %v", err)
	}

	// 4. ListFraudFlags — total across all users must be 4.
	rows, total, err := repo.ListFraudFlags(ctx, repository.FraudFlagFilter{Limit: 20})
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 4 {
		t.Errorf("want total=4, got %d", total)
	}
	t.Logf("Listed all flags: total=%d returned=%d", total, len(rows))

	// 5. Filter by user_id — user1 has 3 flags.
	rows, total, err = repo.ListFraudFlags(ctx, repository.FraudFlagFilter{
		UserID: userUUID, Limit: 20,
	})
	if err != nil {
		t.Fatalf("list by user: %v", err)
	}
	if total != 3 {
		t.Errorf("want total=3 for user1, got %d", total)
	}

	// 6. Filter by severity=high.
	rows, total, err = repo.ListFraudFlags(ctx, repository.FraudFlagFilter{
		Severity: "high", Limit: 20,
	})
	if err != nil {
		t.Fatalf("list by severity: %v", err)
	}
	if total != 1 {
		t.Errorf("want total=1 for severity=high, got %d", total)
	}
	if rows[0].Severity != "high" {
		t.Errorf("want severity=high, got %s", rows[0].Severity)
	}

	// 7. Filter by flag_type.
	rows, total, err = repo.ListFraudFlags(ctx, repository.FraudFlagFilter{
		FlagType: "chargeback", Limit: 20,
	})
	if err != nil {
		t.Fatalf("list by flag_type: %v", err)
	}
	if total != 1 {
		t.Errorf("want total=1 for flag_type=chargeback, got %d", total)
	}

	// 8. CountActiveFlagsForUser — user1 has 3 active flags, user2 has 1.
	activeUser1, err := repo.CountActiveFlagsForUser(ctx, userUUID)
	if err != nil {
		t.Fatalf("count active user1: %v", err)
	}
	if activeUser1 != 3 {
		t.Errorf("want active=3 for user1, got %d", activeUser1)
	}

	activeUser2, err := repo.CountActiveFlagsForUser(ctx, user2UUID)
	if err != nil {
		t.Fatalf("count active user2: %v", err)
	}
	if activeUser2 != 1 {
		t.Errorf("want active=1 for user2, got %d", activeUser2)
	}

	// 9. GetMaxSeverityForUser — user1 max = "high", user2 max = "critical".
	maxSev1, err := repo.GetMaxSeverityForUser(ctx, userUUID)
	if err != nil {
		t.Fatalf("max severity user1: %v", err)
	}
	if maxSev1 != "high" {
		t.Errorf("want max_severity=high for user1, got %s", maxSev1)
	}

	maxSev2, err := repo.GetMaxSeverityForUser(ctx, user2UUID)
	if err != nil {
		t.Fatalf("max severity user2: %v", err)
	}
	if maxSev2 != "critical" {
		t.Errorf("want max_severity=critical for user2, got %s", maxSev2)
	}
	t.Logf("Max severity — user1=%s user2=%s", maxSev1, maxSev2)

	// 10. ResolveFraudFlag — resolve the "high" flag for user1.
	highFlagID := createdIDs[1]
	resolved, err := repo.ResolveFraudFlag(ctx, highFlagID, adminUUID, "false positive — verified", time.Now().UTC())
	if err != nil {
		t.Fatalf("resolve flag: %v", err)
	}
	if !resolved.Resolved {
		t.Error("want resolved=true after resolution")
	}
	if !resolved.ResolvedBy.Valid || resolved.ResolvedBy.String != adminUUID {
		t.Errorf("want resolved_by=%s, got %+v", adminUUID, resolved.ResolvedBy)
	}
	if !resolved.ResolvedAt.Valid {
		t.Error("resolved_at must be set")
	}
	if !resolved.ResolutionNote.Valid || resolved.ResolutionNote.String != "false positive — verified" {
		t.Errorf("unexpected resolution_note: %+v", resolved.ResolutionNote)
	}
	t.Logf("Resolved flag id=%s by admin=%s", highFlagID, adminUUID)

	// 11. Attempting to resolve the same flag again must return sql.ErrNoRows
	// (the WHERE resolved=false clause matches nothing).
	_, err = repo.ResolveFraudFlag(ctx, highFlagID, adminUUID, "again", time.Now().UTC())
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows on double-resolve, got %v", err)
	}

	// 12. After resolution — user1 active count drops to 2.
	activeAfter, err := repo.CountActiveFlagsForUser(ctx, userUUID)
	if err != nil {
		t.Fatalf("count after resolve: %v", err)
	}
	if activeAfter != 2 {
		t.Errorf("want active=2 after resolving high flag, got %d", activeAfter)
	}

	// 13. GetMaxSeverityForUser after resolve — user1 max drops to "medium".
	maxAfter, err := repo.GetMaxSeverityForUser(ctx, userUUID)
	if err != nil {
		t.Fatalf("max severity after resolve: %v", err)
	}
	if maxAfter != "medium" {
		t.Errorf("want max_severity=medium after resolving high flag, got %s", maxAfter)
	}

	// 14. Filter resolved=true — should return 1 resolved flag.
	trueVal := true
	rows, total, err = repo.ListFraudFlags(ctx, repository.FraudFlagFilter{
		Resolved: &trueVal, Limit: 20,
	})
	if err != nil {
		t.Fatalf("list resolved: %v", err)
	}
	if total != 1 {
		t.Errorf("want total=1 resolved, got %d", total)
	}

	// 15. Filter resolved=false — should return 3 unresolved flags.
	falseVal := false
	rows, total, err = repo.ListFraudFlags(ctx, repository.FraudFlagFilter{
		Resolved: &falseVal, Limit: 20,
	})
	if err != nil {
		t.Fatalf("list unresolved: %v", err)
	}
	if total != 3 {
		t.Errorf("want total=3 unresolved, got %d", total)
	}
	_ = rows
}

// ── E2E: IP blacklist block, validate, and unblock flow ───────────────────────

func TestSecurityRepository_IPBlacklistE2E(t *testing.T) {
	t.Log("Starting IP blacklist E2E…")

	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewSecurityRepository(db, nil)
	ctx := context.Background()

	// 1. Block several IPs.
	ips := []struct {
		addr   string
		reason string
	}{
		{"203.0.113.10", "brute-force attack"},
		{"203.0.113.20", "credential stuffing"},
		{"198.51.100.5", "port scanning"},
	}
	entries := make([]*repository.IPBlacklistEntry, 0, len(ips))
	for _, ip := range ips {
		e, err := repo.InsertIPBlock(ctx, &repository.IPBlacklistEntry{
			IPAddress: ip.addr,
			Reason:    sql.NullString{String: ip.reason, Valid: true},
			BlockedBy: sql.NullString{String: adminUUID, Valid: true},
		})
		if err != nil {
			t.Fatalf("block %s: %v", ip.addr, err)
		}
		if e.ID == "" {
			t.Errorf("expected non-empty ID for %s", ip.addr)
		}
		if !e.IsActive {
			t.Errorf("newly blocked IP %s must be active", ip.addr)
		}
		entries = append(entries, e)
		t.Logf("Blocked ip=%s id=%s", e.IPAddress, e.ID)
	}

	// 2. GetActiveIPBlock — all three IPs must be found.
	for _, ip := range ips {
		e, err := repo.GetActiveIPBlock(ctx, ip.addr)
		if err != nil {
			t.Fatalf("get active block for %s: %v", ip.addr, err)
		}
		if e.IPAddress != ip.addr {
			t.Errorf("want ip=%s, got %s", ip.addr, e.IPAddress)
		}
		if e.Reason.String != ip.reason {
			t.Errorf("want reason=%q for %s, got %q", ip.reason, ip.addr, e.Reason.String)
		}
	}

	// 3. GetActiveIPBlock — unknown IP must return sql.ErrNoRows.
	_, err := repo.GetActiveIPBlock(ctx, "1.1.1.1")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows for non-blocked IP, got %v", err)
	}

	// 4. GetIPBlockByID — look up by primary key.
	byID, err := repo.GetIPBlockByID(ctx, entries[0].ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if byID.IPAddress != entries[0].IPAddress {
		t.Errorf("want ip=%s, got %s", entries[0].IPAddress, byID.IPAddress)
	}

	// 5. ListBlockedIPs active_only=true — must return 3.
	rows, total, err := repo.ListBlockedIPs(ctx, repository.IPBlacklistFilter{
		ActiveOnly: true, Limit: 20,
	})
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if total != 3 {
		t.Errorf("want total=3 active, got %d", total)
	}
	t.Logf("Listed active IPs: total=%d returned=%d", total, len(rows))

	// 6. ListBlockedIPs with query prefix filter.
	rows, total, err = repo.ListBlockedIPs(ctx, repository.IPBlacklistFilter{
		Query: "203.0.113", ActiveOnly: true, Limit: 20,
	})
	if err != nil {
		t.Fatalf("list prefix: %v", err)
	}
	if total != 2 {
		t.Errorf("want total=2 for prefix 203.0.113, got %d", total)
	}

	// 7. ListBlockedIPs pagination.
	rows, total, err = repo.ListBlockedIPs(ctx, repository.IPBlacklistFilter{
		ActiveOnly: true, Limit: 2, Offset: 0,
	})
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if total != 3 || len(rows) != 2 {
		t.Errorf("page1: want total=3 rows=2, got total=%d rows=%d", total, len(rows))
	}

	// 8. DeactivateIPBlock — unblock the first IP.
	if err := repo.DeactivateIPBlock(ctx, entries[0].ID); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	t.Logf("Deactivated ip=%s", entries[0].IPAddress)

	// 9. After deactivation — GetActiveIPBlock must return sql.ErrNoRows.
	_, err = repo.GetActiveIPBlock(ctx, entries[0].IPAddress)
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows after deactivation, got %v", err)
	}

	// 10. ListBlockedIPs active_only=true — now only 2 remain active.
	_, total, err = repo.ListBlockedIPs(ctx, repository.IPBlacklistFilter{
		ActiveOnly: true, Limit: 20,
	})
	if err != nil {
		t.Fatalf("list after deactivate: %v", err)
	}
	if total != 2 {
		t.Errorf("want total=2 after deactivation, got %d", total)
	}

	// 11. ListBlockedIPs active_only=false — all 3 still present in table.
	_, total, err = repo.ListBlockedIPs(ctx, repository.IPBlacklistFilter{
		ActiveOnly: false, Limit: 20,
	})
	if err != nil {
		t.Fatalf("list all including inactive: %v", err)
	}
	if total != 3 {
		t.Errorf("want total=3 including inactive, got %d", total)
	}

	// 12. DeactivateIPBlock on missing ID must return sql.ErrNoRows.
	err = repo.DeactivateIPBlock(ctx, "00000000-0000-0000-0000-000000000000")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows for missing block id, got %v", err)
	}

	// 13. InsertIPBlock with expires_at set.
	expiry := time.Now().UTC().Add(24 * time.Hour)
	exp, err := repo.InsertIPBlock(ctx, &repository.IPBlacklistEntry{
		IPAddress: "192.0.2.1",
		Reason:    sql.NullString{String: "temporary ban", Valid: true},
		BlockedBy: sql.NullString{String: adminUUID, Valid: true},
		ExpiresAt: sql.NullTime{Time: expiry, Valid: true},
	})
	if err != nil {
		t.Fatalf("insert with expiry: %v", err)
	}
	if !exp.ExpiresAt.Valid {
		t.Error("expected expires_at to be set")
	}
	t.Logf("Blocked with expiry: ip=%s expires=%s", exp.IPAddress, exp.ExpiresAt.Time)
}

// ── E2E: user fraud summary — risk score inputs, severity, flag/login history ─

func TestSecurityRepository_UserFraudSummaryE2E(t *testing.T) {
	t.Log("Starting user fraud summary E2E…")

	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewSecurityRepository(db, nil)
	ctx := context.Background()

	// 1. Seed login attempts — 2 failed within 24h window.
	for _, ok := range []bool{true, false, false} {
		_, err := repo.InsertLoginAttempt(ctx, &repository.LoginAttempt{
			UserID: userUUID, IPAddress: "10.0.0.1", Success: ok,
			FailureReason: sql.NullString{String: "wrong_password", Valid: !ok},
		})
		if err != nil {
			t.Fatalf("insert attempt: %v", err)
		}
	}
	t.Log("Inserted 3 login attempts (1 success, 2 failed)")

	// 2. Seed fraud flags: low + medium + critical for user1, none for user2.
	severities := []string{"low", "medium", "critical"}
	flagIDs := make([]string, 0, len(severities))
	for _, sev := range severities {
		ff, err := repo.InsertFraudFlag(ctx, &repository.FraudFlag{
			UserID:      userUUID,
			FlagType:    "test_flag",
			Severity:    sev,
			Description: sql.NullString{String: "summary E2E test", Valid: true},
		})
		if err != nil {
			t.Fatalf("insert flag (%s): %v", sev, err)
		}
		flagIDs = append(flagIDs, ff.ID)
	}
	t.Logf("Inserted 3 fraud flags for user1: %v", severities)

	// 3. CountActiveFlagsForUser — 3 active flags.
	activeFlags, err := repo.CountActiveFlagsForUser(ctx, userUUID)
	if err != nil {
		t.Fatalf("count active: %v", err)
	}
	if activeFlags != 3 {
		t.Errorf("want active=3, got %d", activeFlags)
	}

	// 4. GetMaxSeverityForUser — max = "critical".
	maxSev, err := repo.GetMaxSeverityForUser(ctx, userUUID)
	if err != nil {
		t.Fatalf("max severity: %v", err)
	}
	if maxSev != "critical" {
		t.Errorf("want max_severity=critical, got %s", maxSev)
	}
	t.Logf("Max severity: %s — user is BLOCKED (critical)", maxSev)

	// 5. CountFailedLoginsInWindow (24h) — should return 2.
	since24h := time.Now().Add(-24 * time.Hour)
	recentFailed, err := repo.CountFailedLoginsInWindow(ctx, userUUID, since24h)
	if err != nil {
		t.Fatalf("count failed 24h: %v", err)
	}
	if recentFailed != 2 {
		t.Errorf("want 2 recent failed logins, got %d", recentFailed)
	}

	// 6. ListFraudFlags for user — total=3.
	allFlags, total, err := repo.ListFraudFlags(ctx, repository.FraudFlagFilter{
		UserID: userUUID, Limit: 100,
	})
	if err != nil {
		t.Fatalf("list flags: %v", err)
	}
	if total != 3 {
		t.Errorf("want total=3 flags, got %d", total)
	}
	_ = allFlags

	// 7. ListLoginAttempts recent 10 — should return all 3.
	recentAttempts, _, err := repo.ListLoginAttempts(ctx, repository.LoginAttemptFilter{
		UserID: userUUID, Limit: 10,
	})
	if err != nil {
		t.Fatalf("list recent attempts: %v", err)
	}
	if len(recentAttempts) != 3 {
		t.Errorf("want 3 recent attempts, got %d", len(recentAttempts))
	}

	// 8. Resolve the critical flag — user should no longer be blocked.
	criticalFlagID := flagIDs[2]
	_, err = repo.ResolveFraudFlag(ctx, criticalFlagID, adminUUID, "verified safe by ops", time.Now().UTC())
	if err != nil {
		t.Fatalf("resolve critical: %v", err)
	}
	t.Logf("Resolved critical flag id=%s", criticalFlagID)

	// 9. GetMaxSeverityForUser after resolution — drops to "medium".
	maxAfter, err := repo.GetMaxSeverityForUser(ctx, userUUID)
	if err != nil {
		t.Fatalf("max severity after: %v", err)
	}
	if maxAfter != "medium" {
		t.Errorf("want max_severity=medium after resolving critical, got %s", maxAfter)
	}
	t.Logf("Max severity after resolving critical: %s — user is now ALLOWED", maxAfter)

	// 10. CountActiveFlagsForUser after resolution — drops to 2.
	activeAfter, err := repo.CountActiveFlagsForUser(ctx, userUUID)
	if err != nil {
		t.Fatalf("count active after: %v", err)
	}
	if activeAfter != 2 {
		t.Errorf("want active=2 after resolving critical, got %d", activeAfter)
	}

	// 11. User2 has no flags or attempts — everything returns zero.
	active2, err := repo.CountActiveFlagsForUser(ctx, user2UUID)
	if err != nil {
		t.Fatalf("count user2: %v", err)
	}
	if active2 != 0 {
		t.Errorf("want active=0 for user2, got %d", active2)
	}

	sev2, err := repo.GetMaxSeverityForUser(ctx, user2UUID)
	if err != nil {
		t.Fatalf("max sev user2: %v", err)
	}
	if sev2 != "" {
		t.Errorf("want empty max_severity for user2, got %q", sev2)
	}
	t.Log("User2 baseline verified: no flags, no severity")
}
