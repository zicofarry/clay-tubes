//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/zicofarry/clay-app/backend/services/merchant-service/internal/model"
)

// helper: creates sqlmock and merchant repo
func newMerchantRepoMock(t *testing.T) (*MerchantRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewMerchantRepository(db), mock
}

func now() time.Time { return time.Now().UTC() }

func strPtr(s string) *string { return &s }

// ── Create ────────────────────────────────────────────────────────────────────

func TestMerchantRepo_Create_Success(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)

	mock.ExpectExec(`^INSERT INTO merchants`).
		WithArgs(
			sqlmock.AnyArg(), // id
			"u-1",            // user_id
			"Warung",         // name
			(*string)(nil),   // description
			model.CategoryFood,
			model.StatusPendingReview,
			"081",            // phone_number
			(*string)(nil),   // email
			"Jl A",           // address
			"Kota",           // city
			-6.2,             // lat
			106.8,            // lng
			int64(15000),     // min_order_cents
			30,               // est_delivery_min
			0.0,              // rating
			0,                // total_reviews
			sqlmock.AnyArg(), // created_at
			sqlmock.AnyArg(), // updated_at
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	m := &model.Merchant{
		UserID: "u-1", Name: "Warung", Category: model.CategoryFood,
		PhoneNumber: "081", Address: "Jl A", City: "Kota",
		Lat: -6.2, Lng: 106.8, MinOrderCents: 15000, EstDeliveryMin: 30,
	}
	if err := repo.Create(context.Background(), m); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if m.ID == "" {
		t.Error("expected generated ID")
	}
	if m.Status != model.StatusPendingReview {
		t.Errorf("status: %s", m.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled: %v", err)
	}
}

func TestMerchantRepo_Create_DBError(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)

	mock.ExpectExec(`^INSERT INTO merchants`).
		WillReturnError(sql.ErrConnDone)

	m := &model.Merchant{UserID: "u-1", Name: "Warung"}
	err := repo.Create(context.Background(), m)
	if err == nil {
		t.Error("expected error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled: %v", err)
	}
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestMerchantRepo_GetByID_Found(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)
	n := now()

	cols := []string{
		"id", "user_id", "name", "description", "category", "status",
		"phone_number", "email", "address", "city", "lat", "lng",
		"logo_url", "banner_url", "min_order_cents", "est_delivery_min",
		"rating", "total_reviews", "created_at", "updated_at",
	}

	mock.ExpectQuery(`SELECT (.+) FROM merchants WHERE id = \$1`).
		WithArgs("m-1").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			"m-1", "u-1", "Warung", nil, "food", "active",
			"081", nil, "Jl A", "Kota", -6.2, 106.8,
			nil, nil, int64(15000), 30,
			4.5, 10, n, n,
		))

	m, err := repo.GetByID(context.Background(), "m-1")
	if err != nil || m == nil {
		t.Fatalf("GetByID: err=%v m=%v", err, m)
	}
	if m.ID != "m-1" {
		t.Errorf("id: %s", m.ID)
	}
	if m.Status != model.StatusActive {
		t.Errorf("status: %s", m.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled: %v", err)
	}
}

func TestMerchantRepo_GetByID_NotFound(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)

	mock.ExpectQuery(`SELECT (.+) FROM merchants WHERE id = \$1`).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	m, err := repo.GetByID(context.Background(), "missing")
	if err != nil {
		t.Errorf("expected nil err, got %v", err)
	}
	if m != nil {
		t.Error("expected nil merchant")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled: %v", err)
	}
}

// ── GetByUserID ───────────────────────────────────────────────────────────────

func TestMerchantRepo_GetByUserID_Found(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)
	n := now()

	cols := []string{
		"id", "user_id", "name", "description", "category", "status",
		"phone_number", "email", "address", "city", "lat", "lng",
		"logo_url", "banner_url", "min_order_cents", "est_delivery_min",
		"rating", "total_reviews", "created_at", "updated_at",
	}

	mock.ExpectQuery(`SELECT (.+) FROM merchants WHERE user_id = \$1`).
		WithArgs("u-1").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			"m-1", "u-1", "Warung", nil, "food", "pending_review",
			"081", nil, "Jl A", "Kota", -6.2, 106.8,
			nil, nil, int64(0), 30,
			0.0, 0, n, n,
		))

	m, err := repo.GetByUserID(context.Background(), "u-1")
	if err != nil || m == nil {
		t.Fatalf("GetByUserID: err=%v", err)
	}
	if m.UserID != "u-1" {
		t.Errorf("user_id: %s", m.UserID)
	}
}

func TestMerchantRepo_GetByUserID_NotFound(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)

	mock.ExpectQuery(`SELECT (.+) FROM merchants WHERE user_id = \$1`).
		WithArgs("u-x").
		WillReturnError(sql.ErrNoRows)

	m, err := repo.GetByUserID(context.Background(), "u-x")
	if err != nil || m != nil {
		t.Fatalf("expected nil, got err=%v m=%v", err, m)
	}
}

// ── ExistsByUserID ────────────────────────────────────────────────────────────

