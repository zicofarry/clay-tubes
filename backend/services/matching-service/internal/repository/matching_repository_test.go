//go:build unit

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newRepo wires a MatchingRepository against an in-process miniredis.
func newRepo(t *testing.T) (*MatchingRepository, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewMatchingRepository(rdb), mr
}

const (
	rDriverID = "driver-uuid-1"
	rOrderID  = "order-uuid-1"
	rZoneID   = "zone-cbd"
)

// ── Driver status ──────────────────────────────────────────────────────────

func TestRepo_DriverStatus_RoundTrip(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()

	st := &DriverStatus{
		DriverID: rDriverID, Status: "online", VehicleType: "motor",
		Lat: -6.91, Lng: 107.6,
	}
	if err := r.SetDriverStatus(ctx, st); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetDriverStatus(ctx, rDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "online" || got.VehicleType != "motor" || got.Lat != -6.91 {
		t.Errorf("unexpected: %+v", got)
	}
	if got.UpdatedAt.IsZero() || got.OnlineSince.IsZero() {
		t.Error("timestamps should auto-populate")
	}
}

func TestRepo_DriverStatus_NotFound(t *testing.T) {
	r, _ := newRepo(t)
	_, err := r.GetDriverStatus(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestRepo_DriverStatus_RejectsEmptyID(t *testing.T) {
	r, _ := newRepo(t)
	if err := r.SetDriverStatus(context.Background(), &DriverStatus{}); err == nil {
		t.Error("expected error for empty driver id")
	}
	if err := r.SetDriverStatus(context.Background(), nil); err == nil {
		t.Error("expected error for nil status")
	}
}

func TestRepo_RefreshDriverStatusTTL(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()

	if _, err := r.RefreshDriverStatusTTL(ctx, "ghost"); !errors.Is(err, ErrNotFound) {
		t.Errorf("ghost driver → ErrNotFound, got %v", err)
	}

	_ = r.SetDriverStatus(ctx, &DriverStatus{DriverID: rDriverID, Status: "online"})
	d, err := r.RefreshDriverStatusTTL(ctx, rDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if d != StatusTTL {
		t.Errorf("want %v, got %v", StatusTTL, d)
	}
}

func TestRepo_DeleteDriverStatus(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	_ = r.SetDriverStatus(ctx, &DriverStatus{DriverID: rDriverID, Status: "online"})
	if err := r.DeleteDriverStatus(ctx, rDriverID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetDriverStatus(ctx, rDriverID); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete → ErrNotFound, got %v", err)
	}
}

func TestRepo_UpdateDriverLocation(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	_ = r.SetDriverStatus(ctx, &DriverStatus{DriverID: rDriverID, Status: "online", Lat: 0, Lng: 0})
	if err := r.UpdateDriverLocation(ctx, rDriverID, 1.234, 5.678); err != nil {
		t.Fatal(err)
	}
	got, _ := r.GetDriverStatus(ctx, rDriverID)
	if got.Lat != 1.234 || got.Lng != 5.678 {
		t.Errorf("location not updated: %+v", got)
	}
}

// ── Geo index ──────────────────────────────────────────────────────────────

func TestRepo_GeoIndex_AddAndSearch(t *testing.T) {
	t.Skip("miniredis does not support GEOSEARCH; full geo radius tests run in functional/repository_test.go against real Redis")
	r, _ := newRepo(t)
	ctx := context.Background()

	// Two drivers within 2km of pickup (-6.91, 107.6); one ~5km away.
	must(t, r.AddDriverToGeo(ctx, "motor", "near-1", -6.911, 107.601))
	must(t, r.AddDriverToGeo(ctx, "motor", "near-2", -6.915, 107.605))
	must(t, r.AddDriverToGeo(ctx, "motor", "far",    -6.95, 107.65))

	res, err := r.NearbyDrivers(ctx, "motor", -6.91, 107.6, 2.0, 10)
	if err != nil {
		t.Fatalf("NearbyDrivers: %v", err)
	}
	if len(res) < 1 {
		t.Fatalf("want at least 1 nearby driver, got %d", len(res))
	}
	for _, d := range res {
		if d.DriverID == "far" {
			t.Errorf("far driver shouldn't appear in 2km radius: %+v", d)
		}
	}
}

func TestRepo_GeoIndex_RejectsInvalidCoords(t *testing.T) {
	r, _ := newRepo(t)
	if err := r.AddDriverToGeo(context.Background(), "motor", "x", 999, 0); err == nil {
		t.Error("expected error for invalid lat")
	}
}

func TestRepo_GeoIndex_Remove(t *testing.T) {
	t.Skip("miniredis does not support GEOSEARCH; full geo radius tests run in functional/repository_test.go against real Redis")
	r, _ := newRepo(t)
	ctx := context.Background()
	must(t, r.AddDriverToGeo(ctx, "motor", "x", -6.91, 107.6))
	must(t, r.RemoveDriverFromGeo(ctx, "motor", "x"))
	res, _ := r.NearbyDrivers(ctx, "motor", -6.91, 107.6, 5.0, 10)
	for _, d := range res {
		if d.DriverID == "x" {
			t.Error("driver should be removed from index")
		}
	}
}

// ── Driver mode ────────────────────────────────────────────────────────────

func TestRepo_DriverMode_RoundTrip(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	if _, err := r.GetDriverMode(ctx, rDriverID); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing → ErrNotFound, got %v", err)
	}
	must(t, r.SetDriverMode(ctx, rDriverID, &DriverMode{Mode: "priority", DailyTarget: 200000}))
	got, err := r.GetDriverMode(ctx, rDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "priority" || got.DailyTarget != 200000 {
		t.Errorf("unexpected: %+v", got)
	}
	must(t, r.DeleteDriverMode(ctx, rDriverID))
	if _, err := r.GetDriverMode(ctx, rDriverID); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete → ErrNotFound, got %v", err)
	}
}

func TestRepo_DriverMode_NilRejected(t *testing.T) {
	r, _ := newRepo(t)
	if err := r.SetDriverMode(context.Background(), rDriverID, nil); err == nil {
		t.Error("expected error for nil mode")
	}
}

// ── Earnings ───────────────────────────────────────────────────────────────

func TestRepo_Earnings_Increment(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	const date = "2026-04-28"

	must(t, r.IncrementDriverEarnings(ctx, rDriverID, date, 25000))
	must(t, r.IncrementDriverEarnings(ctx, rDriverID, date, 30000))

	got, err := r.GetDriverEarnings(ctx, rDriverID, date)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalEarnings != 55000 || got.TripCount != 2 {
		t.Errorf("want 55000/2, got %+v", got)
	}
}

func TestRepo_Earnings_GetEmptyReturnsZeroes(t *testing.T) {
	r, _ := newRepo(t)
	got, err := r.GetDriverEarnings(context.Background(), rDriverID, "2026-04-28")
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalEarnings != 0 || got.TripCount != 0 {
		t.Errorf("empty earnings should be zero, got %+v", got)
	}
}

// ── Acceptance counters ────────────────────────────────────────────────────

func TestRepo_Acceptance(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		_ = r.IncrementAcceptance(ctx, rDriverID, true)
	}
	_ = r.IncrementAcceptance(ctx, rDriverID, false)

	got, err := r.GetAcceptance(ctx, rDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Accepted != 4 || got.Rejected != 1 {
		t.Errorf("want 4/1, got %+v", got)
	}
}

// ── Active order lock (SETNX) ──────────────────────────────────────────────

func TestRepo_ActiveOrder_SETNXBehavior(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()

	// First set succeeds
	if err := r.SetActiveOrder(ctx, rDriverID, rOrderID); err != nil {
		t.Fatal(err)
	}
	// Second set on same driver must conflict
	if err := r.SetActiveOrder(ctx, rDriverID, "another-order"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second SETNX should fail with ErrNotFound (caller treats as conflict), got %v", err)
	}
	// Read back
	v, err := r.GetActiveOrder(ctx, rDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if v != rOrderID {
		t.Errorf("want %s, got %s", rOrderID, v)
	}
	// Clear, then SETNX should work again
	must(t, r.ClearActiveOrder(ctx, rDriverID))
	if _, err := r.GetActiveOrder(ctx, rDriverID); !errors.Is(err, ErrNotFound) {
		t.Errorf("cleared → ErrNotFound, got %v", err)
	}
	if err := r.SetActiveOrder(ctx, rDriverID, "new-order"); err != nil {
		t.Errorf("after clear, SETNX should succeed, got %v", err)
	}
}

// ── Cached rating ──────────────────────────────────────────────────────────

func TestRepo_CachedRating(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()

	if _, err := r.GetCachedRating(ctx, rDriverID); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing → ErrNotFound, got %v", err)
	}
	must(t, r.SetCachedRating(ctx, rDriverID, 4.85))
	got, err := r.GetCachedRating(ctx, rDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if got != 4.85 {
		t.Errorf("want 4.85, got %f", got)
	}
}

// ── Matching session ───────────────────────────────────────────────────────

func TestRepo_Session_CreateGetUpdateDelete(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()

	sess := &MatchingSession{
		OrderID: rOrderID, SessionID: "sess-1", ServiceType: "ride",
		Status: "searching", PickupLat: -6.91, PickupLng: 107.6,
		SearchRadiusKm: 5, MaxRounds: 5,
	}
	must(t, r.CreateSession(ctx, sess))

	// Duplicate create rejected
	if err := r.CreateSession(ctx, sess); !errors.Is(err, ErrNotFound) {
		t.Errorf("duplicate → ErrNotFound (treated as conflict), got %v", err)
	}

	got, err := r.GetSession(ctx, rOrderID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != "sess-1" || got.ServiceType != "ride" {
		t.Errorf("unexpected: %+v", got)
	}

	got.Status = "driver_found"
	got.MatchedDriverID = rDriverID
	got.MatchedAt = time.Now().UTC()
	must(t, r.UpdateSession(ctx, got))

	got2, _ := r.GetSession(ctx, rOrderID)
	if got2.Status != "driver_found" || got2.MatchedDriverID != rDriverID {
		t.Errorf("update didn't persist: %+v", got2)
	}

	must(t, r.DeleteSession(ctx, rOrderID))
	if _, err := r.GetSession(ctx, rOrderID); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete → ErrNotFound, got %v", err)
	}
}

func TestRepo_Session_RejectsEmptyOrderID(t *testing.T) {
	r, _ := newRepo(t)
	if err := r.CreateSession(context.Background(), &MatchingSession{}); err == nil {
		t.Error("expected error for empty order id")
	}
	if err := r.UpdateSession(context.Background(), &MatchingSession{}); err == nil {
		t.Error("expected error for empty order id on update")
	}
}

// ── Reject markers ─────────────────────────────────────────────────────────

func TestRepo_RejectMarker(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()

	rejected, err := r.IsRejected(ctx, rOrderID, rDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if rejected {
		t.Error("fresh order/driver should not be rejected")
	}

	must(t, r.MarkRejected(ctx, rOrderID, rDriverID))
	rejected, _ = r.IsRejected(ctx, rOrderID, rDriverID)
	if !rejected {
		t.Error("after mark, IsRejected should return true")
	}
}

// ── Zone stats ─────────────────────────────────────────────────────────────

func TestRepo_ZoneStats_NotFound(t *testing.T) {
	r, _ := newRepo(t)
	if _, err := r.GetZoneStats(context.Background(), "motor", rZoneID); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing → ErrNotFound, got %v", err)
	}
}

func TestRepo_ZoneStats_Upsert(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()

	must(t, r.UpsertZoneStats(ctx, "motor", rZoneID, 12, 18))
	got, err := r.GetZoneStats(ctx, "motor", rZoneID)
	if err != nil {
		t.Fatal(err)
	}
	if got.OnlineDrivers != 12 || got.PendingOrders != 18 {
		t.Errorf("unexpected: %+v", got)
	}

	// Overwrite
	must(t, r.UpsertZoneStats(ctx, "motor", rZoneID, 5, 30))
	got, _ = r.GetZoneStats(ctx, "motor", rZoneID)
	if got.OnlineDrivers != 5 || got.PendingOrders != 30 {
		t.Errorf("overwrite failed: %+v", got)
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
}
