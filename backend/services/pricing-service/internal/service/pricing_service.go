// Package service implements the business logic for the Pricing Service.
package service

import (
	"context"
	"log/slog"
	"math"
	"net/http"

	"github.com/zicofarry/clay-app/backend/services/pricing-service/internal/repository"
)

// ── Service Error ────────────────────────────────────────────────────────────

type ServiceError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

func (e *ServiceError) Error() string { return e.Message }

var (
	ErrFareRuleNotFound = &ServiceError{http.StatusNotFound, "FARE_RULE_NOT_FOUND", "no fare rule found for this service type/zone"}
	ErrInvalidInput     = &ServiceError{http.StatusBadRequest, "INVALID_INPUT", "invalid request parameters"}
	ErrInvalidPromo     = &ServiceError{http.StatusUnprocessableEntity, "INVALID_PROMO", "promo code is invalid or expired"}
)

// ── Request/Response DTOs ────────────────────────────────────────────────────

// --- Estimate Requests ---

type RideEstimateRequest struct {
	PickupLat   float64 `json:"pickup_lat"`
	PickupLng   float64 `json:"pickup_lng"`
	DestLat     float64 `json:"dest_lat"`
	DestLng     float64 `json:"dest_lng"`
	VehicleType string  `json:"vehicle_type"` // motorcycle, car
	PromoCode   *string `json:"promo_code,omitempty"`
}

type DeliveryEstimateRequest struct {
	PickupLat      float64 `json:"pickup_lat"`
	PickupLng      float64 `json:"pickup_lng"`
	DestLat        float64 `json:"dest_lat"`
	DestLng        float64 `json:"dest_lng"`
	WeightKg       float64 `json:"weight_kg"`
	IsFragile      bool    `json:"is_fragile"`
	InsuranceValue int     `json:"insurance_value"`
	PromoCode      *string `json:"promo_code,omitempty"`
}

type FoodEstimateRequest struct {
	RestaurantLat float64 `json:"restaurant_lat"`
	RestaurantLng float64 `json:"restaurant_lng"`
	DestLat       float64 `json:"dest_lat"`
	DestLng       float64 `json:"dest_lng"`
	FoodSubtotal  int     `json:"food_subtotal"`
	PromoCode     *string `json:"promo_code,omitempty"`
}

// --- Estimate Responses ---

type RideFareEstimate struct {
	BaseFare            int     `json:"base_fare"`
	DistanceFare        int     `json:"distance_fare"`
	DurationFare        int     `json:"duration_fare"`
	BookingFee          int     `json:"booking_fee"`
	ServiceFee          int     `json:"service_fee"`
	SurgeMultiplier     float64 `json:"surge_multiplier"`
	SurgeAmount         int     `json:"surge_amount"`
	PromoDiscount       int     `json:"promo_discount"`
	TotalFare           int     `json:"total_fare"`
	DistanceKm          float64 `json:"distance_km"`
	EstimatedDurationMin int    `json:"estimated_duration_min"`
}

type DeliveryFareEstimate struct {
	BaseFare        int     `json:"base_fare"`
	DistanceFare    int     `json:"distance_fare"`
	WeightSurcharge int     `json:"weight_surcharge"`
	InsuranceFee    int     `json:"insurance_fee"`
	BookingFee      int     `json:"booking_fee"`
	PromoDiscount   int     `json:"promo_discount"`
	TotalFare       int     `json:"total_fare"`
	DistanceKm      float64 `json:"distance_km"`
}

type FoodFareEstimate struct {
	DeliveryFee        int     `json:"delivery_fee"`
	ServiceFee         int     `json:"service_fee"`
	SmallOrderFee      int     `json:"small_order_fee"`
	PromoDiscount      int     `json:"promo_discount"`
	PromoType          *string `json:"promo_type,omitempty"`
	TotalDeliveryCharge int    `json:"total_delivery_charge"`
	DistanceKm         float64 `json:"distance_km"`
}

// --- Surge ---

type SurgeResponse struct {
	ZoneID     *string  `json:"zone_id,omitempty"`
	ZoneName   *string  `json:"zone_name,omitempty"`
	Multiplier float64  `json:"multiplier"`
	IsSurge    bool     `json:"is_surge"`
	Reason     *string  `json:"reason,omitempty"`
	ValidUntil *string  `json:"valid_until,omitempty"`
}

