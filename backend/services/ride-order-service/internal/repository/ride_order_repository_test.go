//go:build unit

package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func newRepo(t *testing.T) (*RideOrderRepository, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock open: %v", err)
	}
	return NewRideOrderRepository(db, nil), mock, db
}

// ── CreateOrder ─────────────────────────────────────────────────────────────

func TestCreateOrder_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	o := &RideOrder{
		UserID:        "user-1",
		ServiceType:   "goride",
		VehicleType:   "motor",
		Status:        "pending",
		OriginLat:     -6.91, OriginLng: 107.60,
		DestLat: -6.92, DestLng: 107.61,
		PaymentMethod: "gopay",
	}

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO ride_orders`)).
		WithArgs(o.UserID, o.ServiceType, o.VehicleType, o.Status,
			o.OriginLat, o.OriginLng, o.OriginAddress,
			o.DestLat, o.DestLng, o.DestAddress,
			o.FareEstimate, o.PromoID, o.PaymentMethod).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "created_at", "updated_at"}).
			AddRow("uuid-1", "pending", time.Now(), time.Now()))

	created, err := repo.CreateOrder(context.Background(), o)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if created.ID != "uuid-1" {
		t.Errorf("want id=uuid-1, got %s", created.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet: %v", err)
	}
}

func TestCreateOrder_DBError(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO ride_orders`)).
		WillReturnError(sql.ErrConnDone)

	_, err := repo.CreateOrder(context.Background(), &RideOrder{})
	if err != sql.ErrConnDone {
		t.Errorf("want sql.ErrConnDone, got %v", err)
	}
}

// ── GetOrderByID ────────────────────────────────────────────────────────────

func TestGetOrderByID_Found(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "driver_id", "service_type", "vehicle_type", "status",
		"origin_lat", "origin_lng", "origin_address",
		"dest_lat", "dest_lng", "dest_address",
		"fare_estimate", "fare_final", "promo_id", "payment_method",
		"otp_code", "cancel_reason", "cancelled_by",
		"created_at", "updated_at",
	}).AddRow(
		"uuid-1", "user-1", nil, "goride", "motor", "pending",
		-6.91, 107.60, nil,
		-6.92, 107.61, nil,
		nil, nil, nil, "gopay",
		nil, nil, nil,
		time.Now(), time.Now(),
	)

	mock.ExpectQuery(regexp.QuoteMeta(`FROM ride_orders WHERE id = $1`)).
		WithArgs("uuid-1").
		WillReturnRows(rows)

	got, err := repo.GetOrderByID(context.Background(), "uuid-1")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if got.UserID != "user-1" || got.ServiceType != "goride" {
		t.Errorf("unexpected order: %+v", got)
	}
}

func TestGetOrderByID_NotFound(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`FROM ride_orders WHERE id = $1`)).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetOrderByID(context.Background(), "missing")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows, got %v", err)
	}
}

// ── GetActiveOrderByUserID ──────────────────────────────────────────────────

func TestGetActiveOrderByUserID_None(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`WHERE user_id = $1`)).
		WithArgs("user-1").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetActiveOrderByUserID(context.Background(), "user-1")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows, got %v", err)
	}
}

// ── UpdateStatus ────────────────────────────────────────────────────────────

func TestUpdateStatus_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE ride_orders SET status`)).
		WithArgs("finding_driver", "uuid-1", "pending").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.UpdateStatus(context.Background(), "uuid-1", "pending", "finding_driver"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestUpdateStatus_NoRows(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE ride_orders SET status`)).
		WithArgs("finding_driver", "uuid-1", "pending").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.UpdateStatus(context.Background(), "uuid-1", "pending", "finding_driver")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows when no row updated, got %v", err)
	}
}

// ── AssignDriver ────────────────────────────────────────────────────────────

func TestAssignDriver_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE ride_orders`)).
		WithArgs("driver-1", "847291", "uuid-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.AssignDriver(context.Background(), "uuid-1", "driver-1", "847291"); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestAssignDriver_AlreadyTaken(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE ride_orders`)).
		WithArgs("driver-1", "847291", "uuid-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.AssignDriver(context.Background(), "uuid-1", "driver-1", "847291")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows on race, got %v", err)
	}
}

