//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/pricing-service/internal/service"
	"github.com/zicofarry/clay-app/backend/services/pricing-service/mocks"
	"go.uber.org/mock/gomock"
)

func TestEstimateRideFare_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPricingServiceInterface(ctrl)
	mockSvc.EXPECT().EstimateRideFare(gomock.Any(), gomock.Any()).
		Return(&service.RideFareEstimate{
			BaseFare: 7000, DistanceFare: 21000, DurationFare: 5040,
			BookingFee: 2000, ServiceFee: 1652, SurgeMultiplier: 1.0,
			PromoDiscount: 0, TotalFare: 36692, DistanceKm: 8.4, EstimatedDurationMin: 16,
		}, nil)

	h := NewPricingHandler(mockSvc)
	body := `{"pickup_lat":-6.9733,"pickup_lng":107.6310,"dest_lat":-6.9000,"dest_lng":107.6000,"vehicle_type":"motorcycle"}`
	req := httptest.NewRequest("POST", "/estimate/ride", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.EstimateRideFare(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestEstimateDeliveryFare_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPricingServiceInterface(ctrl)
	mockSvc.EXPECT().EstimateDeliveryFare(gomock.Any(), gomock.Any()).
		Return(&service.DeliveryFareEstimate{
			BaseFare: 8000, DistanceFare: 25200, TotalFare: 34200, DistanceKm: 8.4,
		}, nil)

	h := NewPricingHandler(mockSvc)
	body := `{"pickup_lat":-6.97,"pickup_lng":107.63,"dest_lat":-6.90,"dest_lng":107.60,"weight_kg":3.5}`
	req := httptest.NewRequest("POST", "/estimate/delivery", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.EstimateDeliveryFare(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestEstimateFoodFare_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPricingServiceInterface(ctrl)
	mockSvc.EXPECT().EstimateFoodFare(gomock.Any(), gomock.Any()).
		Return(&service.FoodFareEstimate{
			DeliveryFee: 11800, ServiceFee: 2500, SmallOrderFee: 0,
			TotalDeliveryCharge: 14300, DistanceKm: 3.4,
		}, nil)

	h := NewPricingHandler(mockSvc)
	body := `{"restaurant_lat":-6.97,"restaurant_lng":107.63,"dest_lat":-6.95,"dest_lng":107.64,"food_subtotal":50000}`
	req := httptest.NewRequest("POST", "/estimate/food", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.EstimateFoodFare(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetSurge_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPricingServiceInterface(ctrl)
	mockSvc.EXPECT().GetSurge(gomock.Any(), -6.97, 107.63, "ride").
		Return(&service.SurgeResponse{Multiplier: 1.0, IsSurge: false}, nil)

	h := NewPricingHandler(mockSvc)
	req := httptest.NewRequest("GET", "/surge?lat=-6.97&lng=107.63&service_type=ride", nil)
	w := httptest.NewRecorder()
	h.GetSurge(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCalculateFinalFare_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPricingServiceInterface(ctrl)
	mockSvc.EXPECT().CalculateFinalFare(gomock.Any(), gomock.Any()).
		Return(&service.FinalFareResponse{
			OrderID: "order-123", TotalAmount: 35000,
			Breakdown: service.FareBreakdown{BaseFare: 7000, Total: 35000},
		}, nil)

	h := NewPricingHandler(mockSvc)
	body := `{"order_id":"order-123","service_type":"ride","actual_distance_km":8.4,"actual_duration_min":16}`
	req := httptest.NewRequest("POST", "/fare/calculate", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.CalculateFinalFare(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGetFareRules_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSvc := mocks.NewMockPricingServiceInterface(ctrl)
	mockSvc.EXPECT().GetFareRules(gomock.Any(), "ride", (*string)(nil)).
		Return(nil, service.ErrFareRuleNotFound)

	h := NewPricingHandler(mockSvc)
	req := httptest.NewRequest("GET", "/internal/fare-rules?service_type=ride", nil)
	w := httptest.NewRecorder()
	h.GetFareRules(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
