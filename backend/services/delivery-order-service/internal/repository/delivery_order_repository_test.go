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

func newRepo(t *testing.T) (*DeliveryOrderRepository, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock open: %v", err)
	}
	return NewDeliveryOrderRepository(db, nil), mock, db
}

// ── CreateOrder ─────────────────────────────────────────────────────────────

func TestCreateOrder_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO delivery_orders`)).
		WithArgs(
			"user-1", "pending",
			"Budi", "+6281111",
			-6.91, 107.60, "Jl. Braga No.1", sqlmock.AnyArg(),
			"Ani", "+6282222",
			-6.92, 107.61, "Jl. Dago No.5", sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), "gopay",
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "created_at", "updated_at"}).
			AddRow("order-uuid-1", "pending", now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO delivery_packages`)).
		WithArgs(
			"order-uuid-1", "document", sqlmock.AnyArg(),
			"small", false, sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("pkg-uuid-1"))
	mock.ExpectCommit()

	o := &DeliveryOrder{
		UserID:        "user-1",
		Status:        "pending",
		SenderName:    "Budi",
		SenderPhone:   "+6281111",
		PickupLat:     -6.91, PickupLng: 107.60,
		PickupAddress: "Jl. Braga No.1",
		RecipientName:  "Ani",
		RecipientPhone: "+6282222",
		DestLat:       -6.92, DestLng: 107.61,
		DestAddress:   "Jl. Dago No.5",
		PaymentMethod: "gopay",
	}
	pkg := &DeliveryPackage{
		Category: "document",
		Size:     "small",
	}

	created, err := repo.CreateOrder(context.Background(), o, pkg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.ID != "order-uuid-1" {
		t.Errorf("want id=order-uuid-1, got %s", created.ID)
	}
	if pkg.ID != "pkg-uuid-1" {
		t.Errorf("want pkg id=pkg-uuid-1, got %s", pkg.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCreateOrder_OrderInsertError(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO delivery_orders`)).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	_, err := repo.CreateOrder(context.Background(), &DeliveryOrder{}, &DeliveryPackage{})
	if err != sql.ErrConnDone {
		t.Errorf("want sql.ErrConnDone, got %v", err)
	}
}

func TestCreateOrder_PackageInsertError(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	now := time.Now()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO delivery_orders`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "created_at", "updated_at"}).
			AddRow("order-uuid-1", "pending", now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO delivery_packages`)).
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	_, err := repo.CreateOrder(context.Background(), &DeliveryOrder{}, &DeliveryPackage{})
	if err != sql.ErrConnDone {
		t.Errorf("want sql.ErrConnDone on package insert, got %v", err)
	}
}

// ── GetOrderByID ─────────────────────────────────────────────────────────────

func TestGetOrderByID_Found(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "driver_id", "status",
		"sender_name", "sender_phone",
		"pickup_lat", "pickup_lng", "pickup_address", "pickup_notes",
		"recipient_name", "recipient_phone",
		"dest_lat", "dest_lng", "dest_address", "dest_notes",
		"fare_estimate", "fare_final", "promo_id", "payment_method",
		"cancel_reason", "cancelled_by",
		"pickup_photo_url", "delivery_photo_url",
		"picked_up_at", "delivered_at",
		"actual_distance_km", "actual_duration_min",
		"created_at", "updated_at",
	}).AddRow(
		"order-uuid-1", "user-1", nil, "pending",
		"Budi", "+6281111",
		-6.91, 107.60, "Jl. Braga", nil,
		"Ani", "+6282222",
		-6.92, 107.61, "Jl. Dago", nil,
		nil, nil, nil, "gopay",
		nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
		now, now,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`FROM delivery_orders WHERE id = $1`)).
		WithArgs("order-uuid-1").
		WillReturnRows(rows)

	got, err := repo.GetOrderByID(context.Background(), "order-uuid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.UserID != "user-1" || got.SenderName != "Budi" {
		t.Errorf("unexpected order: %+v", got)
	}
}

func TestGetOrderByID_NotFound(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`FROM delivery_orders WHERE id = $1`)).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetOrderByID(context.Background(), "missing")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows, got %v", err)
	}
}

// ── GetPackageByOrderID ──────────────────────────────────────────────────────

func TestGetPackageByOrderID_Found(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"id", "order_id", "category", "weight_kg",
		"size", "is_fragile", "description", "insurance_value", "photo_url",
	}).AddRow("pkg-1", "order-uuid-1", "document", nil, "small", false, nil, nil, nil)

	mock.ExpectQuery(regexp.QuoteMeta(`FROM delivery_packages WHERE order_id = $1`)).
		WithArgs("order-uuid-1").
		WillReturnRows(rows)

	pkg, err := repo.GetPackageByOrderID(context.Background(), "order-uuid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pkg.Category != "document" || pkg.Size != "small" {
		t.Errorf("unexpected package: %+v", pkg)
	}
}

func TestGetPackageByOrderID_NotFound(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`FROM delivery_packages WHERE order_id = $1`)).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetPackageByOrderID(context.Background(), "missing")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows, got %v", err)
	}
}

// ── GetActiveOrderByUserID ───────────────────────────────────────────────────

func TestGetActiveOrderByUserID_Found(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"id", "user_id", "driver_id", "status",
		"sender_name", "sender_phone",
		"pickup_lat", "pickup_lng", "pickup_address", "pickup_notes",
		"recipient_name", "recipient_phone",
		"dest_lat", "dest_lng", "dest_address", "dest_notes",
		"fare_estimate", "fare_final", "promo_id", "payment_method",
		"cancel_reason", "cancelled_by",
		"pickup_photo_url", "delivery_photo_url",
		"picked_up_at", "delivered_at",
		"actual_distance_km", "actual_duration_min",
		"created_at", "updated_at",
	}).AddRow(
		"order-uuid-1", "user-1", nil, "finding_driver",
		"Budi", "+6281111",
		-6.91, 107.60, "Jl. Braga", nil,
		"Ani", "+6282222",
		-6.92, 107.61, "Jl. Dago", nil,
		nil, nil, nil, "gopay",
		nil, nil,
		nil, nil,
		nil, nil,
		nil, nil,
		now, now,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`WHERE user_id = $1`)).
		WithArgs("user-1").
		WillReturnRows(rows)

	got, err := repo.GetActiveOrderByUserID(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "order-uuid-1" {
		t.Errorf("want id=order-uuid-1, got %s", got.ID)
	}
}

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

// ── ListUserHistory ──────────────────────────────────────────────────────────

func TestListUserHistory_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	now := time.Now()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(1) FROM delivery_orders`)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	dataRows := sqlmock.NewRows([]string{
		"id", "user_id", "driver_id", "status",
		"sender_name", "sender_phone",
		"pickup_lat", "pickup_lng", "pickup_address", "pickup_notes",
		"recipient_name", "recipient_phone",
		"dest_lat", "dest_lng", "dest_address", "dest_notes",
		"fare_estimate", "fare_final", "promo_id", "payment_method",
		"cancel_reason", "cancelled_by",
		"pickup_photo_url", "delivery_photo_url",
		"picked_up_at", "delivered_at",
		"actual_distance_km", "actual_duration_min",
		"created_at", "updated_at",
	}).
		AddRow("order-uuid-1", "user-1", nil, "delivered", "Budi", "+6281111",
			-6.91, 107.60, "Jl. Braga", nil, "Ani", "+6282222",
			-6.92, 107.61, "Jl. Dago", nil,
			nil, 25000.0, nil, "gopay",
			nil, nil, nil, nil, nil, nil, nil, nil, now, now).
		AddRow("order-uuid-2", "user-1", nil, "delivered", "Budi", "+6281111",
			-6.91, 107.60, "Jl. Braga", nil, "Ani", "+6282222",
			-6.92, 107.61, "Jl. Dago", nil,
			nil, 18000.0, nil, "gopay",
			nil, nil, nil, nil, nil, nil, nil, nil, now, now)

	mock.ExpectQuery(regexp.QuoteMeta(`FROM delivery_orders`)).
		WillReturnRows(dataRows)

	out, total, err := repo.ListUserHistory(context.Background(), "user-1", HistoryFilter{
		Status: "delivered", Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("want total=2, got %d", total)
	}
	if len(out) != 2 {
		t.Errorf("want 2 rows, got %d", len(out))
	}
}

func TestListUserHistory_CountError(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COUNT(1) FROM delivery_orders`)).
		WillReturnError(sql.ErrConnDone)

	_, _, err := repo.ListUserHistory(context.Background(), "user-1", HistoryFilter{})
	if err != sql.ErrConnDone {
		t.Errorf("want sql.ErrConnDone, got %v", err)
	}
}

// ── UpdateStatus ─────────────────────────────────────────────────────────────

func TestUpdateStatus_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE delivery_orders SET status = $1`)).
		WithArgs("finding_driver", "order-uuid-1", "pending").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.UpdateStatus(context.Background(), "order-uuid-1", "pending", "finding_driver"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateStatus_NoRows(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE delivery_orders SET status = $1`)).
		WithArgs("finding_driver", "order-uuid-1", "pending").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.UpdateStatus(context.Background(), "order-uuid-1", "pending", "finding_driver")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows when no row updated, got %v", err)
	}
}

func TestUpdateStatus_DBError(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE delivery_orders SET status = $1`)).
		WithArgs("finding_driver", "order-uuid-1", "pending").
		WillReturnError(sql.ErrConnDone)

	err := repo.UpdateStatus(context.Background(), "order-uuid-1", "pending", "finding_driver")
	if err != sql.ErrConnDone {
		t.Errorf("want sql.ErrConnDone, got %v", err)
	}
}

