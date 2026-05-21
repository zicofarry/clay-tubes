//go:build unit

package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/zicofarry/clay-app/backend/services/pricing-service/internal/repository"
	"github.com/zicofarry/clay-app/backend/services/pricing-service/mocks/repomock"
	"go.uber.org/mock/gomock"
)

func newTestService(t *testing.T) (*PricingService, *repomock.MockPricingRepositoryInterface) {
	ctrl := gomock.NewController(t)
	mockRepo := repomock.NewMockPricingRepositoryInterface(ctrl)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewPricingService(mockRepo, logger)
	return svc, mockRepo
}

func TestEstimateRideFare_WithDefaults(t *testing.T) {
	svc, mockRepo := newTestService(t)
	// Return error so defaults are used
	mockRepo.EXPECT().GetFareRule(gomock.Any(), "ride", gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("not found"))

	result, err := svc.EstimateRideFare(context.Background(), &RideEstimateRequest{
		PickupLat: -6.9733, PickupLng: 107.6310,
		DestLat: -6.9000, DestLng: 107.6000,
		VehicleType: "motorcycle",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BaseFare != 7000 {
		t.Errorf("expected base_fare 7000 (motorcycle default), got %d", result.BaseFare)
	}
	if result.TotalFare <= 0 {
		t.Error("expected positive total fare")
	}
	if result.DistanceKm <= 0 {
		t.Error("expected positive distance")
	}
	t.Logf("Ride estimate: distance=%.2fkm, total=%d IDR", result.DistanceKm, result.TotalFare)
}

func TestEstimateRideFare_WithDBRule(t *testing.T) {
	svc, mockRepo := newTestService(t)
	perMin := 400
	mockRepo.EXPECT().GetFareRule(gomock.Any(), "ride", gomock.Any(), gomock.Any()).
		Return(&repository.FareRule{
			ServiceType: "ride", BaseFare: 8000, PerKmRate: 3000, PerMinRate: &perMin,
			BookingFee: 2500, ServiceFeePct: 0.05, MinFare: 12000,
		}, nil)

	result, err := svc.EstimateRideFare(context.Background(), &RideEstimateRequest{
		PickupLat: -6.9733, PickupLng: 107.6310,
		DestLat: -6.9000, DestLng: 107.6000,
		VehicleType: "car",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BaseFare != 8000 {
		t.Errorf("expected base_fare 8000 (from DB), got %d", result.BaseFare)
	}
	t.Logf("Ride (DB rule): distance=%.2fkm, total=%d IDR", result.DistanceKm, result.TotalFare)
}

func TestEstimateRideFare_WithPromo(t *testing.T) {
	svc, mockRepo := newTestService(t)
	mockRepo.EXPECT().GetFareRule(gomock.Any(), "ride", gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("not found"))

	promo := "DISKON10"
	result, err := svc.EstimateRideFare(context.Background(), &RideEstimateRequest{
		PickupLat: -6.9733, PickupLng: 107.6310,
		DestLat: -6.9000, DestLng: 107.6000,
		VehicleType: "motorcycle", PromoCode: &promo,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PromoDiscount <= 0 {
		t.Error("expected positive promo discount")
	}
	t.Logf("Ride with promo: discount=%d, total=%d IDR", result.PromoDiscount, result.TotalFare)
}

func TestEstimateDeliveryFare_WithDefaults(t *testing.T) {
	svc, mockRepo := newTestService(t)
	mockRepo.EXPECT().GetFareRule(gomock.Any(), "delivery", gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("not found"))

	result, err := svc.EstimateDeliveryFare(context.Background(), &DeliveryEstimateRequest{
		PickupLat: -6.97, PickupLng: 107.63, DestLat: -6.90, DestLng: 107.60,
		WeightKg: 8.0, IsFragile: true, InsuranceValue: 500000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WeightSurcharge <= 0 {
		t.Error("expected weight surcharge for 8kg parcel")
	}
	if result.InsuranceFee <= 0 {
		t.Error("expected insurance fee for fragile item")
	}
	t.Logf("Delivery: weight_surcharge=%d, insurance=%d, total=%d IDR",
		result.WeightSurcharge, result.InsuranceFee, result.TotalFare)
}

func TestEstimateFoodFare_SmallOrder(t *testing.T) {
	svc, mockRepo := newTestService(t)
	mockRepo.EXPECT().GetFareRule(gomock.Any(), "food", gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("not found"))

	result, err := svc.EstimateFoodFare(context.Background(), &FoodEstimateRequest{
		RestaurantLat: -6.97, RestaurantLng: 107.63, DestLat: -6.96, DestLng: 107.64,
		FoodSubtotal: 15000, // below 20000 threshold
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SmallOrderFee != 5000 {
		t.Errorf("expected small_order_fee 5000 for subtotal below threshold, got %d", result.SmallOrderFee)
	}
	t.Logf("Food (small order): small_order_fee=%d, total=%d IDR", result.SmallOrderFee, result.TotalDeliveryCharge)
}

func TestEstimateFoodFare_FreeDeliveryPromo(t *testing.T) {
	svc, mockRepo := newTestService(t)
	mockRepo.EXPECT().GetFareRule(gomock.Any(), "food", gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("not found"))

	promo := "GRATISONGKIR"
	result, err := svc.EstimateFoodFare(context.Background(), &FoodEstimateRequest{
		RestaurantLat: -6.97, RestaurantLng: 107.63, DestLat: -6.96, DestLng: 107.64,
		FoodSubtotal: 50000, PromoCode: &promo,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PromoDiscount != result.DeliveryFee {
		t.Errorf("expected free delivery promo (discount=%d should equal delivery_fee=%d)",
			result.PromoDiscount, result.DeliveryFee)
	}
	if result.PromoType == nil || *result.PromoType != "free_delivery" {
		t.Error("expected promo_type=free_delivery")
	}
	t.Logf("Food (free delivery): promo=%d, total=%d IDR", result.PromoDiscount, result.TotalDeliveryCharge)
}

func TestGetSurge_NoSurge(t *testing.T) {
	svc, _ := newTestService(t)
	result, err := svc.GetSurge(context.Background(), -6.97, 107.63, "ride")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsSurge {
		t.Error("expected no surge")
	}
	if result.Multiplier != 1.0 {
		t.Errorf("expected multiplier 1.0, got %f", result.Multiplier)
	}
}

func TestCalculateFinalFare_Ride(t *testing.T) {
	svc, mockRepo := newTestService(t)
	mockRepo.EXPECT().GetFareRule(gomock.Any(), "ride", gomock.Any(), gomock.Any()).
		Return(nil, fmt.Errorf("not found"))

	result, err := svc.CalculateFinalFare(context.Background(), &FinalFareRequest{
		OrderID: "ord-123", ServiceType: "ride",
		ActualDistanceKm: 8.4, ActualDurationMin: 16,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OrderID != "ord-123" {
		t.Errorf("expected order ID ord-123, got %s", result.OrderID)
	}
	if result.TotalAmount <= 0 {
		t.Error("expected positive total amount")
	}
	t.Logf("Final ride fare: total=%d IDR", result.TotalAmount)
}

func TestGetFareRules_Found(t *testing.T) {
	svc, mockRepo := newTestService(t)
	perMin := 300
	mockRepo.EXPECT().GetFareRule(gomock.Any(), "ride", (*string)(nil), (*string)(nil)).
		Return(&repository.FareRule{
			ServiceType: "ride", BaseFare: 7000, PerKmRate: 2500, PerMinRate: &perMin,
			BookingFee: 2000, ServiceFeePct: 0.05, MinFare: 10000,
		}, nil)

	result, err := svc.GetFareRules(context.Background(), "ride", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BaseFare != 7000 {
		t.Errorf("expected base_fare 7000, got %d", result.BaseFare)
	}
}

func TestGetFareRules_NotFound(t *testing.T) {
	svc, mockRepo := newTestService(t)
	mockRepo.EXPECT().GetFareRule(gomock.Any(), "unknown", (*string)(nil), (*string)(nil)).
		Return(nil, fmt.Errorf("not found"))

	_, err := svc.GetFareRules(context.Background(), "unknown", nil)
	if err != ErrFareRuleNotFound {
		t.Errorf("expected ErrFareRuleNotFound, got %v", err)
	}
}
