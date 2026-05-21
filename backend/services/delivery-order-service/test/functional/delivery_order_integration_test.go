//go:build functional

// Package functional contains end-to-end integration tests that connect
// directly to a real PostgreSQL instance provisioned via docker-compose.
//
// These tests will FAIL when the docker compose stack is not running
// (no DB to connect to) and PASS once `docker compose up -d` has provisioned
// PostgreSQL on localhost:5448.
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
	"github.com/zicofarry/clay-app/backend/services/delivery-order-service/internal/repository"
)

// dsn returns the test DSN, allowing CI override via TEST_DATABASE_URL.
func dsn() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://clay_user:clay_password@localhost:5448/delivery_order_db?sslmode=disable"
}

// setupTestDB connects to the docker-compose PostgreSQL, applies the schema,
// truncates all tables, and returns a clean *sql.DB.
// Fails the test loudly if the DB is unreachable.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("postgres", dsn())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Wait up to 10s for DB to become reachable.
	deadline := time.Now().Add(10 * time.Second)
	for {
		if err = db.PingContext(context.Background()); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf(
				"cannot reach PostgreSQL at %s — did you run `docker compose up -d`? last err: %v",
				dsn(), err,
			)
		}
		time.Sleep(500 * time.Millisecond)
	}

	if _, err := db.Exec(schemaDDL); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
	if _, err := db.Exec(`
		TRUNCATE TABLE
			delivery_fare_breakdown,
			delivery_state_logs,
			delivery_packages,
			delivery_orders
		CASCADE
	`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

const schemaDDL = `
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS delivery_orders (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL,
    driver_id           UUID,
    status              VARCHAR(40) NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending','finding_driver','assigned','on_pickup','picked_up','on_delivery','delivered','cancelled')),
    sender_name         VARCHAR(100) NOT NULL,
    sender_phone        VARCHAR(20)  NOT NULL,
    pickup_lat          DECIMAL(10,7) NOT NULL,
    pickup_lng          DECIMAL(10,7) NOT NULL,
    pickup_address      TEXT NOT NULL,
    pickup_notes        TEXT,
    recipient_name      VARCHAR(100) NOT NULL,
    recipient_phone     VARCHAR(20)  NOT NULL,
    dest_lat            DECIMAL(10,7) NOT NULL,
    dest_lng            DECIMAL(10,7) NOT NULL,
    dest_address        TEXT NOT NULL,
    dest_notes          TEXT,
    fare_estimate       DECIMAL(12,2),
    fare_final          DECIMAL(12,2),
    promo_id            UUID,
    payment_method      VARCHAR(20) NOT NULL CHECK (payment_method IN ('gopay','cash')),
    cancel_reason       TEXT,
    cancelled_by        VARCHAR(20)  CHECK (cancelled_by IN ('user','driver','system')),
    pickup_photo_url    TEXT,
    delivery_photo_url  TEXT,
    picked_up_at        TIMESTAMPTZ,
    delivered_at        TIMESTAMPTZ,
    actual_distance_km  DECIMAL(8,3),
    actual_duration_min INTEGER,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_delivery_orders_user_id     ON delivery_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_delivery_orders_driver_id   ON delivery_orders(driver_id);
CREATE INDEX IF NOT EXISTS idx_delivery_orders_status      ON delivery_orders(status);
CREATE INDEX IF NOT EXISTS idx_delivery_orders_created_at  ON delivery_orders(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_delivery_orders_user_status ON delivery_orders(user_id, status);

CREATE TABLE IF NOT EXISTS delivery_packages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id        UUID NOT NULL UNIQUE REFERENCES delivery_orders(id) ON DELETE CASCADE,
    category        VARCHAR(30) NOT NULL CHECK (category IN ('document','food','electronics','clothing','fragile','other')),
    weight_kg       DECIMAL(5,2),
    size            VARCHAR(20) NOT NULL CHECK (size IN ('small','medium','large')),
    is_fragile      BOOLEAN NOT NULL DEFAULT FALSE,
    description     TEXT,
    insurance_value DECIMAL(12,2),
    photo_url       TEXT
);
CREATE INDEX IF NOT EXISTS idx_delivery_packages_order_id ON delivery_packages(order_id);

CREATE TABLE IF NOT EXISTS delivery_state_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL REFERENCES delivery_orders(id) ON DELETE CASCADE,
    from_state  VARCHAR(40),
    to_state    VARCHAR(40) NOT NULL,
    actor_id    UUID,
    actor_type  VARCHAR(20) NOT NULL CHECK (actor_type IN ('user','driver','system')),
    reason      TEXT,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_delivery_state_logs_order_id   ON delivery_state_logs(order_id);
CREATE INDEX IF NOT EXISTS idx_delivery_state_logs_changed_at ON delivery_state_logs(changed_at DESC);

CREATE TABLE IF NOT EXISTS delivery_fare_breakdown (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         UUID NOT NULL UNIQUE REFERENCES delivery_orders(id) ON DELETE CASCADE,
    base_fare        DECIMAL(12,2) NOT NULL,
    distance_fare    DECIMAL(12,2) NOT NULL,
    weight_surcharge DECIMAL(12,2) NOT NULL DEFAULT 0,
    insurance_fee    DECIMAL(12,2) NOT NULL DEFAULT 0,
    promo_discount   DECIMAL(12,2) NOT NULL DEFAULT 0,
    platform_fee     DECIMAL(12,2) NOT NULL DEFAULT 0,
    total            DECIMAL(12,2) NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_delivery_fare_breakdown_order_id ON delivery_fare_breakdown(order_id);
`

// Deterministic UUIDs for test actors.
func userUUID(_ *testing.T) string  { return "00000000-0000-0000-0000-00000000000a" }
func driverUUID(_ *testing.T) string { return "00000000-0000-0000-0000-00000000000b" }

// baseOrder returns a minimal valid DeliveryOrder for seeding.
func baseOrder(userID string) *repository.DeliveryOrder {
	return &repository.DeliveryOrder{
		UserID:         userID,
		Status:         "finding_driver",
		SenderName:     "Budi Santoso",
		SenderPhone:    "+6281234567890",
		PickupLat:      -6.914744,
		PickupLng:      107.609810,
		PickupAddress:  "Jl. Braga No.1, Bandung",
		RecipientName:  "Ani Wijaya",
		RecipientPhone: "+6289876543210",
		DestLat:        -6.921000,
		DestLng:        107.607000,
		DestAddress:    "Jl. Dago No.5, Bandung",
		PaymentMethod:  "gopay",
		FareEstimate:   sql.NullFloat64{Float64: 22100, Valid: true},
	}
}

// basePackage returns a minimal valid DeliveryPackage for seeding.
func basePackage() *repository.DeliveryPackage {
	return &repository.DeliveryPackage{
		Category:  "document",
		Size:      "small",
		IsFragile: false,
		WeightKg:  sql.NullFloat64{Float64: 0.5, Valid: true},
	}
}

// ── E2E: full lifecycle ──────────────────────────────────────────────────────

func TestDeliveryOrderRepository_FullLifecycleE2E(t *testing.T) {
	t.Log("Starting delivery-order functional E2E test (DB integration)…")

	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewDeliveryOrderRepository(db, nil)
	ctx := context.Background()

	userID := userUUID(t)
	driverID := driverUUID(t)

	// 1. Create order + package atomically
	o, err := repo.CreateOrder(ctx, baseOrder(userID), basePackage())
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if o.ID == "" {
		t.Fatal("expected generated ID for order")
	}
	t.Logf("Inserted order id=%s status=%s", o.ID, o.Status)

	// 2. Package should be retrievable
	pkg, err := repo.GetPackageByOrderID(ctx, o.ID)
	if err != nil {
		t.Fatalf("GetPackageByOrderID: %v", err)
	}
	if pkg.Category != "document" || pkg.Size != "small" {
		t.Errorf("unexpected package: %+v", pkg)
	}
	if pkg.OrderID != o.ID {
		t.Errorf("package order_id mismatch: want %s, got %s", o.ID, pkg.OrderID)
	}

	// 3. Active order visible to user
	active, err := repo.GetActiveOrderByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetActiveOrderByUserID: %v", err)
	}
	if active.ID != o.ID {
		t.Errorf("active order mismatch: %s vs %s", active.ID, o.ID)
	}

	// 4. Assign driver (finding_driver → assigned)
	if err := repo.AssignDriver(ctx, o.ID, driverID); err != nil {
		t.Fatalf("AssignDriver: %v", err)
	}
	if err := repo.InsertStateLog(ctx, &repository.DeliveryStateLog{
		OrderID:   o.ID,
		FromState: sql.NullString{String: "finding_driver", Valid: true},
		ToState:   "assigned",
		ActorID:   sql.NullString{String: driverID, Valid: true},
		ActorType: "driver",
	}); err != nil {
		t.Fatalf("InsertStateLog (assigned): %v", err)
	}

	// 5. Race condition: second driver should fail
	if err := repo.AssignDriver(ctx, o.ID, "00000000-0000-0000-0000-00000000000c"); err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows on race, got %v", err)
	}

	// 6. Driver arrives at pickup (assigned → on_pickup)
	if err := repo.UpdateStatus(ctx, o.ID, "assigned", "on_pickup"); err != nil {
		t.Fatalf("UpdateStatus (on_pickup): %v", err)
	}
	if err := repo.InsertStateLog(ctx, &repository.DeliveryStateLog{
		OrderID:   o.ID,
		FromState: sql.NullString{String: "assigned", Valid: true},
		ToState:   "on_pickup",
		ActorID:   sql.NullString{String: driverID, Valid: true},
		ActorType: "driver",
	}); err != nil {
		t.Fatalf("InsertStateLog (on_pickup): %v", err)
	}

	// 7. Driver picks up package (on_pickup → picked_up) + photo proof
	if err := repo.UpdateStatus(ctx, o.ID, "on_pickup", "picked_up"); err != nil {
		t.Fatalf("UpdateStatus (picked_up): %v", err)
	}
	pickupPhotoURL := "https://cdn.example.com/pickup-proof-001.jpg"
	if err := repo.SetPickupProof(ctx, o.ID, pickupPhotoURL); err != nil {
		t.Fatalf("SetPickupProof: %v", err)
	}

	// 8. After pickup, user cannot cancel
	if err := repo.SetCancelled(ctx, o.ID, "Mau batal", "user"); err != sql.ErrNoRows {
		t.Errorf("expected cancel to fail after picked_up, got err=%v", err)
	}

	// 9. Driver starts delivery (picked_up → on_delivery)
	if err := repo.UpdateStatus(ctx, o.ID, "picked_up", "on_delivery"); err != nil {
		t.Fatalf("UpdateStatus (on_delivery): %v", err)
	}

	// 10. Driver completes delivery (on_delivery → delivered) + delivery proof + distance
	if err := repo.UpdateStatus(ctx, o.ID, "on_delivery", "delivered"); err != nil {
		t.Fatalf("UpdateStatus (delivered): %v", err)
	}
	deliveryPhotoURL := "https://cdn.example.com/delivery-proof-001.jpg"
	if err := repo.SetDeliveryDetails(ctx, o.ID, deliveryPhotoURL, 5.2, 30); err != nil {
		t.Fatalf("SetDeliveryDetails: %v", err)
	}

	// 11. Calculate and persist fare breakdown
	fb := &repository.DeliveryFareBreakdown{
		OrderID:         o.ID,
		BaseFare:        5000,
		DistanceFare:    15600, // 5.2km * 3000
		WeightSurcharge: 0,
		InsuranceFee:    0,
		PromoDiscount:   0,
		PlatformFee:     1000,
		Total:           21600,
	}
	if err := repo.UpsertFareBreakdown(ctx, fb); err != nil {
		t.Fatalf("UpsertFareBreakdown: %v", err)
	}
	if err := repo.SetFareFinal(ctx, o.ID, fb.Total); err != nil {
		t.Fatalf("SetFareFinal: %v", err)
	}

	// 12. Verify fare breakdown persisted correctly
	storedFB, err := repo.GetFareBreakdown(ctx, o.ID)
	if err != nil {
		t.Fatalf("GetFareBreakdown: %v", err)
	}
	if storedFB.Total != 21600 {
		t.Errorf("want fare total=21600, got %.2f", storedFB.Total)
	}
	if storedFB.DistanceFare != 15600 {
		t.Errorf("want distance_fare=15600, got %.2f", storedFB.DistanceFare)
	}

	// 13. Upsert idempotency (ON CONFLICT DO UPDATE)
	fb.Total = 22000 // updated
	if err := repo.UpsertFareBreakdown(ctx, fb); err != nil {
		t.Fatalf("UpsertFareBreakdown (upsert): %v", err)
	}
	updated, err := repo.GetFareBreakdown(ctx, o.ID)
	if err != nil {
		t.Fatalf("GetFareBreakdown after upsert: %v", err)
	}
	if updated.Total != 22000 {
		t.Errorf("want updated total=22000 after upsert, got %.2f", updated.Total)
	}

	// 14. Final order state checks
	final, err := repo.GetOrderByID(ctx, o.ID)
	if err != nil {
		t.Fatalf("GetOrderByID (final): %v", err)
	}
	if final.Status != "delivered" {
		t.Errorf("want status=delivered, got %s", final.Status)
	}
	if !final.PickupPhotoURL.Valid || final.PickupPhotoURL.String != pickupPhotoURL {
		t.Errorf("want pickup_photo_url=%s, got %+v", pickupPhotoURL, final.PickupPhotoURL)
	}
	if !final.DeliveryPhotoURL.Valid || final.DeliveryPhotoURL.String != deliveryPhotoURL {
		t.Errorf("want delivery_photo_url=%s, got %+v", deliveryPhotoURL, final.DeliveryPhotoURL)
	}
	if !final.ActualDistanceKm.Valid || final.ActualDistanceKm.Float64 != 5.2 {
		t.Errorf("want actual_distance_km=5.2, got %+v", final.ActualDistanceKm)
	}
	if !final.ActualDurationMin.Valid || final.ActualDurationMin.Int32 != 30 {
		t.Errorf("want actual_duration_min=30, got %+v", final.ActualDurationMin)
	}
	if !final.PickedUpAt.Valid {
		t.Error("want picked_up_at to be set")
	}
	if !final.DeliveredAt.Valid {
		t.Error("want delivered_at to be set")
	}

	// 15. State logs captured
	logs, err := repo.ListStateLogs(ctx, o.ID)
	if err != nil {
		t.Fatalf("ListStateLogs: %v", err)
	}
	if len(logs) < 2 {
		t.Errorf("want at least 2 state logs, got %d", len(logs))
	}

	// 16. No active order after delivery
	_, err = repo.GetActiveOrderByUserID(ctx, userID)
	if err != sql.ErrNoRows {
		t.Errorf("expected no active order after delivery, got err=%v", err)
	}
}