// --- Final Fare ---

type FinalFareRequest struct {
	OrderID          string  `json:"order_id"`
	ServiceType      string  `json:"service_type"`
	ActualDistanceKm float64 `json:"actual_distance_km"`
	ActualDurationMin int    `json:"actual_duration_min"`
	VehicleType      *string `json:"vehicle_type,omitempty"`
	WeightKg         *float64 `json:"weight_kg,omitempty"`
	InsuranceValue   *int    `json:"insurance_value,omitempty"`
	IsFragile        *bool   `json:"is_fragile,omitempty"`
	FoodSubtotal     *int    `json:"food_subtotal,omitempty"`
	PromoCode        *string `json:"promo_code,omitempty"`
	PickupLat        *float64 `json:"pickup_lat,omitempty"`
	PickupLng        *float64 `json:"pickup_lng,omitempty"`
}

type FareBreakdown struct {
	BaseFare        int  `json:"base_fare"`
	DistanceFare    int  `json:"distance_fare"`
	DurationFare    *int `json:"duration_fare,omitempty"`
	WeightSurcharge *int `json:"weight_surcharge,omitempty"`
	InsuranceFee    *int `json:"insurance_fee,omitempty"`
	DeliveryFee     *int `json:"delivery_fee,omitempty"`
	ServiceFee      int  `json:"service_fee"`
	BookingFee      int  `json:"booking_fee"`
	SurgeAmount     *int `json:"surge_amount,omitempty"`
	SmallOrderFee   *int `json:"small_order_fee,omitempty"`
	PromoDiscount   int  `json:"promo_discount"`
	Subtotal        int  `json:"subtotal"`
	Total           int  `json:"total"`
}

type FinalFareResponse struct {
	OrderID     string        `json:"order_id"`
	Breakdown   FareBreakdown `json:"breakdown"`
	TotalAmount int           `json:"total_amount"`
}

// --- Internal ---

type FareRulesResponse struct {
	ServiceType         string   `json:"service_type"`
	ZoneID              *string  `json:"zone_id,omitempty"`
	BaseFare            int      `json:"base_fare"`
	PerKmRate           int      `json:"per_km_rate"`
	PerMinRate          *int     `json:"per_min_rate,omitempty"`
	BookingFee          int      `json:"booking_fee"`
	ServiceFeePct       float64  `json:"service_fee_pct"`
	MinFare             int      `json:"min_fare"`
	WeightRatePerKg     *int     `json:"weight_rate_per_kg,omitempty"`
	SmallOrderThreshold *int     `json:"small_order_threshold,omitempty"`
	SmallOrderFee       *int     `json:"small_order_fee,omitempty"`
}

// ── Interface ────────────────────────────────────────────────────────────────

//go:generate mockgen -source=pricing_service.go -destination=../../mocks/mock_pricing_service.go -package=mocks
type PricingServiceInterface interface {
	EstimateRideFare(ctx context.Context, req *RideEstimateRequest) (*RideFareEstimate, error)
	EstimateDeliveryFare(ctx context.Context, req *DeliveryEstimateRequest) (*DeliveryFareEstimate, error)
	EstimateFoodFare(ctx context.Context, req *FoodEstimateRequest) (*FoodFareEstimate, error)
	GetSurge(ctx context.Context, lat, lng float64, serviceType string) (*SurgeResponse, error)
	CalculateFinalFare(ctx context.Context, req *FinalFareRequest) (*FinalFareResponse, error)
	GetFareRules(ctx context.Context, serviceType string, zoneID *string) (*FareRulesResponse, error)
}

// ── Implementation ───────────────────────────────────────────────────────────

type PricingService struct {
	repo   repository.PricingRepositoryInterface
	logger *slog.Logger
}

func NewPricingService(repo repository.PricingRepositoryInterface, logger *slog.Logger) *PricingService {
	return &PricingService{repo: repo, logger: logger}
}