// ── AssignDriver ─────────────────────────────────────────────────────────────

func TestAssignDriver_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE delivery_orders`)).
		WithArgs("driver-1", "order-uuid-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.AssignDriver(context.Background(), "order-uuid-1", "driver-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAssignDriver_AlreadyTaken(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE delivery_orders`)).
		WithArgs("driver-2", "order-uuid-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.AssignDriver(context.Background(), "order-uuid-1", "driver-2")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows on race condition, got %v", err)
	}
}

// ── SetCancelled ─────────────────────────────────────────────────────────────

func TestSetCancelled_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE delivery_orders`)).
		WithArgs("Salah tujuan", "user", "order-uuid-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.SetCancelled(context.Background(), "order-uuid-1", "Salah tujuan", "user"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetCancelled_BlockedPickedUp(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	// Simulate WHERE status NOT IN ('picked_up','on_delivery','delivered','cancelled') failing
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE delivery_orders`)).
		WithArgs("Salah tujuan", "user", "order-uuid-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.SetCancelled(context.Background(), "order-uuid-1", "Salah tujuan", "user")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows when order is in non-cancellable state, got %v", err)
	}
}

// ── SetFareFinal ─────────────────────────────────────────────────────────────

func TestSetFareFinal_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE delivery_orders SET fare_final`)).
		WithArgs(25000.0, "order-uuid-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.SetFareFinal(context.Background(), "order-uuid-1", 25000.0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetFareFinal_DBError(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE delivery_orders SET fare_final`)).
		WillReturnError(sql.ErrConnDone)

	err := repo.SetFareFinal(context.Background(), "order-uuid-1", 25000.0)
	if err != sql.ErrConnDone {
		t.Errorf("want sql.ErrConnDone, got %v", err)
	}
}

