//go:build unit

package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/zicofarry/clay-app/backend/services/matching-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/matching-service/mocks"
)

const (
	testDriverID = "00000000-0000-0000-0000-00000000aaaa"
	testOrderID  = "00000000-0000-0000-0000-00000000bbbb"
	testZoneID   = "zone-cbd-001"
)

// newTestHandler builds a MatchingHandler backed by a gomock MatchingServiceInterface.
func newTestHandler(t *testing.T) (*MatchingHandler, *mocks.MockMatchingServiceInterface, *gomock.Controller) {
	t.Helper()
	ctrl := gomock.NewController(t)
	svc := mocks.NewMockMatchingServiceInterface(ctrl)
	return NewMatchingHandler(svc), svc, ctrl
}

// requestWithBody builds an httptest request with the X-User-ID header populated.
func requestWithBody(method, path string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("X-User-ID", testDriverID)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// ── GoOnline ────────────────────────────────────────────────────────────────

func TestGoOnline_Success(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.GoOnlineRequest{ServiceType: "ride", Lat: -6.91, Lng: 107.6}
	expected := &service.DriverStatusResponse{DriverID: testDriverID, Status: "online", UpdatedAt: time.Now()}

	svc.EXPECT().GoOnline(gomock.Any(), testDriverID, gomock.Any()).Return(expected, nil)

	rec := httptest.NewRecorder()
	h.GoOnline(rec, requestWithBody(http.MethodPost, "/dispatcher/go-online", body))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGoOnline_BadJSON(t *testing.T) {
	h, _, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	req := httptest.NewRequest(http.MethodPost, "/dispatcher/go-online", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.GoOnline(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

func TestGoOnline_DriverHasActive(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.GoOnlineRequest{ServiceType: "ride", Lat: -6.91, Lng: 107.6}
	svc.EXPECT().GoOnline(gomock.Any(), testDriverID, gomock.Any()).Return(nil, service.ErrDriverHasActive)

	rec := httptest.NewRecorder()
	h.GoOnline(rec, requestWithBody(http.MethodPost, "/dispatcher/go-online", body))

	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// ── GoOffline ───────────────────────────────────────────────────────────────

func TestGoOffline_Success(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	expected := &service.DriverStatusResponse{DriverID: testDriverID, Status: "offline", UpdatedAt: time.Now()}
	svc.EXPECT().GoOffline(gomock.Any(), testDriverID).Return(expected, nil)

	rec := httptest.NewRecorder()
	h.GoOffline(rec, requestWithBody(http.MethodPost, "/dispatcher/go-offline", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── UpdateLocation ──────────────────────────────────────────────────────────

func TestUpdateLocation_Success(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.LocationUpdateRequest{Lat: -6.91, Lng: 107.6, Bearing: 90, SpeedKmh: 25}
	svc.EXPECT().UpdateLocation(gomock.Any(), testDriverID, gomock.Any()).Return(nil)

	rec := httptest.NewRecorder()
	h.UpdateLocation(rec, requestWithBody(http.MethodPut, "/dispatcher/location", body))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

func TestUpdateLocation_NotOnline(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.LocationUpdateRequest{Lat: -6.91, Lng: 107.6}
	svc.EXPECT().UpdateLocation(gomock.Any(), testDriverID, gomock.Any()).Return(service.ErrDriverNotOnline)

	rec := httptest.NewRecorder()
	h.UpdateLocation(rec, requestWithBody(http.MethodPut, "/dispatcher/location", body))

	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// ── Heartbeat ──────────────────────────────────────────────────────────────

func TestHeartbeat_Success(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	expected := &service.HeartbeatResponse{DriverID: testDriverID, Status: "online", TTLSeconds: 60}
	svc.EXPECT().Heartbeat(gomock.Any(), testDriverID).Return(expected, nil)

	rec := httptest.NewRecorder()
	h.Heartbeat(rec, requestWithBody(http.MethodPost, "/dispatcher/heartbeat", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

func TestHeartbeat_NotOnline(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	svc.EXPECT().Heartbeat(gomock.Any(), testDriverID).Return(nil, service.ErrDriverNotOnline)

	rec := httptest.NewRecorder()
	h.Heartbeat(rec, requestWithBody(http.MethodPost, "/dispatcher/heartbeat", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// ── Respond ────────────────────────────────────────────────────────────────

func TestRespond_Accept(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.OfferResponseRequest{OrderID: testOrderID, Action: "accept"}
	svc.EXPECT().Respond(gomock.Any(), testDriverID, gomock.Any()).Return(nil)

	rec := httptest.NewRecorder()
	h.Respond(rec, requestWithBody(http.MethodPost, "/dispatcher/respond", body))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

func TestRespond_OfferExpired(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.OfferResponseRequest{OrderID: testOrderID, Action: "accept"}
	svc.EXPECT().Respond(gomock.Any(), testDriverID, gomock.Any()).Return(service.ErrOfferNotFound)

	rec := httptest.NewRecorder()
	h.Respond(rec, requestWithBody(http.MethodPost, "/dispatcher/respond", body))

	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

func TestRespond_AlreadyClosed(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.OfferResponseRequest{OrderID: testOrderID, Action: "accept"}
	svc.EXPECT().Respond(gomock.Any(), testDriverID, gomock.Any()).Return(service.ErrOfferAlreadyClosed)

	rec := httptest.NewRecorder()
	h.Respond(rec, requestWithBody(http.MethodPost, "/dispatcher/respond", body))

	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

// ── SetMode ────────────────────────────────────────────────────────────────

func TestSetMode_PrioritySuccess(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.SetDispatchModeRequest{Mode: "priority", DailyTarget: 200000}
	expected := &service.DispatchModeResponse{Mode: "priority", DailyTarget: 200000, ActivatedAt: time.Now()}
	svc.EXPECT().SetMode(gomock.Any(), testDriverID, gomock.Any()).Return(expected, nil)

	rec := httptest.NewRecorder()
	h.SetMode(rec, requestWithBody(http.MethodPut, "/dispatcher/mode", body))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

func TestSetMode_InvalidTarget(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.SetDispatchModeRequest{Mode: "priority", DailyTarget: 0}
	svc.EXPECT().SetMode(gomock.Any(), testDriverID, gomock.Any()).Return(nil, service.ErrInvalidTarget)

	rec := httptest.NewRecorder()
	h.SetMode(rec, requestWithBody(http.MethodPut, "/dispatcher/mode", body))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}

// ── GetFullStatus ──────────────────────────────────────────────────────────

func TestGetFullStatus_Success(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	expected := &service.FullDriverStatusResponse{
		DriverID: testDriverID, Status: "online", Mode: "normal", Rating: 4.8,
	}
	svc.EXPECT().GetFullStatus(gomock.Any(), testDriverID).Return(expected, nil)

	rec := httptest.NewRecorder()
	h.GetFullStatus(rec, requestWithBody(http.MethodGet, "/dispatcher/status", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── GetTodayEarnings ───────────────────────────────────────────────────────

func TestGetTodayEarnings_Success(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	expected := &service.EarningsTodayResponse{
		Date: "2026-04-28", TotalEarnings: 85000, TripCount: 5, Mode: "priority",
		DailyTarget: 200000, TargetProgressPct: 0.425, AvgFare: 17000,
	}
	svc.EXPECT().GetTodayEarnings(gomock.Any(), testDriverID).Return(expected, nil)

	rec := httptest.NewRecorder()
	h.GetTodayEarnings(rec, requestWithBody(http.MethodGet, "/dispatcher/earnings/today", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// ── StartDispatch ──────────────────────────────────────────────────────────

func TestStartDispatch_Success(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.DispatchRequest{
		OrderID: testOrderID, ServiceType: "ride",
		PickupLat: -6.91, PickupLng: 107.6,
	}
	expected := &service.DispatchSessionResponse{
		SessionID: "sess-1", OrderID: testOrderID, Status: "searching", CreatedAt: time.Now(),
	}
	svc.EXPECT().StartDispatch(gomock.Any(), gomock.Any()).Return(expected, nil)

	rec := httptest.NewRecorder()
	h.StartDispatch(rec, requestWithBody(http.MethodPost, "/internal/dispatcher/dispatch", body))

	if rec.Code != http.StatusAccepted {
		t.Errorf("want 202, got %d", rec.Code)
	}
}

func TestStartDispatch_AlreadyExists(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.DispatchRequest{
		OrderID: testOrderID, ServiceType: "ride",
		PickupLat: -6.91, PickupLng: 107.6,
	}
	svc.EXPECT().StartDispatch(gomock.Any(), gomock.Any()).Return(nil, service.ErrSessionExists)

	rec := httptest.NewRecorder()
	h.StartDispatch(rec, requestWithBody(http.MethodPost, "/internal/dispatcher/dispatch", body))

	if rec.Code != http.StatusConflict {
		t.Errorf("want 409, got %d", rec.Code)
	}
}

// ── CancelDispatch ─────────────────────────────────────────────────────────

func TestCancelDispatch_Success(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.CancelDispatchRequest{OrderID: testOrderID, Reason: "order_cancelled"}
	svc.EXPECT().CancelDispatch(gomock.Any(), gomock.Any()).Return(nil)

	rec := httptest.NewRecorder()
	h.CancelDispatch(rec, requestWithBody(http.MethodPost, "/internal/dispatcher/cancel", body))

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

func TestCancelDispatch_NotFound(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.CancelDispatchRequest{OrderID: testOrderID}
	svc.EXPECT().CancelDispatch(gomock.Any(), gomock.Any()).Return(service.ErrSessionNotFound)

	rec := httptest.NewRecorder()
	h.CancelDispatch(rec, requestWithBody(http.MethodPost, "/internal/dispatcher/cancel", body))

	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// ── NearbyActiveDrivers ────────────────────────────────────────────────────

func TestNearbyActiveDrivers_Success(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	expected := &service.NearbyDriversResponse{
		Drivers: []service.NearbyDriverSummary{
			{DriverID: "d1", DistanceKm: 0.5, Rating: 4.9, Score: 0.92},
			{DriverID: "d2", DistanceKm: 1.2, Rating: 4.7, Score: 0.78},
		},
		Total: 2,
	}
	svc.EXPECT().NearbyActiveDrivers(gomock.Any(), gomock.Any()).Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet,
		"/internal/dispatcher/nearby-drivers?lat=-6.91&lng=107.6&radius_km=5&service_type=ride&limit=10", nil)
	rec := httptest.NewRecorder()

	h.NearbyActiveDrivers(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── GetSession ─────────────────────────────────────────────────────────────

func TestGetSession_Success(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	expected := &service.DispatchSessionDetail{
		DispatchSessionResponse: service.DispatchSessionResponse{
			SessionID: "sess-1", OrderID: testOrderID, Status: "searching",
		},
		ServiceType: "ride", CandidatesTried: 2,
	}
	svc.EXPECT().GetSession(gomock.Any(), testOrderID).Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/internal/dispatcher/order/"+testOrderID+"/status", nil)
	req.SetPathValue("orderId", testOrderID)
	rec := httptest.NewRecorder()

	h.GetSession(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	svc.EXPECT().GetSession(gomock.Any(), testOrderID).Return(nil, service.ErrSessionNotFound)

	req := httptest.NewRequest(http.MethodGet, "/internal/dispatcher/order/"+testOrderID+"/status", nil)
	req.SetPathValue("orderId", testOrderID)
	rec := httptest.NewRecorder()

	h.GetSession(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}

// ── GetZoneStats ───────────────────────────────────────────────────────────

func TestGetZoneStats_Success(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	expected := &service.ZoneStatsResponse{
		ZoneID: testZoneID, OnlineDrivers: 12, PendingOrders: 18,
		SupplyDemandRatio: 0.667, SuggestedSurge: 1.5, UpdatedAt: time.Now(),
	}
	svc.EXPECT().GetZoneStats(gomock.Any(), "motor", testZoneID).Return(expected, nil)

	req := httptest.NewRequest(http.MethodGet, "/internal/dispatcher/zone/"+testZoneID+"/stats", nil)
	req.SetPathValue("zoneId", testZoneID)
	rec := httptest.NewRecorder()

	h.GetZoneStats(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── FreeDriver ─────────────────────────────────────────────────────────────

func TestFreeDriver_Success(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	body := service.FreeDriverRequest{TripFare: 25000}
	svc.EXPECT().FreeDriver(gomock.Any(), testDriverID, gomock.Any()).Return(nil)

	req := requestWithBody(http.MethodPut, "/internal/drivers/"+testDriverID+"/free", body)
	req.SetPathValue("driverId", testDriverID)
	rec := httptest.NewRecorder()

	h.FreeDriver(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestFreeDriver_NoBody(t *testing.T) {
	h, svc, ctrl := newTestHandler(t)
	defer ctrl.Finish()

	svc.EXPECT().FreeDriver(gomock.Any(), testDriverID, gomock.Any()).Return(nil)

	req := httptest.NewRequest(http.MethodPut, "/internal/drivers/"+testDriverID+"/free", nil)
	req.SetPathValue("driverId", testDriverID)
	rec := httptest.NewRecorder()

	h.FreeDriver(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("want 200, got %d", rec.Code)
	}
}

// ── Error mapping ──────────────────────────────────────────────────────────

func TestHandleServiceError_Unknown(t *testing.T) {
	rec := httptest.NewRecorder()
	handleServiceError(rec, errBoundary{})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("want 500 for unknown error, got %d", rec.Code)
	}
}

type errBoundary struct{}

func (errBoundary) Error() string { return "boundary error" }