// ── Estimate: Ride ───────────────────────────────────────────────────────────
// Formula: base_fare + (distance_km × per_km_rate) + (duration_min × per_min_rate)
//          + booking_fee + service_fee − promo_discount
// Surge applied to subtotal when surge_multiplier > 1.0

func (s *PricingService) EstimateRideFare(ctx context.Context, req *RideEstimateRequest) (*RideFareEstimate, error) {
	distKm := haversineKm(req.PickupLat, req.PickupLng, req.DestLat, req.DestLng)
	durationMin := int(distKm / 30.0 * 60) // assume 30 km/h avg

	// Lookup fare rule
	rule, err := s.repo.GetFareRule(ctx, "ride", &req.VehicleType, nil)
	if err != nil {
		// Use defaults if no rule found
		s.logger.Warn("fare rule not found, using defaults", slog.String("service", "ride"))
		rule = defaultRideRule(req.VehicleType)
	}

	baseFare := rule.BaseFare
	distanceFare := int(distKm * float64(rule.PerKmRate))
	durationFare := 0
	if rule.PerMinRate != nil {
		durationFare = durationMin * *rule.PerMinRate
	}
	bookingFee := rule.BookingFee

	subtotal := baseFare + distanceFare + durationFare
	serviceFee := int(float64(subtotal) * rule.ServiceFeePct)

	// Surge (stub — would read from Redis pricing:surge:{zone_id})
	surgeMultiplier := 1.0
	surgeAmount := 0

	totalBeforePromo := subtotal + surgeAmount + bookingFee + serviceFee

	// Min fare check
	if totalBeforePromo < rule.MinFare {
		totalBeforePromo = rule.MinFare
	}

	// Promo discount (stub)
	promoDiscount := 0
	if req.PromoCode != nil && *req.PromoCode != "" {
		promoDiscount = int(float64(totalBeforePromo) * 0.10) // stub: 10% off
	}

	totalFare := totalBeforePromo - promoDiscount
	if totalFare < 0 {
		totalFare = 0
	}

	s.logger.Info("ride fare estimated", slog.Float64("distance_km", distKm), slog.Int("total", totalFare))

	return &RideFareEstimate{
		BaseFare: baseFare, DistanceFare: distanceFare, DurationFare: durationFare,
		BookingFee: bookingFee, ServiceFee: serviceFee,
		SurgeMultiplier: surgeMultiplier, SurgeAmount: surgeAmount,
		PromoDiscount: promoDiscount, TotalFare: totalFare,
		DistanceKm: math.Round(distKm*100) / 100, EstimatedDurationMin: durationMin,
	}, nil
}

// ── Estimate: Delivery ───────────────────────────────────────────────────────
// Formula: base_fare + (distance_km × per_km_rate) + weight_surcharge
//          + insurance_fee + booking_fee − promo_discount

func (s *PricingService) EstimateDeliveryFare(ctx context.Context, req *DeliveryEstimateRequest) (*DeliveryFareEstimate, error) {
	distKm := haversineKm(req.PickupLat, req.PickupLng, req.DestLat, req.DestLng)

	rule, err := s.repo.GetFareRule(ctx, "delivery", nil, nil)
	if err != nil {
		rule = defaultDeliveryRule()
	}

	baseFare := rule.BaseFare
	distanceFare := int(distKm * float64(rule.PerKmRate))

	weightSurcharge := 0
	if rule.WeightRatePerKg != nil && req.WeightKg > 5.0 {
		weightSurcharge = int((req.WeightKg - 5.0) * float64(*rule.WeightRatePerKg))
	}

	insuranceFee := 0
	if req.IsFragile || req.InsuranceValue > 0 {
		insuranceFee = int(float64(req.InsuranceValue) * 0.005) // 0.5% of declared value
		if insuranceFee < 2000 {
			insuranceFee = 2000 // minimum insurance fee
		}
	}

	bookingFee := rule.BookingFee

	totalBeforePromo := baseFare + distanceFare + weightSurcharge + insuranceFee + bookingFee
	if totalBeforePromo < rule.MinFare {
		totalBeforePromo = rule.MinFare
	}

	promoDiscount := 0
	if req.PromoCode != nil && *req.PromoCode != "" {
		promoDiscount = int(float64(totalBeforePromo) * 0.10)
	}

	totalFare := totalBeforePromo - promoDiscount
	if totalFare < 0 {
		totalFare = 0
	}

	return &DeliveryFareEstimate{
		BaseFare: baseFare, DistanceFare: distanceFare,
		WeightSurcharge: weightSurcharge, InsuranceFee: insuranceFee,
		BookingFee: bookingFee, PromoDiscount: promoDiscount,
		TotalFare: totalFare, DistanceKm: math.Round(distKm*100) / 100,
	}, nil
}