// ── SetPickupProof ────────────────────────────────────────────────────────────

func TestSetPickupProof_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE delivery_orders`)).
		WithArgs("https://cdn.example.com/pickup.jpg", "order-uuid-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.SetPickupProof(context.Background(), "order-uuid-1", "https://cdn.example.com/pickup.jpg"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetPickupProof_DBError(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE delivery_orders`)).
		WillReturnError(sql.ErrConnDone)

	err := repo.SetPickupProof(context.Background(), "order-uuid-1", "https://cdn.example.com/pickup.jpg")
	if err != sql.ErrConnDone {
		t.Errorf("want sql.ErrConnDone, got %v", err)
	}
}

// ── SetDeliveryDetails ───────────────────────────────────────────────────────

func TestSetDeliveryDetails_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE delivery_orders`)).
		WithArgs("https://cdn.example.com/delivery.jpg", 5.2, 30, "order-uuid-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := repo.SetDeliveryDetails(context.Background(), "order-uuid-1", "https://cdn.example.com/delivery.jpg", 5.2, 30); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetDeliveryDetails_DBError(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE delivery_orders`)).
		WillReturnError(sql.ErrConnDone)

	err := repo.SetDeliveryDetails(context.Background(), "order-uuid-1", "https://cdn.example.com/delivery.jpg", 5.2, 30)
	if err != sql.ErrConnDone {
		t.Errorf("want sql.ErrConnDone, got %v", err)
	}
}

// ── InsertStateLog ───────────────────────────────────────────────────────────

