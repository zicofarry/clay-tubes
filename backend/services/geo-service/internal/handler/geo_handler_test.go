//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/geo-service/internal/cache"
	"github.com/zicofarry/clay-app/backend/services/geo-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/geo-service/mocks"
	"go.uber.org/mock/gomock"
)

func TestUpdateDriverLocation_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockGeoServiceInterface(ctrl)
	mockSvc.EXPECT().UpdateDriverLocation(gomock.Any(), "drv-1", gomock.Any()).Return(nil)

	h := NewGeoHandler(mockSvc)
	body := `{"lat":-6.9733,"lng":107.6310,"service_type":"ride","bearing":180.0,"speed_kmh":35.0}`
	req := httptest.NewRequest("PUT", "/drivers/drv-1/location", strings.NewReader(body))
	req.SetPathValue("driverId", "drv-1")
	w := httptest.NewRecorder()
	h.UpdateDriverLocation(w, req)
	if w.Code != http.StatusOK { t.Errorf("expected 200, got %d", w.Code) }
}

func TestGetDriverLocation_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockGeoServiceInterface(ctrl)
	mockSvc.EXPECT().GetDriverLocation(gomock.Any(), "drv-1").
		Return(&cache.DriverLocation{DriverID: "drv-1", Lat: -6.97, Lng: 107.63}, nil)

	h := NewGeoHandler(mockSvc)
	req := httptest.NewRequest("GET", "/drivers/drv-1/location", nil)
	req.SetPathValue("driverId", "drv-1")
	w := httptest.NewRecorder()
	h.GetDriverLocation(w, req)
	if w.Code != http.StatusOK { t.Errorf("expected 200, got %d", w.Code) }
}

func TestGetDriverLocation_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockGeoServiceInterface(ctrl)
	mockSvc.EXPECT().GetDriverLocation(gomock.Any(), "drv-unknown").Return(nil, service.ErrDriverNotFound)

	h := NewGeoHandler(mockSvc)
	req := httptest.NewRequest("GET", "/drivers/drv-unknown/location", nil)
	req.SetPathValue("driverId", "drv-unknown")
	w := httptest.NewRecorder()
	h.GetDriverLocation(w, req)
	if w.Code != http.StatusNotFound { t.Errorf("expected 404, got %d", w.Code) }
}

func TestFindNearbyDrivers_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockGeoServiceInterface(ctrl)
	mockSvc.EXPECT().FindNearbyDrivers(gomock.Any(), -6.97, 107.63, 5.0, "ride", 20).
		Return(&service.NearbyDriversResponse{
			Drivers: []cache.NearbyDriver{{DriverID: "drv-1", DistanceKm: 1.2}}, Total: 1,
		}, nil)

	h := NewGeoHandler(mockSvc)
	req := httptest.NewRequest("GET", "/drivers/nearby?lat=-6.97&lng=107.63&radius_km=5.0&service_type=ride&limit=20", nil)
	w := httptest.NewRecorder()
	h.FindNearbyDrivers(w, req)
	if w.Code != http.StatusOK { t.Errorf("expected 200, got %d", w.Code) }
}

func TestEstimateRoute_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockGeoServiceInterface(ctrl)
	mockSvc.EXPECT().EstimateRoute(gomock.Any(), gomock.Any()).
		Return(&service.RouteEstimateResponse{DistanceKm: 8.4, DurationSeconds: 1260, DurationText: "21 menit"}, nil)

	h := NewGeoHandler(mockSvc)
	body := `{"origin":{"lat":-6.97,"lng":107.63},"destination":{"lat":-6.90,"lng":107.60}}`
	req := httptest.NewRequest("POST", "/maps/estimate", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.EstimateRoute(w, req)
	if w.Code != http.StatusOK { t.Errorf("expected 200, got %d", w.Code) }
}

func TestCheckGeofence_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockGeoServiceInterface(ctrl)
	mockSvc.EXPECT().CheckGeofence(gomock.Any(), gomock.Any()).
		Return(&service.GeofenceCheckResponse{InsideZones: []service.GeofenceZoneResponse{}, IsRestricted: false}, nil)

	h := NewGeoHandler(mockSvc)
	body := `{"lat":-6.97,"lng":107.63}`
	req := httptest.NewRequest("POST", "/geofence/check", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.CheckGeofence(w, req)
	if w.Code != http.StatusOK { t.Errorf("expected 200, got %d", w.Code) }
}

func TestGetDriverETA_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockGeoServiceInterface(ctrl)
	mockSvc.EXPECT().GetDriverETA(gomock.Any(), "drv-1", "ord-1").Return(nil, service.ErrETANotFound)

	h := NewGeoHandler(mockSvc)
	req := httptest.NewRequest("GET", "/internal/maps/eta/drv-1/ord-1", nil)
	req.SetPathValue("driverId", "drv-1")
	req.SetPathValue("orderId", "ord-1")
	w := httptest.NewRecorder()
	h.GetDriverETA(w, req)
	if w.Code != http.StatusNotFound { t.Errorf("expected 404, got %d", w.Code) }
}

func TestUpdateDriverETA_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockGeoServiceInterface(ctrl)
	mockSvc.EXPECT().UpdateDriverETA(gomock.Any(), "drv-1", "ord-1", gomock.Any()).
		Return(&service.LiveEtaResponse{DriverID: "drv-1", OrderID: "ord-1", ETASeconds: 480, ETAText: "8 menit"}, nil)

	h := NewGeoHandler(mockSvc)
	body := `{"eta_seconds":480,"distance_remaining_km":3.2,"destination_type":"pickup"}`
	req := httptest.NewRequest("PUT", "/internal/maps/eta/drv-1/ord-1", strings.NewReader(body))
	req.SetPathValue("driverId", "drv-1")
	req.SetPathValue("orderId", "ord-1")
	w := httptest.NewRecorder()
	h.UpdateDriverETA(w, req)
	if w.Code != http.StatusOK { t.Errorf("expected 200, got %d", w.Code) }
}

func TestForwardGeocode_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockGeoServiceInterface(ctrl)
	mockSvc.EXPECT().ForwardGeocode(gomock.Any(), gomock.Any()).
		Return(&service.ForwardGeocodeResponse{Results: []service.GeocodeResult{{FormattedAddress: "Test"}}}, nil)

	h := NewGeoHandler(mockSvc)
	body := `{"address":"Gedung Sate, Bandung"}`
	req := httptest.NewRequest("POST", "/maps/geocode", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ForwardGeocode(w, req)
	if w.Code != http.StatusOK { t.Errorf("expected 200, got %d", w.Code) }
}

func TestCalculateDistance_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockGeoServiceInterface(ctrl)
	mockSvc.EXPECT().CalculateDistance(gomock.Any(), -6.97, 107.63, -6.90, 107.60).
		Return(&service.DistanceResponse{DistanceKm: 8.1, DistanceM: 8100}, nil)

	h := NewGeoHandler(mockSvc)
	req := httptest.NewRequest("GET", "/distance?origin_lat=-6.97&origin_lng=107.63&dest_lat=-6.90&dest_lng=107.60", nil)
	w := httptest.NewRecorder()
	h.CalculateDistance(w, req)
	if w.Code != http.StatusOK { t.Errorf("expected 200, got %d", w.Code) }
}