// ── Estimate: Food ───────────────────────────────────────────────────────────
// Formula: delivery_fee + service_fee + small_order_fee − promo_discount

func (s *PricingService) EstimateFoodFare(ctx context.Context, req *FoodEstimateRequest) (*FoodFareEstimate, error) {
	distKm := haversineKm(req.RestaurantLat, req.RestaurantLng, req.DestLat, req.DestLng)

	rule, err := s.repo.GetFareRule(ctx, "food", nil, nil)
	if err != nil {
		rule = defaultFoodRule()
	}

	deliveryFee := rule.BaseFare + int(distKm*float64(rule.PerKmRate))
	serviceFee := int(float64(req.FoodSubtotal) * rule.ServiceFeePct)

	smallOrderFee := 0
	if rule.SmallOrderThreshold != nil && rule.SmallOrderFee != nil {
		if req.FoodSubtotal < *rule.SmallOrderThreshold {
			smallOrderFee = *rule.SmallOrderFee
		}
	}

	totalBeforePromo := deliveryFee + serviceFee + smallOrderFee

	promoDiscount := 0
	var promoType *string
	if req.PromoCode != nil && *req.PromoCode != "" {
		freeDelivery := "free_delivery"
		promoType = &freeDelivery
		promoDiscount = deliveryFee // stub: free delivery promo
	}

	totalCharge := totalBeforePromo - promoDiscount
	if totalCharge < 0 {
		totalCharge = 0
	}

	return &FoodFareEstimate{
		DeliveryFee: deliveryFee, ServiceFee: serviceFee,
		SmallOrderFee: smallOrderFee, PromoDiscount: promoDiscount,
		PromoType: promoType, TotalDeliveryCharge: totalCharge,
		DistanceKm: math.Round(distKm*100) / 100,
	}, nil
}

// ── Surge ────────────────────────────────────────────────────────────────────

func (s *PricingService) GetSurge(_ context.Context, lat, lng float64, serviceType string) (*SurgeResponse, error) {
	// TODO: Read from Redis key pricing:surge:{zone_id}
	// Stub: always return no surge
	return &SurgeResponse{
		Multiplier: 1.0,
		IsSurge:    false,
	}, nil
}

// ── Final Fare ───────────────────────────────────────────────────────────────