// ── E2E: cancellation by user ────────────────────────────────────────────────

func TestDeliveryOrderRepository_CancelE2E(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewDeliveryOrderRepository(db, nil)
	ctx := context.Background()
	userID := userUUID(t)

	// Create an order in finding_driver state
	o, err := repo.CreateOrder(ctx, &repository.DeliveryOrder{
		UserID:         userID,
		Status:         "finding_driver",
		SenderName:     "Budi",
		SenderPhone:    "+6281111",
		PickupLat:      -6.91, PickupLng: 107.60,
		PickupAddress:  "Jl. Braga",
		RecipientName:  "Ani",
		RecipientPhone: "+6282222",
		DestLat:        -6.92, DestLng: 107.61,
		DestAddress:    "Jl. Dago",
		PaymentMethod:  "cash",
	}, &repository.DeliveryPackage{
		Category: "food",
		Size:     "medium",
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}

	// Cancel before driver assigned — must succeed
	if err := repo.SetCancelled(ctx, o.ID, "Salah pilih tujuan", "user"); err != nil {
		t.Fatalf("SetCancelled: %v", err)
	}

	got, err := repo.GetOrderByID(ctx, o.ID)
	if err != nil {
		t.Fatalf("GetOrderByID: %v", err)
	}
	if got.Status != "cancelled" {
		t.Errorf("want status=cancelled, got %s", got.Status)
	}
	if !got.CancelReason.Valid || got.CancelReason.String != "Salah pilih tujuan" {
		t.Errorf("unexpected cancel_reason: %+v", got.CancelReason)
	}
	if !got.CancelledBy.Valid || got.CancelledBy.String != "user" {
		t.Errorf("unexpected cancelled_by: %+v", got.CancelledBy)
	}

	// Double-cancel must fail (already cancelled)
	if err := repo.SetCancelled(ctx, o.ID, "double cancel", "user"); err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows on double cancel, got %v", err)
	}
}