// ── SetCancelled ────────────────────────────────────────────────────────────

func TestSetCancelled_BlockedOnTrip(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	// Simulate WHERE status NOT IN ('completed','cancelled','on_trip') failing
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE ride_orders`)).
		WithArgs("Salah pilih", "user", "uuid-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.SetCancelled(context.Background(), "uuid-1", "Salah pilih", "user")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows on non-cancellable state, got %v", err)
	}
}

// ── SetFareFinal ────────────────────────────────────────────────────────────

func TestSetFareFinal_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE ride_orders SET fare_final`)).
		WithArgs(18000.0, "uuid-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.SetFareFinal(context.Background(), "uuid-1", 18000.0); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// ── State logs ──────────────────────────────────────────────────────────────

func TestInsertStateLog_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO order_state_logs`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "changed_at"}).
			AddRow("log-1", time.Now()))

	l := &OrderStateLog{
		OrderID: "uuid-1", ToState: "finding_driver", ActorType: "system",
	}
	if err := repo.InsertStateLog(context.Background(), l); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if l.ID != "log-1" {
		t.Errorf("want id=log-1, got %s", l.ID)
	}
}

func TestListStateLogs_Empty(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`FROM order_state_logs WHERE order_id`)).
		WithArgs("uuid-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "order_id", "from_state", "to_state",
			"actor_id", "actor_type", "reason", "metadata", "changed_at",
		}))

	logs, err := repo.ListStateLogs(context.Background(), "uuid-1")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("want 0 logs, got %d", len(logs))
	}
}

// ── Trip details ────────────────────────────────────────────────────────────

func TestUpsertTripDetails_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO trip_details`)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	td := &TripDetails{OrderID: "uuid-1"}
	if err := repo.UpsertTripDetails(context.Background(), td); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// ── Fare breakdown ──────────────────────────────────────────────────────────

func TestGetFareBreakdown_Found(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"id", "order_id", "base_fare", "distance_fare", "time_fare",
		"surge_multiplier", "promo_discount", "platform_fee", "total",
	}).AddRow("fb-1", "uuid-1", 5000.0, 8000.0, 3000.0, 1.0, 2000.0, 1000.0, 15000.0)

	mock.ExpectQuery(regexp.QuoteMeta(`FROM order_fare_breakdown WHERE order_id`)).
		WithArgs("uuid-1").
		WillReturnRows(rows)

	fb, err := repo.GetFareBreakdown(context.Background(), "uuid-1")
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if fb.Total != 15000.0 {
		t.Errorf("want total 15000, got %.2f", fb.Total)
	}
}

func TestUpsertFareBreakdown_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO order_fare_breakdown`)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	fb := &FareBreakdown{OrderID: "uuid-1", Total: 15000.0}
	if err := repo.UpsertFareBreakdown(context.Background(), fb); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

// ── ListUserHistory ─────────────────────────────────────────────────────────

func TestListUserHistory_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(1) FROM ride_orders`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "driver_id", "service_type", "vehicle_type", "status",
		"origin_lat", "origin_lng", "origin_address",
		"dest_lat", "dest_lng", "dest_address",
		"fare_estimate", "fare_final", "promo_id", "payment_method",
		"otp_code", "cancel_reason", "cancelled_by",
		"created_at", "updated_at",
	}).AddRow(
		"uuid-1", "user-1", nil, "goride", "motor", "completed",
		-6.91, 107.60, nil,
		-6.92, 107.61, nil,
		nil, 18000.0, nil, "gopay",
		nil, nil, nil,
		time.Now(), time.Now(),
	)

	mock.ExpectQuery(regexp.QuoteMeta(`FROM ride_orders`)).
		WillReturnRows(rows)

	out, total, err := repo.ListUserHistory(context.Background(), "user-1", HistoryFilter{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if total != 1 {
		t.Errorf("want total 1, got %d", total)
	}
	if len(out) != 1 {
		t.Errorf("want 1 row, got %d", len(out))
	}
}