func (s *PricingService) CalculateFinalFare(ctx context.Context, req *FinalFareRequest) (*FinalFareResponse, error) {
	rule, err := s.repo.GetFareRule(ctx, req.ServiceType, req.VehicleType, nil)
	if err != nil {
		switch req.ServiceType {
		case "ride":
			vt := "motorcycle"
			if req.VehicleType != nil {
				vt = *req.VehicleType
			}
			rule = defaultRideRule(vt)
		case "delivery":
			rule = defaultDeliveryRule()
		case "food":
			rule = defaultFoodRule()
		default:
			return nil, ErrInvalidInput
		}
	}

	baseFare := rule.BaseFare
	distanceFare := int(req.ActualDistanceKm * float64(rule.PerKmRate))

	var durationFare *int
	if rule.PerMinRate != nil {
		df := req.ActualDurationMin * *rule.PerMinRate
		durationFare = &df
	}

	bookingFee := rule.BookingFee
	subtotal := baseFare + distanceFare
	if durationFare != nil {
		subtotal += *durationFare
	}

	serviceFee := int(float64(subtotal) * rule.ServiceFeePct)

	var weightSurcharge, insuranceFee, deliveryFee, smallOrderFee, surgeAmount *int

	if req.ServiceType == "delivery" {
		ws := 0
		if rule.WeightRatePerKg != nil && req.WeightKg != nil && *req.WeightKg > 5.0 {
			ws = int((*req.WeightKg - 5.0) * float64(*rule.WeightRatePerKg))
		}
		weightSurcharge = &ws

		ins := 0
		if req.InsuranceValue != nil && *req.InsuranceValue > 0 {
			ins = int(float64(*req.InsuranceValue) * 0.005)
			if ins < 2000 {
				ins = 2000
			}
		}
		insuranceFee = &ins
		subtotal += ws + ins
	}

	if req.ServiceType == "food" {
		df := baseFare + distanceFare
		deliveryFee = &df

		sof := 0
		if rule.SmallOrderThreshold != nil && rule.SmallOrderFee != nil &&
			req.FoodSubtotal != nil && *req.FoodSubtotal < *rule.SmallOrderThreshold {
			sof = *rule.SmallOrderFee
		}
		smallOrderFee = &sof
		subtotal += sof
	}

	promoDiscount := 0
	if req.PromoCode != nil && *req.PromoCode != "" {
		promoDiscount = int(float64(subtotal) * 0.10) // stub
	}

	total := subtotal + bookingFee + serviceFee - promoDiscount
	if total < rule.MinFare {
		total = rule.MinFare
	}
	if total < 0 {
		total = 0
	}

	return &FinalFareResponse{
		OrderID: req.OrderID,
		Breakdown: FareBreakdown{
			BaseFare: baseFare, DistanceFare: distanceFare,
			DurationFare: durationFare, WeightSurcharge: weightSurcharge,
			InsuranceFee: insuranceFee, DeliveryFee: deliveryFee,
			ServiceFee: serviceFee, BookingFee: bookingFee,
			SurgeAmount: surgeAmount, SmallOrderFee: smallOrderFee,
			PromoDiscount: promoDiscount, Subtotal: subtotal, Total: total,
		},
		TotalAmount: total,
	}, nil
}

// ── Internal ─────────────────────────────────────────────────────────────────

func (s *PricingService) GetFareRules(ctx context.Context, serviceType string, zoneID *string) (*FareRulesResponse, error) {
	rule, err := s.repo.GetFareRule(ctx, serviceType, nil, zoneID)
	if err != nil {
		return nil, ErrFareRuleNotFound
	}

	return &FareRulesResponse{
		ServiceType: rule.ServiceType, ZoneID: rule.ZoneID,
		BaseFare: rule.BaseFare, PerKmRate: rule.PerKmRate, PerMinRate: rule.PerMinRate,
		BookingFee: rule.BookingFee, ServiceFeePct: rule.ServiceFeePct, MinFare: rule.MinFare,
		WeightRatePerKg: rule.WeightRatePerKg, SmallOrderThreshold: rule.SmallOrderThreshold,
		SmallOrderFee: rule.SmallOrderFee,
	}, nil
}

// ── Default Rules (fallback when DB has no matching rule) ────────────────────

func defaultRideRule(vehicleType string) *repository.FareRule {
	perMin := 300
	if vehicleType == "car" {
		perMin = 500
	}
	baseFare := 7000
	if vehicleType == "car" {
		baseFare = 12000
	}
	return &repository.FareRule{
		ServiceType: "ride", BaseFare: baseFare,
		PerKmRate: 2500, PerMinRate: &perMin,
		BookingFee: 2000, ServiceFeePct: 0.05, MinFare: 10000,
	}
}

func defaultDeliveryRule() *repository.FareRule {
	weightRate := 2000
	return &repository.FareRule{
		ServiceType: "delivery", BaseFare: 8000,
		PerKmRate: 3000, BookingFee: 1000, ServiceFeePct: 0.03,
		MinFare: 10000, WeightRatePerKg: &weightRate,
	}
}

func defaultFoodRule() *repository.FareRule {
	threshold := 20000
	smallFee := 5000
	return &repository.FareRule{
		ServiceType: "food", BaseFare: 5000,
		PerKmRate: 2000, BookingFee: 0, ServiceFeePct: 0.05,
		MinFare: 5000, SmallOrderThreshold: &threshold, SmallOrderFee: &smallFee,
	}
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371.0
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func toRad(deg float64) float64 { return deg * math.Pi / 180 }