func TestInsertStateLog_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO delivery_state_logs`)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "changed_at"}).
			AddRow("log-uuid-1", now))

	l := &DeliveryStateLog{
		OrderID:   "order-uuid-1",
		ToState:   "finding_driver",
		ActorType: "user",
	}
	if err := repo.InsertStateLog(context.Background(), l); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l.ID != "log-uuid-1" {
		t.Errorf("want id=log-uuid-1, got %s", l.ID)
	}
	if !l.ChangedAt.Equal(now) {
		t.Errorf("changed_at not set")
	}
}

func TestInsertStateLog_DBError(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO delivery_state_logs`)).
		WillReturnError(sql.ErrConnDone)

	err := repo.InsertStateLog(context.Background(), &DeliveryStateLog{
		OrderID: "order-uuid-1", ToState: "finding_driver", ActorType: "user",
	})
	if err != sql.ErrConnDone {
		t.Errorf("want sql.ErrConnDone, got %v", err)
	}
}

// ── ListStateLogs ────────────────────────────────────────────────────────────

func TestListStateLogs_Empty(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`FROM delivery_state_logs WHERE order_id = $1`)).
		WithArgs("order-uuid-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "order_id", "from_state", "to_state",
			"actor_id", "actor_type", "reason", "changed_at",
		}))

	logs, err := repo.ListStateLogs(context.Background(), "order-uuid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("want 0 logs, got %d", len(logs))
	}
}

func TestListStateLogs_WithEntries(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(`FROM delivery_state_logs WHERE order_id = $1`)).
		WithArgs("order-uuid-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "order_id", "from_state", "to_state",
			"actor_id", "actor_type", "reason", "changed_at",
		}).
			AddRow("log-1", "order-uuid-1", nil, "finding_driver", nil, "user", nil, now).
			AddRow("log-2", "order-uuid-1", "finding_driver", "assigned", "driver-1", "driver", nil, now))

	logs, err := repo.ListStateLogs(context.Background(), "order-uuid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("want 2 logs, got %d", len(logs))
	}
	if logs[0].ID != "log-1" {
		t.Errorf("want first log id=log-1, got %s", logs[0].ID)
	}
}

// ── UpsertFareBreakdown ──────────────────────────────────────────────────────

func TestUpsertFareBreakdown_Success(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO delivery_fare_breakdown`)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	fb := &DeliveryFareBreakdown{
		OrderID:         "order-uuid-1",
		BaseFare:        5000,
		DistanceFare:    15600,
		WeightSurcharge: 500,
		InsuranceFee:    0,
		PromoDiscount:   0,
		PlatformFee:     1000,
		Total:           22100,
	}
	if err := repo.UpsertFareBreakdown(context.Background(), fb); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpsertFareBreakdown_DBError(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO delivery_fare_breakdown`)).
		WillReturnError(sql.ErrConnDone)

	err := repo.UpsertFareBreakdown(context.Background(), &DeliveryFareBreakdown{OrderID: "order-uuid-1"})
	if err != sql.ErrConnDone {
		t.Errorf("want sql.ErrConnDone, got %v", err)
	}
}

// ── GetFareBreakdown ─────────────────────────────────────────────────────────

func TestGetFareBreakdown_Found(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{
		"id", "order_id", "base_fare", "distance_fare",
		"weight_surcharge", "insurance_fee", "promo_discount", "platform_fee", "total",
	}).AddRow("fb-1", "order-uuid-1", 5000.0, 15600.0, 500.0, 0.0, 0.0, 1000.0, 22100.0)

	mock.ExpectQuery(regexp.QuoteMeta(`FROM delivery_fare_breakdown WHERE order_id = $1`)).
		WithArgs("order-uuid-1").
		WillReturnRows(rows)

	fb, err := repo.GetFareBreakdown(context.Background(), "order-uuid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fb.Total != 22100.0 {
		t.Errorf("want total=22100, got %.2f", fb.Total)
	}
	if fb.WeightSurcharge != 500.0 {
		t.Errorf("want weight_surcharge=500, got %.2f", fb.WeightSurcharge)
	}
}

func TestGetFareBreakdown_NotFound(t *testing.T) {
	repo, mock, db := newRepo(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(`FROM delivery_fare_breakdown WHERE order_id = $1`)).
		WithArgs("order-uuid-1").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetFareBreakdown(context.Background(), "order-uuid-1")
	if err != sql.ErrNoRows {
		t.Errorf("want sql.ErrNoRows, got %v", err)
	}
}
