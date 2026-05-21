//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *sql.DB) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: db,
	}), &gorm.Config{})

	if err != nil {
		t.Fatalf("failed to open gorm db: %v", err)
	}

	return gormDB, mock, db
}

func TestGetPromoCodeByID_Found(t *testing.T) {
	gormDB, mock, db := setupMockDB(t)
	defer db.Close()

	repo := NewPromotionRepository(gormDB)

	id := uuid.New()

	rows := sqlmock.NewRows([]string{
		"id", "code", "type", "value", "min_order_amount", "max_discount",
		"quota", "used_count", "service_type", "valid_from", "valid_until", "is_active",
	}).AddRow(
		id.String(), "DISCOUNT10", "percentage_off", 10.0, 50000.0, 10000.0,
		100, 5, "ride", time.Now().Add(-time.Hour), time.Now().Add(time.Hour), true,
	)

	mock.ExpectQuery(`^SELECT (.+) FROM "promo_codes" WHERE id = \$1 ORDER BY "promo_codes"."id" LIMIT \$2`).
		WithArgs(id.String(), 1).
		WillReturnRows(rows)

	promo, err := repo.GetPromoCodeByID(context.Background(), id)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if promo == nil || promo.Code != "DISCOUNT10" {
		t.Errorf("expected promo code DISCOUNT10")
	}
}

func TestGetPromoCodeByID_NotFound(t *testing.T) {
	gormDB, mock, db := setupMockDB(t)
	defer db.Close()

	repo := NewPromotionRepository(gormDB)

	id := uuid.New()

	mock.ExpectQuery(`^SELECT (.+) FROM "promo_codes" WHERE id = \$1 ORDER BY "promo_codes"."id" LIMIT \$2`).
		WithArgs(id.String(), 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := repo.GetPromoCodeByID(context.Background(), id)

	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
	}
}

func TestCreatePromoCode_Success(t *testing.T) {
	gormDB, mock, db := setupMockDB(t)
	defer db.Close()

	repo := NewPromotionRepository(gormDB)

	minOrder := 50000.0
	maxDisc := 10000.0

	promo := &PromoCode{
		ID:             uuid.New(),
		Code:           "DISCOUNT10",
		Type:           "percentage_off",
		Value:          10.0,
		MinOrderAmount: &minOrder,
		MaxDiscount:    &maxDisc,
		Quota:          100,
		UsedCount:      0,
		ServiceType:    "ride",
		ValidFrom:      time.Now(),
		ValidUntil:     time.Now().Add(24 * time.Hour),
		IsActive:       true,
	}

	mock.ExpectBegin()
	mock.ExpectExec(`^INSERT INTO "promo_codes"`).
		WithArgs(
			promo.ID.String(),
			promo.Code,
			promo.Type,
			promo.Value,
			sqlmock.AnyArg(), // MinOrderAmount
			sqlmock.AnyArg(), // MaxDiscount
			promo.Quota,
			promo.UsedCount,
			promo.ServiceType,
			sqlmock.AnyArg(), // ValidFrom
			sqlmock.AnyArg(), // ValidUntil
			promo.IsActive,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.CreatePromoCode(context.Background(), promo)

	if err != nil {
		t.Errorf("unexpected error while inserting promo code: %s", err)
	}
}
