//go:build functional

// Package functional contains end-to-end integration tests that connect
// directly to a real PostgreSQL instance provisioned via docker-compose.
//
// These tests will FAIL when the docker compose stack is not running
// (no DB to connect to) and PASS once `docker compose up -d` has provisioned
// PostgreSQL on localhost:5446.
//
// Run with:
//   docker compose up -d
//   go test -tags=functional -v ./test/functional/...
package functional

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/zicofarry/clay-app/backend/services/ride-order-service/internal/repository"
)

// dsn returns the test DSN, allowing CI override via TEST_DATABASE_URL.
func dsn() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://clay_user:clay_password@localhost:5446/ride_order_db?sslmode=disable"
}

// setupTestDB connects to the docker-compose PostgreSQL, applies the schema
// and returns a clean *sql.DB. Fails the test if the DB is unreachable.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("postgres", dsn())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	// Wait up to 10s for the DB to become reachable. If it never does,
	// fail loudly so the developer knows to start docker-compose.
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
	if _, err := db.Exec(`TRUNCATE TABLE ride_orders, order_state_logs, trip_details, order_fare_breakdown CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

const schemaDDL = `
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS ride_orders (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL,
    driver_id       UUID,
    service_type    VARCHAR(20) NOT NULL CHECK (service_type IN ('goride','gocar')),
    vehicle_type    VARCHAR(20) NOT NULL CHECK (vehicle_type IN ('motor','car')),
    status          VARCHAR(30) NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending','finding_driver','assigned','on_pickup','on_trip','completed','cancelled')),
    origin_lat      DECIMAL(10,7) NOT NULL,
    origin_lng      DECIMAL(10,7) NOT NULL,
    origin_address  TEXT,
    dest_lat        DECIMAL(10,7) NOT NULL,
    dest_lng        DECIMAL(10,7) NOT NULL,
    dest_address    TEXT,
    fare_estimate   DECIMAL(12,2),
    fare_final      DECIMAL(12,2),
    promo_id        UUID,
    payment_method  VARCHAR(20) NOT NULL CHECK (payment_method IN ('gopay','cash')),
    otp_code        VARCHAR(6),
    cancel_reason   TEXT,
    cancelled_by    VARCHAR(20) CHECK (cancelled_by IN ('user','driver','system')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ride_orders_user_id    ON ride_orders(user_id);
CREATE INDEX IF NOT EXISTS idx_ride_orders_driver_id  ON ride_orders(driver_id);
CREATE INDEX IF NOT EXISTS idx_ride_orders_status     ON ride_orders(status);
CREATE INDEX IF NOT EXISTS idx_ride_orders_created_at ON ride_orders(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ride_orders_user_status ON ride_orders(user_id, status);

CREATE TABLE IF NOT EXISTS order_state_logs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id    UUID NOT NULL REFERENCES ride_orders(id) ON DELETE CASCADE,
    from_state  VARCHAR(30),
    to_state    VARCHAR(30) NOT NULL,
    actor_id    UUID,
    actor_type  VARCHAR(20) NOT NULL CHECK (actor_type IN ('user','driver','system')),
    reason      TEXT,
    metadata    JSONB,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_state_logs_order_id  ON order_state_logs(order_id);
CREATE INDEX IF NOT EXISTS idx_state_logs_changed   ON order_state_logs(changed_at DESC);

CREATE TABLE IF NOT EXISTS trip_details (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id             UUID NOT NULL UNIQUE REFERENCES ride_orders(id) ON DELETE CASCADE,
    polyline             TEXT,
    est_distance_km      DECIMAL(8,3),
    est_duration_min     INTEGER,
    actual_distance_km   DECIMAL(8,3),
    actual_duration_min  INTEGER,
    route_deviation_km   DECIMAL(8,3),
    pickup_time          TIMESTAMPTZ,
    dropoff_time         TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS order_fare_breakdown (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id          UUID NOT NULL UNIQUE REFERENCES ride_orders(id) ON DELETE CASCADE,
    base_fare         DECIMAL(12,2) NOT NULL,
    distance_fare     DECIMAL(12,2) NOT NULL,
    time_fare         DECIMAL(12,2) NOT NULL,
    surge_multiplier  DECIMAL(4,2)  NOT NULL DEFAULT 1.0,
    promo_discount    DECIMAL(12,2) NOT NULL DEFAULT 0,
    platform_fee      DECIMAL(12,2) NOT NULL DEFAULT 0,
    total             DECIMAL(12,2) NOT NULL
);
`

// userUUID returns a deterministic UUID for tests.
func userUUID(t *testing.T) string {
	t.Helper()
	return "00000000-0000-0000-0000-00000000000a"
}

func driverUUID(t *testing.T) string {
	t.Helper()
	return "00000000-0000-0000-0000-00000000000b"
}

// ── E2E flow: create → assign → on_pickup → start → complete ────────────────

func TestRideOrderRepository_FullLifecycleE2E(t *testing.T) {
	t.Log("Starting ride-order functional E2E test (DB integration)…")

	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewRideOrderRepository(db, nil)
	ctx := context.Background()

	userID := userUUID(t)
	driverID := driverUUID(t)

	// 1. Create order
	o, err := repo.CreateOrder(ctx, &repository.RideOrder{
		UserID:        userID,
		ServiceType:   "goride",
		VehicleType:   "motor",
		Status:        "finding_driver",
		OriginLat:     -6.914744,
		OriginLng:     107.609810,
		OriginAddress: sql.NullString{String: "Jl. Braga No.1, Bandung", Valid: true},
		DestLat:       -6.921000,
		DestLng:       107.607000,
		DestAddress:   sql.NullString{String: "Jl. Dago No.5, Bandung", Valid: true},
		PaymentMethod: "gopay",
		FareEstimate:  sql.NullFloat64{Float64: 18000, Valid: true},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if o.ID == "" {
		t.Fatal("expected generated ID")
	}
	t.Logf("Inserted order id=%s status=%s", o.ID, o.Status)

	// 2. Active order should match
	active, err := repo.GetActiveOrderByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("active: %v", err)
	}
	if active.ID != o.ID {
		t.Errorf("active mismatch: %s vs %s", active.ID, o.ID)
	}

	// 3. Assign driver
	if err := repo.AssignDriver(ctx, o.ID, driverID, "847291"); err != nil {
		t.Fatalf("assign: %v", err)
	}
	if err := repo.InsertStateLog(ctx, &repository.OrderStateLog{
		OrderID:   o.ID,
		FromState: sql.NullString{String: "finding_driver", Valid: true},
		ToState:   "assigned",
		ActorID:   sql.NullString{String: driverID, Valid: true},
		ActorType: "driver",
	}); err != nil {
		t.Fatalf("state log: %v", err)
	}

	// 4. Race: another driver should fail
	if err := repo.AssignDriver(ctx, o.ID, "00000000-0000-0000-0000-00000000000c", "111111"); err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows on race, got %v", err)
	}

	// 5. Driver progresses: assigned -> on_pickup -> on_trip -> completed
	if err := repo.UpdateStatus(ctx, o.ID, "assigned", "on_pickup"); err != nil {
		t.Fatalf("on_pickup: %v", err)
	}
	if err := repo.UpdateStatus(ctx, o.ID, "on_pickup", "on_trip"); err != nil {
		t.Fatalf("on_trip: %v", err)
	}
	if err := repo.UpsertTripDetails(ctx, &repository.TripDetails{
		OrderID:    o.ID,
		PickupTime: sql.NullTime{Time: time.Now().UTC(), Valid: true},
	}); err != nil {
		t.Fatalf("trip details: %v", err)
	}
	if err := repo.UpdateStatus(ctx, o.ID, "on_trip", "completed"); err != nil {
		t.Fatalf("completed: %v", err)
	}

	// 6. Final fare + breakdown
	if err := repo.UpsertFareBreakdown(ctx, &repository.FareBreakdown{
		OrderID:         o.ID,
		BaseFare:        5000,
		DistanceFare:    8000,
		TimeFare:        3000,
		SurgeMultiplier: 1.0,
		PromoDiscount:   0,
		PlatformFee:     1000,
		Total:           17000,
	}); err != nil {
		t.Fatalf("upsert fare: %v", err)
	}
	if err := repo.SetFareFinal(ctx, o.ID, 17000); err != nil {
		t.Fatalf("fare final: %v", err)
	}

	fb, err := repo.GetFareBreakdown(ctx, o.ID)
	if err != nil {
		t.Fatalf("get fare: %v", err)
	}
	if fb.Total != 17000 {
		t.Errorf("want total=17000, got %.2f", fb.Total)
	}

	// 7. Final order state
	final, err := repo.GetOrderByID(ctx, o.ID)
	if err != nil {
		t.Fatalf("final get: %v", err)
	}
	if final.Status != "completed" {
		t.Errorf("want status=completed, got %s", final.Status)
	}
	if !final.FareFinal.Valid || final.FareFinal.Float64 != 17000 {
		t.Errorf("want fare_final=17000, got %+v", final.FareFinal)
	}

	// 8. State logs persisted
	logs, err := repo.ListStateLogs(ctx, o.ID)
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if len(logs) == 0 {
		t.Error("expected at least one state log")
	}

	// 9. After completion the user has no active order
	_, err = repo.GetActiveOrderByUserID(ctx, userID)
	if err != sql.ErrNoRows {
		t.Errorf("expected no active order, got err=%v", err)
	}
}

// ── E2E: cancellation by user ───────────────────────────────────────────────

func TestRideOrderRepository_CancelE2E(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewRideOrderRepository(db, nil)
	ctx := context.Background()
	userID := userUUID(t)

	o, err := repo.CreateOrder(ctx, &repository.RideOrder{
		UserID:        userID,
		ServiceType:   "gocar",
		VehicleType:   "car",
		Status:        "finding_driver",
		OriginLat:     -6.91, OriginLng: 107.6,
		DestLat: -6.92, DestLng: 107.61,
		PaymentMethod: "cash",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.SetCancelled(ctx, o.ID, "Salah pilih tujuan", "user"); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	got, err := repo.GetOrderByID(ctx, o.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "cancelled" {
		t.Errorf("want cancelled, got %s", got.Status)
	}
	if !got.CancelReason.Valid || got.CancelReason.String != "Salah pilih tujuan" {
		t.Errorf("unexpected cancel_reason: %+v", got.CancelReason)
	}
}

// ── E2E: history paging ─────────────────────────────────────────────────────

func TestRideOrderRepository_HistoryE2E(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewRideOrderRepository(db, nil)
	ctx := context.Background()
	userID := userUUID(t)

	for i := 0; i < 3; i++ {
		o, err := repo.CreateOrder(ctx, &repository.RideOrder{
			UserID:        userID,
			ServiceType:   "goride",
			VehicleType:   "motor",
			Status:        "finding_driver",
			OriginLat:     -6.91, OriginLng: 107.6,
			DestLat: -6.92, DestLng: 107.61,
			PaymentMethod: "gopay",
		})
		if err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
		// Move to terminal state so history filter `status=completed` returns it.
		if err := repo.UpdateStatus(ctx, o.ID, "finding_driver", "assigned"); err != nil {
			t.Fatalf("assign: %v", err)
		}
		if err := repo.UpdateStatus(ctx, o.ID, "assigned", "on_pickup"); err != nil {
			t.Fatalf("pickup: %v", err)
		}
		if err := repo.UpdateStatus(ctx, o.ID, "on_pickup", "on_trip"); err != nil {
			t.Fatalf("trip: %v", err)
		}
		if err := repo.UpdateStatus(ctx, o.ID, "on_trip", "completed"); err != nil {
			t.Fatalf("complete: %v", err)
		}
	}

	rows, total, err := repo.ListUserHistory(ctx, userID, repository.HistoryFilter{
		Status: "completed", Limit: 2, Offset: 0,
	})
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if total != 3 {
		t.Errorf("want total=3, got %d", total)
	}
	if len(rows) != 2 {
		t.Errorf("want 2 rows on first page, got %d", len(rows))
	}
}
