//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
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

func TestCreateRating_Success(t *testing.T) {
	gormDB, mock, db := setupMockDB(t)
	defer db.Close()

	repo := NewRatingRepository(gormDB)

	rating := &Rating{
		ID:        uuid.New(),
		OrderID:   uuid.New(),
		OrderType: "ride",
		RaterID:   uuid.New(),
		RateeID:   uuid.New(),
		RateeType: "driver",
		Score:     5,
		Comment:   "Excellent!",
		Tags:      pq.StringArray{"safe", "polite"},
		CreatedAt: time.Now(),
	}

	mock.ExpectBegin()
	mock.ExpectExec(`^INSERT INTO "ratings"`).
		WithArgs(
			rating.ID.String(),
			rating.OrderID.String(),
			rating.OrderType,
			rating.RaterID.String(),
			rating.RateeID.String(),
			rating.RateeType,
			rating.Score,
			rating.Comment,
			sqlmock.AnyArg(), // Tags (pq.StringArray)
			sqlmock.AnyArg(), // CreatedAt
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.CreateRating(context.Background(), rating)

	if err != nil {
		t.Errorf("unexpected error while inserting rating: %s", err)
	}
}

func TestGetRatingsByOrder_Success(t *testing.T) {
	gormDB, mock, db := setupMockDB(t)
	defer db.Close()

	repo := NewRatingRepository(gormDB)

	orderID := uuid.New()
	
	rows := sqlmock.NewRows([]string{"id", "order_id", "score", "comment"}).
		AddRow(uuid.New().String(), orderID.String(), 5, "Good")

	mock.ExpectQuery(`^SELECT (.+) FROM "ratings" WHERE order_id = \$1`).
		WithArgs(orderID.String()).
		WillReturnRows(rows)

	ratings, err := repo.GetRatingsByOrder(context.Background(), orderID)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(ratings) != 1 || ratings[0].Score != 5 {
		t.Error("expected 1 rating with score 5")
	}
}

func TestGetDriverAggregate_Found(t *testing.T) {
	gormDB, mock, db := setupMockDB(t)
	defer db.Close()

	repo := NewRatingRepository(gormDB)

	driverID := uuid.New()

	rows := sqlmock.NewRows([]string{"driver_id", "avg_score", "total_ratings"}).
		AddRow(driverID.String(), 4.75, 50)

	mock.ExpectQuery(`^SELECT (.+) FROM "driver_score_aggregates" WHERE driver_id = \$1 ORDER BY "driver_score_aggregates"."driver_id" LIMIT \$2`).
		WithArgs(driverID.String(), 1).
		WillReturnRows(rows)

	agg, err := repo.GetDriverAggregate(context.Background(), driverID)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if agg == nil || agg.TotalRatings != 50 {
		t.Errorf("expected total ratings 50, got %v", agg.TotalRatings)
	}
}

func TestGetDriverAggregate_NotFound(t *testing.T) {
	gormDB, mock, db := setupMockDB(t)
	defer db.Close()

	repo := NewRatingRepository(gormDB)

	driverID := uuid.New()

	mock.ExpectQuery(`^SELECT (.+) FROM "driver_score_aggregates" WHERE driver_id = \$1 ORDER BY "driver_score_aggregates"."driver_id" LIMIT \$2`).
		WithArgs(driverID.String(), 1).
		WillReturnError(gorm.ErrRecordNotFound)

	_, err := repo.GetDriverAggregate(context.Background(), driverID)

	if err != gorm.ErrRecordNotFound {
		t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
	}
}