// ── E2E: history paging ──────────────────────────────────────────────────────

func TestDeliveryOrderRepository_HistoryE2E(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewDeliveryOrderRepository(db, nil)
	ctx := context.Background()
	userID := userUUID(t)
	driverID := driverUUID(t)

	// Seed 3 orders, drive each to delivered state
	for i := 0; i < 3; i++ {
		o, err := repo.CreateOrder(ctx, baseOrder(userID), basePackage())
		if err != nil {
			t.Fatalf("CreateOrder %d: %v", i, err)
		}

		if err := repo.AssignDriver(ctx, o.ID, driverID); err != nil {
			t.Fatalf("AssignDriver %d: %v", i, err)
		}
		if err := repo.UpdateStatus(ctx, o.ID, "assigned", "on_pickup"); err != nil {
			t.Fatalf("on_pickup %d: %v", i, err)
		}
		if err := repo.UpdateStatus(ctx, o.ID, "on_pickup", "picked_up"); err != nil {
			t.Fatalf("picked_up %d: %v", i, err)
		}
		if err := repo.SetPickupProof(ctx, o.ID, "https://cdn.example.com/pickup.jpg"); err != nil {
			t.Fatalf("pickup proof %d: %v", i, err)
		}
		if err := repo.UpdateStatus(ctx, o.ID, "picked_up", "on_delivery"); err != nil {
			t.Fatalf("on_delivery %d: %v", i, err)
		}
		if err := repo.UpdateStatus(ctx, o.ID, "on_delivery", "delivered"); err != nil {
			t.Fatalf("delivered %d: %v", i, err)
		}
		if err := repo.SetDeliveryDetails(ctx, o.ID, "https://cdn.example.com/delivery.jpg", 4.0, 25); err != nil {
			t.Fatalf("delivery details %d: %v", i, err)
		}
	}

	// Page 1: limit=2, all 3 delivered
	rows, total, err := repo.ListUserHistory(ctx, userID, repository.HistoryFilter{
		Status: "delivered", Limit: 2, Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListUserHistory page1: %v", err)
	}
	if total != 3 {
		t.Errorf("want total=3, got %d", total)
	}
	if len(rows) != 2 {
		t.Errorf("want 2 rows on page 1, got %d", len(rows))
	}

	// Page 2: should return the remaining 1
	rows2, _, err := repo.ListUserHistory(ctx, userID, repository.HistoryFilter{
		Status: "delivered", Limit: 2, Offset: 2,
	})
	if err != nil {
		t.Fatalf("ListUserHistory page2: %v", err)
	}
	if len(rows2) != 1 {
		t.Errorf("want 1 row on page 2, got %d", len(rows2))
	}

	// No-filter variant (all history)
	all, allTotal, err := repo.ListUserHistory(ctx, userID, repository.HistoryFilter{
		Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListUserHistory (no filter): %v", err)
	}
	if allTotal != 3 {
		t.Errorf("want allTotal=3, got %d", allTotal)
	}
	if len(all) != 3 {
		t.Errorf("want 3 rows (no filter), got %d", len(all))
	}
}
