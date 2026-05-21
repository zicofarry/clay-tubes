//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/zicofarry/clay-app/backend/services/matching-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/matching-service/mocks/geomock"
	"github.com/zicofarry/clay-app/backend/services/matching-service/mocks/repomock"
)

// ── Test helpers ───────────────────────────────────────────────────────────

const (
	tDriverID  = "driver-aaa"
	tDriverID2 = "driver-bbb"
	tOrderID   = "order-xyz"
	tZoneID    = "zone-cbd"
)

var fixedNow = time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC)

// newSvc wires a MatchingService with both mocks and a deterministic clock.
func newSvc(t *testing.T) (
	*MatchingService,
	*repomock.MockMatchingRepositoryInterface,
	*geomock.MockClient,
	*gomock.Controller,
) {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := repomock.NewMockMatchingRepositoryInterface(ctrl)
	geoCli := geomock.NewMockClient(ctrl)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewMatchingService(repo, geoCli, logger)
	svc.now = func() time.Time { return fixedNow }
	return svc, repo, geoCli, ctrl
}

// ── Pure helper tests ──────────────────────────────────────────────────────

func TestVehicleTypeForService(t *testing.T) {
	cases := map[string]string{
		"ride":     "motor",
		"delivery": "motor",
		"food":     "motor",
		"unknown":  "motor",
	}
	for in, want := range cases {
		if got := vehicleTypeForService(in); got != want {
			t.Errorf("vehicleTypeForService(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidServiceType(t *testing.T) {
	for _, s := range []string{"ride", "delivery", "food"} {
		if !validServiceType(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	for _, s := range []string{"", "RIDE", "taxi", "courier"} {
		if validServiceType(s) {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestValidMode(t *testing.T) {
	if !validMode("priority") || !validMode("normal") {
		t.Error("priority & normal should both be valid")
	}
	if validMode("") || validMode("ultra") {
		t.Error("empty / unknown modes must be rejected")
	}
}

func TestValidateLatLng(t *testing.T) {
	cases := []struct {
		lat, lng float64
		ok       bool
	}{
		{-6.91, 107.6, true},
		{0, 0, true},
		{90, 180, true},
		{-90, -180, true},
		{91, 0, false},
		{-91, 0, false},
		{0, 181, false},
		{0, -181, false},
	}
	for _, c := range cases {
		err := validateLatLng(c.lat, c.lng)
		if c.ok && err != nil {
			t.Errorf("(%f,%f) should be valid, got %v", c.lat, c.lng, err)
		}
		if !c.ok && err == nil {
			t.Errorf("(%f,%f) should be invalid", c.lat, c.lng)
		}
	}
}

func TestComputeAcceptanceRate(t *testing.T) {
	if got := computeAcceptanceRate(nil); got != 0.5 {
		t.Errorf("nil → 0.5 baseline, got %f", got)
	}
	if got := computeAcceptanceRate(&repository.DriverAcceptance{}); got != 0.5 {
		t.Errorf("zero counters → 0.5 baseline, got %f", got)
	}
	if got := computeAcceptanceRate(&repository.DriverAcceptance{Accepted: 8, Rejected: 2}); got != 0.8 {
		t.Errorf("8/(8+2) want 0.8, got %f", got)
	}
	if got := computeAcceptanceRate(&repository.DriverAcceptance{Accepted: 0, Rejected: 5}); got != 0.0 {
		t.Errorf("0/(0+5) want 0.0, got %f", got)
	}
}

func TestSuggestedSurge(t *testing.T) {
	cases := map[float64]float64{
		0.1: 2.0,
		0.4: 2.0,
		0.5: 1.5,
		0.7: 1.5,
		0.75: 1.25,
		0.9: 1.25,
		1.0: 1.0,
		2.0: 1.0,
	}
	for ratio, want := range cases {
		if got := suggestedSurge(ratio); got != want {
			t.Errorf("suggestedSurge(%f) = %f, want %f", ratio, got, want)
		}
	}
}

func TestComputeScore_Components(t *testing.T) {
	w := DefaultWeights()

	// Perfect score: distance 0, rating 5, accept 1, priority, no trips.
	// We allow a tiny epsilon because the weights (0.35+0.20+0.15+0.20+0.10)
	// don't sum to exactly 1.0 in IEEE-754.
	got := computeScore(ScoreInputs{
		DistanceKm: 0, SearchRadiusKm: 5, Rating: 5, AcceptanceRate: 1, IsPriority: true, TripsToday: 0,
	}, w)
	if got < 0.99 || got > 1.0001 {
		t.Errorf("perfect score should be ~1.0, got %f", got)
	}

	// Worst: at edge of radius, rating 1, accept 0, normal mode, 10 trips
	got = computeScore(ScoreInputs{
		DistanceKm: 5, SearchRadiusKm: 5, Rating: 1, AcceptanceRate: 0, IsPriority: false, TripsToday: 10,
	}, w)
	if got != 0 {
		t.Errorf("worst score should be 0, got %f", got)
	}

	// Priority bonus: same distance/rating/accept, only mode flips, priority must outscore.
	in := ScoreInputs{DistanceKm: 1, SearchRadiusKm: 5, Rating: 4.5, AcceptanceRate: 0.8, TripsToday: 2}
	in.IsPriority = false
	normal := computeScore(in, w)
	in.IsPriority = true
	priority := computeScore(in, w)
	if priority <= normal {
		t.Errorf("priority should outscore normal: priority=%f normal=%f", priority, normal)
	}

	// Distribution: fewer trips today → higher score, all else equal
	in = ScoreInputs{DistanceKm: 1, SearchRadiusKm: 5, Rating: 4.5, AcceptanceRate: 0.8, IsPriority: false}
	in.TripsToday = 0
	fresh := computeScore(in, w)
	in.TripsToday = 8
	tired := computeScore(in, w)
	if fresh <= tired {
		t.Errorf("fewer trips should score higher: fresh=%f tired=%f", fresh, tired)
	}
}

func TestComputeScore_Clamping(t *testing.T) {
	w := DefaultWeights()
	// Out-of-range inputs must still produce score in [0,1]
	got := computeScore(ScoreInputs{
		DistanceKm: 100, SearchRadiusKm: 5, Rating: 9, AcceptanceRate: 5, TripsToday: 999,
	}, w)
	if got < 0 || got > 1 {
		t.Errorf("score must be clamped to [0,1], got %f", got)
	}
}

// ── GoOnline ────────────────────────────────────────────────────────────────

func TestGoOnline_Success(t *testing.T) {
	svc, repo, geo, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetActiveOrder(gomock.Any(), tDriverID).Return("", repository.ErrNotFound)
	repo.EXPECT().SetDriverStatus(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().AddDriverToGeo(gomock.Any(), "motor", tDriverID, -6.91, 107.6).Return(nil)
	geo.EXPECT().RegisterDriver(gomock.Any(), gomock.Any()).Return(nil)

	resp, err := svc.GoOnline(context.Background(), tDriverID, &GoOnlineRequest{
		ServiceType: "ride", Lat: -6.91, Lng: 107.6,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Status != "online" || resp.DriverID != tDriverID {
		t.Errorf("unexpected resp: %+v", resp)
	}
}

func TestGoOnline_GeoFailures_AreSwallowed(t *testing.T) {
	// Even if the geo index add OR upstream geo register fails, GoOnline should succeed.
	svc, repo, geo, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetActiveOrder(gomock.Any(), tDriverID).Return("", repository.ErrNotFound)
	repo.EXPECT().SetDriverStatus(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().AddDriverToGeo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("geo down"))
	geo.EXPECT().RegisterDriver(gomock.Any(), gomock.Any()).Return(errors.New("upstream down"))

	if _, err := svc.GoOnline(context.Background(), tDriverID, &GoOnlineRequest{
		ServiceType: "ride", Lat: -6.91, Lng: 107.6,
	}); err != nil {
		t.Errorf("GoOnline should swallow geo failures, got %v", err)
	}
}

func TestGoOnline_EmptyDriverID(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	_, err := svc.GoOnline(context.Background(), "", &GoOnlineRequest{ServiceType: "ride"})
	if !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

func TestGoOnline_NilRequest(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	_, err := svc.GoOnline(context.Background(), tDriverID, nil)
	if !errors.Is(err, ErrInvalidServiceType) {
		t.Errorf("want ErrInvalidServiceType, got %v", err)
	}
}

func TestGoOnline_InvalidServiceType(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	_, err := svc.GoOnline(context.Background(), tDriverID, &GoOnlineRequest{ServiceType: "taxi", Lat: 0, Lng: 0})
	if !errors.Is(err, ErrInvalidServiceType) {
		t.Errorf("want ErrInvalidServiceType, got %v", err)
	}
}

func TestGoOnline_InvalidCoords(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	_, err := svc.GoOnline(context.Background(), tDriverID, &GoOnlineRequest{ServiceType: "ride", Lat: 999, Lng: 0})
	if !errors.Is(err, ErrInvalidCoords) {
		t.Errorf("want ErrInvalidCoords, got %v", err)
	}
}

func TestGoOnline_DriverHasActive(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetActiveOrder(gomock.Any(), tDriverID).Return("active-order-1", nil)

	_, err := svc.GoOnline(context.Background(), tDriverID, &GoOnlineRequest{
		ServiceType: "ride", Lat: -6.91, Lng: 107.6,
	})
	if !errors.Is(err, ErrDriverHasActive) {
		t.Errorf("want ErrDriverHasActive, got %v", err)
	}
}

func TestGoOnline_RepoSetStatusFails(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetActiveOrder(gomock.Any(), tDriverID).Return("", repository.ErrNotFound)
	repo.EXPECT().SetDriverStatus(gomock.Any(), gomock.Any()).Return(errors.New("redis down"))

	_, err := svc.GoOnline(context.Background(), tDriverID, &GoOnlineRequest{ServiceType: "ride", Lat: 0, Lng: 0})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── GoOffline ───────────────────────────────────────────────────────────────

func TestGoOffline_WithVehicleType(t *testing.T) {
	svc, repo, geo, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(&repository.DriverStatus{
		DriverID: tDriverID, VehicleType: "motor", Status: "online",
	}, nil)
	repo.EXPECT().RemoveDriverFromGeo(gomock.Any(), "motor", tDriverID).Return(nil)
	repo.EXPECT().DeleteDriverStatus(gomock.Any(), tDriverID).Return(nil)
	repo.EXPECT().DeleteDriverMode(gomock.Any(), tDriverID).Return(nil)
	geo.EXPECT().UnregisterDriver(gomock.Any(), tDriverID).Return(nil)

	resp, err := svc.GoOffline(context.Background(), tDriverID)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Status != "offline" {
		t.Errorf("status want offline, got %s", resp.Status)
	}
}

func TestGoOffline_NoStatus_StillSucceeds(t *testing.T) {
	svc, repo, geo, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(nil, repository.ErrNotFound)
	// No RemoveDriverFromGeo because vehicleType is unknown.
	repo.EXPECT().DeleteDriverStatus(gomock.Any(), tDriverID).Return(nil)
	repo.EXPECT().DeleteDriverMode(gomock.Any(), tDriverID).Return(nil)
	geo.EXPECT().UnregisterDriver(gomock.Any(), tDriverID).Return(nil)

	if _, err := svc.GoOffline(context.Background(), tDriverID); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestGoOffline_GetStatusOtherError_Bubbles(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(nil, errors.New("redis exploded"))

	_, err := svc.GoOffline(context.Background(), tDriverID)
	if err == nil {
		t.Fatal("expected error to bubble up")
	}
}

func TestGoOffline_EmptyID(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	_, err := svc.GoOffline(context.Background(), "")
	if !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

// ── UpdateLocation ──────────────────────────────────────────────────────────

func TestUpdateLocation_Success(t *testing.T) {
	svc, repo, geo, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(&repository.DriverStatus{
		DriverID: tDriverID, VehicleType: "motor", Status: "online",
	}, nil)
	repo.EXPECT().UpdateDriverLocation(gomock.Any(), tDriverID, -6.91, 107.6).Return(nil)
	repo.EXPECT().AddDriverToGeo(gomock.Any(), "motor", tDriverID, -6.91, 107.6).Return(nil)
	geo.EXPECT().UpdateLocation(gomock.Any(), gomock.Any()).Return(nil)

	if err := svc.UpdateLocation(context.Background(), tDriverID, &LocationUpdateRequest{
		Lat: -6.91, Lng: 107.6, Bearing: 90, SpeedKmh: 25,
	}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestUpdateLocation_NotOnline(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(nil, repository.ErrNotFound)

	err := svc.UpdateLocation(context.Background(), tDriverID, &LocationUpdateRequest{Lat: 0, Lng: 0})
	if !errors.Is(err, ErrDriverNotOnline) {
		t.Errorf("want ErrDriverNotOnline, got %v", err)
	}
}

func TestUpdateLocation_RepoOtherError(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(nil, errors.New("boom"))

	if err := svc.UpdateLocation(context.Background(), tDriverID, &LocationUpdateRequest{Lat: 0, Lng: 0}); err == nil {
		t.Fatal("expected error to bubble")
	}
}

func TestUpdateLocation_Validation(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	if err := svc.UpdateLocation(context.Background(), "", &LocationUpdateRequest{}); !errors.Is(err, ErrValidation) {
		t.Errorf("empty id → want ErrValidation, got %v", err)
	}
	if err := svc.UpdateLocation(context.Background(), tDriverID, nil); !errors.Is(err, ErrValidation) {
		t.Errorf("nil req → want ErrValidation, got %v", err)
	}
	if err := svc.UpdateLocation(context.Background(), tDriverID, &LocationUpdateRequest{Lat: 999}); !errors.Is(err, ErrInvalidCoords) {
		t.Errorf("bad coords → want ErrInvalidCoords, got %v", err)
	}
}

// ── Heartbeat ───────────────────────────────────────────────────────────────

func TestHeartbeat_Success(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(&repository.DriverStatus{
		DriverID: tDriverID, Status: "online",
	}, nil)
	repo.EXPECT().RefreshDriverStatusTTL(gomock.Any(), tDriverID).Return(60*time.Second, nil)

	resp, err := svc.Heartbeat(context.Background(), tDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if resp.TTLSeconds != 60 || resp.Status != "online" {
		t.Errorf("unexpected: %+v", resp)
	}
}

func TestHeartbeat_StatusNotFound(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(nil, repository.ErrNotFound)

	if _, err := svc.Heartbeat(context.Background(), tDriverID); !errors.Is(err, ErrDriverNotOnline) {
		t.Errorf("want ErrDriverNotOnline, got %v", err)
	}
}

func TestHeartbeat_RefreshNotFound(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(&repository.DriverStatus{Status: "online"}, nil)
	repo.EXPECT().RefreshDriverStatusTTL(gomock.Any(), tDriverID).Return(time.Duration(0), repository.ErrNotFound)

	if _, err := svc.Heartbeat(context.Background(), tDriverID); !errors.Is(err, ErrDriverNotOnline) {
		t.Errorf("want ErrDriverNotOnline, got %v", err)
	}
}

func TestHeartbeat_Validation(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()
	if _, err := svc.Heartbeat(context.Background(), ""); !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

// ── Respond ─────────────────────────────────────────────────────────────────

func TestRespond_Validation(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	cases := []struct {
		driverID string
		req      *OfferResponseRequest
	}{
		{"", &OfferResponseRequest{OrderID: tOrderID, Action: "accept"}},
		{tDriverID, nil},
		{tDriverID, &OfferResponseRequest{OrderID: "", Action: "accept"}},
	}
	for i, c := range cases {
		if err := svc.Respond(context.Background(), c.driverID, c.req); !errors.Is(err, ErrValidation) {
			t.Errorf("case %d: want ErrValidation, got %v", i, err)
		}
	}
}

func TestRespond_InvalidAction(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	err := svc.Respond(context.Background(), tDriverID, &OfferResponseRequest{OrderID: tOrderID, Action: "huh"})
	if err == nil {
		t.Fatal("expected validation error for unknown action")
	}
	var sErr *ServiceError
	if !errors.As(err, &sErr) || sErr.StatusCode != 400 {
		t.Errorf("want 400 ServiceError, got %v", err)
	}
}

func TestRespond_SessionNotFound(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetSession(gomock.Any(), tOrderID).Return(nil, repository.ErrNotFound)

	if err := svc.Respond(context.Background(), tDriverID, &OfferResponseRequest{
		OrderID: tOrderID, Action: "accept",
	}); !errors.Is(err, ErrOfferNotFound) {
		t.Errorf("want ErrOfferNotFound, got %v", err)
	}
}

func TestRespond_NotCurrentCandidate(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetSession(gomock.Any(), tOrderID).Return(&repository.MatchingSession{
		OrderID: tOrderID, Status: "searching", CurrentCandidateID: tDriverID2,
		OfferExpiresAt: fixedNow.Add(10 * time.Second),
	}, nil)

	if err := svc.Respond(context.Background(), tDriverID, &OfferResponseRequest{
		OrderID: tOrderID, Action: "accept",
	}); !errors.Is(err, ErrOfferNotFound) {
		t.Errorf("want ErrOfferNotFound for non-current candidate, got %v", err)
	}
}

func TestRespond_AlreadyClosed(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetSession(gomock.Any(), tOrderID).Return(&repository.MatchingSession{
		OrderID: tOrderID, Status: "driver_found", CurrentCandidateID: tDriverID,
	}, nil)

	if err := svc.Respond(context.Background(), tDriverID, &OfferResponseRequest{
		OrderID: tOrderID, Action: "accept",
	}); !errors.Is(err, ErrOfferAlreadyClosed) {
		t.Errorf("want ErrOfferAlreadyClosed, got %v", err)
	}
}

func TestRespond_OfferExpired(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetSession(gomock.Any(), tOrderID).Return(&repository.MatchingSession{
		OrderID: tOrderID, Status: "searching", CurrentCandidateID: tDriverID,
		OfferExpiresAt: fixedNow.Add(-1 * time.Second), // expired
	}, nil)

	if err := svc.Respond(context.Background(), tDriverID, &OfferResponseRequest{
		OrderID: tOrderID, Action: "accept",
	}); !errors.Is(err, ErrOfferNotFound) {
		t.Errorf("want ErrOfferNotFound for expired offer, got %v", err)
	}
}

func TestRespond_AcceptSuccess(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	sess := &repository.MatchingSession{
		OrderID: tOrderID, Status: "searching", CurrentCandidateID: tDriverID,
		OfferExpiresAt: fixedNow.Add(10 * time.Second),
	}
	repo.EXPECT().GetSession(gomock.Any(), tOrderID).Return(sess, nil)
	repo.EXPECT().SetActiveOrder(gomock.Any(), tDriverID, tOrderID).Return(nil)
	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(&repository.DriverStatus{
		DriverID: tDriverID, Status: "online",
	}, nil)
	repo.EXPECT().SetDriverStatus(gomock.Any(), gomock.Any()).Do(func(_ context.Context, s *repository.DriverStatus) {
		if s.Status != "busy" || s.OrderID != tOrderID {
			t.Errorf("driver status should be flipped to busy with order: %+v", s)
		}
	}).Return(nil)
	repo.EXPECT().IncrementAcceptance(gomock.Any(), tDriverID, true).Return(nil)
	repo.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).Do(func(_ context.Context, s *repository.MatchingSession) {
		if s.Status != "driver_found" || s.MatchedDriverID != tDriverID {
			t.Errorf("session not finalized: %+v", s)
		}
	}).Return(nil)

	if err := svc.Respond(context.Background(), tDriverID, &OfferResponseRequest{
		OrderID: tOrderID, Action: "accept",
	}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestRespond_AcceptLockFailed(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetSession(gomock.Any(), tOrderID).Return(&repository.MatchingSession{
		OrderID: tOrderID, Status: "searching", CurrentCandidateID: tDriverID,
		OfferExpiresAt: fixedNow.Add(10 * time.Second),
	}, nil)
	// SETNX fails — repo returns ErrNotFound (caller treats as conflict)
	repo.EXPECT().SetActiveOrder(gomock.Any(), tDriverID, tOrderID).Return(repository.ErrNotFound)

	if err := svc.Respond(context.Background(), tDriverID, &OfferResponseRequest{
		OrderID: tOrderID, Action: "accept",
	}); !errors.Is(err, ErrDriverHasActive) {
		t.Errorf("want ErrDriverHasActive, got %v", err)
	}
}

func TestRespond_RejectSuccess(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	sess := &repository.MatchingSession{
		OrderID: tOrderID, Status: "searching", CurrentCandidateID: tDriverID,
		OfferExpiresAt: fixedNow.Add(10 * time.Second),
	}
	repo.EXPECT().GetSession(gomock.Any(), tOrderID).Return(sess, nil)
	repo.EXPECT().MarkRejected(gomock.Any(), tOrderID, tDriverID).Return(nil)
	repo.EXPECT().IncrementAcceptance(gomock.Any(), tDriverID, false).Return(nil)
	repo.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).Do(func(_ context.Context, s *repository.MatchingSession) {
		if s.CurrentCandidateID != "" {
			t.Errorf("rejected session should clear current candidate")
		}
	}).Return(nil)

	if err := svc.Respond(context.Background(), tDriverID, &OfferResponseRequest{
		OrderID: tOrderID, Action: "reject", RejectReason: "too_far",
	}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

// ── SetMode ─────────────────────────────────────────────────────────────────

func TestSetMode_Validation(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	if _, err := svc.SetMode(context.Background(), "", &SetDispatchModeRequest{Mode: "normal"}); !errors.Is(err, ErrValidation) {
		t.Errorf("empty id → want ErrValidation, got %v", err)
	}
	if _, err := svc.SetMode(context.Background(), tDriverID, nil); !errors.Is(err, ErrValidation) {
		t.Errorf("nil req → want ErrValidation, got %v", err)
	}
	if _, err := svc.SetMode(context.Background(), tDriverID, &SetDispatchModeRequest{Mode: "ultra"}); !errors.Is(err, ErrInvalidMode) {
		t.Errorf("bad mode → want ErrInvalidMode, got %v", err)
	}
	if _, err := svc.SetMode(context.Background(), tDriverID, &SetDispatchModeRequest{Mode: "priority", DailyTarget: 0}); !errors.Is(err, ErrInvalidTarget) {
		t.Errorf("priority+0 target → want ErrInvalidTarget, got %v", err)
	}
}

func TestSetMode_DriverNotOnline(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(nil, repository.ErrNotFound)

	if _, err := svc.SetMode(context.Background(), tDriverID, &SetDispatchModeRequest{Mode: "normal"}); !errors.Is(err, ErrDriverNotOnline) {
		t.Errorf("want ErrDriverNotOnline, got %v", err)
	}
}

func TestSetMode_PrioritySuccessWithProgress(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(&repository.DriverStatus{Status: "online"}, nil)
	repo.EXPECT().SetDriverMode(gomock.Any(), tDriverID, gomock.Any()).Return(nil)
	repo.EXPECT().GetDriverEarnings(gomock.Any(), tDriverID, "2026-04-28").Return(&repository.DriverEarnings{
		Date: "2026-04-28", TotalEarnings: 50000, TripCount: 3,
	}, nil)

	resp, err := svc.SetMode(context.Background(), tDriverID, &SetDispatchModeRequest{Mode: "priority", DailyTarget: 200000})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Mode != "priority" || resp.DailyTarget != 200000 || resp.EarningsToday != 50000 {
		t.Errorf("unexpected resp: %+v", resp)
	}
	if resp.TargetProgressPct != 0.25 { // 50000/200000
		t.Errorf("progress want 0.25, got %f", resp.TargetProgressPct)
	}
}

func TestSetMode_PriorityProgressClampedTo1(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(&repository.DriverStatus{Status: "online"}, nil)
	repo.EXPECT().SetDriverMode(gomock.Any(), tDriverID, gomock.Any()).Return(nil)
	repo.EXPECT().GetDriverEarnings(gomock.Any(), tDriverID, "2026-04-28").Return(&repository.DriverEarnings{
		TotalEarnings: 999999, TripCount: 50,
	}, nil)

	resp, err := svc.SetMode(context.Background(), tDriverID, &SetDispatchModeRequest{Mode: "priority", DailyTarget: 100000})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TargetProgressPct != 1.0 {
		t.Errorf("over-target progress should clamp to 1.0, got %f", resp.TargetProgressPct)
	}
}

// ── GetFullStatus ───────────────────────────────────────────────────────────

func TestGetFullStatus_OfflineWhenNotFound(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(nil, repository.ErrNotFound)

	resp, err := svc.GetFullStatus(context.Background(), tDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "offline" || resp.Mode != "normal" {
		t.Errorf("unexpected: %+v", resp)
	}
}

func TestGetFullStatus_FullPath(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(&repository.DriverStatus{
		DriverID: tDriverID, Status: "online", OnlineSince: fixedNow.Add(-1 * time.Hour),
	}, nil)
	repo.EXPECT().GetDriverMode(gomock.Any(), tDriverID).Return(&repository.DriverMode{
		Mode: "priority", DailyTarget: 200000,
	}, nil)
	repo.EXPECT().GetDriverEarnings(gomock.Any(), tDriverID, "2026-04-28").Return(&repository.DriverEarnings{
		TotalEarnings: 50000, TripCount: 3,
	}, nil)
	repo.EXPECT().GetAcceptance(gomock.Any(), tDriverID).Return(&repository.DriverAcceptance{
		Accepted: 9, Rejected: 1,
	}, nil)
	repo.EXPECT().GetCachedRating(gomock.Any(), tDriverID).Return(4.85, nil)
	repo.EXPECT().GetActiveOrder(gomock.Any(), tDriverID).Return("order-active", nil)

	resp, err := svc.GetFullStatus(context.Background(), tDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Mode != "priority" || resp.DailyTarget != 200000 {
		t.Errorf("mode/target wrong: %+v", resp)
	}
	if resp.AcceptanceRate != 0.9 {
		t.Errorf("acceptance want 0.9, got %f", resp.AcceptanceRate)
	}
	if resp.Rating != 4.85 {
		t.Errorf("rating want 4.85, got %f", resp.Rating)
	}
	if resp.ActiveOrderID != "order-active" {
		t.Errorf("active order missing")
	}
	if resp.TripsToday != 3 {
		t.Errorf("trips today want 3, got %d", resp.TripsToday)
	}
}

// ── GetTodayEarnings ────────────────────────────────────────────────────────

func TestGetTodayEarnings_NoMode_NoEarnings(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverEarnings(gomock.Any(), tDriverID, "2026-04-28").Return(nil, repository.ErrNotFound)
	repo.EXPECT().GetDriverMode(gomock.Any(), tDriverID).Return(nil, repository.ErrNotFound)

	resp, err := svc.GetTodayEarnings(context.Background(), tDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Mode != "normal" || resp.TotalEarnings != 0 {
		t.Errorf("unexpected: %+v", resp)
	}
}

func TestGetTodayEarnings_PriorityWithProgress(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverEarnings(gomock.Any(), tDriverID, "2026-04-28").Return(&repository.DriverEarnings{
		TotalEarnings: 80000, TripCount: 4,
	}, nil)
	repo.EXPECT().GetDriverMode(gomock.Any(), tDriverID).Return(&repository.DriverMode{
		Mode: "priority", DailyTarget: 200000,
	}, nil)

	resp, err := svc.GetTodayEarnings(context.Background(), tDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if resp.AvgFare != 20000 {
		t.Errorf("avg fare want 20000, got %d", resp.AvgFare)
	}
	if resp.TargetProgressPct != 0.4 {
		t.Errorf("progress want 0.4, got %f", resp.TargetProgressPct)
	}
	if resp.TargetRemaining != 120000 {
		t.Errorf("remaining want 120000, got %d", resp.TargetRemaining)
	}
}

func TestGetTodayEarnings_PriorityOverTarget_RemainingZero(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetDriverEarnings(gomock.Any(), tDriverID, "2026-04-28").Return(&repository.DriverEarnings{
		TotalEarnings: 250000, TripCount: 12,
	}, nil)
	repo.EXPECT().GetDriverMode(gomock.Any(), tDriverID).Return(&repository.DriverMode{
		Mode: "priority", DailyTarget: 200000,
	}, nil)

	resp, err := svc.GetTodayEarnings(context.Background(), tDriverID)
	if err != nil {
		t.Fatal(err)
	}
	if resp.TargetProgressPct != 1.0 {
		t.Errorf("progress should clamp to 1.0, got %f", resp.TargetProgressPct)
	}
	if resp.TargetRemaining != 0 {
		t.Errorf("remaining should be 0, got %d", resp.TargetRemaining)
	}
}

// ── StartDispatch ───────────────────────────────────────────────────────────

func TestStartDispatch_Validation(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	if _, err := svc.StartDispatch(context.Background(), nil); !errors.Is(err, ErrValidation) {
		t.Errorf("nil req → want ErrValidation, got %v", err)
	}
	if _, err := svc.StartDispatch(context.Background(), &DispatchRequest{}); !errors.Is(err, ErrValidation) {
		t.Errorf("empty order → want ErrValidation, got %v", err)
	}
}

func TestStartDispatch_InvalidServiceType(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	_, err := svc.StartDispatch(context.Background(), &DispatchRequest{
		OrderID: tOrderID, ServiceType: "bus", PickupLat: 0, PickupLng: 0,
	})
	if !errors.Is(err, ErrInvalidServiceType) {
		t.Errorf("want ErrInvalidServiceType, got %v", err)
	}
}

func TestStartDispatch_InvalidCoords(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	_, err := svc.StartDispatch(context.Background(), &DispatchRequest{
		OrderID: tOrderID, ServiceType: "ride", PickupLat: 999,
	})
	if !errors.Is(err, ErrInvalidCoords) {
		t.Errorf("want ErrInvalidCoords, got %v", err)
	}
}

func TestStartDispatch_SessionExists(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(repository.ErrNotFound)

	_, err := svc.StartDispatch(context.Background(), &DispatchRequest{
		OrderID: tOrderID, ServiceType: "ride", PickupLat: -6.91, PickupLng: 107.6,
	})
	if !errors.Is(err, ErrSessionExists) {
		t.Errorf("want ErrSessionExists, got %v", err)
	}
}

func TestStartDispatch_SuccessAndPicksCandidate(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().CreateSession(gomock.Any(), gomock.Any()).Return(nil)

	// scoreCandidates path: NearbyDrivers returns one candidate, then enrichment lookups.
	repo.EXPECT().NearbyDrivers(gomock.Any(), "motor", -6.91, 107.6, 5.0, gomock.Any()).Return([]repository.GeoDriver{
		{DriverID: tDriverID, Lat: -6.91, Lng: 107.6, DistanceKm: 0.5},
	}, nil)
	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(&repository.DriverStatus{
		DriverID: tDriverID, Status: "online",
	}, nil)
	repo.EXPECT().GetActiveOrder(gomock.Any(), tDriverID).Return("", repository.ErrNotFound)
	repo.EXPECT().IsRejected(gomock.Any(), tOrderID, tDriverID).Return(false, nil)
	repo.EXPECT().GetDriverMode(gomock.Any(), tDriverID).Return(nil, repository.ErrNotFound)
	repo.EXPECT().GetAcceptance(gomock.Any(), tDriverID).Return(&repository.DriverAcceptance{Accepted: 8, Rejected: 2}, nil)
	repo.EXPECT().GetCachedRating(gomock.Any(), tDriverID).Return(4.7, nil)
	repo.EXPECT().GetDriverEarnings(gomock.Any(), tDriverID, "2026-04-28").Return(nil, repository.ErrNotFound)

	// After picking, session is updated with the candidate.
	repo.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).Do(func(_ context.Context, s *repository.MatchingSession) {
		if s.CurrentCandidateID != tDriverID || s.CandidatesTried != 1 {
			t.Errorf("session should be primed: %+v", s)
		}
		if s.OfferExpiresAt.IsZero() {
			t.Error("offer expiry should be set")
		}
	}).Return(nil)

	resp, err := svc.StartDispatch(context.Background(), &DispatchRequest{
		OrderID: tOrderID, ServiceType: "ride", PickupLat: -6.91, PickupLng: 107.6,
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Status != "searching" || resp.OrderID != tOrderID {
		t.Errorf("unexpected resp: %+v", resp)
	}
}

// ── CancelDispatch ──────────────────────────────────────────────────────────

func TestCancelDispatch_Validation(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	if err := svc.CancelDispatch(context.Background(), nil); !errors.Is(err, ErrValidation) {
		t.Errorf("nil → want ErrValidation, got %v", err)
	}
	if err := svc.CancelDispatch(context.Background(), &CancelDispatchRequest{}); !errors.Is(err, ErrValidation) {
		t.Errorf("empty → want ErrValidation, got %v", err)
	}
}

func TestCancelDispatch_NotFound(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetSession(gomock.Any(), tOrderID).Return(nil, repository.ErrNotFound)

	if err := svc.CancelDispatch(context.Background(), &CancelDispatchRequest{OrderID: tOrderID}); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
}

func TestCancelDispatch_FreesCurrentCandidate(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetSession(gomock.Any(), tOrderID).Return(&repository.MatchingSession{
		OrderID: tOrderID, CurrentCandidateID: tDriverID, Status: "searching",
	}, nil)
	repo.EXPECT().UpdateSession(gomock.Any(), gomock.Any()).Do(func(_ context.Context, s *repository.MatchingSession) {
		if s.Status != "cancelled" {
			t.Errorf("status should be cancelled")
		}
	}).Return(nil)
	repo.EXPECT().ClearActiveOrder(gomock.Any(), tDriverID).Return(nil)
	repo.EXPECT().DeleteSession(gomock.Any(), tOrderID).Return(nil)

	if err := svc.CancelDispatch(context.Background(), &CancelDispatchRequest{OrderID: tOrderID, Reason: "timeout"}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

// ── NearbyActiveDrivers ─────────────────────────────────────────────────────

func TestNearbyActiveDrivers_InvalidServiceType(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	if _, err := svc.NearbyActiveDrivers(context.Background(), NearbyDriversQuery{
		ServiceType: "bus",
	}); !errors.Is(err, ErrInvalidServiceType) {
		t.Errorf("want ErrInvalidServiceType, got %v", err)
	}
}

func TestNearbyActiveDrivers_BadCoords(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	if _, err := svc.NearbyActiveDrivers(context.Background(), NearbyDriversQuery{
		ServiceType: "ride", Lat: 99, Lng: 0,
	}); !errors.Is(err, ErrInvalidCoords) {
		t.Errorf("want ErrInvalidCoords, got %v", err)
	}
}

func TestNearbyActiveDrivers_SortedByScoreDesc(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().NearbyDrivers(gomock.Any(), "motor", gomock.Any(), gomock.Any(), 5.0, gomock.Any()).Return([]repository.GeoDriver{
		{DriverID: "far-low", Lat: 0, Lng: 0, DistanceKm: 4.0},
		{DriverID: "near-high", Lat: 0, Lng: 0, DistanceKm: 0.3},
	}, nil)

	// Driver "far-low": online, no active, no priority, ratings poor
	repo.EXPECT().GetDriverStatus(gomock.Any(), "far-low").Return(&repository.DriverStatus{Status: "online"}, nil)
	repo.EXPECT().GetActiveOrder(gomock.Any(), "far-low").Return("", repository.ErrNotFound)
	repo.EXPECT().GetDriverMode(gomock.Any(), "far-low").Return(nil, repository.ErrNotFound)
	repo.EXPECT().GetAcceptance(gomock.Any(), "far-low").Return(&repository.DriverAcceptance{Accepted: 1, Rejected: 9}, nil)
	repo.EXPECT().GetCachedRating(gomock.Any(), "far-low").Return(3.0, nil)
	repo.EXPECT().GetDriverEarnings(gomock.Any(), "far-low", "2026-04-28").Return(nil, repository.ErrNotFound)

	// Driver "near-high": online, priority, top ratings
	repo.EXPECT().GetDriverStatus(gomock.Any(), "near-high").Return(&repository.DriverStatus{Status: "online"}, nil)
	repo.EXPECT().GetActiveOrder(gomock.Any(), "near-high").Return("", repository.ErrNotFound)
	repo.EXPECT().GetDriverMode(gomock.Any(), "near-high").Return(&repository.DriverMode{Mode: "priority"}, nil)
	repo.EXPECT().GetAcceptance(gomock.Any(), "near-high").Return(&repository.DriverAcceptance{Accepted: 9, Rejected: 1}, nil)
	repo.EXPECT().GetCachedRating(gomock.Any(), "near-high").Return(4.95, nil)
	repo.EXPECT().GetDriverEarnings(gomock.Any(), "near-high", "2026-04-28").Return(nil, repository.ErrNotFound)

	resp, err := svc.NearbyActiveDrivers(context.Background(), NearbyDriversQuery{
		ServiceType: "ride", Lat: 0, Lng: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Drivers) != 2 {
		t.Fatalf("want 2 drivers, got %d", len(resp.Drivers))
	}
	if resp.Drivers[0].DriverID != "near-high" {
		t.Errorf("near-high should top the ranking, got %+v", resp.Drivers)
	}
	if resp.Drivers[0].Score <= resp.Drivers[1].Score {
		t.Errorf("scores must be sorted desc: %+v", resp.Drivers)
	}
}

func TestNearbyActiveDrivers_FiltersBusyAndOffline(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().NearbyDrivers(gomock.Any(), "motor", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]repository.GeoDriver{
		{DriverID: "busy", DistanceKm: 0.1},
		{DriverID: "offline", DistanceKm: 0.2},
		{DriverID: "ok", DistanceKm: 1.0},
	}, nil)

	// busy: online but has active order
	repo.EXPECT().GetDriverStatus(gomock.Any(), "busy").Return(&repository.DriverStatus{Status: "online"}, nil)
	repo.EXPECT().GetActiveOrder(gomock.Any(), "busy").Return("some-order", nil)

	// offline: not online → skipped without further calls
	repo.EXPECT().GetDriverStatus(gomock.Any(), "offline").Return(&repository.DriverStatus{Status: "offline"}, nil)

	// ok: passes
	repo.EXPECT().GetDriverStatus(gomock.Any(), "ok").Return(&repository.DriverStatus{Status: "online"}, nil)
	repo.EXPECT().GetActiveOrder(gomock.Any(), "ok").Return("", repository.ErrNotFound)
	repo.EXPECT().GetDriverMode(gomock.Any(), "ok").Return(nil, repository.ErrNotFound)
	repo.EXPECT().GetAcceptance(gomock.Any(), "ok").Return(nil, nil)
	repo.EXPECT().GetCachedRating(gomock.Any(), "ok").Return(0.0, nil)
	repo.EXPECT().GetDriverEarnings(gomock.Any(), "ok", "2026-04-28").Return(nil, repository.ErrNotFound)

	resp, err := svc.NearbyActiveDrivers(context.Background(), NearbyDriversQuery{
		ServiceType: "ride", Lat: 0, Lng: 0, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Drivers) != 1 || resp.Drivers[0].DriverID != "ok" {
		t.Errorf("only 'ok' should remain, got %+v", resp.Drivers)
	}
}

func TestNearbyActiveDrivers_DefaultsApplied(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	// radius 0 → default 5km, limit 0 → default 20 → NearbyDrivers limit*2 = 40
	repo.EXPECT().NearbyDrivers(gomock.Any(), "motor", 0.0, 0.0, 5.0, 40).Return([]repository.GeoDriver{}, nil)

	resp, err := svc.NearbyActiveDrivers(context.Background(), NearbyDriversQuery{
		ServiceType: "ride",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 0 {
		t.Errorf("expected empty result, got %d", resp.Total)
	}
}

// ── GetSession ──────────────────────────────────────────────────────────────

func TestGetSession_Validation(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()
	if _, err := svc.GetSession(context.Background(), ""); !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()
	repo.EXPECT().GetSession(gomock.Any(), tOrderID).Return(nil, repository.ErrNotFound)

	if _, err := svc.GetSession(context.Background(), tOrderID); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
}

func TestGetSession_Success(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()
	repo.EXPECT().GetSession(gomock.Any(), tOrderID).Return(&repository.MatchingSession{
		OrderID: tOrderID, SessionID: "sess-1", Status: "searching", ServiceType: "ride",
		CandidatesTried: 2, CurrentCandidateID: tDriverID,
	}, nil)

	resp, err := svc.GetSession(context.Background(), tOrderID)
	if err != nil {
		t.Fatal(err)
	}
	if resp.SessionID != "sess-1" || resp.CandidatesTried != 2 {
		t.Errorf("unexpected: %+v", resp)
	}
}

// ── GetZoneStats ────────────────────────────────────────────────────────────

func TestGetZoneStats_Validation(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()
	if _, err := svc.GetZoneStats(context.Background(), "motor", ""); !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

func TestGetZoneStats_NotFound_ReturnsZeros(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().GetZoneStats(gomock.Any(), "motor", tZoneID).Return(nil, repository.ErrNotFound)

	resp, err := svc.GetZoneStats(context.Background(), "", tZoneID) // empty vehicle defaults to motor
	if err != nil {
		t.Fatal(err)
	}
	if resp.SuggestedSurge != 1.0 || resp.SupplyDemandRatio != 1.0 || resp.OnlineDrivers != 0 {
		t.Errorf("missing-zone defaults wrong: %+v", resp)
	}
}

func TestGetZoneStats_SurgeTiers(t *testing.T) {
	cases := []struct {
		online, pending int
		wantSurge       float64
	}{
		{1, 5, 2.0},   // ratio 0.2 → 2.0x
		{3, 5, 1.5},   // ratio 0.6 → 1.5x
		{4, 5, 1.25},  // ratio 0.8 → 1.25x
		{5, 5, 1.0},   // ratio 1.0 → 1.0x
		{20, 0, 1.0},  // PendingOrders=0 → ratio defaults to 1.0
	}
	for i, c := range cases {
		svc, repo, _, ctrl := newSvc(t)
		repo.EXPECT().GetZoneStats(gomock.Any(), "motor", tZoneID).Return(&repository.ZoneStats{
			ZoneID:        tZoneID,
			OnlineDrivers: c.online,
			PendingOrders: c.pending,
			UpdatedAt:     fixedNow,
		}, nil)
		resp, err := svc.GetZoneStats(context.Background(), "motor", tZoneID)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if resp.SuggestedSurge != c.wantSurge {
			t.Errorf("case %d (online=%d pending=%d): want surge %f, got %f",
				i, c.online, c.pending, c.wantSurge, resp.SuggestedSurge)
		}
		ctrl.Finish()
	}
}

// ── FreeDriver ──────────────────────────────────────────────────────────────

func TestFreeDriver_Validation(t *testing.T) {
	svc, _, _, ctrl := newSvc(t)
	defer ctrl.Finish()
	if err := svc.FreeDriver(context.Background(), "", nil); !errors.Is(err, ErrValidation) {
		t.Errorf("want ErrValidation, got %v", err)
	}
}

func TestFreeDriver_NoFare(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().ClearActiveOrder(gomock.Any(), tDriverID).Return(nil)
	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(&repository.DriverStatus{
		DriverID: tDriverID, Status: "busy", OrderID: tOrderID,
	}, nil)
	repo.EXPECT().SetDriverStatus(gomock.Any(), gomock.Any()).Do(func(_ context.Context, s *repository.DriverStatus) {
		if s.Status != "online" || s.OrderID != "" {
			t.Errorf("driver should be flipped to online with no order, got %+v", s)
		}
	}).Return(nil)

	if err := svc.FreeDriver(context.Background(), tDriverID, nil); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestFreeDriver_WithFareAndPriorityAutoDisable(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().ClearActiveOrder(gomock.Any(), tDriverID).Return(nil)
	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(&repository.DriverStatus{
		DriverID: tDriverID, Status: "busy",
	}, nil)
	repo.EXPECT().SetDriverStatus(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().IncrementDriverEarnings(gomock.Any(), tDriverID, "2026-04-28", 50000).Return(nil)
	repo.EXPECT().GetDriverMode(gomock.Any(), tDriverID).Return(&repository.DriverMode{
		Mode: "priority", DailyTarget: 100000,
	}, nil)
	repo.EXPECT().GetDriverEarnings(gomock.Any(), tDriverID, "2026-04-28").Return(&repository.DriverEarnings{
		TotalEarnings: 100000,
	}, nil)
	repo.EXPECT().DeleteDriverMode(gomock.Any(), tDriverID).Return(nil) // priority auto-disabled

	if err := svc.FreeDriver(context.Background(), tDriverID, &FreeDriverRequest{TripFare: 50000}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestFreeDriver_WithFareBelowTarget_PriorityStays(t *testing.T) {
	svc, repo, _, ctrl := newSvc(t)
	defer ctrl.Finish()

	repo.EXPECT().ClearActiveOrder(gomock.Any(), tDriverID).Return(nil)
	repo.EXPECT().GetDriverStatus(gomock.Any(), tDriverID).Return(&repository.DriverStatus{}, nil)
	repo.EXPECT().SetDriverStatus(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().IncrementDriverEarnings(gomock.Any(), tDriverID, "2026-04-28", 25000).Return(nil)
	repo.EXPECT().GetDriverMode(gomock.Any(), tDriverID).Return(&repository.DriverMode{
		Mode: "priority", DailyTarget: 100000,
	}, nil)
	repo.EXPECT().GetDriverEarnings(gomock.Any(), tDriverID, "2026-04-28").Return(&repository.DriverEarnings{
		TotalEarnings: 50000, // below target
	}, nil)
	// no DeleteDriverMode call expected

	if err := svc.FreeDriver(context.Background(), tDriverID, &FreeDriverRequest{TripFare: 25000}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}