func TestMerchantRepo_ExistsByUserID_True(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM merchants WHERE user_id = \$1`).
		WithArgs("u-1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	exists, err := repo.ExistsByUserID(context.Background(), "u-1")
	if err != nil || !exists {
		t.Fatalf("err=%v exists=%v", err, exists)
	}
}

func TestMerchantRepo_ExistsByUserID_False(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM merchants WHERE user_id = \$1`).
		WithArgs("u-x").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	exists, err := repo.ExistsByUserID(context.Background(), "u-x")
	if err != nil || exists {
		t.Fatalf("expected false: err=%v exists=%v", err, exists)
	}
}

// ── UpdateStatus ──────────────────────────────────────────────────────────────

func TestMerchantRepo_UpdateStatus_Success(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)
	n := now()

	mock.ExpectExec(`UPDATE merchants SET status = \$1, updated_at = \$2 WHERE id = \$3`).
		WithArgs(model.StatusActive, sqlmock.AnyArg(), "m-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	cols := []string{
		"id", "user_id", "name", "description", "category", "status",
		"phone_number", "email", "address", "city", "lat", "lng",
		"logo_url", "banner_url", "min_order_cents", "est_delivery_min",
		"rating", "total_reviews", "created_at", "updated_at",
	}
	mock.ExpectQuery(`SELECT (.+) FROM merchants WHERE id = \$1`).
		WithArgs("m-1").
		WillReturnRows(sqlmock.NewRows(cols).AddRow(
			"m-1", "u-1", "Warung", nil, "food", "active",
			"081", nil, "Jl A", "Kota", -6.2, 106.8,
			nil, nil, int64(0), 30,
			0.0, 0, n, n,
		))

	m, err := repo.UpdateStatus(context.Background(), "m-1", model.StatusActive)
	if err != nil || m == nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	if m.Status != model.StatusActive {
		t.Errorf("status: %s", m.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled: %v", err)
	}
}

// ── ListBankAccounts ──────────────────────────────────────────────────────────

func TestMerchantRepo_ListBankAccounts_Success(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)
	n := now()

	mock.ExpectQuery(`SELECT (.+) FROM merchant_bank_accounts WHERE merchant_id = \$1`).
		WithArgs("m-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "merchant_id", "bank_code", "account_number", "account_name", "is_primary", "created_at",
		}).AddRow("ba-1", "m-1", "BCA", "1234567890", "Budi Santoso", true, n))

	accounts, err := repo.ListBankAccounts(context.Background(), "m-1")
	if err != nil || len(accounts) != 1 {
		t.Fatalf("ListBankAccounts: err=%v len=%d", err, len(accounts))
	}
	if accounts[0].BankCode != "BCA" {
		t.Errorf("bank_code: %s", accounts[0].BankCode)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled: %v", err)
	}
}

func TestMerchantRepo_ListBankAccounts_Empty(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)

	mock.ExpectQuery(`SELECT (.+) FROM merchant_bank_accounts WHERE merchant_id = \$1`).
		WithArgs("m-empty").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "merchant_id", "bank_code", "account_number", "account_name", "is_primary", "created_at",
		}))

	accounts, err := repo.ListBankAccounts(context.Background(), "m-empty")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(accounts) != 0 {
		t.Errorf("expected 0, got %d", len(accounts))
	}
}

// ── DeleteBankAccount ─────────────────────────────────────────────────────────

func TestMerchantRepo_DeleteBankAccount_Success(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)

	mock.ExpectExec(`DELETE FROM merchant_bank_accounts WHERE id = \$1`).
		WithArgs("ba-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.DeleteBankAccount(context.Background(), "ba-1"); err != nil {
		t.Fatalf("DeleteBankAccount: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled: %v", err)
	}
}

// ── GetOperatingHours ─────────────────────────────────────────────────────────

func TestMerchantRepo_GetOperatingHours_Success(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)
	n := now()

	mock.ExpectQuery(`SELECT (.+) FROM merchant_operating_hours WHERE merchant_id = \$1`).
		WithArgs("m-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "merchant_id", "day_of_week", "open_time", "close_time", "is_closed", "updated_at",
		}).
			AddRow("h-1", "m-1", 1, "08:00", "22:00", false, n).
			AddRow("h-2", "m-1", 0, "", "", true, n))

	hours, err := repo.GetOperatingHours(context.Background(), "m-1")
	if err != nil || len(hours) != 2 {
		t.Fatalf("GetOperatingHours: err=%v len=%d", err, len(hours))
	}
	if !hours[1].IsClosed {
		t.Error("expected day 0 to be closed")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled: %v", err)
	}
}

func TestMerchantRepo_GetOperatingHours_Empty(t *testing.T) {
	repo, mock := newMerchantRepoMock(t)

	mock.ExpectQuery(`SELECT (.+) FROM merchant_operating_hours WHERE merchant_id = \$1`).
		WithArgs("m-new").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "merchant_id", "day_of_week", "open_time", "close_time", "is_closed", "updated_at",
		}))

	hours, err := repo.GetOperatingHours(context.Background(), "m-new")
	if err != nil || hours != nil {
		t.Fatalf("expected nil slice: err=%v hours=%v", err, hours)
	}
}
