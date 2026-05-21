//go:build functional

package functional

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zicofarry/clay-app/backend/services/matching-service/internal/repository"
)

const (
	fDriverID  = "fdriver-1"
	fDriverID2 = "fdriver-2"
	fOrderID   = "forder-1"
	fZoneID    = "fzone-cbd"
)

// TestFunctional_DriverStatus_TTLAndRoundTrip validates that the StatusTTL set
// by SetDriverStatus is actually applied at the Redis level — something
// miniredis can fake but only a real Redis server proves end-to-end.
func TestFunctional_DriverStatus_TTLAndRoundTrip(t *testing.T) {
	repo, rdb := newRepo(t)
	ctx := context.Background()

	st := &repository.DriverStatus{
		DriverID: fDriverID, Status: "online", VehicleType: "motor",
		Lat: -6.91, Lng: 107.6,
	}
	if err := repo.SetDriverStatus(ctx, st); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetDriverStatus(ctx, fDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "online" || got.Lat != -6.91 {
		t.Errorf("round-trip mismatch: %+v", got)
	}

	// TTL should be in (0, StatusTTL]
	ttl, err := rdb.TTL(ctx, "driver:status:"+fDriverID).Result()
	if err != nil {
		t.Fatal(err)
	}
	if ttl <= 0 || ttl > repository.StatusTTL {
		t.Errorf("TTL out of range: %v (want (0,%v])", ttl, repository.StatusTTL)
	}
}

func TestFunctional_GeoIndex_RealRadiusSearch(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()

	// Bandung CBD pickup point
	pickupLat, pickupLng := -6.91, 107.6

	// Drivers at varying distances
	drivers := []struct {
		id  string
		lat float64
		lng float64
	}{
		{"near-1", -6.911, 107.601}, // ~150m
		{"near-2", -6.913, 107.604}, // ~500m
		{"mid",    -6.92,  107.61},  // ~1.5km
		{"far",    -6.97,  107.66},  // ~9km
	}
	for _, d := range drivers {
		if err := repo.AddDriverToGeo(ctx, "motor", d.id, d.lat, d.lng); err != nil {
			t.Fatalf("AddDriverToGeo(%s): %v", d.id, err)
		}
	}

	res, err := repo.NearbyDrivers(ctx, "motor", pickupLat, pickupLng, 2.0, 10)
	if err != nil {
		t.Fatal(err)
	}
	// We expect near-1, near-2, mid (all <2km) but not far.
	seen := map[string]bool{}
	for _, d := range res {
		seen[d.DriverID] = true
		if d.DistanceKm > 2.0 {
			t.Errorf("driver %s outside radius: %f km", d.DriverID, d.DistanceKm)
		}
	}
	if !seen["near-1"] || !seen["near-2"] {
		t.Errorf("expected near-1 and near-2 in results, got %+v", res)
	}
	if seen["far"] {
		t.Error("far driver (~9km) shouldn't appear in 2km radius search")
	}

	// Sorted ascending by distance
	for i := 1; i < len(res); i++ {
		if res[i-1].DistanceKm > res[i].DistanceKm {
			t.Errorf("results not sorted by distance asc: %+v", res)
			break
		}
	}
}

func TestFunctional_ActiveOrder_SETNXAcrossClients(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()

	if err := repo.SetActiveOrder(ctx, fDriverID, fOrderID); err != nil {
		t.Fatal(err)
	}
	// Second SETNX must conflict — the lock is real.
	if err := repo.SetActiveOrder(ctx, fDriverID, "another-order"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("second SETNX should conflict, got %v", err)
	}
	// Cleared, the lock should release.
	if err := repo.ClearActiveOrder(ctx, fDriverID); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetActiveOrder(ctx, fDriverID, "fresh-order"); err != nil {
		t.Errorf("after clear, SETNX should succeed: %v", err)
	}
}

func TestFunctional_RejectMarker_TTL(t *testing.T) {
	repo, rdb := newRepo(t)
	ctx := context.Background()

	if err := repo.MarkRejected(ctx, fOrderID, fDriverID); err != nil {
		t.Fatal(err)
	}
	rejected, err := repo.IsRejected(ctx, fOrderID, fDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if !rejected {
		t.Fatal("should be rejected after MarkRejected")
	}

	// Reject markers carry RejectTTL — verify the key actually has a TTL.
	ttl, err := rdb.TTL(ctx, "matching:reject:"+fOrderID+":"+fDriverID).Result()
	if err != nil {
		t.Fatal(err)
	}
	if ttl <= 0 {
		t.Errorf("reject marker should have positive TTL, got %v", ttl)
	}
}

func TestFunctional_Earnings_AccumulationAndTTL(t *testing.T) {
	repo, rdb := newRepo(t)
	ctx := context.Background()

	const date = "2026-04-28"
	for _, fare := range []int{15000, 22000, 18000} {
		if err := repo.IncrementDriverEarnings(ctx, fDriverID, date, fare); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.GetDriverEarnings(ctx, fDriverID, date)
	if err != nil {
		t.Fatal(err)
	}
	if got.TotalEarnings != 55000 || got.TripCount != 3 {
		t.Errorf("want 55000/3, got %+v", got)
	}

	ttl, _ := rdb.TTL(ctx, "driver:earnings:"+fDriverID+":"+date).Result()
	if ttl <= 0 || ttl > repository.EarningsTTL {
		t.Errorf("earnings TTL out of range: %v", ttl)
	}
}

func TestFunctional_Session_Lifecycle(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()

	sess := &repository.MatchingSession{
		OrderID: fOrderID, SessionID: "s1", ServiceType: "ride",
		Status: "searching", PickupLat: -6.91, PickupLng: 107.6,
		SearchRadiusKm: 5, MaxRounds: 5, CreatedAt: time.Now().UTC(),
	}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	// Round-trip
	got, err := repo.GetSession(ctx, fOrderID)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != "s1" || got.Status != "searching" {
		t.Errorf("unexpected: %+v", got)
	}

	// Update with offer expiry & current candidate
	got.CurrentCandidateID = fDriverID
	got.OfferExpiresAt = time.Now().Add(15 * time.Second).UTC()
	got.CandidatesTried = 1
	if err := repo.UpdateSession(ctx, got); err != nil {
		t.Fatal(err)
	}

	got2, _ := repo.GetSession(ctx, fOrderID)
	if got2.CurrentCandidateID != fDriverID || got2.CandidatesTried != 1 {
		t.Errorf("update missing fields: %+v", got2)
	}

	// Delete
	if err := repo.DeleteSession(ctx, fOrderID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetSession(ctx, fOrderID); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("after delete → ErrNotFound, got %v", err)
	}
}

func TestFunctional_ZoneStats_Snapshot(t *testing.T) {
	repo, _ := newRepo(t)
	ctx := context.Background()

	if err := repo.UpsertZoneStats(ctx, "motor", fZoneID, 12, 18); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetZoneStats(ctx, "motor", fZoneID)
	if err != nil {
		t.Fatal(err)
	}
	if got.OnlineDrivers != 12 || got.PendingOrders != 18 {
		t.Errorf("unexpected: %+v", got)
	}
}
